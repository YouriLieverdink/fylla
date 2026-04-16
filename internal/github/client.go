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
)

var issueKeyPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)

// ErrUnsupported is returned for write operations that don't apply to PRs.
var ErrUnsupported = errors.New("operation not supported for GitHub pull requests")

// Client handles fetching pull requests needing review from GitHub.
type Client struct {
	client *gh.Client
	query  string
	Repos  []string // optional: only include PRs from these repos (e.g. "owner/repo")

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

// FetchTasks searches for open PRs requesting the user's review and returns them as tasks.
func (c *Client) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	if query == "" {
		query = "is:pr state:open review-requested:@me"
	}

	// When repos are configured, add repo: qualifiers to narrow the search.
	if len(c.Repos) > 0 {
		for _, r := range c.Repos {
			query += " repo:" + r
		}
	}

	var allIssues []*gh.Issue
	opts := &gh.SearchOptions{ListOptions: gh.ListOptions{PerPage: 50}}
	for {
		c.checkRateLimit()
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		c.updateRateLimit(resp)
		if err != nil {
			return nil, fmt.Errorf("github search: %w", err)
		}
		allIssues = append(allIssues, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Build tasks from search results. Use a flat estimate for all PRs
	// to avoid extra API calls per PR (which made loading very slow).
	const prEstimate = 30 * time.Minute

	tasks := make([]task.Task, 0, len(allIssues))
	for _, issue := range allIssues {
		if issue.PullRequestLinks == nil {
			continue
		}
		owner, repo, number, err := parseIssue(issue)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s#%d", repo, number)
		tasks = append(tasks, task.Task{
			Key:               key,
			Provider:          "github",
			Summary:           issue.GetTitle(),
			Priority:          2,
			Created:           issue.GetCreatedAt().Time,
			Project:           fmt.Sprintf("%s/%s", owner, repo),
			IssueType:         "Pull Request",
			OriginalEstimate:  prEstimate,
			RemainingEstimate: prEstimate,
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

// resolveRepo maps a short repo name to its owner and repo using the
// configured repos list (e.g. "fylla" → "iruoy", "fylla").
func resolveRepo(repoName string, repos []string) (string, string, error) {
	for _, r := range repos {
		parts := strings.SplitN(r, "/", 2)
		if len(parts) == 2 && parts[1] == repoName {
			return parts[0], parts[1], nil
		}
	}
	return "", "", fmt.Errorf("repo %q not found in configured repos", repoName)
}

// parsePRKey splits "repo#123" into owner, repo, number using the configured
// repos list to resolve the owner.
func parsePRKey(key string, repos []string) (string, string, int, error) {
	idx := strings.Index(key, "#")
	if idx < 0 {
		return "", "", 0, fmt.Errorf("missing '#' in key")
	}
	repoName := key[:idx]
	num, err := strconv.Atoi(key[idx+1:])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number: %w", err)
	}

	owner, repo, err := resolveRepo(repoName, repos)
	if err != nil {
		return "", "", 0, err
	}
	return owner, repo, num, nil
}

func (c *Client) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	owner, repo, err := resolveRepo(input.Project, c.Repos)
	if err != nil {
		return "", fmt.Errorf("github create issue: %w", err)
	}
	req := &gh.IssueRequest{
		Title: &input.Summary,
	}
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

func (c *Client) CompleteTask(_ context.Context, _ string) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) DeleteTask(_ context.Context, _ string) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) PostWorklog(_ context.Context, _ string, _ time.Duration, _ string, _ time.Time) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) GetEstimate(_ context.Context, _ string) (time.Duration, error) {
	return 0, fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) UpdateEstimate(_ context.Context, _ string, _ time.Duration) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) GetDueDate(_ context.Context, _ string) (*time.Time, error) {
	return nil, nil
}

func (c *Client) UpdateDueDate(_ context.Context, _ string, _ time.Time) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) RemoveDueDate(_ context.Context, _ string) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) GetPriority(_ context.Context, _ string) (int, error) {
	return 2, nil // High default
}

func (c *Client) UpdatePriority(_ context.Context, _ string, _ int) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) GetSummary(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("github: %w", ErrUnsupported)
}

func (c *Client) UpdateSummary(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("github: %w", ErrUnsupported)
}

// SetHTTPClient sets the underlying HTTP client (for testing).
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.client = gh.NewClient(httpClient)
}

// SetBaseURL sets the base URL for API requests (for testing).
func (c *Client) SetBaseURL(url string) {
	c.client.BaseURL, _ = c.client.BaseURL.Parse(url)
}
