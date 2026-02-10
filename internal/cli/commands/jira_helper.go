package commands

import (
	"fmt"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/jira"
)

func loadJiraClient() (*jira.Client, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	creds, err := config.LoadCredentials()
	if err != nil {
		return nil, nil, fmt.Errorf("load credentials: %w", err)
	}

	return jira.NewClient(cfg.Jira.URL, cfg.Jira.Email, creds.JiraToken), cfg, nil
}
