package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// ErrPartialProviders signals that one or more providers failed but partial
// results are available. Callers may proceed with the returned tasks.
var ErrPartialProviders = errors.New("partial provider failure")

// TaskCache stores per-provider task lists with a TTL and collapses
// concurrent fetches for the same provider into a single in-flight call.
// Safe for concurrent use.
type TaskCache struct {
	mu      sync.Mutex
	data    map[string]taskCacheEntry
	flights map[string]*taskFlight
	ttl     time.Duration
}

type taskCacheEntry struct {
	tasks []task.Task
	at    time.Time
}

type taskFlight struct {
	done  chan struct{}
	tasks []task.Task
	err   error
}

// NewTaskCache returns an empty cache with the given TTL. A non-positive TTL
// disables freshness checks (entries always reported stale but still usable).
func NewTaskCache(ttl time.Duration) *TaskCache {
	return &TaskCache{
		data:    map[string]taskCacheEntry{},
		flights: map[string]*taskFlight{},
		ttl:     ttl,
	}
}

// Get returns cached tasks for a provider. found reports whether an entry
// exists; fresh reports whether it is within TTL.
func (c *TaskCache) Get(provider string) (tasks []task.Task, found, fresh bool) {
	if c == nil {
		return nil, false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[provider]
	if !ok {
		return nil, false, false
	}
	fresh = c.ttl > 0 && time.Since(e.at) < c.ttl
	return e.tasks, true, fresh
}

// Set stores a task list for a provider.
func (c *TaskCache) Set(provider string, tasks []task.Task) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[provider] = taskCacheEntry{tasks: tasks, at: time.Now()}
}

// Invalidate drops the cached entry for a provider.
func (c *TaskCache) Invalidate(provider string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, provider)
}

// InvalidateAll drops all cached entries.
func (c *TaskCache) InvalidateAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.data {
		delete(c.data, k)
	}
}

// FetchOrShare returns fresh cached tasks when available, otherwise collapses
// concurrent calls for the same provider onto a single invocation of fetch.
// The returned bool reports whether the result was served from fresh cache
// (meaning fetch was not invoked for this caller). Waiters on an in-flight
// fetch respect ctx cancellation.
func (c *TaskCache) FetchOrShare(ctx context.Context, provider string, fetch func() ([]task.Task, error)) ([]task.Task, bool, error) {
	if c == nil {
		tasks, err := fetch()
		return tasks, false, err
	}

	c.mu.Lock()
	if e, ok := c.data[provider]; ok && c.ttl > 0 && time.Since(e.at) < c.ttl {
		c.mu.Unlock()
		return e.tasks, true, nil
	}
	if f, ok := c.flights[provider]; ok {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.tasks, false, f.err
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}
	f := &taskFlight{done: make(chan struct{})}
	c.flights[provider] = f
	c.mu.Unlock()

	tasks, err := fetch()

	c.mu.Lock()
	delete(c.flights, provider)
	if err == nil {
		c.data[provider] = taskCacheEntry{tasks: tasks, at: time.Now()}
	}
	c.mu.Unlock()

	f.tasks, f.err = tasks, err
	close(f.done)
	return tasks, false, err
}

// MultiTaskSource wraps multiple named TaskSource instances and routes
// operations to the correct provider based on the task key.
type MultiTaskSource struct {
	sources map[string]TaskSource
	order   []string // provider names in config order
	cache   *TaskCache
}

// Compile-time check that MultiTaskSource satisfies TaskSource.
var _ TaskSource = (*MultiTaskSource)(nil)

// NewMultiTaskSource creates a MultiTaskSource from named providers.
func NewMultiTaskSource(sources map[string]TaskSource, order []string) *MultiTaskSource {
	return &MultiTaskSource{sources: sources, order: order}
}

// SetCache attaches a shared task cache. Mutations on this source will
// invalidate the cache entry for the affected provider.
func (m *MultiTaskSource) SetCache(c *TaskCache) {
	m.cache = c
}

// providerNameForKey returns the provider name that owns the given task key.
func (m *MultiTaskSource) providerNameForKey(taskKey string) string {
	name := providerForKey(taskKey)
	if _, ok := m.sources[name]; ok {
		return name
	}
	return m.order[0]
}

func (m *MultiTaskSource) invalidate(taskKey, provider string) {
	if m.cache == nil {
		return
	}
	if provider != "" {
		m.cache.Invalidate(provider)
		return
	}
	m.cache.Invalidate(m.providerNameForKey(taskKey))
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
	key, err := m.sources[m.order[0]].CreateTask(ctx, input)
	if err == nil {
		m.invalidate("", m.order[0])
	}
	return key, err
}

func (m *MultiTaskSource) CompleteTask(ctx context.Context, taskKey string) error {
	err := m.routeTo(taskKey).CompleteTask(ctx, taskKey)
	if err == nil {
		m.invalidate(taskKey, "")
	}
	return err
}

func (m *MultiTaskSource) DeleteTask(ctx context.Context, taskKey string) error {
	err := m.routeTo(taskKey).DeleteTask(ctx, taskKey)
	if err == nil {
		m.invalidate(taskKey, "")
	}
	return err
}

func (m *MultiTaskSource) PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error {
	return m.routeTo(issueKey).PostWorklog(ctx, issueKey, timeSpent, description, started)
}

func (m *MultiTaskSource) GetEstimate(ctx context.Context, issueKey string) (time.Duration, error) {
	return m.routeTo(issueKey).GetEstimate(ctx, issueKey)
}

func (m *MultiTaskSource) UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error {
	err := m.routeTo(issueKey).UpdateEstimate(ctx, issueKey, remaining)
	if err == nil {
		m.invalidate(issueKey, "")
	}
	return err
}

func (m *MultiTaskSource) GetDueDate(ctx context.Context, issueKey string) (*time.Time, error) {
	return m.routeTo(issueKey).GetDueDate(ctx, issueKey)
}

func (m *MultiTaskSource) UpdateDueDate(ctx context.Context, issueKey string, dueDate time.Time) error {
	err := m.routeTo(issueKey).UpdateDueDate(ctx, issueKey, dueDate)
	if err == nil {
		m.invalidate(issueKey, "")
	}
	return err
}

func (m *MultiTaskSource) GetPriority(ctx context.Context, issueKey string) (int, error) {
	return m.routeTo(issueKey).GetPriority(ctx, issueKey)
}

func (m *MultiTaskSource) UpdatePriority(ctx context.Context, issueKey string, priority int) error {
	err := m.routeTo(issueKey).UpdatePriority(ctx, issueKey, priority)
	if err == nil {
		m.invalidate(issueKey, "")
	}
	return err
}

func (m *MultiTaskSource) UpdateDueDateString(ctx context.Context, issueKey string, dueString string) error {
	src := m.routeTo(issueKey)
	dsu, ok := src.(DueStringUpdater)
	if !ok {
		return fmt.Errorf("provider does not support recurring/natural-language due dates")
	}
	err := dsu.UpdateDueDateString(ctx, issueKey, dueString)
	if err == nil {
		m.invalidate(issueKey, "")
	}
	return err
}

func (m *MultiTaskSource) RemoveDueDate(ctx context.Context, issueKey string) error {
	err := m.routeTo(issueKey).RemoveDueDate(ctx, issueKey)
	if err == nil {
		m.invalidate(issueKey, "")
	}
	return err
}

func (m *MultiTaskSource) GetSummary(ctx context.Context, issueKey string) (string, error) {
	return m.routeTo(issueKey).GetSummary(ctx, issueKey)
}

func (m *MultiTaskSource) UpdateSummary(ctx context.Context, issueKey string, summary string) error {
	err := m.routeTo(issueKey).UpdateSummary(ctx, issueKey, summary)
	if err == nil {
		m.invalidate(issueKey, "")
	}
	return err
}

// cachedFetcher wraps a single TaskFetcher with a shared cache + timeout. On
// fetch failure it falls back to the most recent cached result (surfacing the
// error via ErrPartialProviders).
type cachedFetcher struct {
	inner    TaskFetcher
	cache    *TaskCache
	provider string
	timeout  time.Duration
}

func (f *cachedFetcher) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	tasks, _, err := f.cache.FetchOrShare(ctx, f.provider, func() ([]task.Task, error) {
		callCtx := ctx
		var cancel context.CancelFunc
		if f.timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, f.timeout)
			defer cancel()
		}
		return f.inner.FetchTasks(callCtx, query)
	})
	if err == nil {
		return tasks, nil
	}
	if cached, ok, _ := f.cache.Get(f.provider); ok {
		return cached, fmt.Errorf("%w: %s (stale cache): %v", ErrPartialProviders, f.provider, err)
	}
	return nil, err
}

// multiFetcher implements TaskFetcher by concurrently querying multiple providers
// and merging results. Each provider call is bounded by a per-call timeout;
// providers that fail or time out fall back to cached results when available.
type multiFetcher struct {
	queries map[string]string     // provider name -> query
	sources map[string]TaskSource // provider name -> client
	cache   *TaskCache
	timeout time.Duration // 0 = no per-provider timeout
}

func (mf *multiFetcher) FetchTasks(ctx context.Context, _ string) ([]task.Task, error) {
	type result struct {
		tasks  []task.Task
		err    error
		name   string
		cached bool // served from cache after failure
	}

	var wg sync.WaitGroup
	ch := make(chan result, len(mf.queries))
	for name, query := range mf.queries {
		wg.Add(1)
		go func(n, q string) {
			defer wg.Done()
			tasks, _, err := mf.cache.FetchOrShare(ctx, n, func() ([]task.Task, error) {
				callCtx := ctx
				var cancel context.CancelFunc
				if mf.timeout > 0 {
					callCtx, cancel = context.WithTimeout(ctx, mf.timeout)
					defer cancel()
				}
				return mf.sources[n].FetchTasks(callCtx, q)
			})
			if err == nil {
				ch <- result{tasks: tasks, name: n}
				return
			}
			if cached, ok, _ := mf.cache.Get(n); ok {
				ch <- result{tasks: cached, err: err, name: n, cached: true}
				return
			}
			ch <- result{err: err, name: n}
		}(name, query)
	}
	wg.Wait()
	close(ch)

	var allTasks []task.Task
	var errs []string
	for r := range ch {
		if r.err != nil {
			label := r.name
			if r.cached {
				label = fmt.Sprintf("%s (stale cache)", r.name)
			}
			errs = append(errs, fmt.Sprintf("%s: %v", label, r.err))
		}
		allTasks = append(allTasks, r.tasks...)
	}

	if len(errs) > 0 {
		return allTasks, fmt.Errorf("%w: %s", ErrPartialProviders, strings.Join(errs, "; "))
	}

	return allTasks, nil
}
