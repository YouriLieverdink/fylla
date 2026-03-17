package kendo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/jira"
	"github.com/iruoy/fylla/internal/task"
)

// Client handles communication with the Kendo REST API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	UserID     int
	DoneLane   string

	projects     []project
	projectsOnce sync.Once
	projectsErr  error
}

// NewClient creates a Kendo client with the given credentials.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type project struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Slug   string `json:"slug"`
}

type issueJSON struct {
	ID               int        `json:"id"`
	Title            string     `json:"title"`
	Priority         *string    `json:"priority"`
	DueDate          *string    `json:"due_date"`
	CreatedAt        string     `json:"created_at"`
	EstimateMinutes  *int       `json:"estimate_minutes"`
	Lane             *string    `json:"lane"`
	Number           int        `json:"number"`
	Project          projectRef `json:"project"`
}

type projectRef struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Slug   string `json:"slug"`
}

type timeEntryJSON struct {
	ID          int    `json:"id"`
	Minutes     int    `json:"minutes"`
	Description string `json:"description"`
	StartedAt   string `json:"started_at"`
	IssueTitle  string `json:"issue_title"`
	IssueNumber int    `json:"issue_number"`
	IssueKey    string `json:"issue_key"`
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (c *Client) loadProjects(ctx context.Context) error {
	c.projectsOnce.Do(func() {
		resp, err := c.do(ctx, http.MethodGet, "/api/projects", nil)
		if err != nil {
			c.projectsErr = fmt.Errorf("fetch projects: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.projectsErr = fmt.Errorf("kendo list projects: status %d: %s", resp.StatusCode, string(body))
			return
		}

		var result struct {
			Data []project `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			c.projectsErr = fmt.Errorf("decode projects: %w", err)
			return
		}
		c.projects = result.Data
	})
	return c.projectsErr
}

func (c *Client) fetchUserID(ctx context.Context) error {
	if c.UserID != 0 {
		return nil
	}

	resp, err := c.do(ctx, http.MethodGet, "/api/auth/user", nil)
	if err != nil {
		return fmt.Errorf("fetch user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo user: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode user: %w", err)
	}
	c.UserID = result.ID
	return nil
}

// parseKey splits a Kendo issue key like "PROJ-123" into (project slug, issue number).
func (c *Client) parseKey(ctx context.Context, key string) (string, int, error) {
	prefix, numStr := splitKey(key)
	if prefix == "" || numStr == "" {
		return "", 0, fmt.Errorf("invalid kendo key %q", key)
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid issue number in key %q: %w", key, err)
	}

	if err := c.loadProjects(ctx); err != nil {
		return "", 0, err
	}

	for _, p := range c.projects {
		if strings.EqualFold(p.Prefix, prefix) {
			return p.Slug, num, nil
		}
	}
	return "", 0, fmt.Errorf("no kendo project found for prefix %q", prefix)
}

// splitKey splits "PROJ-123" into ("PROJ", "123").
func splitKey(key string) (string, string) {
	i := strings.LastIndexByte(key, '-')
	if i <= 0 || i == len(key)-1 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

var priorityNameToLevel = map[string]int{
	"urgent":  1,
	"high":    2,
	"medium":  3,
	"low":     4,
	"trivial": 5,
}

var priorityLevelToName = map[int]string{
	1: "urgent",
	2: "high",
	3: "medium",
	4: "low",
	5: "trivial",
}

func parseIssue(issue issueJSON) task.Task {
	key := fmt.Sprintf("%s-%d", issue.Project.Prefix, issue.Number)
	t := task.Task{
		Key:      key,
		Provider: "kendo",
		Summary:  issue.Title,
		Priority: 3,
		Project:  issue.Project.Prefix,
	}

	if issue.Priority != nil {
		if level, ok := priorityNameToLevel[strings.ToLower(*issue.Priority)]; ok {
			t.Priority = level
		}
	}

	if issue.DueDate != nil && *issue.DueDate != "" {
		if d, err := time.Parse("2006-01-02", *issue.DueDate); err == nil {
			t.DueDate = &d
		}
	}

	if issue.CreatedAt != "" {
		if c, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			t.Created = c
		}
	}

	if issue.EstimateMinutes != nil {
		est := time.Duration(*issue.EstimateMinutes) * time.Minute
		t.OriginalEstimate = est
		t.RemainingEstimate = est
	}

	// Extract scheduling constraints from summary
	if t.RemainingEstimate == 0 && t.OriginalEstimate == 0 {
		if est, cleaned := task.ParseTitleEstimate(t.Summary); est > 0 {
			t.OriginalEstimate = est
			t.RemainingEstimate = est
			t.Summary = cleaned
		}
	}
	if t.DueDate == nil {
		if due, cleaned := task.ParseTitleDueDate(t.Summary); due != nil {
			t.DueDate = due
			t.Summary = cleaned
		}
	}

	cleanedRec, rec := task.ExtractRecurrence(t.Summary)
	if rec != nil {
		t.Summary = cleanedRec
		t.Recurrence = rec
	}

	cleaned, notBefore, notBeforeRaw, upNext, noSplit := task.ExtractConstraints(t.Summary, time.Now(), t.DueDate)
	t.Summary = cleaned
	t.NotBefore = notBefore
	t.NotBeforeRaw = notBeforeRaw
	t.UpNext = upNext
	t.NoSplit = noSplit

	return t
}

// FetchTasks retrieves issues from a Kendo project.
func (c *Client) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	// query can be a project slug/prefix or empty (fetch from all projects)
	var slugs []string
	if query != "" {
		// Try to find the project by prefix or slug
		for _, p := range c.projects {
			if strings.EqualFold(p.Prefix, query) || strings.EqualFold(p.Slug, query) || strings.EqualFold(p.Name, query) {
				slugs = append(slugs, p.Slug)
			}
		}
		if len(slugs) == 0 {
			// Treat as slug directly
			slugs = append(slugs, query)
		}
	} else {
		for _, p := range c.projects {
			slugs = append(slugs, p.Slug)
		}
	}

	var allTasks []task.Task
	for _, slug := range slugs {
		resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/issues", slug), nil)
		if err != nil {
			return nil, fmt.Errorf("fetch issues: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("kendo fetch issues: status %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Data []issueJSON `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode issues: %w", err)
		}
		resp.Body.Close()

		for _, issue := range result.Data {
			allTasks = append(allTasks, parseIssue(issue))
		}
	}

	return allTasks, nil
}

// CreateTask creates a new issue in Kendo.
func (c *Client) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	if err := c.loadProjects(ctx); err != nil {
		return "", err
	}

	slug := ""
	prefix := ""
	for _, p := range c.projects {
		if strings.EqualFold(p.Prefix, input.Project) || strings.EqualFold(p.Slug, input.Project) || strings.EqualFold(p.Name, input.Project) {
			slug = p.Slug
			prefix = p.Prefix
			break
		}
	}
	if slug == "" {
		return "", fmt.Errorf("no kendo project found for %q", input.Project)
	}

	payload := map[string]interface{}{
		"title": input.Summary,
	}
	if input.Description != "" {
		payload["description"] = input.Description
	}
	if input.Estimate > 0 {
		payload["estimate_minutes"] = int(input.Estimate.Minutes())
	}
	if input.DueDate != nil {
		payload["due_date"] = input.DueDate.Format("2006-01-02")
	}
	if input.Priority != "" {
		payload["priority"] = strings.ToLower(input.Priority)
	}

	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%s/issues", slug), payload)
	if err != nil {
		return "", fmt.Errorf("create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("kendo create issue: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data issueJSON `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create response: %w", err)
	}
	return fmt.Sprintf("%s-%d", prefix, result.Data.Number), nil
}

// CompleteTask moves a Kendo issue to the done lane.
func (c *Client) CompleteTask(ctx context.Context, issueKey string) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	lane := c.DoneLane
	if lane == "" {
		lane = "done"
	}

	payload := map[string]interface{}{
		"lane": lane,
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), payload)
	if err != nil {
		return fmt.Errorf("complete issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo complete issue: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteTask deletes a Kendo issue.
func (c *Client) DeleteTask(ctx context.Context, issueKey string) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), nil)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo delete issue: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// PostWorklog adds a time entry to the specified Kendo issue.
func (c *Client) PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"minutes":     int(timeSpent.Minutes()),
		"description": description,
		"started_at":  started.Format(time.RFC3339),
	}

	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%s/issues/%d/time-entries", slug, num), payload)
	if err != nil {
		return fmt.Errorf("post worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo post worklog: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetEstimate fetches the remaining estimate for the specified issue.
func (c *Client) GetEstimate(ctx context.Context, issueKey string) (time.Duration, error) {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return 0, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), nil)
	if err != nil {
		return 0, fmt.Errorf("get estimate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("kendo get estimate: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data issueJSON `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode issue: %w", err)
	}

	if result.Data.EstimateMinutes == nil {
		return 0, nil
	}
	return time.Duration(*result.Data.EstimateMinutes) * time.Minute, nil
}

// UpdateEstimate sets the estimate for the specified issue.
func (c *Client) UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"estimate_minutes": int(remaining.Minutes()),
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), payload)
	if err != nil {
		return fmt.Errorf("update estimate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo update estimate: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetDueDate fetches the due date for the specified issue.
func (c *Client) GetDueDate(ctx context.Context, issueKey string) (*time.Time, error) {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), nil)
	if err != nil {
		return nil, fmt.Errorf("get due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo get due date: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data issueJSON `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode issue: %w", err)
	}

	if result.Data.DueDate == nil || *result.Data.DueDate == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", *result.Data.DueDate)
	if err != nil {
		return nil, fmt.Errorf("parse due date: %w", err)
	}
	return &d, nil
}

// UpdateDueDate sets the due date for the specified issue.
func (c *Client) UpdateDueDate(ctx context.Context, issueKey string, dueDate time.Time) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"due_date": dueDate.Format("2006-01-02"),
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), payload)
	if err != nil {
		return fmt.Errorf("update due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo update due date: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// RemoveDueDate clears the due date for the specified issue.
func (c *Client) RemoveDueDate(ctx context.Context, issueKey string) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"due_date": nil,
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), payload)
	if err != nil {
		return fmt.Errorf("remove due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo remove due date: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetPriority fetches the priority level for the specified issue.
func (c *Client) GetPriority(ctx context.Context, issueKey string) (int, error) {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return 0, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), nil)
	if err != nil {
		return 0, fmt.Errorf("get priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("kendo get priority: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data issueJSON `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode issue: %w", err)
	}

	if result.Data.Priority == nil {
		return 3, nil
	}
	if level, ok := priorityNameToLevel[strings.ToLower(*result.Data.Priority)]; ok {
		return level, nil
	}
	return 3, nil
}

// UpdatePriority sets the priority for the specified issue.
func (c *Client) UpdatePriority(ctx context.Context, issueKey string, priority int) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	var priorityVal interface{}
	if priority == 0 {
		priorityVal = nil
	} else {
		name, ok := priorityLevelToName[priority]
		if !ok {
			return fmt.Errorf("invalid priority level %d (must be 1-5)", priority)
		}
		priorityVal = name
	}

	payload := map[string]interface{}{
		"priority": priorityVal,
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), payload)
	if err != nil {
		return fmt.Errorf("update priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo update priority: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetSummary fetches the title of the specified issue.
func (c *Client) GetSummary(ctx context.Context, issueKey string) (string, error) {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return "", err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), nil)
	if err != nil {
		return "", fmt.Errorf("get summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("kendo get summary: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data issueJSON `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode issue: %w", err)
	}
	return result.Data.Title, nil
}

// UpdateSummary sets the title for the specified issue.
func (c *Client) UpdateSummary(ctx context.Context, issueKey string, summary string) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"title": summary,
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d", slug, num), payload)
	if err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo update summary: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// FetchWorklogs retrieves time entries for the current user in the given date range.
func (c *Client) FetchWorklogs(ctx context.Context, since, until time.Time) ([]jira.WorklogEntry, error) {
	if err := c.fetchUserID(ctx); err != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", err)
	}

	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	sinceDate := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	untilDate := time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, until.Location())

	var allEntries []jira.WorklogEntry
	for _, p := range c.projects {
		resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%s/time-entries?since=%s&until=%s",
			p.Slug, since.Format("2006-01-02"), until.Format("2006-01-02")), nil)
		if err != nil {
			return nil, fmt.Errorf("fetch time entries: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("kendo fetch time entries: status %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Data []timeEntryJSON `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode time entries: %w", err)
		}
		resp.Body.Close()

		for _, te := range result.Data {
			started, err := time.Parse(time.RFC3339, te.StartedAt)
			if err != nil {
				continue
			}
			if started.Before(sinceDate) || started.After(untilDate) {
				continue
			}

			issueKey := te.IssueKey
			if issueKey == "" {
				issueKey = fmt.Sprintf("%s-%d", p.Prefix, te.IssueNumber)
			}

			allEntries = append(allEntries, jira.WorklogEntry{
				ID:           strconv.Itoa(te.ID),
				IssueKey:     issueKey,
				IssueSummary: te.IssueTitle,
				Description:  te.Description,
				Started:      started,
				TimeSpent:    time.Duration(te.Minutes) * time.Minute,
			})
		}
	}

	return allEntries, nil
}

// UpdateWorklog updates an existing time entry.
func (c *Client) UpdateWorklog(ctx context.Context, issueKey, worklogID string, timeSpent time.Duration, description string, started time.Time) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"minutes":     int(timeSpent.Minutes()),
		"description": description,
		"started_at":  started.Format(time.RFC3339),
	}

	resp, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/projects/%s/issues/%d/time-entries/%s", slug, num, worklogID), payload)
	if err != nil {
		return fmt.Errorf("update worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo update worklog: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteWorklog deletes a time entry.
func (c *Client) DeleteWorklog(ctx context.Context, issueKey, worklogID string) error {
	slug, num, err := c.parseKey(ctx, issueKey)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%s/issues/%d/time-entries/%s", slug, num, worklogID), nil)
	if err != nil {
		return fmt.Errorf("delete worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo delete worklog: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ListProjects returns the prefixes of all accessible Kendo projects.
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	keys := make([]string, len(c.projects))
	for i, p := range c.projects {
		keys[i] = p.Prefix
	}
	return keys, nil
}
