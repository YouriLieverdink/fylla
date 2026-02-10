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

func TestGCAL002_TokenCaching(t *testing.T) {
	t.Run("SaveToken and LoadToken round-trip", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")

		original := &oauth2.Token{
			AccessToken:  "access-123",
			RefreshToken: "refresh-456",
			TokenType:    "Bearer",
			Expiry:       time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		if err := SaveToken(original, path); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}

		loaded, err := LoadToken(path)
		if err != nil {
			t.Fatalf("LoadToken: %v", err)
		}

		if loaded.AccessToken != original.AccessToken {
			t.Errorf("access token = %q, want %q", loaded.AccessToken, original.AccessToken)
		}
		if loaded.RefreshToken != original.RefreshToken {
			t.Errorf("refresh token = %q, want %q", loaded.RefreshToken, original.RefreshToken)
		}
		if loaded.TokenType != original.TokenType {
			t.Errorf("token type = %q, want %q", loaded.TokenType, original.TokenType)
		}
		if !loaded.Expiry.Equal(original.Expiry) {
			t.Errorf("expiry = %v, want %v", loaded.Expiry, original.Expiry)
		}
	})

	t.Run("SaveToken uses restricted permissions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")

		if err := SaveToken(&oauth2.Token{AccessToken: "x"}, path); err != nil {
			t.Fatalf("SaveToken: %v", err)
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

	t.Run("LoadToken returns error for missing file", func(t *testing.T) {
		_, err := LoadToken(filepath.Join(t.TempDir(), "nonexistent.json"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("CachedToken returns valid cached token without auth", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")

		cached := &oauth2.Token{
			AccessToken:  "cached-token",
			RefreshToken: "cached-refresh",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(1 * time.Hour),
		}
		if err := SaveToken(cached, path); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}

		cfg := testOAuthConfig("http://localhost:0/token")

		// Browser should never be called for cached valid token
		original := browserOpener
		browserOpener = func(url string) error {
			t.Fatal("browser should not be opened for cached token")
			return nil
		}
		defer func() { browserOpener = original }()

		token, err := CachedToken(context.Background(), cfg, path)
		if err != nil {
			t.Fatalf("CachedToken: %v", err)
		}
		if token.AccessToken != "cached-token" {
			t.Errorf("access token = %q, want %q", token.AccessToken, "cached-token")
		}
	})

	t.Run("CachedToken triggers auth when no cached token", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"new-token","token_type":"Bearer","refresh_token":"new-refresh","expires_in":3600}`)
		}))
		defer tokenServer.Close()

		cfg := testOAuthConfig(tokenServer.URL)

		authCalled := false
		original := browserOpener
		browserOpener = func(url string) error {
			authCalled = true
			parts := strings.Split(url, "redirect_uri=")
			redirectURI := strings.Split(parts[1], "&")[0]
			redirectURI = strings.ReplaceAll(redirectURI, "%3A", ":")
			redirectURI = strings.ReplaceAll(redirectURI, "%2F", "/")

			go func() {
				time.Sleep(50 * time.Millisecond)
				http.Get(redirectURI + "?code=new-code&state=state-token")
			}()
			return nil
		}
		defer func() { browserOpener = original }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		token, err := CachedToken(ctx, cfg, path)
		if err != nil {
			t.Fatalf("CachedToken: %v", err)
		}

		if !authCalled {
			t.Error("expected auth flow to be triggered")
		}
		if token.AccessToken != "new-token" {
			t.Errorf("access token = %q, want %q", token.AccessToken, "new-token")
		}

		// Verify token was saved to disk
		saved, err := LoadToken(path)
		if err != nil {
			t.Fatalf("LoadToken after CachedToken: %v", err)
		}
		if saved.AccessToken != "new-token" {
			t.Errorf("saved access token = %q, want %q", saved.AccessToken, "new-token")
		}
	})

	t.Run("CachedToken triggers auth for expired token without refresh", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")

		expired := &oauth2.Token{
			AccessToken: "expired-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(-1 * time.Hour),
		}
		if err := SaveToken(expired, path); err != nil {
			t.Fatalf("SaveToken: %v", err)
		}

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"fresh-token","token_type":"Bearer","expires_in":3600}`)
		}))
		defer tokenServer.Close()

		cfg := testOAuthConfig(tokenServer.URL)

		original := browserOpener
		browserOpener = func(url string) error {
			parts := strings.Split(url, "redirect_uri=")
			redirectURI := strings.Split(parts[1], "&")[0]
			redirectURI = strings.ReplaceAll(redirectURI, "%3A", ":")
			redirectURI = strings.ReplaceAll(redirectURI, "%2F", "/")

			go func() {
				time.Sleep(50 * time.Millisecond)
				http.Get(redirectURI + "?code=refresh-code&state=state-token")
			}()
			return nil
		}
		defer func() { browserOpener = original }()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		token, err := CachedToken(ctx, cfg, path)
		if err != nil {
			t.Fatalf("CachedToken: %v", err)
		}
		if token.AccessToken != "fresh-token" {
			t.Errorf("access token = %q, want %q", token.AccessToken, "fresh-token")
		}
	})

	t.Run("TokenPath returns path in config directory", func(t *testing.T) {
		path, err := TokenPath()
		if err != nil {
			t.Fatalf("TokenPath: %v", err)
		}
		if !strings.HasSuffix(path, filepath.Join("fylla", "google_token.json")) {
			t.Errorf("path = %q, want to end with fylla/google_token.json", path)
		}
	})
}
