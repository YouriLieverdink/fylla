package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/iruoy/fylla/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googlecalendar "google.golang.org/api/calendar/v3"
)

// OAuthScopes are the required Google Calendar API scopes.
var OAuthScopes = []string{
	googlecalendar.CalendarEventsScope,
	googlecalendar.CalendarReadonlyScope,
}

// GoogleCredentials stores OAuth client config and token in a single file.
type GoogleCredentials struct {
	ClientID     string        `json:"clientId"`
	ClientSecret string        `json:"clientSecret"`
	AuthURI      string        `json:"authUri"`
	TokenURI     string        `json:"tokenUri"`
	Token        *oauth2.Token `json:"token"`
}

// NewGoogleCredentials creates GoogleCredentials from an OAuth config and token.
func NewGoogleCredentials(cfg *oauth2.Config, token *oauth2.Token) *GoogleCredentials {
	return &GoogleCredentials{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		AuthURI:      cfg.Endpoint.AuthURL,
		TokenURI:     cfg.Endpoint.TokenURL,
		Token:        token,
	}
}

// OAuthConfig reconstructs an *oauth2.Config from stored fields.
func (c *GoogleCredentials) OAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.AuthURI,
			TokenURL: c.TokenURI,
		},
		Scopes: OAuthScopes,
	}
}

// SaveGoogleCredentials writes GoogleCredentials to the given path with restricted permissions.
func SaveGoogleCredentials(creds *GoogleCredentials, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadGoogleCredentials reads GoogleCredentials from the given path.
func LoadGoogleCredentials(path string) (*GoogleCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds GoogleCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &creds, nil
}

// EnsureValidToken checks whether the stored token is valid and refreshes it
// if expired. The creds.Token field is mutated in place on refresh.
func EnsureValidToken(ctx context.Context, creds *GoogleCredentials) error {
	if creds.Token == nil {
		return fmt.Errorf("no token in credentials")
	}
	if creds.Token.Valid() {
		return nil
	}
	if creds.Token.RefreshToken == "" {
		return fmt.Errorf("token expired and no refresh token available")
	}
	cfg := creds.OAuthConfig()
	src := cfg.TokenSource(ctx, creds.Token)
	refreshed, err := src.Token()
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	creds.Token = refreshed
	return nil
}

// TokenPath returns the default path for the Google credentials file.
func TokenPath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "google_credentials.json"), nil
}

// OAuthConfigFromFile parses a Google OAuth client credentials JSON file
// and returns an oauth2.Config for the installed application flow.
func OAuthConfigFromFile(path string) (*oauth2.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read client credentials: %w", err)
	}
	cfg, err := google.ConfigFromJSON(data, OAuthScopes...)
	if err != nil {
		return nil, fmt.Errorf("parse client credentials: %w", err)
	}
	return cfg, nil
}

// browserOpener is the function used to open URLs in the browser.
// Replaced in tests.
var browserOpener = openBrowser

// Authenticate runs the OAuth2 authorization code flow:
// starts a local callback server, opens the browser for consent,
// exchanges the code for a token, and returns it.
func Authenticate(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start callback server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("oauth error: %s", errMsg)
			fmt.Fprintf(w, "Authentication failed: %s", errMsg)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code in callback")
			fmt.Fprint(w, "Error: no authorization code received.")
			return
		}
		codeCh <- code
		fmt.Fprint(w, "Authentication successful! You can close this window.")
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Close()

	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	if err := browserOpener(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	select {
	case code := <-codeCh:
		token, err := cfg.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("exchange code: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
