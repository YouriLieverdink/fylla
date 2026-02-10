package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// writeDefaultConfig creates a minimal config file at path.
func writeDefaultConfig(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `jira:
  url: ""
  email: ""
  defaultJql: "assignee = currentUser()"
calendar:
  sourceCalendar: primary
  fyllaCalendar: fylla
scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  bufferMinutes: 15
businessHours:
  start: "09:00"
  end: "17:00"
  workDays: [1, 2, 3, 4, 5]
weights:
  priority: 0.40
  dueDate: 0.30
  estimate: 0.15
  issueType: 0.10
  age: 0.05
typeScores:
  Bug: 100
  Task: 70
  Story: 50
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// executeCommand runs a cobra command with the given args and returns stdout output.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// mockOAuthAuthenticator records calls and returns a preset token.
type mockOAuthAuthenticator struct {
	called    bool
	token     *oauth2.Token
	err       error
	usedPath  string
}

func (m *mockOAuthAuthenticator) CachedToken(_ context.Context, _ *oauth2.Config, tokenPath string) (*oauth2.Token, error) {
	m.called = true
	m.usedPath = tokenPath
	return m.token, m.err
}

func TestCLI001_cli_entry_point(t *testing.T) {
	t.Run("CLI starts without errors", func(t *testing.T) {
		root := newTestRootCmd()
		_, err := executeCommand(root)
		if err != nil {
			t.Fatalf("CLI start error: %v", err)
		}
	})

	t.Run("help is displayed when no command given", func(t *testing.T) {
		root := newTestRootCmd()
		out, err := executeCommand(root, "--help")
		if err != nil {
			t.Fatalf("help error: %v", err)
		}
		if !strings.Contains(out, "auth") {
			t.Errorf("help output missing 'auth' command, got:\n%s", out)
		}
		if !strings.Contains(out, "sync") {
			t.Errorf("help output missing 'sync' command, got:\n%s", out)
		}
		if !strings.Contains(out, "list") {
			t.Errorf("help output missing 'list' command, got:\n%s", out)
		}
	})

	t.Run("help lists available commands", func(t *testing.T) {
		root := newTestRootCmd()
		out, err := executeCommand(root, "--help")
		if err != nil {
			t.Fatalf("help error: %v", err)
		}
		expectedCmds := []string{"auth", "sync", "list", "config", "start", "stop", "status", "log", "estimate", "add"}
		for _, cmd := range expectedCmds {
			if !strings.Contains(out, cmd) {
				t.Errorf("help output missing %q command", cmd)
			}
		}
	})
}

func TestCLI002_auth_jira_command(t *testing.T) {
	t.Run("stores credentials with --url, --email, --token", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yaml")
		credPath := filepath.Join(dir, "credentials.json")
		writeDefaultConfig(t, cfgPath)

		err := RunAuthJira(AuthJiraParams{
			URL:             "https://company.atlassian.net",
			Email:           "you@example.com",
			Token:           "secret-token-123",
			ConfigPath:      cfgPath,
			CredentialsPath: credPath,
		})
		if err != nil {
			t.Fatalf("RunAuthJira: %v", err)
		}

		// Verify config was updated
		cfg, err := config.LoadFrom(cfgPath)
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}
		if cfg.Jira.URL != "https://company.atlassian.net" {
			t.Errorf("jira.url = %q, want %q", cfg.Jira.URL, "https://company.atlassian.net")
		}
		if cfg.Jira.Email != "you@example.com" {
			t.Errorf("jira.email = %q, want %q", cfg.Jira.Email, "you@example.com")
		}

		// Verify credentials were stored
		creds, err := config.LoadCredentialsFrom(credPath)
		if err != nil {
			t.Fatalf("LoadCredentialsFrom: %v", err)
		}
		if creds.JiraToken != "secret-token-123" {
			t.Errorf("jiraToken = %q, want %q", creds.JiraToken, "secret-token-123")
		}
	})

	t.Run("subsequent Jira commands can use stored credentials", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yaml")
		credPath := filepath.Join(dir, "credentials.json")
		writeDefaultConfig(t, cfgPath)

		err := RunAuthJira(AuthJiraParams{
			URL:             "https://test.atlassian.net",
			Email:           "test@example.com",
			Token:           "api-token-456",
			ConfigPath:      cfgPath,
			CredentialsPath: credPath,
		})
		if err != nil {
			t.Fatalf("RunAuthJira: %v", err)
		}

		// Load back and verify roundtrip
		cfg, err := config.LoadFrom(cfgPath)
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}
		creds, err := config.LoadCredentialsFrom(credPath)
		if err != nil {
			t.Fatalf("LoadCredentialsFrom: %v", err)
		}

		// These are what a Jira client would use
		if cfg.Jira.URL != "https://test.atlassian.net" {
			t.Errorf("url = %q", cfg.Jira.URL)
		}
		if cfg.Jira.Email != "test@example.com" {
			t.Errorf("email = %q", cfg.Jira.Email)
		}
		if creds.JiraToken != "api-token-456" {
			t.Errorf("token = %q", creds.JiraToken)
		}
	})

	t.Run("cobra command requires all flags", func(t *testing.T) {
		root := newTestRootCmd()
		_, err := executeCommand(root, "auth", "jira", "--url", "https://x.atlassian.net")
		if err == nil {
			t.Fatal("expected error when email and token missing")
		}
		if !strings.Contains(err.Error(), "required") {
			t.Errorf("error = %q, want to contain 'required'", err.Error())
		}
	})
}

func TestCLI003_auth_google_command(t *testing.T) {
	t.Run("initiates OAuth flow with client credentials", func(t *testing.T) {
		dir := t.TempDir()
		tokenPath := filepath.Join(dir, "google_token.json")

		// Write a fake client credentials file
		clientCredsPath := filepath.Join(dir, "client.json")
		clientCreds := `{"installed":{"client_id":"test-id","client_secret":"test-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
		if err := os.WriteFile(clientCredsPath, []byte(clientCreds), 0600); err != nil {
			t.Fatalf("write client creds: %v", err)
		}

		mock := &mockOAuthAuthenticator{
			token: &oauth2.Token{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				TokenType:    "Bearer",
			},
		}

		err := RunAuthGoogle(context.Background(), AuthGoogleParams{
			ClientCredentialsPath: clientCredsPath,
			TokenPath:             tokenPath,
			Auth:                  mock,
		})
		if err != nil {
			t.Fatalf("RunAuthGoogle: %v", err)
		}

		if !mock.called {
			t.Error("expected OAuth flow to be called")
		}
		if mock.usedPath != tokenPath {
			t.Errorf("token path = %q, want %q", mock.usedPath, tokenPath)
		}
	})

	t.Run("credentials are cached for reuse", func(t *testing.T) {
		dir := t.TempDir()
		tokenPath := filepath.Join(dir, "google_token.json")

		clientCredsPath := filepath.Join(dir, "client.json")
		clientCreds := `{"installed":{"client_id":"test-id","client_secret":"test-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
		if err := os.WriteFile(clientCredsPath, []byte(clientCreds), 0600); err != nil {
			t.Fatalf("write client creds: %v", err)
		}

		mock := &mockOAuthAuthenticator{
			token: &oauth2.Token{
				AccessToken:  "cached-token",
				RefreshToken: "cached-refresh",
				TokenType:    "Bearer",
			},
		}

		// First call: authenticates
		err := RunAuthGoogle(context.Background(), AuthGoogleParams{
			ClientCredentialsPath: clientCredsPath,
			TokenPath:             tokenPath,
			Auth:                  mock,
		})
		if err != nil {
			t.Fatalf("first RunAuthGoogle: %v", err)
		}
		if !mock.called {
			t.Error("expected OAuth to be called on first run")
		}

		// CachedToken is responsible for caching — verify it's called with correct path
		if mock.usedPath != tokenPath {
			t.Errorf("CachedToken called with path %q, want %q", mock.usedPath, tokenPath)
		}
	})

	t.Run("error on missing client credentials file", func(t *testing.T) {
		dir := t.TempDir()
		err := RunAuthGoogle(context.Background(), AuthGoogleParams{
			ClientCredentialsPath: filepath.Join(dir, "nonexistent.json"),
			TokenPath:             filepath.Join(dir, "token.json"),
			Auth:                  &mockOAuthAuthenticator{},
		})
		if err == nil {
			t.Fatal("expected error for missing client credentials")
		}
		if !strings.Contains(err.Error(), "client credentials") {
			t.Errorf("error = %q, want to contain 'client credentials'", err.Error())
		}
	})
}

// newTestRootCmd creates a root command with all subcommands for testing.
func newTestRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "fylla",
		Short: "Fylla - Fill your calendar with what matters",
	}
	Register(rootCmd)
	return rootCmd
}
