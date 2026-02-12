package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Credentials holds sensitive authentication tokens stored separately from config.
type Credentials struct {
	JiraToken        string `json:"jiraToken"`
	TodoistToken     string `json:"todoistToken"`
	GoogleOAuthToken string `json:"googleOAuthToken"`
}

// CredentialsPath returns the default credentials file path (~/.config/fylla/credentials.json).
func CredentialsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "fylla", "credentials.json"), nil
}

// LoadCredentialsFrom reads credentials from the given path.
// Returns an empty Credentials struct if the file does not exist.
func LoadCredentialsFrom(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Credentials{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &creds, nil
}

// LoadCredentials reads credentials from the default path.
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}
	return LoadCredentialsFrom(path)
}

// SaveCredentialsTo writes credentials to the given path with restricted permissions.
func SaveCredentialsTo(creds *Credentials, path string) error {
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

// SaveCredentials writes credentials to the default path.
func SaveCredentials(creds *Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	return SaveCredentialsTo(creds, path)
}
