package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/github"
	"github.com/iruoy/fylla/internal/jira"
	"github.com/iruoy/fylla/internal/local"
	"github.com/iruoy/fylla/internal/task"
	"github.com/iruoy/fylla/internal/todoist"
)

// DueDateRemover abstracts clearing the due date from a task.
type DueDateRemover interface {
	RemoveDueDate(ctx context.Context, issueKey string) error
}

// SummaryGetter abstracts fetching the raw summary/title of a task.
type SummaryGetter interface {
	GetSummary(ctx context.Context, issueKey string) (string, error)
}

// SummaryUpdater abstracts updating the summary/title of a task.
type SummaryUpdater interface {
	UpdateSummary(ctx context.Context, issueKey string, summary string) error
}

// TaskSource combines all task-related interfaces that every source must implement.
type TaskSource interface {
	TaskFetcher
	TaskCreator
	TaskCompleter
	TaskDeleter
	WorklogPoster
	EstimateGetter
	EstimateUpdater
	DueDateGetter
	DueDateUpdater
	DueDateRemover
	PriorityGetter
	PriorityUpdater
	SummaryGetter
	SummaryUpdater
}

// Compile-time checks that all clients satisfy TaskSource.
var (
	_ TaskSource = (*jira.Client)(nil)
	_ TaskSource = (*todoist.Client)(nil)
	_ TaskSource = (*github.Client)(nil)
	_ TaskSource = (*local.Client)(nil)
)

// EpicLister lists open epics from a provider, optionally scoped to a project.
type EpicLister interface {
	ListEpics(ctx context.Context, project string) ([]jira.Epic, error)
}

// ParentUpdater updates the parent of a task.
type ParentUpdater interface {
	UpdateParent(ctx context.Context, issueKey, parentKey string) error
}

// ParentGetter fetches the parent key of a task.
type ParentGetter interface {
	GetParent(ctx context.Context, issueKey string) (string, error)
}

// JiraKeyResolver resolves a non-Jira task key (e.g. GitHub PR) to a Jira issue key.
type JiraKeyResolver interface {
	ResolveJiraKey(ctx context.Context, taskKey string) (string, error)
}

var (
	jiraKeyRe  = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	localKeyRe = regexp.MustCompile(`^L-\d+$`)
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

// MultiTaskSource wraps multiple named TaskSource instances and routes
// operations to the correct provider based on the task key.
type MultiTaskSource struct {
	sources  map[string]TaskSource
	order    []string // provider names in config order
}

// Compile-time check that MultiTaskSource satisfies TaskSource.
var _ TaskSource = (*MultiTaskSource)(nil)

// NewMultiTaskSource creates a MultiTaskSource from named providers.
func NewMultiTaskSource(sources map[string]TaskSource, order []string) *MultiTaskSource {
	return &MultiTaskSource{sources: sources, order: order}
}

func (m *MultiTaskSource) routeTo(taskKey string) TaskSource {
	name := providerForKey(taskKey)
	if src, ok := m.sources[name]; ok {
		return src
	}
	// Fall back to first configured provider
	return m.sources[m.order[0]]
}

func (m *MultiTaskSource) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	// MultiTaskSource uses multiFetcher for fetching, not this method directly.
	// But to satisfy the interface, delegate to the first provider.
	return m.sources[m.order[0]].FetchTasks(ctx, query)
}

func (m *MultiTaskSource) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	return m.sources[m.order[0]].CreateTask(ctx, input)
}

// CreateTaskOn creates a task using a specific named provider.
func (m *MultiTaskSource) CreateTaskOn(ctx context.Context, provider string, input task.CreateInput) (string, error) {
	if src, ok := m.sources[provider]; ok {
		return src.CreateTask(ctx, input)
	}
	return "", fmt.Errorf("unknown provider %q", provider)
}

func (m *MultiTaskSource) CompleteTask(ctx context.Context, taskKey string) error {
	return m.routeTo(taskKey).CompleteTask(ctx, taskKey)
}

func (m *MultiTaskSource) DeleteTask(ctx context.Context, taskKey string) error {
	return m.routeTo(taskKey).DeleteTask(ctx, taskKey)
}

func (m *MultiTaskSource) PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error {
	return m.routeTo(issueKey).PostWorklog(ctx, issueKey, timeSpent, description, started)
}

func (m *MultiTaskSource) GetEstimate(ctx context.Context, issueKey string) (time.Duration, error) {
	return m.routeTo(issueKey).GetEstimate(ctx, issueKey)
}

func (m *MultiTaskSource) UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error {
	return m.routeTo(issueKey).UpdateEstimate(ctx, issueKey, remaining)
}

func (m *MultiTaskSource) GetDueDate(ctx context.Context, issueKey string) (*time.Time, error) {
	return m.routeTo(issueKey).GetDueDate(ctx, issueKey)
}

func (m *MultiTaskSource) UpdateDueDate(ctx context.Context, issueKey string, dueDate time.Time) error {
	return m.routeTo(issueKey).UpdateDueDate(ctx, issueKey, dueDate)
}

func (m *MultiTaskSource) GetPriority(ctx context.Context, issueKey string) (int, error) {
	return m.routeTo(issueKey).GetPriority(ctx, issueKey)
}

func (m *MultiTaskSource) UpdatePriority(ctx context.Context, issueKey string, priority int) error {
	return m.routeTo(issueKey).UpdatePriority(ctx, issueKey, priority)
}

func (m *MultiTaskSource) RemoveDueDate(ctx context.Context, issueKey string) error {
	return m.routeTo(issueKey).RemoveDueDate(ctx, issueKey)
}

func (m *MultiTaskSource) GetSummary(ctx context.Context, issueKey string) (string, error) {
	return m.routeTo(issueKey).GetSummary(ctx, issueKey)
}

func (m *MultiTaskSource) UpdateSummary(ctx context.Context, issueKey string, summary string) error {
	return m.routeTo(issueKey).UpdateSummary(ctx, issueKey, summary)
}

func (m *MultiTaskSource) ResolveJiraKey(ctx context.Context, taskKey string) (string, error) {
	src := m.routeTo(taskKey)
	if resolver, ok := src.(JiraKeyResolver); ok {
		return resolver.ResolveJiraKey(ctx, taskKey)
	}
	return "", fmt.Errorf("provider for %q does not support Jira key resolution", taskKey)
}

// multiFetcher implements TaskFetcher by concurrently querying multiple providers
// and merging results.
type multiFetcher struct {
	queries map[string]string     // provider name -> query
	sources map[string]TaskSource // provider name -> client
}

func (mf *multiFetcher) FetchTasks(ctx context.Context, _ string) ([]task.Task, error) {
	type result struct {
		tasks []task.Task
		err   error
		name  string
	}

	var wg sync.WaitGroup
	ch := make(chan result, len(mf.queries))
	for name, query := range mf.queries {
		wg.Add(1)
		go func(n, q string) {
			defer wg.Done()
			tasks, err := mf.sources[n].FetchTasks(ctx, q)
			ch <- result{tasks: tasks, err: err, name: n}
		}(name, query)
	}
	wg.Wait()
	close(ch)

	var allTasks []task.Task
	var errs []string
	succeeded := 0
	for r := range ch {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.name, r.err))
			continue
		}
		succeeded++
		allTasks = append(allTasks, r.tasks...)
	}

	// Partial success: log warnings but continue if at least one provider succeeded
	if succeeded == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all providers failed: %s", strings.Join(errs, "; "))
	}

	return allTasks, nil
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
		}
	}
	return queries
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
		}
	}

	// Single provider: return directly (no wrapping)
	if len(providers) == 1 {
		return sources[providers[0]], cfg, nil
	}

	return NewMultiTaskSource(sources, providers), cfg, nil
}
