package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

// Client handles communication with the Jira REST API.
type Client struct {
	BaseURL         string
	Email           string
	Token           string
	HTTPClient      *http.Client
	AccountID       string
	DoneTransitions map[string]string
}

// NewClient creates a Jira client with the given credentials.
func NewClient(baseURL, email, token string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Email:      email,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
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
	req.SetBasicAuth(c.Email, c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// searchResponse represents the Jira search API response.
type searchResponse struct {
	Issues        []issueJSON `json:"issues"`
	NextPageToken string      `json:"nextPageToken"`
}

type issueJSON struct {
	Key    string     `json:"key"`
	Fields fieldsJSON `json:"fields"`
}

type fieldsJSON struct {
	Summary      string            `json:"summary"`
	Priority     *priorityJSON     `json:"priority"`
	DueDate      string            `json:"duedate"`
	IssueType    issueTypeJSON     `json:"issuetype"`
	Created      string            `json:"created"`
	Project      projectJSON       `json:"project"`
	TimeTracking *timeTrackingJSON `json:"timetracking"`
	Parent       *parentJSON       `json:"parent"`
	Status       *statusJSON       `json:"status"`
}

type statusJSON struct {
	Name string `json:"name"`
}

type parentJSON struct {
	Key    string           `json:"key"`
	Fields parentFieldsJSON `json:"fields"`
}

type parentFieldsJSON struct {
	Summary string `json:"summary"`
}

type priorityJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type issueTypeJSON struct {
	Name string `json:"name"`
}

type projectJSON struct {
	Key string `json:"key"`
}

type timeTrackingJSON struct {
	OriginalEstimateSeconds  int `json:"originalEstimateSeconds"`
	RemainingEstimateSeconds int `json:"remainingEstimateSeconds"`
}

// priorityNameToLevel maps Jira priority names to numeric levels.
var priorityNameToLevel = map[string]int{
	"Highest": 1,
	"High":    2,
	"Medium":  3,
	"Low":     4,
	"Lowest":  5,
}

// priorityLevelToName maps numeric levels back to Jira priority names.
var priorityLevelToName = map[int]string{
	1: "Highest",
	2: "High",
	3: "Medium",
	4: "Low",
	5: "Lowest",
}

func parseIssue(issue issueJSON) task.Task {
	t := task.Task{
		Key:       issue.Key,
		Provider:  "jira",
		Summary:   issue.Fields.Summary,
		Priority:  3, // default Medium
		IssueType: issue.Fields.IssueType.Name,
		Project:   issue.Fields.Project.Key,
	}

	if issue.Fields.Status != nil {
		t.Status = issue.Fields.Status.Name
	}

	if issue.Fields.Parent != nil && issue.Fields.Parent.Fields.Summary != "" {
		t.Section = issue.Fields.Parent.Fields.Summary
	}

	if issue.Fields.Priority != nil {
		if level, ok := priorityNameToLevel[issue.Fields.Priority.Name]; ok {
			t.Priority = level
		}
	}

	if issue.Fields.DueDate != "" {
		if d, err := time.Parse("2006-01-02", issue.Fields.DueDate); err == nil {
			t.DueDate = &d
		}
	}

	if issue.Fields.Created != "" {
		if c, err := time.Parse("2006-01-02T15:04:05.000-0700", issue.Fields.Created); err == nil {
			t.Created = c
		}
	}

	if tt := issue.Fields.TimeTracking; tt != nil {
		t.OriginalEstimate = time.Duration(tt.OriginalEstimateSeconds) * time.Second
		t.RemainingEstimate = time.Duration(tt.RemainingEstimateSeconds) * time.Second
	}

	// Fallback: parse estimate and due date from title brackets
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

	// Extract recurrence from summary (title-encoded)
	cleanedRec, rec := task.ExtractRecurrence(t.Summary)
	if rec != nil {
		t.Summary = cleanedRec
		t.Recurrence = rec
	}

	// Extract scheduling constraints from summary
	cleaned, notBefore, notBeforeRaw, upNext, noSplit := task.ExtractConstraints(t.Summary, time.Now(), t.DueDate)
	t.Summary = cleaned
	t.NotBefore = notBefore
	t.NotBeforeRaw = notBeforeRaw
	t.UpNext = upNext
	t.NoSplit = noSplit

	return t
}

// FetchTasks retrieves issues from Jira matching the given JQL query.
func (c *Client) FetchTasks(ctx context.Context, jql string) ([]task.Task, error) {
	var tasks []task.Task
	var nextPageToken string

	for {
		payload := map[string]interface{}{
			"jql":    jql,
			"fields": []string{"summary", "priority", "duedate", "issuetype", "created", "project", "timetracking", "parent", "status"},
		}
		if nextPageToken != "" {
			payload["nextPageToken"] = nextPageToken
		}

		resp, err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", payload)
		if err != nil {
			return nil, fmt.Errorf("fetch tasks: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("jira search: status %d: %s", resp.StatusCode, string(body))
		}

		var result searchResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode search response: %w", err)
		}
		resp.Body.Close()

		for _, issue := range result.Issues {
			tasks = append(tasks, parseIssue(issue))
		}

		if result.NextPageToken == "" {
			break
		}
		nextPageToken = result.NextPageToken
	}

	return tasks, nil
}

// PostWorklog adds a worklog entry to the specified Jira issue.
func (c *Client) PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error {
	payload := map[string]interface{}{
		"timeSpentSeconds": int(timeSpent.Seconds()),
		"started":          started.Format("2006-01-02T15:04:05.000-0700"),
		"comment": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": description,
						},
					},
				},
			},
		},
	}

	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey), payload)
	if err != nil {
		return fmt.Errorf("post worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira worklog: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UpdateEstimate sets the remaining estimate for the specified Jira issue.
func (c *Client) UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"timetracking": map[string]string{
				"originalEstimate":  formatDuration(remaining),
				"remainingEstimate": formatDuration(remaining),
			},
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
	if err != nil {
		return fmt.Errorf("update estimate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update estimate: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// createIssueResponse represents the Jira create issue API response.
type createIssueResponse struct {
	Key string `json:"key"`
}

func (c *Client) fetchAccountID(ctx context.Context) error {
	if c.AccountID != "" {
		return nil
	}

	resp, err := c.do(ctx, http.MethodGet, "/rest/api/3/myself", nil)
	if err != nil {
		return fmt.Errorf("fetch account ID: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira myself: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode myself response: %w", err)
	}
	c.AccountID = result.AccountID
	return nil
}

// CreateTask creates a new issue in Jira and returns the issue key.
func (c *Client) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	fields := map[string]interface{}{
		"project":   map[string]string{"key": input.Project},
		"issuetype": map[string]string{"name": input.IssueType},
		"summary":   input.Summary,
	}

	if err := c.fetchAccountID(ctx); err == nil && c.AccountID != "" {
		fields["assignee"] = map[string]string{"accountId": c.AccountID}
	}

	if input.Description != "" {
		fields["description"] = map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": input.Description,
						},
					},
				},
			},
		}
	}

	if input.Estimate > 0 {
		fields["timetracking"] = map[string]string{
			"originalEstimate": formatDuration(input.Estimate),
		}
	}

	if input.DueDate != nil {
		fields["duedate"] = input.DueDate.Format("2006-01-02")
	}

	if input.Priority != "" {
		fields["priority"] = map[string]string{"name": input.Priority}
	}

	if input.Parent != "" {
		fields["parent"] = map[string]string{"key": input.Parent}
	}

	payload := map[string]interface{}{"fields": fields}

	resp, err := c.do(ctx, http.MethodPost, "/rest/api/3/issue", payload)
	if err != nil {
		return "", fmt.Errorf("create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jira create issue: status %d: %s", resp.StatusCode, string(body))
	}

	var result createIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create response: %w", err)
	}
	return result.Key, nil
}

// getIssueResponse represents a single Jira issue response for field queries.
type getIssueResponse struct {
	Fields struct {
		TimeTracking *timeTrackingJSON `json:"timetracking"`
	} `json:"fields"`
}

// GetEstimate fetches the remaining estimate for the specified Jira issue.
func (c *Client) GetEstimate(ctx context.Context, issueKey string) (time.Duration, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s?fields=timetracking", issueKey), nil)
	if err != nil {
		return 0, fmt.Errorf("get estimate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("jira get estimate: status %d: %s", resp.StatusCode, string(body))
	}

	var result getIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode issue response: %w", err)
	}

	if result.Fields.TimeTracking == nil {
		return 0, nil
	}
	return time.Duration(result.Fields.TimeTracking.RemainingEstimateSeconds) * time.Second, nil
}

// UpdateDueDate sets the due date for the specified Jira issue.
func (c *Client) UpdateDueDate(ctx context.Context, issueKey string, dueDate time.Time) error {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"duedate": dueDate.Format("2006-01-02"),
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
	if err != nil {
		return fmt.Errorf("update due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update due date: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// getDueDateResponse represents a single Jira issue response for the duedate field.
type getDueDateResponse struct {
	Fields struct {
		DueDate string `json:"duedate"`
	} `json:"fields"`
}

// GetDueDate fetches the due date for the specified Jira issue.
func (c *Client) GetDueDate(ctx context.Context, issueKey string) (*time.Time, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s?fields=duedate", issueKey), nil)
	if err != nil {
		return nil, fmt.Errorf("get due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira get due date: status %d: %s", resp.StatusCode, string(body))
	}

	var result getDueDateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode issue response: %w", err)
	}

	if result.Fields.DueDate == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", result.Fields.DueDate)
	if err != nil {
		return nil, fmt.Errorf("parse due date: %w", err)
	}
	return &d, nil
}

// getPriorityResponse represents a single Jira issue response for the priority field.
type getPriorityResponse struct {
	Fields struct {
		Priority *priorityJSON `json:"priority"`
	} `json:"fields"`
}

// GetPriority fetches the priority level for the specified Jira issue.
func (c *Client) GetPriority(ctx context.Context, issueKey string) (int, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s?fields=priority", issueKey), nil)
	if err != nil {
		return 0, fmt.Errorf("get priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("jira get priority: status %d: %s", resp.StatusCode, string(body))
	}

	var result getPriorityResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode issue response: %w", err)
	}

	if result.Fields.Priority == nil {
		return 3, nil // default Medium
	}
	if level, ok := priorityNameToLevel[result.Fields.Priority.Name]; ok {
		return level, nil
	}
	return 3, nil
}

// UpdatePriority sets the priority for the specified Jira issue.
// A priority of 0 clears the priority field.
func (c *Client) UpdatePriority(ctx context.Context, issueKey string, priority int) error {
	var priorityVal interface{}
	if priority == 0 {
		priorityVal = nil
	} else {
		name, ok := priorityLevelToName[priority]
		if !ok {
			return fmt.Errorf("invalid priority level %d (must be 1-5)", priority)
		}
		priorityVal = map[string]string{"name": name}
	}

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"priority": priorityVal,
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
	if err != nil {
		return fmt.Errorf("update priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update priority: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// RemoveDueDate clears the due date for the specified Jira issue.
func (c *Client) RemoveDueDate(ctx context.Context, issueKey string) error {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"duedate": nil,
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
	if err != nil {
		return fmt.Errorf("remove due date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira remove due date: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// getSummaryResponse represents a single Jira issue response for the summary field.
type getSummaryResponse struct {
	Fields struct {
		Summary string `json:"summary"`
	} `json:"fields"`
}

// GetSummary fetches the raw summary for the specified Jira issue.
func (c *Client) GetSummary(ctx context.Context, issueKey string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s?fields=summary", issueKey), nil)
	if err != nil {
		return "", fmt.Errorf("get summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jira get summary: status %d: %s", resp.StatusCode, string(body))
	}

	var result getSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode issue response: %w", err)
	}
	return result.Fields.Summary, nil
}

// UpdateSummary sets the summary for the specified Jira issue.
func (c *Client) UpdateSummary(ctx context.Context, issueKey string, summary string) error {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"summary": summary,
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
	if err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update summary: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteTask permanently deletes a Jira issue.
func (c *Client) DeleteTask(ctx context.Context, issueKey string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), nil)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira delete issue: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// transitionsResponse represents the Jira transitions API response.
type transitionsResponse struct {
	Transitions []transition `json:"transitions"`
}

type transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// projectKey extracts the project prefix from an issue key (e.g. "GIC-564" → "GIC").
func projectKey(issueKey string) string {
	if i := strings.IndexByte(issueKey, '-'); i > 0 {
		return issueKey[:i]
	}
	return issueKey
}

// doneTransitionName returns the configured "done" transition name for the
// given issue key's project, falling back to "Done".
func (c *Client) doneTransitionName(issueKey string) string {
	if c.DoneTransitions != nil {
		if name, ok := c.DoneTransitions[projectKey(issueKey)]; ok {
			return name
		}
	}
	return "Done"
}

// ListTransitions returns the names of available transitions for a Jira issue.
func (c *Client) ListTransitions(ctx context.Context, issueKey string) ([]string, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), nil)
	if err != nil {
		return nil, fmt.Errorf("get transitions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira transitions: status %d: %s", resp.StatusCode, string(body))
	}

	var result transitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode transitions: %w", err)
	}

	names := make([]string, len(result.Transitions))
	for i, t := range result.Transitions {
		names[i] = t.Name
	}
	return names, nil
}

// TransitionTask transitions a Jira issue to the named target status.
func (c *Client) TransitionTask(ctx context.Context, issueKey, target string) error {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), nil)
	if err != nil {
		return fmt.Errorf("get transitions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira transitions: status %d: %s", resp.StatusCode, string(body))
	}

	var result transitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode transitions: %w", err)
	}

	var transitionID string
	var names []string
	for _, t := range result.Transitions {
		names = append(names, t.Name)
		if strings.EqualFold(t.Name, target) {
			transitionID = t.ID
			break
		}
	}

	if transitionID == "" {
		return fmt.Errorf("no %q transition available for %s (available: %s)", target, issueKey, strings.Join(names, ", "))
	}

	payload := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}

	postResp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/rest/api/3/issue/%s/transitions", issueKey), payload)
	if err != nil {
		return fmt.Errorf("post transition: %w", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("jira transition: status %d: %s", postResp.StatusCode, string(body))
	}

	return nil
}

// CompleteTask transitions a Jira issue to "Done" status.
func (c *Client) CompleteTask(ctx context.Context, issueKey string) error {
	return c.TransitionTask(ctx, issueKey, c.doneTransitionName(issueKey))
}

// ListProjects returns the keys of all accessible Jira projects.
func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/rest/api/3/project", nil)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira list projects: status %d: %s", resp.StatusCode, string(body))
	}

	var projects []projectJSON
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("decode projects response: %w", err)
	}

	keys := make([]string, len(projects))
	for i, p := range projects {
		keys[i] = p.Key
	}
	return keys, nil
}

// Epic represents a Jira epic issue.
type Epic struct {
	Key     string
	Summary string
}

// ListEpics searches for open epics in Jira, optionally scoped to a project.
func (c *Client) ListEpics(ctx context.Context, project string) ([]Epic, error) {
	jql := "issuetype = Epic AND status != Done ORDER BY summary ASC"
	if project != "" {
		jql = fmt.Sprintf("project = %s AND issuetype = Epic AND status != Done ORDER BY summary ASC", project)
	}
	payload := map[string]interface{}{
		"jql":    jql,
		"fields": []string{"summary"},
	}

	resp, err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", payload)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira list epics: status %d: %s", resp.StatusCode, string(body))
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode epics response: %w", err)
	}

	epics := make([]Epic, len(result.Issues))
	for i, issue := range result.Issues {
		epics[i] = Epic{Key: issue.Key, Summary: issue.Fields.Summary}
	}
	return epics, nil
}

// UpdateParent sets or clears the parent issue for the specified Jira issue.
// An empty parentKey clears the parent field.
func (c *Client) UpdateParent(ctx context.Context, issueKey, parentKey string) error {
	var parentVal interface{}
	if parentKey == "" {
		parentVal = nil
	} else {
		parentVal = map[string]string{"key": parentKey}
	}
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"parent": parentVal,
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s", issueKey), payload)
	if err != nil {
		return fmt.Errorf("update parent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update parent: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetParent fetches the parent issue key for the specified Jira issue.
func (c *Client) GetParent(ctx context.Context, issueKey string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s?fields=parent", issueKey), nil)
	if err != nil {
		return "", fmt.Errorf("get parent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jira get parent: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Fields struct {
			Parent *struct {
				Key string `json:"key"`
			} `json:"parent"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode parent response: %w", err)
	}

	if result.Fields.Parent == nil {
		return "", nil
	}
	return result.Fields.Parent.Key, nil
}

// WorklogEntry represents a single worklog entry from Jira.
type WorklogEntry struct {
	ID           string
	IssueKey     string
	Provider     string
	IssueSummary string
	Description  string
	Started      time.Time
	TimeSpent    time.Duration
}

// FetchWorklogs retrieves the current user's worklogs in the given date range.
func (c *Client) FetchWorklogs(ctx context.Context, since, until time.Time) ([]WorklogEntry, error) {
	if err := c.fetchAccountID(ctx); err != nil {
		return nil, fmt.Errorf("fetch worklogs: %w", err)
	}

	jql := fmt.Sprintf(
		`worklogDate >= "%s" AND worklogDate <= "%s" AND worklogAuthor = currentUser()`,
		since.Format("2006-01-02"), until.Format("2006-01-02"),
	)

	payload := map[string]interface{}{
		"jql":    jql,
		"fields": []string{"summary"},
	}

	resp, err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", payload)
	if err != nil {
		return nil, fmt.Errorf("fetch worklogs search: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("jira worklog search: status %d: %s", resp.StatusCode, string(body))
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("decode worklog search: %w", err)
	}
	resp.Body.Close()

	type wlResult struct {
		entries []WorklogEntry
	}
	results := make([]wlResult, len(result.Issues))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for i, issue := range result.Issues {
		wg.Add(1)
		go func(idx int, key, summary string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			wls, err := c.fetchIssueWorklogs(ctx, key, summary, since, until)
			if err == nil {
				results[idx] = wlResult{entries: wls}
			}
		}(i, issue.Key, issue.Fields.Summary)
	}
	wg.Wait()

	var entries []WorklogEntry
	for _, r := range results {
		entries = append(entries, r.entries...)
	}
	return entries, nil
}

func (c *Client) fetchIssueWorklogs(ctx context.Context, issueKey, issueSummary string, since, until time.Time) ([]WorklogEntry, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Worklogs []struct {
			ID      string `json:"id"`
			Author  struct {
				AccountID string `json:"accountId"`
			} `json:"author"`
			Started          string `json:"started"`
			TimeSpentSeconds int    `json:"timeSpentSeconds"`
			Comment          interface{} `json:"comment"`
		} `json:"worklogs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	sinceDate := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	untilDate := time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, until.Location())

	var entries []WorklogEntry
	for _, wl := range result.Worklogs {
		if wl.Author.AccountID != c.AccountID {
			continue
		}
		started, err := time.Parse("2006-01-02T15:04:05.000-0700", wl.Started)
		if err != nil {
			continue
		}
		if started.Before(sinceDate) || started.After(untilDate) {
			continue
		}
		entries = append(entries, WorklogEntry{
			ID:           wl.ID,
			IssueKey:     issueKey,
			Provider:     "jira",
			IssueSummary: issueSummary,
			Description:  extractADFText(wl.Comment),
			Started:      started,
			TimeSpent:    time.Duration(wl.TimeSpentSeconds) * time.Second,
		})
	}
	return entries, nil
}

// extractADFText walks an ADF document and extracts plain text.
func extractADFText(comment interface{}) string {
	if comment == nil {
		return ""
	}
	doc, ok := comment.(map[string]interface{})
	if !ok {
		return ""
	}
	content, ok := doc["content"].([]interface{})
	if !ok {
		return ""
	}
	var parts []string
	for _, block := range content {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		innerContent, ok := blockMap["content"].([]interface{})
		if !ok {
			continue
		}
		for _, node := range innerContent {
			nodeMap, ok := node.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := nodeMap["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "")
}

// UpdateWorklog updates an existing worklog entry.
func (c *Client) UpdateWorklog(ctx context.Context, issueKey, worklogID string, timeSpent time.Duration, description string, started time.Time) error {
	payload := map[string]interface{}{
		"timeSpentSeconds": int(timeSpent.Seconds()),
		"started":          started.Format("2006-01-02T15:04:05.000-0700"),
		"comment": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []interface{}{
				map[string]interface{}{
					"type": "paragraph",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": description,
						},
					},
				},
			},
		},
	}

	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/rest/api/3/issue/%s/worklog/%s", issueKey, worklogID), payload)
	if err != nil {
		return fmt.Errorf("update worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update worklog: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteWorklog deletes a worklog entry.
func (c *Client) DeleteWorklog(ctx context.Context, issueKey, worklogID string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/rest/api/3/issue/%s/worklog/%s", issueKey, worklogID), nil)
	if err != nil {
		return fmt.Errorf("delete worklog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira delete worklog: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// formatDuration converts a time.Duration to Jira duration string (e.g. "4h", "2h 30m").
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
