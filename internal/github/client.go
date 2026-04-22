package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	gh "github.com/google/go-github/v68/github"
	"github.com/iruoy/fylla/internal/task"
	"golang.org/x/sync/errgroup"
)

var issueKeyPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)

// splitQueries splits a newline-separated query blob into individual non-empty queries.
func splitQueries(blob string) []string {
	if blob == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(blob, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ErrUnsupported is returned for operations GitHub doesn't model natively (estimates, priority, due dates, worklog).
var ErrUnsupported = errors.New("operation not supported for GitHub issues/pull requests")

// Client handles fetching issues and pull requests from GitHub.
type Client struct {
	client *gh.Client
	query  string
	Repos  []string // optional: narrows search and enables short-name key resolution (e.g. "owner/repo")

	rateMu    sync.Mutex
	rateLimit gh.Rate // last known rate limit state
}

// NewClient creates a GitHub client with the given personal access token.
func NewClient(token string) *Client {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return &Client{
		client: gh.NewClient(httpClient).WithAuthToken(token),
	}
}

// checkRateLimit pauses if we're close to the rate limit.
// Should be called before any API request.
func (c *Client) checkRateLimit() {
	c.rateMu.Lock()
	rate := c.rateLimit
	c.rateMu.Unlock()

	if rate.Remaining > 0 && rate.Remaining < 50 {
		sleepUntil := rate.Reset.Time.Sub(time.Now())
		if sleepUntil > 0 && sleepUntil < 5*time.Minute {
			time.Sleep(sleepUntil)
		}
	}
}

// updateRateLimit records the rate limit info from a response.
func (c *Client) updateRateLimit(resp *gh.Response) {
	if resp == nil {
		return
	}
	c.rateMu.Lock()
	c.rateLimit = resp.Rate
	c.rateMu.Unlock()
}

// RateRemaining returns the number of API requests remaining.
func (c *Client) RateRemaining() int {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	return c.rateLimit.Remaining
}

// FetchTasks searches for open issues and PRs matching the query and returns them as tasks.
// Multiple queries may be passed as newline-separated strings; results are merged and deduped by key.
func (c *Client) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	queries := splitQueries(query)
	if len(queries) == 0 {
		queries = []string{"is:pr state:open review-requested:@me"}
	}

	var mu sync.Mutex
	seen := make(map[int64]struct{})
	var allIssues []*gh.Issue

	g, gctx := errgroup.WithContext(ctx)
	for _, q := range queries {
		q := q
		// When repos are configured, add repo: qualifiers to narrow the search.
		if len(c.Repos) > 0 {
			for _, r := range c.Repos {
				q += " repo:" + r
			}
		}
		g.Go(func() error {
			opts := &gh.SearchOptions{ListOptions: gh.ListOptions{PerPage: 50}}
			for {
				c.checkRateLimit()
				result, resp, err := c.client.Search.Issues(gctx, q, opts)
				c.updateRateLimit(resp)
				if err != nil {
					return fmt.Errorf("github search: %w", err)
				}
				mu.Lock()
				for _, issue := range result.Issues {
					id := issue.GetID()
					if _, ok := seen[id]; ok {
						continue
					}
					seen[id] = struct{}{}
					allIssues = append(allIssues, issue)
				}
				mu.Unlock()
				if resp.NextPage == 0 {
					return nil
				}
				opts.Page = resp.NextPage
			}
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Flat estimate avoids an extra API call per item.
	const flatEstimate = 30 * time.Minute

	tasks := make([]task.Task, 0, len(allIssues))
	for _, issue := range allIssues {
		owner, repo, number, err := parseIssue(issue)
		if err != nil {
			continue
		}
		issueType := "Issue"
		if issue.PullRequestLinks != nil {
			issueType = "Pull Request"
		}
		summary := issue.GetTitle()
		est, summary := task.ParseTitleEstimate(summary)
		due, summary := task.ParseTitleDueDate(summary)
		pri, summary := task.ParseTitlePriority(summary)
		priority := 2
		if pri > 0 {
			priority = pri
		}
		estimate := flatEstimate
		if est > 0 {
			estimate = est
		}
		key := fmt.Sprintf("%s#%d", repo, number)
		tasks = append(tasks, task.Task{
			Key:               key,
			Provider:          "github",
			Summary:           summary,
			Priority:          priority,
			DueDate:           due,
			Created:           issue.GetCreatedAt().Time,
			Project:           fmt.Sprintf("%s/%s", owner, repo),
			IssueType:         issueType,
			OriginalEstimate:  estimate,
			RemainingEstimate: estimate,
		})
	}

	return tasks, nil
}

func parseIssue(issue *gh.Issue) (owner, repo string, number int, err error) {
	number = issue.GetNumber()
	if issue.Repository == nil {
		// Parse from repository URL
		if issue.GetRepositoryURL() == "" {
			return "", "", 0, fmt.Errorf("no repository info")
		}
		// URL format: https://api.github.com/repos/{owner}/{repo}
		url := issue.GetRepositoryURL()
		var o, r string
		if _, scanErr := fmt.Sscanf(extractRepoPath(url), "%s %s", &o, &r); scanErr != nil {
			return "", "", 0, fmt.Errorf("parse repository URL: %w", scanErr)
		}
		return o, r, number, nil
	}
	return issue.Repository.GetOwner().GetLogin(), issue.Repository.GetName(), number, nil
}

func extractRepoPath(url string) string {
	// https://api.github.com/repos/{owner}/{repo}
	const prefix = "/repos/"
	for i := 0; i <= len(url)-len(prefix); i++ {
		if url[i:i+len(prefix)] == prefix {
			rest := url[i+len(prefix):]
			// Find the slash between owner and repo
			for j := 0; j < len(rest); j++ {
				if rest[j] == '/' {
					return rest[:j] + " " + rest[j+1:]
				}
			}
		}
	}
	return ""
}

// ResolveIssueKey fetches the PR identified by prKey (e.g. "repo#123") and
// extracts an issue key (e.g. PROJ-123) from its branch name or body.
// Returns empty string when no key is found.
func (c *Client) ResolveIssueKey(ctx context.Context, prKey string) (string, error) {
	owner, repo, number, err := parsePRKey(prKey, c.Repos)
	if err != nil {
		return "", fmt.Errorf("parse PR key %q: %w", prKey, err)
	}

	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return "", fmt.Errorf("fetch PR %s: %w", prKey, err)
	}

	// Search branch name first, then title, then body.
	if key := issueKeyPattern.FindString(pr.GetHead().GetRef()); key != "" {
		return key, nil
	}
	if key := issueKeyPattern.FindString(pr.GetTitle()); key != "" {
		return key, nil
	}
	if key := issueKeyPattern.FindString(pr.GetBody()); key != "" {
		return key, nil
	}
	return "", nil
}

// splitProject returns owner, repo. Accepts "owner/repo" directly, otherwise
// resolves the short name via the configured repos list.
func splitProject(project string, repos []string) (string, string, error) {
	if strings.Contains(project, "/") {
		parts := strings.SplitN(project, "/", 2)
		if parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid project %q, want owner/repo", project)
		}
		return parts[0], parts[1], nil
	}
	for _, r := range repos {
		parts := strings.SplitN(r, "/", 2)
		if len(parts) == 2 && parts[1] == project {
			return parts[0], parts[1], nil
		}
	}
	return "", "", fmt.Errorf("repo %q not found in configured repos (use 'owner/repo' form or add to github.repos)", project)
}

// parsePRKey splits "repo#123" or "owner/repo#123" into owner, repo, number.
func parsePRKey(key string, repos []string) (string, string, int, error) {
	idx := strings.LastIndex(key, "#")
	if idx < 0 {
		return "", "", 0, fmt.Errorf("missing '#' in key")
	}
	projectPart := key[:idx]
	num, err := strconv.Atoi(key[idx+1:])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid issue number: %w", err)
	}

	owner, repo, err := splitProject(projectPart, repos)
	if err != nil {
		return "", "", 0, err
	}
	return owner, repo, num, nil
}

func (c *Client) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	owner, repo, err := splitProject(input.Project, c.Repos)
	if err != nil {
		return "", fmt.Errorf("github create issue: %w", err)
	}
	title := input.Summary
	if input.Estimate > 0 {
		title = task.SetTitleEstimate(title, input.Estimate)
	}
	if input.DueDate != nil {
		title = task.SetTitleDueDate(title, *input.DueDate)
	}
	if level := task.PriorityNameToLevel(input.Priority); level > 0 {
		title = task.SetTitlePriority(title, level)
	}
	req := &gh.IssueRequest{Title: &title}
	if input.Description != "" {
		req.Body = &input.Description
	}
	issue, _, err := c.client.Issues.Create(ctx, owner, repo, req)
	if err != nil {
		return "", fmt.Errorf("github create issue: %w", err)
	}
	return fmt.Sprintf("%s#%d", repo, issue.GetNumber()), nil
}

// ListProjects returns the short repo names from the configured repos list.
func (c *Client) ListProjects(_ context.Context) ([]string, error) {
	projects := make([]string, 0, len(c.Repos))
	for _, r := range c.Repos {
		parts := strings.SplitN(r, "/", 2)
		if len(parts) == 2 {
			projects = append(projects, parts[1])
		}
	}
	return projects, nil
}

// closeIssue closes an issue or PR with the given state_reason ("completed" or "not_planned").
func (c *Client) closeIssue(ctx context.Context, taskKey, reason string) error {
	owner, repo, number, err := parsePRKey(taskKey, c.Repos)
	if err != nil {
		return fmt.Errorf("parse key %q: %w", taskKey, err)
	}
	state := "closed"
	req := &gh.IssueRequest{State: &state, StateReason: &reason}
	c.checkRateLimit()
	_, resp, err := c.client.Issues.Edit(ctx, owner, repo, number, req)
	c.updateRateLimit(resp)
	if err != nil {
		return fmt.Errorf("github close %s: %w", taskKey, err)
	}
	return nil
}

func (c *Client) CompleteTask(ctx context.Context, taskKey string) error {
	return c.closeIssue(ctx, taskKey, "completed")
}

func (c *Client) DeleteTask(ctx context.Context, taskKey string) error {
	return c.closeIssue(ctx, taskKey, "not_planned")
}

func (c *Client) PostWorklog(_ context.Context, _ string, _ time.Duration, _ string, _ time.Time) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

// issueHandle identifies a GitHub issue or PR for mutation.
type issueHandle struct {
	owner  string
	repo   string
	number int
}

// fetchTitle loads the current title and returns it with an issueHandle for follow-up writes.
func (c *Client) fetchTitle(ctx context.Context, taskKey string) (string, issueHandle, error) {
	owner, repo, number, err := parsePRKey(taskKey, c.Repos)
	if err != nil {
		return "", issueHandle{}, fmt.Errorf("parse key %q: %w", taskKey, err)
	}
	h := issueHandle{owner, repo, number}
	c.checkRateLimit()
	issue, resp, err := c.client.Issues.Get(ctx, owner, repo, number)
	c.updateRateLimit(resp)
	if err != nil {
		return "", h, fmt.Errorf("github get %s: %w", taskKey, err)
	}
	return issue.GetTitle(), h, nil
}

// writeTitle PATCHes the issue title.
func (c *Client) writeTitle(ctx context.Context, taskKey string, h issueHandle, title string) error {
	req := &gh.IssueRequest{Title: &title}
	c.checkRateLimit()
	_, resp, err := c.client.Issues.Edit(ctx, h.owner, h.repo, h.number, req)
	c.updateRateLimit(resp)
	if err != nil {
		return fmt.Errorf("github update %s: %w", taskKey, err)
	}
	return nil
}

func (c *Client) GetEstimate(ctx context.Context, taskKey string) (time.Duration, error) {
	title, _, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return 0, err
	}
	d, _ := task.ParseTitleEstimate(title)
	return d, nil
}

func (c *Client) UpdateEstimate(ctx context.Context, taskKey string, d time.Duration) error {
	title, h, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return err
	}
	return c.writeTitle(ctx, taskKey, h, task.SetTitleEstimate(title, d))
}

func (c *Client) GetDueDate(ctx context.Context, taskKey string) (*time.Time, error) {
	title, _, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return nil, err
	}
	due, _ := task.ParseTitleDueDate(title)
	return due, nil
}

func (c *Client) UpdateDueDate(ctx context.Context, taskKey string, due time.Time) error {
	title, h, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return err
	}
	return c.writeTitle(ctx, taskKey, h, task.SetTitleDueDate(title, due))
}

func (c *Client) RemoveDueDate(ctx context.Context, taskKey string) error {
	title, h, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return err
	}
	return c.writeTitle(ctx, taskKey, h, task.RemoveTitleDueDate(title))
}

func (c *Client) GetPriority(ctx context.Context, taskKey string) (int, error) {
	title, _, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return 0, err
	}
	level, _ := task.ParseTitlePriority(title)
	if level == 0 {
		return 2, nil // High default
	}
	return level, nil
}

func (c *Client) UpdatePriority(ctx context.Context, taskKey string, level int) error {
	if level < 1 || level > 5 {
		return fmt.Errorf("github: priority must be 1..5, got %d", level)
	}
	title, h, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return err
	}
	return c.writeTitle(ctx, taskKey, h, task.SetTitlePriority(title, level))
}

// GetSummary returns the issue title with estimate/due/priority clauses stripped.
func (c *Client) GetSummary(ctx context.Context, taskKey string) (string, error) {
	title, _, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return "", err
	}
	_, title = task.ParseTitleEstimate(title)
	_, title = task.ParseTitleDueDate(title)
	_, title = task.ParseTitlePriority(title)
	return title, nil
}

// UpdateSummary rewrites the title while preserving existing estimate, due date, and priority clauses.
func (c *Client) UpdateSummary(ctx context.Context, taskKey string, summary string) error {
	title, h, err := c.fetchTitle(ctx, taskKey)
	if err != nil {
		return err
	}
	est, _ := task.ParseTitleEstimate(title)
	due, _ := task.ParseTitleDueDate(title)
	pri, _ := task.ParseTitlePriority(title)
	newTitle := summary
	if est > 0 {
		newTitle = task.SetTitleEstimate(newTitle, est)
	}
	if due != nil {
		newTitle = task.SetTitleDueDate(newTitle, *due)
	}
	if pri > 0 {
		newTitle = task.SetTitlePriority(newTitle, pri)
	}
	return c.writeTitle(ctx, taskKey, h, newTitle)
}

// SetHTTPClient sets the underlying HTTP client (for testing).
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.client = gh.NewClient(httpClient)
}

// SetBaseURL sets the base URL for API requests (for testing).
func (c *Client) SetBaseURL(url string) {
	c.client.BaseURL, _ = c.client.BaseURL.Parse(url)
}
