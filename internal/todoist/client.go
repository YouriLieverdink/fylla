package todoist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

const defaultBaseURL = "https://api.todoist.com/api/v1"

// Client handles communication with the Todoist API v1.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	projects   map[string]string          // id → name cache
	sections   map[string]string          // id → name cache
	sectionsByProject map[string][]string // project_id → []section names

	projectsOnce sync.Once
	projectsErr  error
	sectionsOnce sync.Once
	sectionsErr  error
}

// NewClient creates a Todoist client with the given API token.
func NewClient(token string) *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	base := c.BaseURL
	if base == "" {
		base = defaultBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// todoistTask represents a task from the Todoist API v1.
type todoistTask struct {
	ID          string           `json:"id"`
	Content     string           `json:"content"`
	Description string           `json:"description"`
	Priority    int              `json:"priority"` // 1=normal, 4=urgent (inverted from UI)
	Due         *todoistDue      `json:"due"`
	Duration    *todoistDuration `json:"duration"`
	ProjectID   string           `json:"project_id"`
	SectionID   string           `json:"section_id"`
	Labels      []string         `json:"labels"`
	AddedAt     string           `json:"added_at"`
}

// paginatedResults wraps the API v1 paginated response format.
type paginatedResults[T any] struct {
	Results []T `json:"results"`
}

type todoistDue struct {
	Date        string `json:"date"`         // YYYY-MM-DD
	IsRecurring bool   `json:"is_recurring"` // true if task repeats
	String      string `json:"string"`       // human-readable recurrence ("every Monday")
}

type todoistDuration struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"` // "minute" or "day"
}

// todoistProject represents a project from the Todoist REST API.
type todoistProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type todoistSection struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
}

// apiPriorityToLevel maps Todoist API priority (1-4) to fylla priority (1-5).
// API priority 4 = urgent (p1 in UI) → fylla 1 (Highest)
// API priority 3 = high (p2 in UI) → fylla 2 (High)
// API priority 2 = medium (p3 in UI) → fylla 3 (Medium)
// API priority 1 = normal (p4 in UI) → fylla 4 (Low)
func apiPriorityToLevel(apiPriority int) int {
	switch apiPriority {
	case 4:
		return 1
	case 3:
		return 2
	case 2:
		return 3
	default:
		return 4
	}
}

// priorityNameToAPI converts fylla priority name back to Todoist API priority.
var priorityNameToAPI = map[string]int{
	"Highest": 4,
	"High":    3,
	"Medium":  2,
	"Low":     1,
	"Lowest":  1,
}

// levelToAPIPriority converts a fylla priority level (1-5) to Todoist API priority (1-4).
func levelToAPIPriority(level int) int {
	switch level {
	case 1:
		return 4
	case 2:
		return 3
	case 3:
		return 2
	default:
		return 1
	}
}

func (c *Client) loadProjects(ctx context.Context) error {
	c.projectsOnce.Do(func() {
		resp, err := c.do(ctx, http.MethodGet, "/projects", nil)
		if err != nil {
			c.projectsErr = fmt.Errorf("fetch projects: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.projectsErr = fmt.Errorf("todoist projects: status %d: %s", resp.StatusCode, string(body))
			return
		}

		var page paginatedResults[todoistProject]
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			c.projectsErr = fmt.Errorf("decode projects: %w", err)
			return
		}

		c.projects = make(map[string]string, len(page.Results))
		for _, p := range page.Results {
			c.projects[p.ID] = p.Name
		}
	})
	return c.projectsErr
}

// ListProjects returns the names of all projects.
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(c.projects))
	for _, name := range c.projects {
		names = append(names, name)
	}
	return names, nil
}

func (c *Client) loadSections(ctx context.Context) error {
	c.sectionsOnce.Do(func() {
		resp, err := c.do(ctx, http.MethodGet, "/sections", nil)
		if err != nil {
			c.sectionsErr = fmt.Errorf("fetch sections: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.sectionsErr = fmt.Errorf("todoist sections: status %d: %s", resp.StatusCode, string(body))
			return
		}

		var page paginatedResults[todoistSection]
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			c.sectionsErr = fmt.Errorf("decode sections: %w", err)
			return
		}

		c.sections = make(map[string]string, len(page.Results))
		c.sectionsByProject = make(map[string][]string)
		for _, s := range page.Results {
			c.sections[s.ID] = s.Name
			c.sectionsByProject[s.ProjectID] = append(c.sectionsByProject[s.ProjectID], s.Name)
		}
	})
	return c.sectionsErr
}

// ListSections returns available section names, optionally filtered by project.
func (c *Client) ListSections(ctx context.Context, project string) ([]string, error) {
	if err := c.loadSections(ctx); err != nil {
		return nil, err
	}
	if project != "" {
		if err := c.loadProjects(ctx); err != nil {
			return nil, err
		}
		// Find project ID by name
		var projectID string
		for id, name := range c.projects {
			if strings.EqualFold(name, project) {
				projectID = id
				break
			}
		}
		if projectID != "" {
			return c.sectionsByProject[projectID], nil
		}
		return nil, nil
	}
	names := make([]string, 0, len(c.sections))
	for _, name := range c.sections {
		names = append(names, name)
	}
	return names, nil
}

func (c *Client) sectionName(id string) string {
	if id == "" || c.sections == nil {
		return ""
	}
	if name, ok := c.sections[id]; ok {
		return name
	}
	return ""
}

func (c *Client) projectName(id string) string {
	if c.projects == nil {
		return id
	}
	if name, ok := c.projects[id]; ok {
		return name
	}
	return id
}

func (c *Client) parseTask(t todoistTask) task.Task {
	result := task.Task{
		Key:      t.ID,
		Provider: "todoist",
		Summary:  t.Content,
		Priority: apiPriorityToLevel(t.Priority),
	}

	// Project & Section
	result.Project = c.projectName(t.ProjectID)
	result.Section = c.sectionName(t.SectionID)

	// Created
	if t.AddedAt != "" {
		if created, err := time.Parse(time.RFC3339, t.AddedAt); err == nil {
			result.Created = created
		}
	}

	// Estimate: parse [duration] from title (duration field is Pro-only).
	if est, cleaned := task.ParseTitleEstimate(result.Summary); est > 0 {
		result.OriginalEstimate = est
		result.RemainingEstimate = est
		result.Summary = cleaned
	}

	// Due date: use native Todoist field, strip any {date} from title.
	if t.Due != nil && t.Due.Date != "" {
		if d, err := time.Parse("2006-01-02", t.Due.Date); err == nil {
			result.DueDate = &d
		}
	}
	if _, cleaned := task.ParseTitleDueDate(result.Summary); cleaned != result.Summary {
		result.Summary = cleaned
	}
	if result.RemainingEstimate == 0 && t.Duration != nil && t.Duration.Amount > 0 {
		switch t.Duration.Unit {
		case "minute":
			result.OriginalEstimate = time.Duration(t.Duration.Amount) * time.Minute
		case "day":
			result.OriginalEstimate = time.Duration(t.Duration.Amount) * 8 * time.Hour
		}
		result.RemainingEstimate = result.OriginalEstimate
	}

	// Extract scheduling constraints from summary
	cleaned, notBefore, notBeforeRaw, upNext, noSplit, _ := task.ExtractConstraints(result.Summary, time.Now(), result.DueDate)
	result.Summary = cleaned
	result.NotBefore = notBefore
	result.NotBeforeRaw = notBeforeRaw
	result.UpNext = upNext
	result.NoSplit = noSplit

	// Parse Todoist native recurrence
	if t.Due != nil && t.Due.IsRecurring && t.Due.String != "" {
		result.Recurrence = parseTodoistRecurrence(t.Due.String)
	}

	return result
}

// FetchTasks retrieves active tasks from Todoist, optionally filtered.
func (c *Client) FetchTasks(ctx context.Context, filter string) ([]task.Task, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}
	if err := c.loadSections(ctx); err != nil {
		return nil, err
	}

	path := "/tasks"
	if filter != "" {
		path = "/tasks/filter?query=" + url.QueryEscape(filter)
	}

	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch tasks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("todoist tasks: status %d: %s", resp.StatusCode, string(body))
	}

	var page paginatedResults[todoistTask]
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}

	tasks := make([]task.Task, 0, len(page.Results))
	for _, t := range page.Results {
		tasks = append(tasks, c.parseTask(t))
	}

	return tasks, nil
}

// CreateTask creates a new task in Todoist and returns its ID.
func (c *Client) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	content := input.Summary
	if input.Estimate > 0 {
		content = task.SetTitleEstimate(content, input.Estimate)
	}

	payload := map[string]interface{}{
		"content": content,
	}

	if input.Description != "" {
		payload["description"] = input.Description
	}

	if input.Priority != "" {
		if apiPri, ok := priorityNameToAPI[input.Priority]; ok {
			payload["priority"] = apiPri
		}
	}

	if input.Project != "" {
		// Try to find project ID by name
		if err := c.loadProjects(ctx); err == nil {
			for id, name := range c.projects {
				if strings.EqualFold(name, input.Project) {
					payload["project_id"] = id
					break
				}
			}
		}
	}

	if input.Section != "" {
		if err := c.loadSections(ctx); err == nil {
			for id, name := range c.sections {
				if strings.EqualFold(name, input.Section) {
					payload["section_id"] = id
					break
				}
			}
		}
	}

	if input.DueDate != nil {
		payload["due_date"] = input.DueDate.Format("2006-01-02")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal task: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks", strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("create task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("todoist create task: status %d: %s", resp.StatusCode, string(body))
	}

	var result todoistTask
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create response: %w", err)
	}
	return result.ID, nil
}

// PostWorklog adds a comment to a Todoist task to track time.
func (c *Client) PostWorklog(ctx context.Context, taskID string, timeSpent time.Duration, description string, started time.Time) error {
	h := int(timeSpent.Hours())
	m := int(timeSpent.Minutes()) % 60
	var timeStr string
	if h > 0 && m > 0 {
		timeStr = fmt.Sprintf("%dh %dm", h, m)
	} else if h > 0 {
		timeStr = fmt.Sprintf("%dh", h)
	} else {
		timeStr = fmt.Sprintf("%dm", m)
	}

	content := fmt.Sprintf("[Worklog] %s — %s", timeStr, description)

	payload := map[string]interface{}{
		"task_id": taskID,
		"content": content,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal comment: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/comments", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist comment: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CompleteTask closes a Todoist task by posting to the close endpoint.
func (c *Client) CompleteTask(ctx context.Context, taskID string) error {
	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID+"/close", nil)
	if err != nil {
		return fmt.Errorf("close task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist close task: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteTask permanently deletes a Todoist task.
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/tasks/"+taskID, nil)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist delete task: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) fetchTask(ctx context.Context, taskID string) (todoistTask, error) {
	resp, err := c.do(ctx, http.MethodGet, "/tasks/"+taskID, nil)
	if err != nil {
		return todoistTask{}, fmt.Errorf("get task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return todoistTask{}, fmt.Errorf("todoist get task: status %d: %s", resp.StatusCode, string(body))
	}

	var t todoistTask
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return todoistTask{}, fmt.Errorf("decode task: %w", err)
	}
	return t, nil
}

// GetEstimate fetches the current estimate for a Todoist task.
func (c *Client) GetEstimate(ctx context.Context, taskID string) (time.Duration, error) {
	t, err := c.fetchTask(ctx, taskID)
	if err != nil {
		return 0, err
	}

	if est, _ := task.ParseTitleEstimate(t.Content); est > 0 {
		return est, nil
	}
	return 0, nil
}

// GetDueDate fetches the current due date for a Todoist task.
func (c *Client) GetDueDate(ctx context.Context, taskID string) (*time.Time, error) {
	t, err := c.fetchTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if t.Due != nil && t.Due.Date != "" {
		d, err := time.Parse("2006-01-02", t.Due.Date)
		if err != nil {
			return nil, fmt.Errorf("parse due date: %w", err)
		}
		return &d, nil
	}
	return nil, nil
}

// UpdateDueDate sets the due date on a Todoist task.
func (c *Client) UpdateDueDate(ctx context.Context, taskID string, dueDate time.Time) error {
	payload := map[string]interface{}{
		"due_date": dueDate.Format("2006-01-02"),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("update due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist update due date: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UpdateEstimate updates the estimate in the task title using bracket notation.
func (c *Client) UpdateEstimate(ctx context.Context, taskID string, remaining time.Duration) error {
	t, err := c.fetchTask(ctx, taskID)
	if err != nil {
		return err
	}

	newContent := task.SetTitleEstimate(t.Content, remaining)

	payload := map[string]interface{}{
		"content": newContent,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist update task: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetPriority fetches the priority level for a Todoist task.
func (c *Client) GetPriority(ctx context.Context, taskID string) (int, error) {
	t, err := c.fetchTask(ctx, taskID)
	if err != nil {
		return 0, err
	}
	return apiPriorityToLevel(t.Priority), nil
}

// RemoveDueDate clears the due date on a Todoist task.
func (c *Client) RemoveDueDate(ctx context.Context, taskID string) error {
	payload := map[string]interface{}{
		"due_string": "no date",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("remove due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist remove due date: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetSummary fetches the raw content (title) for a Todoist task.
func (c *Client) GetSummary(ctx context.Context, taskID string) (string, error) {
	t, err := c.fetchTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	return t.Content, nil
}

// UpdateSummary sets the content (title) for a Todoist task.
func (c *Client) UpdateSummary(ctx context.Context, taskID string, summary string) error {
	payload := map[string]interface{}{
		"content": summary,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist update summary: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// parseTodoistRecurrence converts a Todoist recurrence string to a task.Recurrence.
func parseTodoistRecurrence(s string) *task.Recurrence {
	// Todoist uses strings like "every day", "every mon, wed", "every! weekday"
	// Normalize: strip "every" prefix and "!" (strict marker)
	lower := strings.ToLower(strings.TrimSpace(s))
	lower = strings.TrimPrefix(lower, "every")
	lower = strings.TrimPrefix(lower, "!")
	lower = strings.TrimSpace(lower)

	// Wrap in "(every ...)" to reuse the shared parser
	_, rec := task.ExtractRecurrence("(" + "every " + lower + ")")
	return rec
}

// UpdateSection moves a Todoist task to a different section (or removes it).
func (c *Client) UpdateSection(ctx context.Context, taskID, section string) error {
	payload := map[string]interface{}{}
	if section != "" {
		if err := c.loadSections(ctx); err != nil {
			return err
		}
		var sectionID string
		for id, name := range c.sections {
			if strings.EqualFold(name, section) {
				sectionID = id
				break
			}
		}
		if sectionID == "" {
			return fmt.Errorf("section %q not found", section)
		}
		payload["section_id"] = sectionID
	} else {
		t, err := c.fetchTask(ctx, taskID)
		if err != nil {
			return fmt.Errorf("fetch task for move: %w", err)
		}
		payload["project_id"] = t.ProjectID
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID+"/move", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("update section: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist update section: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UpdatePriority sets the priority on a Todoist task.
func (c *Client) UpdatePriority(ctx context.Context, taskID string, priority int) error {
	payload := map[string]interface{}{
		"priority": levelToAPIPriority(priority),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update: %w", err)
	}

	resp, err := c.do(ctx, http.MethodPost, "/tasks/"+taskID, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("update priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("todoist update priority: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
