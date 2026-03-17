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
	"github.com/iruoy/fylla/internal/prutil"
	"github.com/iruoy/fylla/internal/task"
)

var jiraKeyPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-\d+`)

// ErrUnsupported is returned for write operations that don't apply to PRs.
var ErrUnsupported = errors.New("operation not supported for GitHub pull requests")

// Client handles fetching pull requests needing review from GitHub.
type Client struct {
	client *gh.Client
	query  string
	Repos  []string // optional: only include PRs from these repos (e.g. "owner/repo")
}

// NewClient creates a GitHub client with the given personal access token.
func NewClient(token string) *Client {
	return &Client{
		client: gh.NewClient(nil).WithAuthToken(token),
	}
}

// FetchTasks searches for open PRs requesting the user's review and returns them as tasks.
func (c *Client) FetchTasks(ctx context.Context, query string) ([]task.Task, error) {
	if query == "" {
		query = "is:pr state:open review-requested:@me"
	}

	// Look up the authenticated user so we can detect prior reviews.
	var myLogin string
	if me, _, err := c.client.Users.Get(ctx, ""); err == nil {
		myLogin = me.GetLogin()
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
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, fmt.Errorf("github search: %w", err)
		}
		allIssues = append(allIssues, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Filter to actual PRs and parse issue metadata.
	type issueInfo struct {
		issue  *gh.Issue
		owner  string
		repo   string
		number int
	}
	var infos []issueInfo
	for _, issue := range allIssues {
		if issue.PullRequestLinks == nil {
			continue
		}
		owner, repo, number, err := parseIssue(issue)
		if err != nil {
			continue
		}
		infos = append(infos, issueInfo{issue, owner, repo, number})
	}

	// Fetch PR details concurrently for diff stats.
	type prResult struct {
		idx      int
		estimate time.Duration
	}
	results := make([]prResult, len(infos))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for i, info := range infos {
		wg.Add(1)
		go func(idx int, owner, repo string, number int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
			if err != nil {
				results[idx] = prResult{idx: idx, estimate: 30 * time.Minute}
				return
			}

			est := prutil.EstimateFromLines(pr.GetAdditions(), pr.GetDeletions())
			if myLogin != "" {
				if delta, ok := c.deltaEstimate(ctx, owner, repo, number, pr.GetHead().GetSHA(), myLogin); ok {
					est = delta
				}
			}
			results[idx] = prResult{idx: idx, estimate: est}
		}(i, info.owner, info.repo, info.number)
	}
	wg.Wait()

	tasks := make([]task.Task, 0, len(infos))
	for i, info := range infos {
		key := fmt.Sprintf("%s#%d", info.repo, info.number)
		t := task.Task{
			Key:               key,
			Provider:          "github",
			Summary:           info.issue.GetTitle(),
			Priority:          2,
			Created:           info.issue.GetCreatedAt().Time,
			Project:           fmt.Sprintf("%s/%s", info.owner, info.repo),
			IssueType:         "Pull Request",
			OriginalEstimate:  results[i].estimate,
			RemainingEstimate: results[i].estimate,
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (c *Client) deltaEstimate(ctx context.Context, owner, repo string, number int, headSHA, myLogin string) (time.Duration, bool) {
	reviews, _, err := c.client.PullRequests.ListReviews(ctx, owner, repo, number, nil)
	if err != nil {
		return 0, false
	}

	var latest *gh.PullRequestReview
	for _, r := range reviews {
		if r.GetUser().GetLogin() != myLogin {
			continue
		}
		if latest == nil || r.GetSubmittedAt().After(latest.GetSubmittedAt().Time) {
			latest = r
		}
	}
	if latest == nil {
		return 0, false
	}

	if latest.GetCommitID() == headSHA {
		return 15 * time.Minute, true
	}

	// Build set of files that belong to the PR itself.
	prFiles := make(map[string]bool)
	opts := &gh.ListOptions{PerPage: 100}
	for {
		files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		if err != nil {
			return 0, false
		}
		for _, f := range files {
			prFiles[f.GetFilename()] = true
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Compare commits since review, but only count lines in PR files.
	// This filters out noise from rebases or merges of the base branch.
	comparison, _, err := c.client.Repositories.CompareCommits(ctx, owner, repo, latest.GetCommitID(), headSHA, nil)
	if err != nil {
		return 0, false
	}

	var added, removed int
	for _, f := range comparison.Files {
		if prFiles[f.GetFilename()] {
			added += f.GetAdditions()
			removed += f.GetDeletions()
		}
	}
	return prutil.EstimateFromLines(added, removed), true
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

// ResolveJiraKey fetches the PR identified by prKey (e.g. "repo#123") and
// extracts a Jira issue key from its branch name or body. Returns empty string
// when no key is found.
func (c *Client) ResolveJiraKey(ctx context.Context, prKey string) (string, error) {
	owner, repo, number, err := parsePRKey(prKey, c.Repos)
	if err != nil {
		return "", fmt.Errorf("parse PR key %q: %w", prKey, err)
	}

	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return "", fmt.Errorf("fetch PR %s: %w", prKey, err)
	}

	// Search branch name first, then body.
	if key := jiraKeyPattern.FindString(pr.GetHead().GetRef()); key != "" {
		return key, nil
	}
	if key := jiraKeyPattern.FindString(pr.GetBody()); key != "" {
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
