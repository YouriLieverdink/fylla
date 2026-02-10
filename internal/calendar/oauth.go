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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googlecalendar "google.golang.org/api/calendar/v3"
)

// OAuthScopes are the required Google Calendar API scopes.
var OAuthScopes = []string{
	googlecalendar.CalendarEventsScope,
	googlecalendar.CalendarReadonlyScope,
}

// TokenPath returns the default path for the cached OAuth token.
func TokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "fylla", "google_token.json"), nil
}

// SaveToken writes an OAuth2 token to the given path with restricted permissions.
func SaveToken(token *oauth2.Token, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken reads an OAuth2 token from the given path.
func LoadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	return &token, nil
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

// CachedToken returns a cached OAuth token from disk, or runs the
// authentication flow if no valid cached token exists. The token is
// saved to disk after a successful authentication.
func CachedToken(ctx context.Context, cfg *oauth2.Config, tokenPath string) (*oauth2.Token, error) {
	token, err := LoadToken(tokenPath)
	if err == nil && token.Valid() {
		return token, nil
	}

	// If we have a refresh token but the access token expired,
	// let the oauth2 library handle refresh via TokenSource.
	if err == nil && token.RefreshToken != "" {
		src := cfg.TokenSource(ctx, token)
		refreshed, err := src.Token()
		if err == nil {
			if saveErr := SaveToken(refreshed, tokenPath); saveErr != nil {
				return nil, fmt.Errorf("save refreshed token: %w", saveErr)
			}
			return refreshed, nil
		}
	}

	token, err = Authenticate(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	if err := SaveToken(token, tokenPath); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}
	return token, nil
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
