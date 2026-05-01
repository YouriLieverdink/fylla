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

	"github.com/iruoy/fylla/internal/task"
)

// Client handles communication with the Kendo REST API.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	UserID     int
	DoneLane   string

	projectsMu   sync.Mutex
	projects     []project
	projectsDone bool

	userMu   sync.Mutex
	userDone bool

	lanesCache sync.Map // projectID (int) → map[int]string
	epicsCache sync.Map // projectID (int) → map[int]string
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

// SprintOption is an alias for the provider-neutral task.SprintOption type.
type SprintOption = task.SprintOption

// decodeTimeEntries decodes a /api/time-entries response, accepting either
// a bare JSON array or a Laravel-resource–style paginated envelope:
//
//	{"data": [...], "meta": {"current_page": 1, "last_page": 3}}
//	{"data": [...], "links": {"next": "..."}}
//
// hasEnvelope is true when the response uses an envelope (regardless of
// whether more pages remain). hasMore is meaningful only for envelope
// responses.
func decodeTimeEntries(body []byte) (entries []timeEntryJSON, hasMore bool, hasEnvelope bool, err error) {
	trimmed := body
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\n' || trimmed[0] == '\r' || trimmed[0] == '\t') {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, false, false, fmt.Errorf("decode time entries: %w", err)
		}
		return entries, false, false, nil
	}

	var env struct {
		Data  []timeEntryJSON `json:"data"`
		Meta  struct {
			CurrentPage int `json:"current_page"`
			LastPage    int `json:"last_page"`
		} `json:"meta"`
		Links struct {
			Next *string `json:"next"`
		} `json:"links"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, false, false, fmt.Errorf("decode time entries envelope: %w", err)
	}
	more := false
	if env.Links.Next != nil && *env.Links.Next != "" {
		more = true
	}
	if env.Meta.LastPage > 0 && env.Meta.CurrentPage > 0 && env.Meta.CurrentPage < env.Meta.LastPage {
		more = true
	}
	return env.Data, more, true, nil
}

type timeEntryJSON struct {
	ID           int    `json:"id"`
	UserID       int    `json:"user_id"`
	MinutesSpent int    `json:"minutes_spent"`
	Note         string `json:"note"`
	StartedAt    string `json:"started_at"`
	CreatedAt    string `json:"created_at"`
	IssueTitle   string `json:"issue_title"`
	IssueKey     string `json:"issue_key"`
	ProjectID    int    `json:"project_id"`
}

// effectiveStarted returns the canonical timestamp for a time entry,
// preferring started_at and falling back to created_at when started_at is
// null/empty (the Kendo API returns null for entries logged without an
// explicit start time).
func (te timeEntryJSON) effectiveStarted() string {
	if te.StartedAt != "" {
		return te.StartedAt
	}
	return te.CreatedAt
}

// maxRetries is the number of retry attempts for rate-limited requests.
const maxRetries = 3

func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	var resp *http.Response
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests || attempt == maxRetries {
			return resp, nil
		}

		// Rate limited — parse Retry-After header or use exponential backoff
		resp.Body.Close()
		wait := retryDelay(resp, attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return resp, nil
}

// retryDelay calculates how long to wait before retrying a 429 response.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	// Exponential backoff: 1s, 2s, 4s
	return time.Duration(1<<uint(attempt)) * time.Second
}

func (c *Client) loadProjects(ctx context.Context) error {
	c.projectsMu.Lock()
	defer c.projectsMu.Unlock()

	if c.projectsDone {
		return nil
	}

	resp, err := c.do(ctx, http.MethodGet, "/api/projects", nil)
	if err != nil {
		return fmt.Errorf("fetch projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kendo list projects: status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&c.projects); err != nil {
		return fmt.Errorf("decode projects: %w", err)
	}

	c.projectsDone = true
	return nil
}

func (c *Client) fetchUserID(ctx context.Context) error {
	c.userMu.Lock()
	defer c.userMu.Unlock()

	if c.userDone || c.UserID != 0 {
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
	c.userDone = true
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

// ProjectIDForKey returns the numeric project ID for a Kendo issue key.
func (c *Client) ProjectIDForKey(ctx context.Context, key string) (int, error) {
	return c.projectIDForKey(ctx, key)
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
		SprintID: issue.SprintID,
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

// isTextSearch returns true if the query is a free-text search term
// (not empty, not "*", not a filter query, and not a known project code/name).
func (c *Client) isTextSearch(query string) bool {
	if query == "" || query == "*" || isFilterQuery(query) {
		return false
	}
	for _, p := range c.projects {
		if strings.EqualFold(p.Code, query) || strings.EqualFold(p.Name, query) {
			return false
		}
	}
	if _, err := strconv.Atoi(query); err == nil {
		return false
	}
	return true
}

// issueMatchesText returns true if the issue key or title contains the search term (case-insensitive).
func issueMatchesText(issue issueJSON, search string) bool {
	lower := strings.ToLower(search)
	if strings.Contains(strings.ToLower(issue.Key), lower) {
		return true
	}
	if strings.Contains(strings.ToLower(issue.Title), lower) {
		return true
	}
	return false
}

// FetchTasks retrieves issues from Kendo projects.
//
// The query can be:
//   - Empty or "*": fetch all issues from all projects
//   - A project code/name: fetch all issues from that project
//   - Filter params (key=value pairs joined by &): fetch and filter issues
//     Supported filters: assignee_id=me, assignee_id=N, project_id=N, lane_id=N, priority=N
//   - Free text: search issues by key or title match
func (c *Client) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	if err := c.loadProjects(ctx); err != nil {
		return nil, err
	}

	// Default to current user's tasks when no query specified
	if query == "" || query == "*" {
		query = "assignee_id=me"
	}

	// Parse filter if query contains key=value pairs
	var filter issueFilter
	if isFilterQuery(query) {
		if err := c.fetchUserID(ctx); err != nil {
			return nil, err
		}
		filter = c.parseFilter(query, c.UserID)
	}

	textSearch := c.isTextSearch(query)

	// Determine which project IDs to fetch
	var projectIDs []int
	if filter.projectID != nil {
		projectIDs = append(projectIDs, *filter.projectID)
	} else if !isFilterQuery(query) && !textSearch && query != "" && query != "*" {
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

	// Fetch issues from all projects sequentially to avoid 429 rate limits.
	// Lanes and epics are cached per project after first fetch.
	var allTasks []task.Task
	for _, pid := range projectIDs {
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

		// Quick filter before fetching epics — skip project entirely if no issues match
		doneLaneID := c.doneLaneIDFromMap(laneMap)
		var matching []issueJSON
		for _, issue := range issues {
			if issue.LaneID == doneLaneID {
				continue
			}
			if !filter.empty() && !filter.matches(issue) {
				continue
			}
			if textSearch && !issueMatchesText(issue, query) {
				continue
			}
			matching = append(matching, issue)
		}
		if len(matching) == 0 {
			continue
		}

		// Only fetch epics if we have matching issues (avoids unnecessary API call)
		epicMap, err := c.fetchEpicMap(ctx, pid)
		if err != nil {
			return nil, err
		}

		code := c.projectCodeByID(pid)
		for _, issue := range matching {
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

	laneID, err := c.findLaneID(ctx, pid, input.Lane)
	if err != nil {
		return "", fmt.Errorf("resolve lane: %w", err)
	}

	issueType := 0 // feature
	if strings.EqualFold(input.IssueType, "Bug") {
		issueType = 1
	} else if strings.EqualFold(input.IssueType, "Task") {
		issueType = 2
	}

	title := input.Summary
	if input.DueDate != nil {
		if mods := task.BuildModifiers(input.DueDate.Format("2006-01-02"), "", false, false); mods != "" {
			title = strings.TrimSpace(title) + " " + mods
		}
	}

	payload := map[string]interface{}{
		"title":       title,
		"description": input.Description,
		"priority":    2, // default medium
		"type":        issueType,
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
	laneMap, err := c.fetchLaneMap(ctx, projectID)
	if err != nil {
		return 0, err
	}
	id := c.doneLaneIDFromMap(laneMap)
	if id == -1 {
		return 0, fmt.Errorf("no done lane found for project %d", projectID)
	}
	return id, nil
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

	laneMap, err := c.fetchLaneMap(ctx, pid)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(laneMap))
	for _, title := range laneMap {
		names = append(names, title)
	}
	return names, nil
}

// ListIssueTypes returns the available issue type names for Kendo.
func (c *Client) ListIssueTypes(_ context.Context, _ string) ([]string, error) {
	return []string{"Feature", "Bug", "Task"}, nil
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
	if cached, ok := c.epicsCache.Load(projectID); ok {
		return cached.(map[int]string), nil
	}

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

	c.epicsCache.Store(projectID, m)
	return m, nil
}

// ListEpics returns open epics for a Kendo project.
func (c *Client) ListEpics(ctx context.Context, project string) ([]task.Epic, error) {
	pid, err := c.projectIDForName(ctx, project)
	if err != nil {
		return nil, err
	}

	epicMap, err := c.fetchEpicMap(ctx, pid)
	if err != nil {
		return nil, err
	}
	epics := make([]task.Epic, 0, len(epicMap))
	for id, title := range epicMap {
		epics = append(epics, task.Epic{Key: strconv.Itoa(id), Summary: title})
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
	if cached, ok := c.lanesCache.Load(projectID); ok {
		return cached.(map[int]string), nil
	}

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

	c.lanesCache.Store(projectID, m)
	return m, nil
}

func (c *Client) findLaneID(ctx context.Context, projectID int, laneName string) (int, error) {
	laneMap, err := c.fetchLaneMap(ctx, projectID)
	if err != nil {
		return 0, err
	}
	for id, title := range laneMap {
		if strings.EqualFold(title, laneName) {
			return id, nil
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

// UpdateSprint moves a Kendo issue to the given sprint, or removes from sprint if nil.
func (c *Client) UpdateSprint(ctx context.Context, issueKey string, sprintID *int) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}
	overrides := map[string]interface{}{}
	if sprintID != nil {
		overrides["sprint_id"] = *sprintID
	} else {
		overrides["sprint_id"] = nil
	}
	return c.putIssue(ctx, pid, issueKey, overrides)
}

func (c *Client) resolveEpicID(ctx context.Context, projectID int, epicKey string) (int, error) {
	epicMap, err := c.fetchEpicMap(ctx, projectID)
	if err != nil {
		return 0, err
	}
	for id, title := range epicMap {
		if strconv.Itoa(id) == epicKey || strings.EqualFold(title, epicKey) {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no epic found for key %q", epicKey)
}

// GetParent returns the epic ID for the given Kendo issue.
func (c *Client) GetParent(ctx context.Context, issueKey string) (string, error) {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return "", err
	}
	issue, err := c.fetchIssue(ctx, pid, issueKey)
	if err != nil {
		return "", err
	}
	if issue.EpicID == nil {
		return "", nil
	}
	return strconv.Itoa(*issue.EpicID), nil
}

// UpdateParent sets or clears the epic for the given Kendo issue.
func (c *Client) UpdateParent(ctx context.Context, issueKey, parentKey string) error {
	pid, err := c.projectIDForKey(ctx, issueKey)
	if err != nil {
		return err
	}
	if parentKey == "" {
		return c.putIssue(ctx, pid, issueKey, map[string]interface{}{
			"epic_id": nil,
		})
	}
	epicID, err := c.resolveEpicID(ctx, pid, parentKey)
	if err != nil {
		return err
	}
	return c.putIssue(ctx, pid, issueKey, map[string]interface{}{
		"epic_id": epicID,
	})
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

	if now := time.Now(); started.After(now) {
		started = now
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

	issue, err := c.fetchIssue(ctx, pid, issueKey)
	if err != nil {
		return 0, err
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

	issue, err := c.fetchIssue(ctx, pid, issueKey)
	if err != nil {
		return 0, err
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

	issue, err := c.fetchIssue(ctx, pid, issueKey)
	if err != nil {
		return "", err
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

// FetchWorklogs retrieves time entries in the given date range.
// The filter narrows results by project (matched against project code) and
// user scope ("me" → current user only, "anyone" → all users).
// An empty UserScope is treated as "me" for backward compatibility.
func (c *Client) FetchWorklogs(ctx context.Context, since, until time.Time, filter task.WorklogFilter) ([]task.WorklogEntry, error) {
	if err := c.fetchUserID(ctx); err != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", err)
	}
	if err := c.loadProjects(ctx); err != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", err)
	}

	sinceDate := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	untilDate := time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, until.Location())

	var projectID int
	if filter.Project != "" {
		for _, p := range c.projects {
			if strings.EqualFold(p.Code, filter.Project) {
				projectID = p.ID
				break
			}
		}
		if projectID == 0 {
			return nil, fmt.Errorf("kendo fetch worklogs: unknown project code %q", filter.Project)
		}
	}

	scopeMe := filter.UserScope != "anyone"

	basePath := fmt.Sprintf("/api/time-entries?start_date=%s&end_date=%s",
		since.Format("2006-01-02"), until.Format("2006-01-02"))
	if scopeMe {
		basePath += fmt.Sprintf("&user_id=%d", c.UserID)
	}
	if projectID != 0 {
		basePath += fmt.Sprintf("&project_id=%d", projectID)
	}

	const perPage = 200
	var entries []timeEntryJSON
	seen := make(map[int]bool)
	for page := 1; page <= 50; page++ {
		path := fmt.Sprintf("%s&page=%d&per_page=%d", basePath, page, perPage)
		resp, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("fetch time entries: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read time entries: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("kendo fetch time entries: status %d: %s", resp.StatusCode, string(body))
		}
		pageEntries, hasMore, hasEnvelope, err := decodeTimeEntries(body)
		if err != nil {
			return nil, err
		}
		// Stop if the page is empty.
		if len(pageEntries) == 0 {
			break
		}
		// Dedupe by ID; if every entry is already seen the server is ignoring
		// the page param and serving the same window — bail out.
		newCount := 0
		for _, te := range pageEntries {
			if seen[te.ID] {
				continue
			}
			seen[te.ID] = true
			entries = append(entries, te)
			newCount++
		}
		if newCount == 0 {
			break
		}
		if hasEnvelope && !hasMore {
			break
		}
	}

	var allEntries []task.WorklogEntry
	for _, te := range entries {
		if scopeMe && te.UserID != c.UserID {
			continue
		}
		if projectID != 0 && te.ProjectID != projectID {
			continue
		}
		started, err := time.Parse(time.RFC3339, te.effectiveStarted())
		if err != nil {
			continue
		}
		if started.Before(sinceDate) || started.After(untilDate) {
			continue
		}

		allEntries = append(allEntries, task.WorklogEntry{
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
