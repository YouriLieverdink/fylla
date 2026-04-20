package calendar

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func testOAuthConfig(tokenURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: tokenURL,
		},
		Scopes: OAuthScopes,
	}
}

func TestGCAL001_OAuthFlow(t *testing.T) {
	t.Run("Authenticate opens browser with auth URL", func(t *testing.T) {
		// Mock token exchange server
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"test-access-token","token_type":"Bearer","refresh_token":"test-refresh","expires_in":3600}`)
		}))
		defer tokenServer.Close()

		cfg := testOAuthConfig(tokenServer.URL)

		var capturedURL string
		original := browserOpener
		browserOpener = func(url string) error {
			capturedURL = url
			// Simulate browser: hit the callback URL
			// Extract the redirect_uri from the auth URL to find the callback port
			// The auth URL contains redirect_uri parameter
			parts := strings.Split(url, "redirect_uri=")
			if len(parts) < 2 {
				return fmt.Errorf("no redirect_uri in auth URL")
			}
			redirectURI := strings.Split(parts[1], "&")[0]
			// URL-decode (simple: replace %3A with : and %2F with /)
			redirectURI = strings.ReplaceAll(redirectURI, "%3A", ":")
			redirectURI = strings.ReplaceAll(redirectURI, "%2F", "/")

			go func() {
				time.Sleep(50 * time.Millisecond)
				http.Get(redirectURI + "?code=test-auth-code&state=state-token")
			}()
			return nil
		}
		defer func() { browserOpener = original }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		token, err := Authenticate(ctx, cfg)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}

		if token.AccessToken != "test-access-token" {
			t.Errorf("access token = %q, want %q", token.AccessToken, "test-access-token")
		}
		if token.RefreshToken != "test-refresh" {
			t.Errorf("refresh token = %q, want %q", token.RefreshToken, "test-refresh")
		}

		// Verify browser was opened with Google OAuth URL
		if !strings.Contains(capturedURL, "accounts.google.com") {
			t.Errorf("auth URL = %q, want to contain accounts.google.com", capturedURL)
		}
		if !strings.Contains(capturedURL, "test-client-id") {
			t.Errorf("auth URL = %q, want to contain client_id", capturedURL)
		}
	})

	t.Run("Authenticate shows success message on callback", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"tok","token_type":"Bearer","expires_in":3600}`)
		}))
		defer tokenServer.Close()

		cfg := testOAuthConfig(tokenServer.URL)

		var callbackResponse string
		original := browserOpener
		browserOpener = func(url string) error {
			parts := strings.Split(url, "redirect_uri=")
			redirectURI := strings.Split(parts[1], "&")[0]
			redirectURI = strings.ReplaceAll(redirectURI, "%3A", ":")
			redirectURI = strings.ReplaceAll(redirectURI, "%2F", "/")

			go func() {
				time.Sleep(50 * time.Millisecond)
				resp, err := http.Get(redirectURI + "?code=test-code&state=state-token")
				if err == nil {
					defer resp.Body.Close()
					buf := make([]byte, 1024)
					n, _ := resp.Body.Read(buf)
					callbackResponse = string(buf[:n])
				}
			}()
			return nil
		}
		defer func() { browserOpener = original }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := Authenticate(ctx, cfg)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}

		// Give time for the response to be captured
		time.Sleep(200 * time.Millisecond)
		if !strings.Contains(callbackResponse, "successful") {
			t.Errorf("callback response = %q, want to contain 'successful'", callbackResponse)
		}
	})

	t.Run("Authenticate handles error in callback", func(t *testing.T) {
		cfg := testOAuthConfig("http://localhost:0/token")

		original := browserOpener
		browserOpener = func(url string) error {
			parts := strings.Split(url, "redirect_uri=")
			redirectURI := strings.Split(parts[1], "&")[0]
			redirectURI = strings.ReplaceAll(redirectURI, "%3A", ":")
			redirectURI = strings.ReplaceAll(redirectURI, "%2F", "/")

			go func() {
				time.Sleep(50 * time.Millisecond)
				http.Get(redirectURI + "?error=access_denied")
			}()
			return nil
		}
		defer func() { browserOpener = original }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := Authenticate(ctx, cfg)
		if err == nil {
			t.Fatal("expected error for access_denied callback")
		}
		if !strings.Contains(err.Error(), "access_denied") {
			t.Errorf("error = %q, want to contain 'access_denied'", err.Error())
		}
	})

	t.Run("Authenticate respects context cancellation", func(t *testing.T) {
		cfg := testOAuthConfig("http://localhost:0/token")

		original := browserOpener
		browserOpener = func(url string) error {
			return nil // Don't call back, let context expire
		}
		defer func() { browserOpener = original }()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := Authenticate(ctx, cfg)
		if err == nil {
			t.Fatal("expected error for context cancellation")
		}
	})
}

func TestGCAL002_GoogleCredentials(t *testing.T) {
	t.Run("SaveGoogleCredentials and LoadGoogleCredentials round-trip", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "creds.json")

		original := &GoogleCredentials{
			ClientID:     "client-id-123",
			ClientSecret: "client-secret-456",
			AuthURI:      "https://accounts.google.com/o/oauth2/auth",
			TokenURI:     "https://oauth2.googleapis.com/token",
			Token: &oauth2.Token{
				AccessToken:  "access-123",
				RefreshToken: "refresh-456",
				TokenType:    "Bearer",
				Expiry:       time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}

		if err := SaveGoogleCredentials(original, path); err != nil {
			t.Fatalf("SaveGoogleCredentials: %v", err)
		}

		loaded, err := LoadGoogleCredentials(path)
		if err != nil {
			t.Fatalf("LoadGoogleCredentials: %v", err)
		}

		if loaded.ClientID != original.ClientID {
			t.Errorf("clientId = %q, want %q", loaded.ClientID, original.ClientID)
		}
		if loaded.ClientSecret != original.ClientSecret {
			t.Errorf("clientSecret = %q, want %q", loaded.ClientSecret, original.ClientSecret)
		}
		if loaded.AuthURI != original.AuthURI {
			t.Errorf("authUri = %q, want %q", loaded.AuthURI, original.AuthURI)
		}
		if loaded.TokenURI != original.TokenURI {
			t.Errorf("tokenUri = %q, want %q", loaded.TokenURI, original.TokenURI)
		}
		if loaded.Token.AccessToken != original.Token.AccessToken {
			t.Errorf("access token = %q, want %q", loaded.Token.AccessToken, original.Token.AccessToken)
		}
		if loaded.Token.RefreshToken != original.Token.RefreshToken {
			t.Errorf("refresh token = %q, want %q", loaded.Token.RefreshToken, original.Token.RefreshToken)
		}
		if !loaded.Token.Expiry.Equal(original.Token.Expiry) {
			t.Errorf("expiry = %v, want %v", loaded.Token.Expiry, original.Token.Expiry)
		}
	})

	t.Run("SaveGoogleCredentials uses restricted permissions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "creds.json")

		creds := &GoogleCredentials{
			ClientID: "x",
			Token:    &oauth2.Token{AccessToken: "x"},
		}
		if err := SaveGoogleCredentials(creds, path); err != nil {
			t.Fatalf("SaveGoogleCredentials: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("permissions = %o, want 0600", perm)
		}
	})

	t.Run("LoadGoogleCredentials returns error for missing file", func(t *testing.T) {
		_, err := LoadGoogleCredentials(filepath.Join(t.TempDir(), "nonexistent.json"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("OAuthConfig reconstructs oauth2.Config from stored fields", func(t *testing.T) {
		creds := &GoogleCredentials{
			ClientID:     "my-client-id",
			ClientSecret: "my-client-secret",
			AuthURI:      "https://auth.example.com",
			TokenURI:     "https://token.example.com",
			Token:        &oauth2.Token{AccessToken: "tok"},
		}

		cfg := creds.OAuthConfig()

		if cfg.ClientID != "my-client-id" {
			t.Errorf("ClientID = %q, want %q", cfg.ClientID, "my-client-id")
		}
		if cfg.ClientSecret != "my-client-secret" {
			t.Errorf("ClientSecret = %q, want %q", cfg.ClientSecret, "my-client-secret")
		}
		if cfg.Endpoint.AuthURL != "https://auth.example.com" {
			t.Errorf("AuthURL = %q, want %q", cfg.Endpoint.AuthURL, "https://auth.example.com")
		}
		if cfg.Endpoint.TokenURL != "https://token.example.com" {
			t.Errorf("TokenURL = %q, want %q", cfg.Endpoint.TokenURL, "https://token.example.com")
		}
		if len(cfg.Scopes) != len(OAuthScopes) {
			t.Errorf("Scopes = %v, want %v", cfg.Scopes, OAuthScopes)
		}
	})

	t.Run("NewGoogleCredentials extracts config fields", func(t *testing.T) {
		cfg := &oauth2.Config{
			ClientID:     "id-from-config",
			ClientSecret: "secret-from-config",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://auth.url",
				TokenURL: "https://token.url",
			},
		}
		token := &oauth2.Token{
			AccessToken:  "access",
			RefreshToken: "refresh",
		}

		creds := NewGoogleCredentials(cfg, token)

		if creds.ClientID != "id-from-config" {
			t.Errorf("ClientID = %q, want %q", creds.ClientID, "id-from-config")
		}
		if creds.ClientSecret != "secret-from-config" {
			t.Errorf("ClientSecret = %q, want %q", creds.ClientSecret, "secret-from-config")
		}
		if creds.AuthURI != "https://auth.url" {
			t.Errorf("AuthURI = %q, want %q", creds.AuthURI, "https://auth.url")
		}
		if creds.TokenURI != "https://token.url" {
			t.Errorf("TokenURI = %q, want %q", creds.TokenURI, "https://token.url")
		}
		if creds.Token.AccessToken != "access" {
			t.Errorf("AccessToken = %q, want %q", creds.Token.AccessToken, "access")
		}
	})

	t.Run("EnsureValidToken returns nil for valid token", func(t *testing.T) {
		creds := &GoogleCredentials{
			ClientID:     "id",
			ClientSecret: "secret",
			AuthURI:      "https://auth.example.com",
			TokenURI:     "https://token.example.com",
			Token: &oauth2.Token{
				AccessToken: "valid-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
		}

		err := EnsureValidToken(context.Background(), creds)
		if err != nil {
			t.Fatalf("EnsureValidToken: %v", err)
		}
		if creds.Token.AccessToken != "valid-token" {
			t.Errorf("token should not have changed, got %q", creds.Token.AccessToken)
		}
	})

	t.Run("EnsureValidToken refreshes expired token", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"refreshed-token","token_type":"Bearer","refresh_token":"new-refresh","expires_in":3600}`)
		}))
		defer tokenServer.Close()

		creds := &GoogleCredentials{
			ClientID:     "id",
			ClientSecret: "secret",
			AuthURI:      "https://auth.example.com",
			TokenURI:     tokenServer.URL,
			Token: &oauth2.Token{
				AccessToken:  "expired-token",
				RefreshToken: "old-refresh",
				TokenType:    "Bearer",
				Expiry:       time.Now().Add(-1 * time.Hour),
			},
		}

		err := EnsureValidToken(context.Background(), creds)
		if err != nil {
			t.Fatalf("EnsureValidToken: %v", err)
		}
		if creds.Token.AccessToken != "refreshed-token" {
			t.Errorf("access token = %q, want %q", creds.Token.AccessToken, "refreshed-token")
		}
	})

	t.Run("EnsureValidToken errors on expired token without refresh", func(t *testing.T) {
		creds := &GoogleCredentials{
			ClientID:     "id",
			ClientSecret: "secret",
			Token: &oauth2.Token{
				AccessToken: "expired",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(-1 * time.Hour),
			},
		}

		err := EnsureValidToken(context.Background(), creds)
		if err == nil {
			t.Fatal("expected error for expired token without refresh")
		}
		if !strings.Contains(err.Error(), "no refresh token") {
			t.Errorf("error = %q, want to contain 'no refresh token'", err.Error())
		}
	})

	t.Run("EnsureValidToken errors on nil token", func(t *testing.T) {
		creds := &GoogleCredentials{
			ClientID:     "id",
			ClientSecret: "secret",
		}

		err := EnsureValidToken(context.Background(), creds)
		if err == nil {
			t.Fatal("expected error for nil token")
		}
		if !strings.Contains(err.Error(), "no token") {
			t.Errorf("error = %q, want to contain 'no token'", err.Error())
		}
	})

	t.Run("TokenPath returns path under active profile dir", func(t *testing.T) {
		path, err := TokenPath()
		if err != nil {
			t.Fatalf("TokenPath: %v", err)
		}
		if filepath.Base(path) != "google_credentials.json" {
			t.Errorf("path = %q, want basename google_credentials.json", path)
		}
		if filepath.Base(filepath.Dir(filepath.Dir(path))) != "profiles" {
			t.Errorf("path = %q, want grandparent profiles", path)
		}
	})
}
