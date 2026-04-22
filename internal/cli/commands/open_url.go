package commands

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/iruoy/fylla/internal/config"
)

type kendoProjectIDResolver interface {
	ProjectIDForKey(ctx context.Context, key string) (int, error)
}

func openTaskInBrowser(ctx context.Context, cfg *config.Config, taskKey, provider, project, issueType string, kendoResolver kendoProjectIDResolver) (string, error) {
	taskURL, err := buildTaskProviderURL(ctx, cfg, taskKey, provider, project, issueType, kendoResolver)
	if err != nil {
		return "", err
	}
	if err := openBrowserURL(taskURL); err != nil {
		return "", fmt.Errorf("open browser: %w", err)
	}
	return taskURL, nil
}

func buildTaskProviderURL(ctx context.Context, cfg *config.Config, taskKey, provider, project, issueType string, kendoResolver kendoProjectIDResolver) (string, error) {
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}

	switch provider {
	case "github":
		owner, repo, number, err := parseGitHubWebTarget(taskKey, project, cfg.GitHub.Repos)
		if err != nil {
			return "", err
		}
		path := "issues"
		if strings.EqualFold(issueType, "pull request") {
			path = "pull"
		}
		return fmt.Sprintf("https://github.com/%s/%s/%s/%d", owner, repo, path, number), nil
	case "kendo":
		baseURL := strings.TrimRight(cfg.Kendo.URL, "/")
		if baseURL == "" {
			return "", fmt.Errorf("kendo.url is not configured")
		}
		if kendoResolver == nil {
			return "", fmt.Errorf("kendo project lookup is unavailable")
		}
		projectID, err := kendoResolver.ProjectIDForKey(ctx, taskKey)
		if err != nil {
			return "", fmt.Errorf("resolve kendo project id: %w", err)
		}
		return fmt.Sprintf("%s/projects/%d/issues/%s", baseURL, projectID, taskKey), nil
	case "todoist":
		return fmt.Sprintf("https://app.todoist.com/app/task/%s", taskKey), nil
	case "local":
		return "", fmt.Errorf("provider %q does not support opening tasks in browser", provider)
	default:
		return "", fmt.Errorf("unsupported provider %q", provider)
	}
}

func parseGitHubWebTarget(taskKey, project string, repos []string) (string, string, int, error) {
	keyProject, number, err := parseGitHubTaskKey(taskKey)
	if err != nil {
		return "", "", 0, err
	}

	owner, repo, err := resolveGitHubProject(keyProject, project, repos)
	if err != nil {
		return "", "", 0, err
	}
	return owner, repo, number, nil
}

func parseGitHubTaskKey(taskKey string) (string, int, error) {
	idx := strings.LastIndex(taskKey, "#")
	if idx <= 0 || idx == len(taskKey)-1 {
		return "", 0, fmt.Errorf("invalid github key %q", taskKey)
	}
	number, err := strconv.Atoi(taskKey[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("invalid github issue number in %q: %w", taskKey, err)
	}
	return taskKey[:idx], number, nil
}

func resolveGitHubProject(keyProject, fallbackProject string, repos []string) (string, string, error) {
	if owner, repo, ok := ownerRepoFromValue(keyProject, repos); ok {
		return owner, repo, nil
	}
	if owner, repo, ok := ownerRepoFromValue(fallbackProject, repos); ok {
		return owner, repo, nil
	}

	switch {
	case keyProject != "":
		return "", "", fmt.Errorf("github repo %q not found in github.repos", keyProject)
	case fallbackProject != "":
		return "", "", fmt.Errorf("github project %q not found in github.repos", fallbackProject)
	default:
		return "", "", fmt.Errorf("cannot determine github repository")
	}
}

func ownerRepoFromValue(value string, repos []string) (string, string, bool) {
	if value == "" {
		return "", "", false
	}
	if strings.Contains(value, "/") {
		parts := strings.SplitN(value, "/", 2)
		if parts[0] == "" || parts[1] == "" {
			return "", "", false
		}
		return parts[0], parts[1], true
	}
	for _, configured := range repos {
		parts := strings.SplitN(configured, "/", 2)
		if len(parts) == 2 && parts[1] == value {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

func openBrowserURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
