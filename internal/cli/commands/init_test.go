package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"golang.org/x/oauth2"
)

// mockSurveyor provides canned responses for testing init flow.
type mockSurveyor struct {
	selectAnswers             []string
	multiSelectAnswers        [][]string
	inputAnswers              []string
	inputWithDefaultAnswers   []string
	passwordAnswer            []string
	selectIdx                 int
	multiSelectIdx            int
	inputIdx                  int
	inputWithDefaultIdx       int
	passwordIdx               int
}

func (m *mockSurveyor) Select(message string, options []string) (string, error) {
	if m.selectIdx >= len(m.selectAnswers) {
		return "", fmt.Errorf("unexpected Select call: %s", message)
	}
	answer := m.selectAnswers[m.selectIdx]
	m.selectIdx++
	return answer, nil
}

func (m *mockSurveyor) MultiSelect(message string, options []string) ([]string, error) {
	if m.multiSelectIdx >= len(m.multiSelectAnswers) {
		return nil, fmt.Errorf("unexpected MultiSelect call: %s", message)
	}
	answer := m.multiSelectAnswers[m.multiSelectIdx]
	m.multiSelectIdx++
	return answer, nil
}

func (m *mockSurveyor) Input(message string) (string, error) {
	if m.inputIdx >= len(m.inputAnswers) {
		return "", fmt.Errorf("unexpected Input call: %s", message)
	}
	answer := m.inputAnswers[m.inputIdx]
	m.inputIdx++
	return answer, nil
}

func (m *mockSurveyor) InputWithDefault(message, defaultVal string) (string, error) {
	if m.inputWithDefaultIdx >= len(m.inputWithDefaultAnswers) {
		// Fall back to default value when no answers are queued
		return defaultVal, nil
	}
	answer := m.inputWithDefaultAnswers[m.inputWithDefaultIdx]
	m.inputWithDefaultIdx++
	return answer, nil
}

func (m *mockSurveyor) Password(message string) (string, error) {
	if m.passwordIdx >= len(m.passwordAnswer) {
		return "", fmt.Errorf("unexpected Password call: %s", message)
	}
	answer := m.passwordAnswer[m.passwordIdx]
	m.passwordIdx++
	return answer, nil
}

func TestRunInit_Jira(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	credPath := filepath.Join(dir, "google_credentials.json")

	writeDefaultConfig(t, cfgPath)

	// Write fake Google client credentials
	clientCredsPath := filepath.Join(dir, "client.json")
	clientCreds := `{"installed":{"client_id":"test-id","client_secret":"test-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(clientCredsPath, []byte(clientCreds), 0600); err != nil {
		t.Fatalf("write client creds: %v", err)
	}

	mock := &mockSurveyor{
		multiSelectAnswers: [][]string{{"jira"}},
		inputAnswers:       []string{"https://company.atlassian.net", "user@example.com", clientCredsPath},
		passwordAnswer:     []string{"jira-token-123"},
	}

	oauthMock := &mockOAuthAuthenticator{
		token: &oauth2.Token{
			AccessToken:  "test-access",
			RefreshToken: "test-refresh",
			TokenType:    "Bearer",
		},
	}

	var buf bytes.Buffer
	err := RunInit(context.Background(), &buf, InitParams{
		Survey:          mock,
		Auth:            oauthMock,
		ConfigPath:      cfgPath,
		CredentialsPath: credPath,
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	// Verify config
	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0] != "jira" {
		t.Errorf("providers = %v, want [jira]", cfg.Providers)
	}
	if cfg.Jira.URL != "https://company.atlassian.net" {
		t.Errorf("jira.url = %q", cfg.Jira.URL)
	}
	if cfg.Jira.Email != "user@example.com" {
		t.Errorf("jira.email = %q", cfg.Jira.Email)
	}
	if cfg.Calendar.Credentials != credPath {
		t.Errorf("calendar.credentials = %q, want %q", cfg.Calendar.Credentials, credPath)
	}

	// Verify per-provider jira credentials
	if cfg.Jira.Credentials == "" {
		t.Fatal("jira.credentials should be set")
	}
	creds, err := config.LoadProviderCredentials(cfg.Jira.Credentials)
	if err != nil {
		t.Fatalf("LoadProviderCredentials: %v", err)
	}
	if creds.Token != "jira-token-123" {
		t.Errorf("jira token = %q", creds.Token)
	}

	// Verify OAuth was called
	if !oauthMock.called {
		t.Error("expected OAuth flow to be called")
	}

	// Verify google_credentials.json contains client config + token
	googleCreds, err := calendar.LoadGoogleCredentials(credPath)
	if err != nil {
		t.Fatalf("LoadGoogleCredentials: %v", err)
	}
	if googleCreds.ClientID != "test-id" {
		t.Errorf("clientId = %q, want %q", googleCreds.ClientID, "test-id")
	}
	if googleCreds.Token.AccessToken != "test-access" {
		t.Errorf("access token = %q, want %q", googleCreds.Token.AccessToken, "test-access")
	}

	// Verify output
	out := buf.String()
	if !strings.Contains(out, "Providers set to [jira]") {
		t.Errorf("output missing providers confirmation, got:\n%s", out)
	}
	if !strings.Contains(out, "Setup complete") {
		t.Errorf("output missing completion message, got:\n%s", out)
	}
}

func TestRunInit_Todoist(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	credPath := filepath.Join(dir, "google_credentials.json")

	writeDefaultConfig(t, cfgPath)

	clientCredsPath := filepath.Join(dir, "client.json")
	clientCreds := `{"installed":{"client_id":"test-id","client_secret":"test-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(clientCredsPath, []byte(clientCreds), 0600); err != nil {
		t.Fatalf("write client creds: %v", err)
	}

	mock := &mockSurveyor{
		multiSelectAnswers: [][]string{{"todoist"}},
		inputAnswers:       []string{clientCredsPath},
		passwordAnswer:     []string{"todoist-token-456"},
	}

	oauthMock := &mockOAuthAuthenticator{
		token: &oauth2.Token{
			AccessToken:  "test-access",
			RefreshToken: "test-refresh",
			TokenType:    "Bearer",
		},
	}

	var buf bytes.Buffer
	err := RunInit(context.Background(), &buf, InitParams{
		Survey:          mock,
		Auth:            oauthMock,
		ConfigPath:      cfgPath,
		CredentialsPath: credPath,
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0] != "todoist" {
		t.Errorf("providers = %v, want [todoist]", cfg.Providers)
	}

	// Verify per-provider todoist credentials
	if cfg.Todoist.Credentials == "" {
		t.Fatal("todoist.credentials should be set")
	}
	creds, err := config.LoadProviderCredentials(cfg.Todoist.Credentials)
	if err != nil {
		t.Fatalf("LoadProviderCredentials: %v", err)
	}
	if creds.Token != "todoist-token-456" {
		t.Errorf("todoist token = %q", creds.Token)
	}

	out := buf.String()
	if !strings.Contains(out, "Providers set to [todoist]") {
		t.Errorf("output missing providers confirmation, got:\n%s", out)
	}
	if !strings.Contains(out, "Todoist credentials stored") {
		t.Errorf("output missing todoist confirmation, got:\n%s", out)
	}
}
