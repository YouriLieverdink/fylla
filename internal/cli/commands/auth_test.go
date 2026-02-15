package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iruoy/fylla/internal/calendar"
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
  credentials: ""
  url: ""
  email: ""
  defaultJql: "assignee = currentUser()"
todoist:
  credentials: ""
calendar:
  credentials: ""
  sourceCalendars: [primary]
  fyllaCalendar: fylla
scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  bufferMinutes: 15
businessHours:
  - start: "09:00"
    end: "17:00"
    workDays: [1, 2, 3, 4, 5]
weights:
  priority: 0.45
  dueDate: 0.30
  estimate: 0.15
  age: 0.10
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
	called bool
	token  *oauth2.Token
	err    error
}

func (m *mockOAuthAuthenticator) Authenticate(_ context.Context, _ *oauth2.Config) (*oauth2.Token, error) {
	m.called = true
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
		if !strings.Contains(out, "task") {
			t.Errorf("help output missing 'task' command, got:\n%s", out)
		}
		if !strings.Contains(out, "schedule") {
			t.Errorf("help output missing 'schedule' command, got:\n%s", out)
		}
	})

	t.Run("help lists available commands", func(t *testing.T) {
		root := newTestRootCmd()
		out, err := executeCommand(root, "--help")
		if err != nil {
			t.Fatalf("help error: %v", err)
		}
		expectedCmds := []string{"auth", "task", "schedule", "timer", "config"}
		for _, cmd := range expectedCmds {
			if !strings.Contains(out, cmd) {
				t.Errorf("help output missing %q command", cmd)
			}
		}
	})
}

func TestCLI002_auth_jira_command(t *testing.T) {
	t.Run("stores credentials in per-provider file and saves path to config", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yaml")
		writeDefaultConfig(t, cfgPath)

		err := RunAuthJira(AuthJiraParams{
			URL:        "https://company.atlassian.net",
			Email:      "you@example.com",
			Token:      "secret-token-123",
			ConfigPath: cfgPath,
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
		if cfg.Jira.Credentials == "" {
			t.Fatal("jira.credentials should be set")
		}

		// Verify per-provider credentials were stored
		creds, err := config.LoadProviderCredentials(cfg.Jira.Credentials)
		if err != nil {
			t.Fatalf("LoadProviderCredentials: %v", err)
		}
		if creds.Token != "secret-token-123" {
			t.Errorf("token = %q, want %q", creds.Token, "secret-token-123")
		}
	})

	t.Run("subsequent Jira commands can use stored credentials", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yaml")
		writeDefaultConfig(t, cfgPath)

		err := RunAuthJira(AuthJiraParams{
			URL:        "https://test.atlassian.net",
			Email:      "test@example.com",
			Token:      "api-token-456",
			ConfigPath: cfgPath,
		})
		if err != nil {
			t.Fatalf("RunAuthJira: %v", err)
		}

		// Load back and verify roundtrip
		cfg, err := config.LoadFrom(cfgPath)
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}
		creds, err := config.LoadProviderCredentials(cfg.Jira.Credentials)
		if err != nil {
			t.Fatalf("LoadProviderCredentials: %v", err)
		}

		if cfg.Jira.URL != "https://test.atlassian.net" {
			t.Errorf("url = %q", cfg.Jira.URL)
		}
		if cfg.Jira.Email != "test@example.com" {
			t.Errorf("email = %q", cfg.Jira.Email)
		}
		if creds.Token != "api-token-456" {
			t.Errorf("token = %q", creds.Token)
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
	t.Run("initiates OAuth flow and saves client config with token", func(t *testing.T) {
		dir := t.TempDir()
		credPath := filepath.Join(dir, "google_credentials.json")
		cfgPath := filepath.Join(dir, "config.yaml")
		writeDefaultConfig(t, cfgPath)

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
			ClientFile:      clientCredsPath,
			CredentialsPath: credPath,
			ConfigPath:      cfgPath,
			Auth:            mock,
		})
		if err != nil {
			t.Fatalf("RunAuthGoogle: %v", err)
		}

		if !mock.called {
			t.Error("expected OAuth flow to be called")
		}

		// Verify config was updated with credentials path
		cfg, err := config.LoadFrom(cfgPath)
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}
		if cfg.Calendar.Credentials != credPath {
			t.Errorf("calendar.credentials = %q, want %q", cfg.Calendar.Credentials, credPath)
		}

		// Verify google_credentials.json contains client config + token
		creds, err := calendar.LoadGoogleCredentials(credPath)
		if err != nil {
			t.Fatalf("LoadGoogleCredentials: %v", err)
		}
		if creds.ClientID != "test-id" {
			t.Errorf("clientId = %q, want %q", creds.ClientID, "test-id")
		}
		if creds.ClientSecret != "test-secret" {
			t.Errorf("clientSecret = %q, want %q", creds.ClientSecret, "test-secret")
		}
		if creds.Token.AccessToken != "test-access-token" {
			t.Errorf("access token = %q, want %q", creds.Token.AccessToken, "test-access-token")
		}
		if creds.Token.RefreshToken != "test-refresh-token" {
			t.Errorf("refresh token = %q, want %q", creds.Token.RefreshToken, "test-refresh-token")
		}
	})

	t.Run("credentials are cached for reuse", func(t *testing.T) {
		dir := t.TempDir()
		credPath := filepath.Join(dir, "google_credentials.json")

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

		// First call: authenticates (no ConfigPath — skip config save)
		err := RunAuthGoogle(context.Background(), AuthGoogleParams{
			ClientFile:      clientCredsPath,
			CredentialsPath: credPath,
			Auth:            mock,
		})
		if err != nil {
			t.Fatalf("first RunAuthGoogle: %v", err)
		}
		if !mock.called {
			t.Error("expected OAuth to be called on first run")
		}

		// Verify credentials were saved
		creds, err := calendar.LoadGoogleCredentials(credPath)
		if err != nil {
			t.Fatalf("LoadGoogleCredentials: %v", err)
		}
		if creds.Token.AccessToken != "cached-token" {
			t.Errorf("access token = %q, want %q", creds.Token.AccessToken, "cached-token")
		}
		if creds.ClientID != "test-id" {
			t.Errorf("clientId = %q, want %q", creds.ClientID, "test-id")
		}
	})

	t.Run("error when no client file and no existing credentials", func(t *testing.T) {
		dir := t.TempDir()
		err := RunAuthGoogle(context.Background(), AuthGoogleParams{
			CredentialsPath: filepath.Join(dir, "google_credentials.json"),
			Auth:            &mockOAuthAuthenticator{},
		})
		if err == nil {
			t.Fatal("expected error when no client file and no existing credentials")
		}
		if !strings.Contains(err.Error(), "no existing credentials") {
			t.Errorf("error = %q, want to contain 'no existing credentials'", err.Error())
		}
	})

	t.Run("error on missing client credentials file", func(t *testing.T) {
		dir := t.TempDir()
		err := RunAuthGoogle(context.Background(), AuthGoogleParams{
			ClientFile:      filepath.Join(dir, "nonexistent.json"),
			CredentialsPath: filepath.Join(dir, "google_credentials.json"),
			Auth:            &mockOAuthAuthenticator{},
		})
		if err == nil {
			t.Fatal("expected error for missing client credentials")
		}
		if !strings.Contains(err.Error(), "client credentials") {
			t.Errorf("error = %q, want to contain 'client credentials'", err.Error())
		}
	})
}

func TestCLI004_auth_todoist_command(t *testing.T) {
	t.Run("stores credentials in per-provider file and saves path to config", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yaml")
		writeDefaultConfig(t, cfgPath)

		err := RunAuthTodoist(AuthTodoistParams{
			Token:      "todoist-secret-789",
			ConfigPath: cfgPath,
		})
		if err != nil {
			t.Fatalf("RunAuthTodoist: %v", err)
		}

		// Verify config was updated
		cfg, err := config.LoadFrom(cfgPath)
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}
		if cfg.Todoist.Credentials == "" {
			t.Fatal("todoist.credentials should be set")
		}

		// Verify per-provider credentials were stored
		creds, err := config.LoadProviderCredentials(cfg.Todoist.Credentials)
		if err != nil {
			t.Fatalf("LoadProviderCredentials: %v", err)
		}
		if creds.Token != "todoist-secret-789" {
			t.Errorf("token = %q, want %q", creds.Token, "todoist-secret-789")
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
