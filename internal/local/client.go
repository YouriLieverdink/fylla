package local

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// Client implements TaskSource backed by a local YAML file.
type Client struct {
	StorePath      string
	DefaultProject string
}

// NewClient creates a local task client.
func NewClient(storePath string) *Client {
	return &Client{StorePath: storePath}
}

func (c *Client) storePath() (string, error) {
	if c.StorePath != "" {
		return c.StorePath, nil
	}
	return defaultStorePath()
}

func (c *Client) load() (*store, string, error) {
	path, err := c.storePath()
	if err != nil {
		return nil, "", err
	}
	s, err := loadStore(path)
	if err != nil {
		return nil, "", err
	}
	return s, path, nil
}

func (c *Client) save(s *store, path string) error {
	return saveStore(path, s)
}

func parseLocalKey(key string) (int, error) {
	if !strings.HasPrefix(key, "L-") {
		return 0, fmt.Errorf("invalid local key %q", key)
	}
	id, err := strconv.Atoi(key[2:])
	if err != nil {
		return 0, fmt.Errorf("invalid local key %q: %w", key, err)
	}
	return id, nil
}

func formatLocalKey(id int) string {
	return fmt.Sprintf("L-%d", id)
}

func toTask(lt localTask) task.Task {
	t := task.Task{
		Key:      formatLocalKey(lt.ID),
		Provider: "local",
		Summary:  lt.Summary,
		Priority: lt.Priority,
		Project:  lt.Project,
		Section:  lt.Section,
		Created:  lt.Created,
	}

	if lt.DueDate != "" {
		if d, err := time.Parse("2006-01-02", lt.DueDate); err == nil {
			t.DueDate = &d
		}
	}

	if lt.Estimate != "" {
		t.OriginalEstimate = parseDuration(lt.Estimate)
		t.RemainingEstimate = t.OriginalEstimate
	}

	// Parse estimate from summary bracket notation as fallback
	if t.RemainingEstimate == 0 {
		if est, cleaned := task.ParseTitleEstimate(t.Summary); est > 0 {
			t.OriginalEstimate = est
			t.RemainingEstimate = est
			t.Summary = cleaned
		}
	}

	// Extract scheduling constraints from summary
	cleaned, notBefore, notBeforeRaw, upNext, noSplit := task.ExtractConstraints(t.Summary, time.Now(), t.DueDate)
	t.Summary = cleaned
	t.NotBefore = notBefore
	t.NotBeforeRaw = notBeforeRaw
	t.UpNext = upNext
	t.NoSplit = noSplit

	// Convert recurrence
	if lt.Recurrence != nil {
		t.Recurrence = &task.Recurrence{
			Freq: lt.Recurrence.Freq,
			Days: lt.Recurrence.Days,
		}
	}

	return t
}

func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	var d time.Duration
	// Simple parser for "1h30m", "2h", "30m"
	for s != "" {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(s) {
			break
		}
		n, _ := strconv.Atoi(s[:i])
		switch s[i] {
		case 'h':
			d += time.Duration(n) * time.Hour
		case 'm':
			d += time.Duration(n) * time.Minute
		}
		s = s[i+1:]
	}
	return d
}

// FetchTasks returns open local tasks, optionally filtered.
// Supports "project:X", "section:X", and free-text filter in the query.
func (c *Client) FetchTasks(_ context.Context, query string) ([]task.Task, error) {
	s, _, err := c.load()
	if err != nil {
		return nil, err
	}

	var projectFilter, sectionFilter, textFilter string
	for _, part := range strings.Fields(query) {
		lower := strings.ToLower(part)
		if strings.HasPrefix(lower, "project:") {
			projectFilter = part[8:]
		} else if strings.HasPrefix(lower, "section:") {
			sectionFilter = part[8:]
		} else {
			if textFilter != "" {
				textFilter += " "
			}
			textFilter += part
		}
	}

	var tasks []task.Task
	for _, lt := range s.Tasks {
		if lt.Completed {
			continue
		}
		if projectFilter != "" && !strings.EqualFold(lt.Project, projectFilter) {
			continue
		}
		if sectionFilter != "" && !strings.EqualFold(lt.Section, sectionFilter) {
			continue
		}
		t := toTask(lt)
		if textFilter != "" && !strings.Contains(strings.ToLower(t.Summary), strings.ToLower(textFilter)) {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// CreateTask creates a new local task and returns its key.
func (c *Client) CreateTask(_ context.Context, input task.CreateInput) (string, error) {
	s, path, err := c.load()
	if err != nil {
		return "", err
	}

	lt := localTask{
		ID:          s.NextID,
		Summary:     input.Summary,
		Project:     input.Project,
		Section:     input.Section,
		Description: input.Description,
		Priority:    priorityNameToLevel(input.Priority),
		Created:     time.Now().UTC(),
	}

	if input.Estimate > 0 {
		lt.Estimate = formatDuration(input.Estimate)
	}
	if input.DueDate != nil {
		lt.DueDate = input.DueDate.Format("2006-01-02")
	}

	s.Tasks = append(s.Tasks, lt)
	s.NextID++

	if err := c.save(s, path); err != nil {
		return "", err
	}
	return formatLocalKey(lt.ID), nil
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

func priorityNameToLevel(name string) int {
	switch name {
	case "Highest":
		return 1
	case "High":
		return 2
	case "Medium":
		return 3
	case "Low":
		return 4
	case "Lowest":
		return 5
	default:
		return 3
	}
}

func priorityLevelToName(level int) string {
	switch level {
	case 1:
		return "Highest"
	case 2:
		return "High"
	case 3:
		return "Medium"
	case 4:
		return "Low"
	case 5:
		return "Lowest"
	default:
		return "Medium"
	}
}

// CompleteTask marks a local task as completed.
func (c *Client) CompleteTask(_ context.Context, taskKey string) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.Completed = true
	return c.save(s, path)
}

// DeleteTask removes a local task from the store.
func (c *Client) DeleteTask(_ context.Context, taskKey string) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	if !removeByID(s, id) {
		return fmt.Errorf("task %s not found", taskKey)
	}
	return c.save(s, path)
}

// PostWorklog is a no-op for local tasks.
func (c *Client) PostWorklog(_ context.Context, _ string, _ time.Duration, _ string, _ time.Time) error {
	return nil
}

// GetEstimate returns the estimate for a local task.
func (c *Client) GetEstimate(_ context.Context, taskKey string) (time.Duration, error) {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return 0, err
	}
	s, _, err := c.load()
	if err != nil {
		return 0, err
	}
	lt := findByID(s, id)
	if lt == nil {
		return 0, fmt.Errorf("task %s not found", taskKey)
	}
	if lt.Estimate != "" {
		return parseDuration(lt.Estimate), nil
	}
	if est, _ := task.ParseTitleEstimate(lt.Summary); est > 0 {
		return est, nil
	}
	return 0, nil
}

// UpdateEstimate sets the estimate for a local task.
func (c *Client) UpdateEstimate(_ context.Context, taskKey string, remaining time.Duration) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.Estimate = formatDuration(remaining)
	return c.save(s, path)
}

// GetDueDate returns the due date for a local task.
func (c *Client) GetDueDate(_ context.Context, taskKey string) (*time.Time, error) {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return nil, err
	}
	s, _, err := c.load()
	if err != nil {
		return nil, err
	}
	lt := findByID(s, id)
	if lt == nil {
		return nil, fmt.Errorf("task %s not found", taskKey)
	}
	if lt.DueDate == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", lt.DueDate)
	if err != nil {
		return nil, fmt.Errorf("parse due date: %w", err)
	}
	return &d, nil
}

// UpdateDueDate sets the due date for a local task.
func (c *Client) UpdateDueDate(_ context.Context, taskKey string, dueDate time.Time) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.DueDate = dueDate.Format("2006-01-02")
	return c.save(s, path)
}

// RemoveDueDate clears the due date for a local task.
func (c *Client) RemoveDueDate(_ context.Context, taskKey string) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.DueDate = ""
	return c.save(s, path)
}

// GetPriority returns the priority for a local task.
func (c *Client) GetPriority(_ context.Context, taskKey string) (int, error) {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return 0, err
	}
	s, _, err := c.load()
	if err != nil {
		return 0, err
	}
	lt := findByID(s, id)
	if lt == nil {
		return 0, fmt.Errorf("task %s not found", taskKey)
	}
	return lt.Priority, nil
}

// UpdatePriority sets the priority for a local task.
func (c *Client) UpdatePriority(_ context.Context, taskKey string, priority int) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.Priority = priority
	return c.save(s, path)
}

// GetSummary returns the summary for a local task.
func (c *Client) GetSummary(_ context.Context, taskKey string) (string, error) {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return "", err
	}
	s, _, err := c.load()
	if err != nil {
		return "", err
	}
	lt := findByID(s, id)
	if lt == nil {
		return "", fmt.Errorf("task %s not found", taskKey)
	}
	return lt.Summary, nil
}

// UpdateSummary sets the summary for a local task.
func (c *Client) UpdateSummary(_ context.Context, taskKey string, summary string) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.Summary = summary
	return c.save(s, path)
}

// UpdateSection sets the section for a local task.
func (c *Client) UpdateSection(_ context.Context, taskKey, section string) error {
	id, err := parseLocalKey(taskKey)
	if err != nil {
		return err
	}
	s, path, err := c.load()
	if err != nil {
		return err
	}
	lt := findByID(s, id)
	if lt == nil {
		return fmt.Errorf("task %s not found", taskKey)
	}
	lt.Section = section
	return c.save(s, path)
}

// ListProjects returns unique project names from existing tasks.
func (c *Client) ListProjects(_ context.Context) ([]string, error) {
	s, _, err := c.load()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var names []string
	for _, lt := range s.Tasks {
		if lt.Project != "" && !seen[lt.Project] {
			seen[lt.Project] = true
			names = append(names, lt.Project)
		}
	}
	return names, nil
}

// ListSections returns unique section names from existing tasks, optionally filtered by project.
func (c *Client) ListSections(_ context.Context, project string) ([]string, error) {
	s, _, err := c.load()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var names []string
	for _, lt := range s.Tasks {
		if lt.Section != "" && !seen[lt.Section] {
			if project != "" && !strings.EqualFold(lt.Project, project) {
				continue
			}
			seen[lt.Section] = true
			names = append(names, lt.Section)
		}
	}
	return names, nil
}
