package commands

import (
	"fmt"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/jira"
	"github.com/iruoy/fylla/internal/todoist"
)

// TaskSource combines all task-related interfaces that every source must implement.
type TaskSource interface {
	TaskFetcher
	TaskCreator
	TaskCompleter
	WorklogPoster
	EstimateGetter
	EstimateUpdater
	DueDateGetter
	DueDateUpdater
}

// Compile-time checks that both clients satisfy TaskSource.
var (
	_ TaskSource = (*jira.Client)(nil)
	_ TaskSource = (*todoist.Client)(nil)
)

// loadTaskSource returns the appropriate task source client based on config.
func loadTaskSource() (TaskSource, *config.Config, error) {
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
