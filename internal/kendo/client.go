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
	ID   int    `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type issueJSON struct {
	ID               int    `json:"id"`
	Key              string `json:"key"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Priority         int    `json:"priority"`
	Type             int    `json:"type"`
	Order            int    `json:"order"`
	AssigneeID       *int   `json:"assignee_id"`
	SprintID         *int   `json:"sprint_id"`
	EpicID           *int   `json:"epic_id"`
	ProjectID        int    `json:"project_id"`
	LaneID           int    `json:"lane_id"`
	EstimatedMinutes *int   `json:"estimated_minutes"`
	BlockedByIDs     []int  `json:"blocked_by_ids"`
	BlocksIDs        []int  `json:"blocks_ids"`
	CreatedAt        string `json:"created_at"`
}

type epicJSON struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// issueUpdatePayload builds the full payload required by PUT /issues/{key},
// starting from the current issue state and applying overrides.
func issueUpdatePayload(current issueJSON, overrides map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{
		"title":          current.Title,
		"description":    current.Description,
		"lane_id":        current.LaneID,
		"priority":       current.Priority,
		"type":           current.Type,
		"order":          current.Order,
		"blocked_by_ids": current.BlockedByIDs,
		"blocks_ids":     current.BlocksIDs,
	}
	if current.AssigneeID != nil {
		payload["assignee_id"] = *current.AssigneeID
	}
	if current.SprintID != nil {
		payload["sprint_id"] = *current.SprintID
	}
	if current.EpicID != nil {
		payload["epic_id"] = *current.EpicID
	}
	if current.EstimatedMinutes != nil {
		payload["estimated_minutes"] = *current.EstimatedMinutes
	}
	for k, v := range overrides {
		payload[k] = v
	}
	return payload
}

type sprintJSON struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Status    int    `json:"status"` // 0=Planned, 1=Active, 2=Completed
	Start     string `json:"start"`
	End       string `json:"end"`
	ProjectID int    `json:"project_id"`
}

// SprintOption represents a selectable sprint for the TUI.
type SprintOption struct {
	ID     int
	Label  string
	Active bool
}

type timeEntryJSON struct {
	ID           int    `json:"id"`
	MinutesSpent int    `json:"minutes_spent"`
	Note         string `json:"note"`
	StartedAt    string `json:"started_at"`
	IssueTitle   string `json:"issue_title"`
	IssueKey     string `json:"issue_key"`
	ProjectID    int    `json:"project_id"`
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

		if err := json.NewDecoder(resp.Body).Decode(&c.projects); err != nil {
			c.projectsErr = fmt.Errorf("decode projects: %w", err)
			return
		}
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

// fetchIssue retrieves the full issue state for a given key.
func (c *Client) fetchIssue(ctx context.Context, pid int, issueKey string) (issueJSON, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/issues/%s", pid, issueKey), nil)
	if err != nil {
		return issueJSON{}, fmt.Errorf("fetch issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return issueJSON{}, fmt.Errorf("kendo fetch issue: status %d: %s", resp.StatusCode, string(body))
	}

	var issue issueJSON
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return issueJSON{}, fmt.Errorf("decode issue: %w", err)
	}
	return issue, nil
}

// putIssue sends a PUT request with all required fields, merging overrides onto the current state.
func (c *Client) putIssue(ctx context.Context, pid int, issueKey string, overrides map[string]interface{}) error {
	current, err := c.fetchIssue(ctx, pid, issueKey)
	if err != nil {
		return err
	}
	payload := issueUpdatePayload(current, overrides)

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/api/projects/%d/issues/%s", pid, issueKey), payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo update issue: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// projectIDForKey extracts the project code from a key like "IRUOY-0001" and
// returns the project ID for API paths.
func (c *Client) projectIDForKey(ctx context.Context, key string) (int, error) {
	code, _ := splitKey(key)
	if code == "" {
		return 0, fmt.Errorf("invalid kendo key %q", key)
	}

	if err := c.loadProjects(ctx); err != nil {
		return 0, err
	}

	for _, p := range c.projects {
		if strings.EqualFold(p.Code, code) {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("no kendo project found for code %q", code)
}

// splitKey splits "PROJ-0001" into ("PROJ", "0001").
func splitKey(key string) (string, string) {
	i := strings.LastIndexByte(key, '-')
	if i <= 0 || i == len(key)-1 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

// Kendo API priority is 0-4 (0=Highest). Fylla uses 1-5 (1=urgent).
func kendoPriorityToFylla(p int) int { return p + 1 }
func fyllaPriorityToKendo(p int) int { return p - 1 }

func (c *Client) projectCodeByID(projectID int) string {
	for _, p := range c.projects {
		if p.ID == projectID {
			return p.Code
		}
	}
	return ""
}

func parseIssue(issue issueJSON, projectCode, laneName string, epicMap map[int]string) task.Task {
	t := task.Task{
		Key:      issue.Key,
		Provider: "kendo",
		Summary:  issue.Title,
		Priority: kendoPriorityToFylla(issue.Priority),
		Project:  projectCode,
		Status:   laneName,
	}

	if issue.CreatedAt != "" {
		if c, err := time.Parse(time.RFC3339, issue.CreatedAt); err == nil {
			t.Created = c
		}
	}

	if issue.EstimatedMinutes != nil {
		est := time.Duration(*issue.EstimatedMinutes) * time.Minute
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

	cleaned, notBefore, notBeforeRaw, upNext, noSplit, titleDue := task.ExtractConstraints(t.Summary, time.Now(), t.DueDate)
	t.Summary = cleaned
	t.NotBefore = notBefore
	t.NotBeforeRaw = notBeforeRaw
	t.UpNext = upNext
	t.NoSplit = noSplit
	if t.DueDate == nil && titleDue != nil {
		t.DueDate = titleDue
	}

	if issue.EpicID != nil {
		if name, ok := epicMap[*issue.EpicID]; ok {
			t.Section = name
		}
	}

	return t
}

// issueFilter holds parsed filter criteria from a query string.
// Supports: assignee_id=me, project_id=N, lane_id=N, priority=N.
type issueFilter struct {
	assigneeID *int
	projectID  *int
	laneID     *int
	priority   *int
}

func (f issueFilter) matches(issue issueJSON) bool {
	if f.assigneeID != nil {
		if issue.AssigneeID == nil || *issue.AssigneeID != *f.assigneeID {
			return false
		}
	}
	if f.projectID != nil && issue.ProjectID != *f.projectID {
		return false
	}
	if f.laneID != nil && issue.LaneID != *f.laneID {
		return false
	}
	if f.priority != nil && issue.Priority != *f.priority {
		return false
	}
	return true
}

func (f issueFilter) empty() bool {
	return f.assigneeID == nil && f.projectID == nil && f.laneID == nil && f.priority == nil
}

// parseFilter parses a query like "assignee_id=me&project_id=4" into an issueFilter.
// "me" is resolved to the current user ID.
func (c *Client) parseFilter(query string, userID int) issueFilter {
	var f issueFilter
	for _, part := range strings.Split(query, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch key {
		case "assignee_id":
			if val == "me" {
				f.assigneeID = &userID
			} else if id, err := strconv.Atoi(val); err == nil {
				f.assigneeID = &id
			}
		case "project_id":
			if id, err := strconv.Atoi(val); err == nil {
				f.projectID = &id
			}
		case "lane_id":
			if id, err := strconv.Atoi(val); err == nil {
				f.laneID = &id
			}
		case "priority":
			if p, err := strconv.Atoi(val); err == nil {
				f.priority = &p
			}
		}
	}
	return f
}

// isFilterQuery returns true if the query looks like key=value filter params.
func isFilterQuery(query string) bool {
	return strings.Contains(query, "=")
}

// FetchTasks retrieves issues from Kendo projects.
//
// The query can be:
//   - Empty or "*": fetch all issues from all projects
//   - A project code/name: fetch all issues from that project
//   - Filter params (key=value pairs joined by &): fetch and filter issues
//     Supported filters: assignee_id=me, assignee_id=N, project_id=N, lane_id=N, priority=N
func (c *Client) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	// Parse filter if query contains key=value pairs
	var filter issueFilter
	if isFilterQuery(query) {
		if err := c.fetchUserID(ctx); err != nil {
			return nil, err
		}
		filter = c.parseFilter(query, c.UserID)
	}

	// Determine which project IDs to fetch
	var projectIDs []int
	if filter.projectID != nil {
		projectIDs = append(projectIDs, *filter.projectID)
	} else if !isFilterQuery(query) && query != "" && query != "*" {
		// Simple project code/name filter
		for _, p := range c.projects {
			if strings.EqualFold(p.Code, query) || strings.EqualFold(p.Name, query) {
				projectIDs = append(projectIDs, p.ID)
			}
		}
		if len(projectIDs) == 0 {
			if id, err := strconv.Atoi(query); err == nil {
				projectIDs = append(projectIDs, id)
			}
		}
	}
	// No filter or no match: fetch all projects
	if len(projectIDs) == 0 {
		for _, p := range c.projects {
			projectIDs = append(projectIDs, p.ID)
		}
	}

	var allTasks []task.Task
	for _, pid := range projectIDs {
		// Fetch lanes for this project to map laneID → name
		laneMap, err := c.fetchLaneMap(ctx, pid)
		if err != nil {
			return nil, err
		}

		resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/issues", pid), nil)
		if err != nil {
			return nil, fmt.Errorf("fetch issues: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("kendo fetch issues: status %d: %s", resp.StatusCode, string(body))
		}

		var issues []issueJSON
		if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode issues: %w", err)
		}
		resp.Body.Close()

		doneLaneID := c.doneLaneIDFromMap(laneMap)

		epicMap, err := c.fetchEpicMap(ctx, pid)
		if err != nil {
			return nil, err
		}

		code := c.projectCodeByID(pid)
		for _, issue := range issues {
			if issue.LaneID == doneLaneID {
				continue
			}
			if !filter.empty() && !filter.matches(issue) {
				continue
			}
			allTasks = append(allTasks, parseIssue(issue, code, laneMap[issue.LaneID], epicMap))
		}
	}

	return allTasks, nil
}

// CreateTask creates a new issue in Kendo.
func (c *Client) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	pid, err := c.projectIDForName(ctx, input.Project)
	if err != nil {
		return "", err
	}

	laneID, err := c.findLaneID(ctx, pid, input.IssueType)
	if err != nil {
		return "", fmt.Errorf("resolve lane: %w", err)
	}

	payload := map[string]interface{}{
		"title":       input.Summary,
		"description": input.Description,
		"priority":    2, // default medium
		"type":        0, // feature
		"order":       0,
		"lane_id":     laneID,
	}
	if err := c.fetchUserID(ctx); err == nil && c.UserID != 0 {
		payload["assignee_id"] = c.UserID
	}
	if input.Estimate > 0 {
		payload["estimated_minutes"] = int(input.Estimate.Minutes())
	}
	if input.Priority != "" {
		if level, ok := priorityNameToLevel[strings.ToLower(input.Priority)]; ok {
			payload["priority"] = fyllaPriorityToKendo(level)
		}
	}
	if input.SprintID != nil {
		payload["sprint_id"] = *input.SprintID
	}
	if input.Parent != "" {
		epicID, err := c.resolveEpicID(ctx, pid, input.Parent)
		if err == nil {
			payload["epic_id"] = epicID
		}
	}

	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/issues", pid), payload)
	if err != nil {
		return "", fmt.Errorf("create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("kendo create issue: status %d: %s", resp.StatusCode, string(body))
	}

	var created issueJSON
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("decode create response: %w", err)
	}
	return created.Key, nil
}

// CompleteTask moves a Kendo issue to the done lane.
func (c *Client) CompleteTask(ctx context.Context, issueKey string) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}

	laneID, err := c.findDoneLaneID(ctx, pid)
	if err != nil {
		return err
	}

	return c.putIssue(ctx, pid, issueKey, map[string]interface{}{
		"lane_id": laneID,
	})
}

type laneJSON struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func (c *Client) findDoneLaneID(ctx context.Context, projectID int) (int, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/lanes", projectID), nil)
	if err != nil {
		return 0, fmt.Errorf("fetch lanes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("kendo fetch lanes: status %d: %s", resp.StatusCode, string(body))
	}

	var lanes []laneJSON
	if err := json.NewDecoder(resp.Body).Decode(&lanes); err != nil {
		return 0, fmt.Errorf("decode lanes: %w", err)
	}

	target := c.DoneLane
	if target == "" {
		target = "done"
	}

	for _, l := range lanes {
		if strings.EqualFold(l.Title, target) {
			return l.ID, nil
		}
	}
	// Fall back to last lane (typically done)
	if len(lanes) > 0 {
		return lanes[len(lanes)-1].ID, nil
	}
	return 0, fmt.Errorf("no done lane found for project %d", projectID)
}

func (c *Client) doneLaneIDFromMap(laneMap map[int]string) int {
	target := c.DoneLane
	if target == "" {
		target = "done"
	}
	for id, title := range laneMap {
		if strings.EqualFold(title, target) {
			return id
		}
	}
	return -1
}

// ListLanes returns lane names for a Kendo project.
func (c *Client) ListLanes(ctx context.Context, project string) ([]string, error) {
	pid, err := c.projectIDForName(ctx, project)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/lanes", pid), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch lanes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo list lanes: status %d: %s", resp.StatusCode, string(body))
	}

	var lanes []laneJSON
	if err := json.NewDecoder(resp.Body).Decode(&lanes); err != nil {
		return nil, fmt.Errorf("decode lanes: %w", err)
	}
	names := make([]string, len(lanes))
	for i, l := range lanes {
		names[i] = l.Title
	}
	return names, nil
}

// ListSprints returns non-completed sprints for a Kendo project, active first.
func (c *Client) ListSprints(ctx context.Context, project string) ([]SprintOption, error) {
	pid, err := c.projectIDForName(ctx, project)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/sprints", pid), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch sprints: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo list sprints: status %d: %s", resp.StatusCode, string(body))
	}

	var sprints []sprintJSON
	if err := json.NewDecoder(resp.Body).Decode(&sprints); err != nil {
		return nil, fmt.Errorf("decode sprints: %w", err)
	}

	var active, planned []SprintOption
	for _, s := range sprints {
		if s.Status == 2 {
			continue // skip completed
		}
		opt := SprintOption{ID: s.ID, Label: s.Title, Active: s.Status == 1}
		if s.Status == 1 {
			opt.Label += " (Active)"
			active = append(active, opt)
		} else {
			planned = append(planned, opt)
		}
	}
	return append(active, planned...), nil
}

// fetchEpicMap returns a map of epic ID → title for the given project.
func (c *Client) fetchEpicMap(ctx context.Context, projectID int) (map[int]string, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/epics", projectID), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch epics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo fetch epics: status %d: %s", resp.StatusCode, string(body))
	}

	var epics []epicJSON
	if err := json.NewDecoder(resp.Body).Decode(&epics); err != nil {
		return nil, fmt.Errorf("decode epics: %w", err)
	}
	m := make(map[int]string, len(epics))
	for _, e := range epics {
		m[e.ID] = e.Title
	}
	return m, nil
}

// ListEpics returns open epics for a Kendo project.
func (c *Client) ListEpics(ctx context.Context, project string) ([]jira.Epic, error) {
	pid, err := c.projectIDForName(ctx, project)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/epics", pid), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch epics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo list epics: status %d: %s", resp.StatusCode, string(body))
	}

	var raw []epicJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode epics: %w", err)
	}
	epics := make([]jira.Epic, 0, len(raw))
	for _, e := range raw {
		epics = append(epics, jira.Epic{Key: strconv.Itoa(e.ID), Summary: e.Title})
	}
	return epics, nil
}

func (c *Client) projectIDForName(ctx context.Context, project string) (int, error) {
	if err := c.loadProjects(ctx); err != nil {
		return 0, err
	}
	for _, p := range c.projects {
		if strings.EqualFold(p.Code, project) || strings.EqualFold(p.Name, project) {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("no kendo project found for %q", project)
}

func (c *Client) fetchLaneMap(ctx context.Context, projectID int) (map[int]string, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/lanes", projectID), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch lanes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo fetch lanes: status %d: %s", resp.StatusCode, string(body))
	}

	var lanes []laneJSON
	if err := json.NewDecoder(resp.Body).Decode(&lanes); err != nil {
		return nil, fmt.Errorf("decode lanes: %w", err)
	}

	m := make(map[int]string, len(lanes))
	for _, l := range lanes {
		m[l.ID] = l.Title
	}
	return m, nil
}

func (c *Client) findLaneID(ctx context.Context, projectID int, laneName string) (int, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/lanes", projectID), nil)
	if err != nil {
		return 0, fmt.Errorf("fetch lanes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("kendo fetch lanes: status %d: %s", resp.StatusCode, string(body))
	}

	var lanes []laneJSON
	if err := json.NewDecoder(resp.Body).Decode(&lanes); err != nil {
		return 0, fmt.Errorf("decode lanes: %w", err)
	}

	for _, l := range lanes {
		if strings.EqualFold(l.Title, laneName) {
			return l.ID, nil
		}
	}
	return 0, fmt.Errorf("no lane found for %q", laneName)
}

// ListTransitions returns the available lane names for the project that owns the given task key.
func (c *Client) ListTransitions(ctx context.Context, taskKey string) ([]string, error) {
	pid, err := c.projectIDForKey(ctx, taskKey)
	if err != nil {
		return nil, err
	}
	return c.ListLanes(ctx, c.projectCodeByID(pid))
}

// TransitionTask moves a Kendo issue to the named lane.
func (c *Client) TransitionTask(ctx context.Context, taskKey, target string) error {
	pid, err := c.projectIDForKey(ctx, taskKey)
	if err != nil {
		return err
	}

	laneID, err := c.findLaneID(ctx, pid, target)
	if err != nil {
		return err
	}

	return c.putIssue(ctx, pid, taskKey, map[string]interface{}{
		"lane_id": laneID,
	})
}

func (c *Client) resolveEpicID(ctx context.Context, projectID int, epicKey string) (int, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/epics", projectID), nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("fetch epics: status %d", resp.StatusCode)
	}
	var epics []epicJSON
	if err := json.NewDecoder(resp.Body).Decode(&epics); err != nil {
		return 0, err
	}
	// Match by ID (ListEpics returns epic IDs as keys) or by title.
	for _, e := range epics {
		if strconv.Itoa(e.ID) == epicKey || strings.EqualFold(e.Title, epicKey) {
			return e.ID, nil
		}
	}
	return 0, fmt.Errorf("no epic found for key %q", epicKey)
}

// DeleteTask deletes a Kendo issue.
func (c *Client) DeleteTask(ctx context.Context, issueKey string) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%d/issues/%s", pid, issueKey), nil)
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
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"minutes_spent": int(timeSpent.Minutes()),
		"note":          description,
		"started_at":    started.Format(time.RFC3339),
	}

	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/projects/%d/issues/%s/time-entries", pid, issueKey), payload)
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
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return 0, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/issues/%s", pid, issueKey), nil)
	if err != nil {
		return 0, fmt.Errorf("get estimate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("kendo get estimate: status %d: %s", resp.StatusCode, string(body))
	}

	var issue issueJSON
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return 0, fmt.Errorf("decode issue: %w", err)
	}

	if issue.EstimatedMinutes == nil {
		return 0, nil
	}
	return time.Duration(*issue.EstimatedMinutes) * time.Minute, nil
}

// UpdateEstimate sets the estimate for the specified issue.
func (c *Client) UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}
	return c.putIssue(ctx, pid, issueKey, map[string]interface{}{
		"estimated_minutes": int(remaining.Minutes()),
	})
}

// GetDueDate extracts the due date from the issue title since Kendo has no native due date field.
func (c *Client) GetDueDate(ctx context.Context, issueKey string) (*time.Time, error) {
	summary, err := c.GetSummary(ctx, issueKey)
	if err != nil {
		return nil, err
	}
	// Try {date} format first
	if due, _ := task.ParseTitleDueDate(summary); due != nil {
		return due, nil
	}
	// Try "due <date>" format
	_, due := task.ExtractDueClause(summary, time.Now())
	return due, nil
}

// UpdateDueDate stores the due date in the issue title since Kendo has no native due date field.
func (c *Client) UpdateDueDate(ctx context.Context, issueKey string, dueDate time.Time) error {
	summary, err := c.GetSummary(ctx, issueKey)
	if err != nil {
		return err
	}
	est, summary := task.ParseTitleEstimate(summary)
	_, summary = task.ParseTitleDueDate(summary) // strip {date} format
	cleaned, notBefore, notBeforeRaw, upNext, noSplit, _ := task.ExtractConstraints(summary, time.Now(), nil)
	nbVal := notBeforeRaw
	if nbVal == "" && notBefore != nil {
		nbVal = notBefore.Format("2006-01-02")
	}
	mods := task.BuildModifiers(dueDate.Format("2006-01-02"), nbVal, upNext, noSplit)
	result := cleaned
	if mods != "" {
		result += " " + mods
	}
	if est > 0 {
		result = task.SetTitleEstimate(result, est)
	}
	return c.UpdateSummary(ctx, issueKey, result)
}

// RemoveDueDate removes the due date from the issue title.
func (c *Client) RemoveDueDate(ctx context.Context, issueKey string) error {
	summary, err := c.GetSummary(ctx, issueKey)
	if err != nil {
		return err
	}
	est, summary := task.ParseTitleEstimate(summary)
	_, summary = task.ParseTitleDueDate(summary) // strip {date} format
	cleaned, notBefore, notBeforeRaw, upNext, noSplit, _ := task.ExtractConstraints(summary, time.Now(), nil)
	nbVal := notBeforeRaw
	if nbVal == "" && notBefore != nil {
		nbVal = notBefore.Format("2006-01-02")
	}
	mods := task.BuildModifiers("", nbVal, upNext, noSplit)
	result := cleaned
	if mods != "" {
		result += " " + mods
	}
	if est > 0 {
		result = task.SetTitleEstimate(result, est)
	}
	return c.UpdateSummary(ctx, issueKey, result)
}

// GetPriority fetches the priority level for the specified issue.
func (c *Client) GetPriority(ctx context.Context, issueKey string) (int, error) {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return 0, err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/issues/%s", pid, issueKey), nil)
	if err != nil {
		return 0, fmt.Errorf("get priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("kendo get priority: status %d: %s", resp.StatusCode, string(body))
	}

	var issue issueJSON
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return 0, fmt.Errorf("decode issue: %w", err)
	}

	return kendoPriorityToFylla(issue.Priority), nil
}

// UpdatePriority sets the priority for the specified issue.
func (c *Client) UpdatePriority(ctx context.Context, issueKey string, priority int) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}
	return c.putIssue(ctx, pid, issueKey, map[string]interface{}{
		"priority": fyllaPriorityToKendo(priority),
	})
}

// GetSummary fetches the title of the specified issue.
func (c *Client) GetSummary(ctx context.Context, issueKey string) (string, error) {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return "", err
	}

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/projects/%d/issues/%s", pid, issueKey), nil)
	if err != nil {
		return "", fmt.Errorf("get summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("kendo get summary: status %d: %s", resp.StatusCode, string(body))
	}

	var issue issueJSON
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return "", fmt.Errorf("decode issue: %w", err)
	}
	return issue.Title, nil
}

// UpdateSummary sets the title for the specified issue.
func (c *Client) UpdateSummary(ctx context.Context, issueKey string, summary string) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}
	return c.putIssue(ctx, pid, issueKey, map[string]interface{}{
		"title": summary,
	})
}

// FetchWorklogs retrieves time entries for the current user in the given date range.
func (c *Client) FetchWorklogs(ctx context.Context, since, until time.Time) ([]jira.WorklogEntry, error) {
	if err := c.fetchUserID(ctx); err != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", err)
	}
	if err := c.loadProjects(ctx); err != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", err)
	}

	sinceDate := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	untilDate := time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, until.Location())

	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/time-entries?start_date=%s&end_date=%s",
		since.Format("2006-01-02"), until.Format("2006-01-02")), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch time entries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kendo fetch time entries: status %d: %s", resp.StatusCode, string(body))
	}

	var entries []timeEntryJSON
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode time entries: %w", err)
	}

	var allEntries []jira.WorklogEntry
	for _, te := range entries {
		started, err := time.Parse(time.RFC3339, te.StartedAt)
		if err != nil {
			continue
		}
		if started.Before(sinceDate) || started.After(untilDate) {
			continue
		}

		allEntries = append(allEntries, jira.WorklogEntry{
			ID:           strconv.Itoa(te.ID),
			IssueKey:     te.IssueKey,
			Provider:     "kendo",
			Project:      c.projectCodeByID(te.ProjectID),
			IssueSummary: te.IssueTitle,
			Description:  te.Note,
			Started:      started,
			TimeSpent:    time.Duration(te.MinutesSpent) * time.Minute,
		})
	}

	return allEntries, nil
}

// UpdateWorklog updates an existing time entry.
func (c *Client) UpdateWorklog(ctx context.Context, issueKey, worklogID string, timeSpent time.Duration, description string, started time.Time) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"minutes_spent": int(timeSpent.Minutes()),
		"note":          description,
		"started_at":    started.Format(time.RFC3339),
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/api/projects/%d/issues/%s/time-entries/%s", pid, issueKey, worklogID), payload)
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
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}

	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/projects/%d/issues/%s/time-entries/%s", pid, issueKey, worklogID), nil)
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

// ListProjects returns the codes of all accessible Kendo projects.
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	keys := make([]string, len(c.projects))
	for i, p := range c.projects {
		keys[i] = p.Code
	}
	return keys, nil
}

var priorityNameToLevel = map[string]int{
	"urgent":  1,
	"high":    2,
	"medium":  3,
	"low":     4,
	"trivial": 5,
}
