package commands

import (
	"fmt"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/jira"
	"github.com/iruoy/fylla/internal/todoist"
)

// loadTaskSource returns the appropriate task source client based on config.
// The returned interface implements TaskFetcher, TaskCreator, WorklogPoster,
// EstimateGetter, and EstimateUpdater.
func loadTaskSource() (interface{}, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	creds, err := config.LoadCredentials()
	if err != nil {
		return nil, nil, fmt.Errorf("load credentials: %w", err)
	}

	switch cfg.Source {
	case "todoist":
		if creds.TodoistToken == "" {
			return nil, nil, fmt.Errorf("todoist token not set: run 'fylla auth todoist --token TOKEN'")
		}
		return todoist.NewClient(creds.TodoistToken), cfg, nil
	default:
		return jira.NewClient(cfg.Jira.URL, cfg.Jira.Email, creds.JiraToken), cfg, nil
	}
}
