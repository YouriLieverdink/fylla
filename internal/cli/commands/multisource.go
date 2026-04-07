package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// MultiTaskSource wraps multiple named TaskSource instances and routes
// operations to the correct provider based on the task key.
type MultiTaskSource struct {
	sources map[string]TaskSource
	order   []string // provider names in config order
}

// Compile-time check that MultiTaskSource satisfies TaskSource.
var _ TaskSource = (*MultiTaskSource)(nil)

// NewMultiTaskSource creates a MultiTaskSource from named providers.
func NewMultiTaskSource(sources map[string]TaskSource, order []string) *MultiTaskSource {
	return &MultiTaskSource{sources: sources, order: order}
}

// RouteToProvider routes to a specific named provider.
func (m *MultiTaskSource) RouteToProvider(provider string) (TaskSource, bool) {
	src, ok := m.sources[provider]
	return src, ok
}

func (m *MultiTaskSource) routeTo(taskKey string) TaskSource {
	name := providerForKey(taskKey)
	if src, ok := m.sources[name]; ok {
		return src
	}
	return m.sources[m.order[0]]
}

func (m *MultiTaskSource) routeToWithProvider(taskKey, provider string) TaskSource {
	if provider != "" {
		if src, ok := m.sources[provider]; ok {
			return src
		}
	}
	return m.routeTo(taskKey)
}

// routedSource resolves a source to the correct provider-specific source.
// If provider is non-empty and source is a MultiTaskSource, it routes to that provider.
// Otherwise it returns source unchanged. Accepts any interface type so it can be
// used with narrow interfaces (TaskCompleter, WorklogPoster, etc.).
func routedSource[T any](source T, provider string) T {
	if provider == "" {
		return source
	}
	if ms, ok := any(source).(*MultiTaskSource); ok {
		if src, ok := ms.RouteToProvider(provider); ok {
			if typed, ok := any(src).(T); ok {
				return typed
			}
		}
	}
	return source
}

// routedSourceFor resolves a TaskSource using both provider name and task key.
// It prefers explicit provider routing, falling back to key-based inference.
func routedSourceFor(source TaskSource, taskKey, provider string) TaskSource {
	if ms, ok := source.(*MultiTaskSource); ok {
		return ms.routeToWithProvider(taskKey, provider)
	}
	return source
}

func (m *MultiTaskSource) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	return m.sources[m.order[0]].FetchTasks(ctx, query)
}

func (m *MultiTaskSource) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	return m.sources[m.order[0]].CreateTask(ctx, input)
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

	if len(errs) > 0 {
		return allTasks, fmt.Errorf("some providers failed: %s", strings.Join(errs, "; "))
	}

	return allTasks, nil
}
