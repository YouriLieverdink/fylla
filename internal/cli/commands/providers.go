package commands

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/github"
	"github.com/iruoy/fylla/internal/kendo"
	"github.com/iruoy/fylla/internal/local"
	"github.com/iruoy/fylla/internal/todoist"
)

var (
	kendoKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	localKeyRe = regexp.MustCompile(`^L-\d+$`)
)

// isKendoKey returns true if key matches the Kendo issue key pattern (e.g. PROJ-123).
func isKendoKey(key string) bool {
	return kendoKeyRe.MatchString(key)
}

// isGitHubKey returns true if key matches the GitHub PR key format (e.g. repo#123).
func isGitHubKey(key string) bool {
	return strings.Contains(key, "#")
}

// isLocalKey returns true if key matches the local task key format (e.g. L-1).
func isLocalKey(key string) bool {
	return localKeyRe.MatchString(key)
}

// providerForKey infers the provider name from a task key.
func providerForKey(key string) string {
	if isGitHubKey(key) {
		return "github"
	}
	if isLocalKey(key) {
		return "local"
	}
	if isKendoKey(key) {
		return "kendo"
	}
	return "todoist"
}

// buildProviderQueries builds per-provider query strings from CLI flags and config defaults.
func buildProviderQueries(cfg *config.Config, filterFlag string) map[string]string {
	queries := make(map[string]string)
	for _, p := range cfg.ActiveProviders() {
		switch p {
		case "todoist":
			q := filterFlag
			if q == "" {
				q = cfg.Todoist.DefaultFilter
			}
			queries["todoist"] = q
		case "github":
			queries["github"] = cfg.GitHub.DefaultQuery
		case "local":
			queries["local"] = cfg.Local.DefaultFilter
		case "kendo":
			q := filterFlag
			if q == "" {
				q = cfg.Kendo.DefaultFilter
			}
			queries["kendo"] = q
		}
	}
	return queries
}

// buildSearchAllQuery returns a broad query for single-provider mode.
func buildSearchAllQuery(cfg *config.Config, search string) string {
	return searchQueryForProvider(cfg.ActiveProviders()[0], search)
}

// searchQueryForProvider returns a search query for a specific provider.
func searchQueryForProvider(provider, search string) string {
	switch provider {
	case "kendo":
		if search != "" {
			return search
		}
		return "*"
	case "github":
		if search != "" {
			return fmt.Sprintf("is:pr state:open %s", search)
		}
		return "is:pr state:open"
	default:
		return search
	}
}

// loadTaskSource returns the appropriate task source client(s) based on config.
func loadTaskSource() (TaskSource, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	providers := cfg.ActiveProviders()

	sources := make(map[string]TaskSource)
	for _, p := range providers {
		switch p {
		case "todoist":
			if cfg.Todoist.Credentials == "" {
				return nil, nil, fmt.Errorf("todoist not configured: run 'fylla auth todoist'")
			}
			creds, err := config.LoadProviderCredentials(cfg.Todoist.Credentials)
			if err != nil {
				return nil, nil, fmt.Errorf("load todoist credentials: %w", err)
			}
			if creds.Token == "" {
				return nil, nil, fmt.Errorf("todoist token not set: run 'fylla auth todoist --token TOKEN'")
			}
			sources["todoist"] = todoist.NewClient(creds.Token)
		case "github":
			if cfg.GitHub.Credentials == "" {
				return nil, nil, fmt.Errorf("github not configured: run 'fylla auth github'")
			}
			creds, err := config.LoadProviderCredentials(cfg.GitHub.Credentials)
			if err != nil {
				return nil, nil, fmt.Errorf("load github credentials: %w", err)
			}
			if creds.Token == "" {
				return nil, nil, fmt.Errorf("github token not set: run 'fylla auth github --token TOKEN'")
			}
			client := github.NewClient(creds.Token)
			client.Repos = cfg.GitHub.Repos
			sources["github"] = client
		case "local":
			storePath := cfg.Local.StorePath
			client := local.NewClient(storePath)
			client.DefaultProject = cfg.Local.DefaultProject
			sources["local"] = client
		case "kendo":
			if cfg.Kendo.Credentials == "" {
				return nil, nil, fmt.Errorf("kendo not configured: run 'fylla auth kendo'")
			}
			creds, err := config.LoadProviderCredentials(cfg.Kendo.Credentials)
			if err != nil {
				return nil, nil, fmt.Errorf("load kendo credentials: %w", err)
			}
			if creds.Token == "" {
				return nil, nil, fmt.Errorf("kendo token not set: run 'fylla auth kendo --url URL --token TOKEN'")
			}
			client := kendo.NewClient(cfg.Kendo.URL, creds.Token)
			client.DoneLane = cfg.Kendo.DoneLane
			sources["kendo"] = client
		}
	}

	if len(providers) == 1 {
		return sources[providers[0]], cfg, nil
	}

	return NewMultiTaskSource(sources, providers), cfg, nil
}
