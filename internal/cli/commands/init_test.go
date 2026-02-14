package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iruoy/fylla/internal/config"
	"golang.org/x/oauth2"
)

// mockSurveyor provides canned responses for testing init flow.
type mockSurveyor struct {
	selectAnswers  []string
	inputAnswers   []string
	passwordAnswer []string
	selectIdx      int
	inputIdx       int
	passwordIdx    int
}

func (m *mockSurveyor) Select(message string, options []string) (string, error) {
	if m.selectIdx >= len(m.selectAnswers) {
		return "", fmt.Errorf("unexpected Select call: %s", message)
	}
	answer := m.selectAnswers[m.selectIdx]
	m.selectIdx++
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
	credPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "google_token.json")

	writeDefaultConfig(t, cfgPath)

	// Write fake Google client credentials
	clientCredsPath := filepath.Join(dir, "client.json")
	clientCreds := `{"installed":{"client_id":"test-id","client_secret":"test-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(clientCredsPath, []byte(clientCreds), 0600); err != nil {
		t.Fatalf("write client creds: %v", err)
	}

	mock := &mockSurveyor{
		selectAnswers:  []string{"jira"},
		inputAnswers:   []string{"https://company.atlassian.net", "user@example.com", clientCredsPath},
		passwordAnswer: []string{"jira-token-123"},
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
		TokenPath:       tokenPath,
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	// Verify config
	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Source != "jira" {
		t.Errorf("source = %q, want jira", cfg.Source)
	}
	if cfg.Jira.URL != "https://company.atlassian.net" {
		t.Errorf("jira.url = %q", cfg.Jira.URL)
	}
	if cfg.Jira.Email != "user@example.com" {
		t.Errorf("jira.email = %q", cfg.Jira.Email)
	}
	if cfg.Calendar.ClientCredentials != clientCredsPath {
		t.Errorf("calendar.clientCredentials = %q", cfg.Calendar.ClientCredentials)
	}

	// Verify credentials
	creds, err := config.LoadCredentialsFrom(credPath)
	if err != nil {
		t.Fatalf("LoadCredentialsFrom: %v", err)
	}
	if creds.JiraToken != "jira-token-123" {
		t.Errorf("jiraToken = %q", creds.JiraToken)
	}

	// Verify OAuth was called
	if !oauthMock.called {
		t.Error("expected OAuth flow to be called")
	}

	// Verify output
	out := buf.String()
	if !strings.Contains(out, "Source set to jira") {
		t.Errorf("output missing source confirmation, got:\n%s", out)
	}
	if !strings.Contains(out, "Setup complete") {
		t.Errorf("output missing completion message, got:\n%s", out)
	}
}

func TestRunInit_Todoist(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	credPath := filepath.Join(dir, "credentials.json")
	tokenPath := filepath.Join(dir, "google_token.json")

	writeDefaultConfig(t, cfgPath)

	clientCredsPath := filepath.Join(dir, "client.json")
	clientCreds := `{"installed":{"client_id":"test-id","client_secret":"test-secret","auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(clientCredsPath, []byte(clientCreds), 0600); err != nil {
		t.Fatalf("write client creds: %v", err)
	}

	mock := &mockSurveyor{
		selectAnswers:  []string{"todoist"},
		inputAnswers:   []string{clientCredsPath},
		passwordAnswer: []string{"todoist-token-456"},
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
		TokenPath:       tokenPath,
	})
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	cfg, err := config.LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Source != "todoist" {
		t.Errorf("source = %q, want todoist", cfg.Source)
	}

	creds, err := config.LoadCredentialsFrom(credPath)
	if err != nil {
		t.Fatalf("LoadCredentialsFrom: %v", err)
	}
	if creds.TodoistToken != "todoist-token-456" {
		t.Errorf("todoistToken = %q", creds.TodoistToken)
	}

	out := buf.String()
	if !strings.Contains(out, "Source set to todoist") {
		t.Errorf("output missing source confirmation, got:\n%s", out)
	}
	if !strings.Contains(out, "Todoist credentials stored") {
		t.Errorf("output missing todoist confirmation, got:\n%s", out)
	}
}
