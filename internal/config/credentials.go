package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProviderCredentials holds a provider's authentication credentials.
type ProviderCredentials struct {
	Token string `json:"token"`
}

// DefaultProviderCredentialsPath returns the active profile's credentials
// file path for the given provider.
func DefaultProviderCredentialsPath(provider string) (string, error) {
	dir, err := ProfileDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, provider+"_credentials.json"), nil
}

// LoadProviderCredentials reads credentials from the given path.
// Returns an empty ProviderCredentials struct if the file does not exist.
func LoadProviderCredentials(path string) (*ProviderCredentials, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ProviderCredentials{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var creds ProviderCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return &creds, nil
}

// SaveProviderCredentials writes credentials to the given path with restricted permissions.
func SaveProviderCredentials(creds *ProviderCredentials, path string) error {
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
