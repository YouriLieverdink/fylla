package commands

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/github"
	"github.com/iruoy/fylla/internal/jira"
	"github.com/iruoy/fylla/internal/kendo"
	"github.com/iruoy/fylla/internal/local"
	"github.com/iruoy/fylla/internal/todoist"
)

var (
	jiraKeyRe          = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	localKeyRe         = regexp.MustCompile(`^L-\d+$`)
	jiraProjectPrefixRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d*$`)
)

// isJiraKey returns true if key matches the Jira issue key pattern (e.g. PROJ-123).
func isJiraKey(key string) bool {
	return jiraKeyRe.MatchString(key)
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
	if isJiraKey(key) {
		return "jira"
	}
	return "todoist"
}

// buildProviderQueries builds per-provider query strings from CLI flags and config defaults.
func buildProviderQueries(cfg *config.Config, jqlFlag, filterFlag string) map[string]string {
	queries := make(map[string]string)
	for _, p := range cfg.ActiveProviders() {
		switch p {
		case "jira":
			q := jqlFlag
			if q == "" {
				q = cfg.Jira.DefaultJQL
			}
			queries["jira"] = q
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

func jiraSearchJQL(search string) string {
	if search == "" {
		return "status != Done ORDER BY updated DESC"
	}
	if isJiraKey(search) {
		return fmt.Sprintf("key = %q", search)
	}
	upper := strings.ToUpper(search)
	if jiraProjectPrefixRe.MatchString(upper) {
		return fmt.Sprintf("key >= %q AND key <= %q AND status != Done ORDER BY key ASC", upper, upper+"\uffff")
	}
	return fmt.Sprintf("status != Done AND text ~ %q ORDER BY updated DESC", search)
}

// buildSearchAllQueries builds per-provider query strings for searching all tasks
// (not just assigned to the current user). When search is non-empty, it filters
// by text match.
func buildSearchAllQueries(cfg *config.Config, search string) map[string]string {
	queries := make(map[string]string)
	for _, p := range cfg.ActiveProviders() {
		switch p {
		case "jira":
			queries["jira"] = jiraSearchJQL(search)
		case "todoist":
			queries["todoist"] = search
		case "github":
			if search != "" {
				queries["github"] = fmt.Sprintf("is:pr state:open %s", search)
			} else {
				queries["github"] = "is:pr state:open"
			}
		case "local":
			queries["local"] = search
		case "kendo":
			if search != "" {
				queries["kendo"] = search
			} else {
				queries["kendo"] = "*"
			}
		}
	}
	return queries
}

// buildSearchAllQuery returns a broad query for single-provider mode.
func buildSearchAllQuery(cfg *config.Config, search string) string {
	switch cfg.ActiveProviders()[0] {
	case "jira":
		return jiraSearchJQL(search)
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
		case "jira":
			if cfg.Jira.Credentials == "" {
				return nil, nil, fmt.Errorf("jira not configured: run 'fylla auth jira'")
			}
			creds, err := config.LoadProviderCredentials(cfg.Jira.Credentials)
			if err != nil {
				return nil, nil, fmt.Errorf("load jira credentials: %w", err)
			}
			client := jira.NewClient(cfg.Jira.URL, cfg.Jira.Email, creds.Token)
			client.DoneTransitions = cfg.Jira.DoneTransitions
			sources["jira"] = client
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
