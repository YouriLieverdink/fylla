package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/timer"
)

// WorklogPoster abstracts Jira worklog posting for testing.
type WorklogPoster interface {
	PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error
}

// StopParams holds inputs for the stop command.
type StopParams struct {
	TimerPath     string
	RoundMinutes  int
	Now           time.Time
	Description   string
	Jira          WorklogPoster
	Estimate      EstimateGetter
	Cfg           *config.Config
	Resolver      JiraKeyResolver
	Survey        Surveyor
	Completer     TaskCompleter
	Done             bool
	FallbackIssue    string // pre-resolved Jira key for non-Jira tasks (used by TUI)
	FallbackProvider string // provider of the fallback issue (used by TUI)
}

// StopResult holds the output of a stop operation.
type StopResult struct {
	TaskKey           string
	TotalElapsed      time.Duration
	Description       string
	SegmentCount      int
	RemainingEstimate time.Duration
	HasRemaining      bool
	Done              bool
	ResumedKey        string
}

// RunStop stops the timer, posts worklogs for all segments, and returns the result.
func RunStop(ctx context.Context, p StopParams) (*StopResult, error) {
	sr, err := timer.Stop(p.Now, p.TimerPath)
	if err != nil {
		return nil, err
	}

	worklogKey := sr.TaskKey
	worklogProvider := p.Cfg.Worklog.Provider
	if sr.Provider != "" {
		worklogProvider = sr.Provider
	}

	applyFallback := func() {
		worklogKey = p.FallbackIssue
		if p.FallbackProvider != "" {
			worklogProvider = p.FallbackProvider
		}
	}

	// Resolve GitHub PR keys to Jira issue keys for worklog posting.
	if isGitHubKey(sr.TaskKey) {
		if p.FallbackIssue != "" {
			applyFallback()
		} else if p.Resolver != nil {
			resolved, err := resolveGitHubToJira(ctx, p.Resolver, p.Survey, sr.TaskKey, p.Cfg)
			if err != nil {
				return nil, fmt.Errorf("resolve jira key: %w", err)
			}
			worklogKey = resolved
		} else if p.Survey != nil {
			resolved, err := resolveToFallbackIssue(p.Survey, p.Cfg)
			if err != nil {
				return nil, fmt.Errorf("resolve worklog target: %w", err)
			}
			worklogKey = resolved
		} else {
			return nil, fmt.Errorf("cannot post worklog for GitHub key %s: no resolver or interactive prompt available", sr.TaskKey)
		}
	}

	// Resolve local task keys to a fallback issue for worklog posting.
	if isLocalKey(sr.TaskKey) {
		if p.FallbackIssue != "" {
			applyFallback()
		} else {
			resolved, err := resolveToFallbackIssue(p.Survey, p.Cfg)
			if err != nil {
				return nil, fmt.Errorf("resolve worklog target: %w", err)
			}
			worklogKey = resolved
		}
	}

	// Resolve anonymous timers (empty key) to a fallback issue.
	if sr.TaskKey == "" {
		if p.FallbackIssue != "" {
			applyFallback()
		} else {
			resolved, err := resolveToFallbackIssue(p.Survey, p.Cfg)
			if err != nil {
				return nil, fmt.Errorf("resolve worklog target: %w", err)
			}
			worklogKey = resolved
		}
	}

	// Resolve non-Jira keys (e.g. Todoist) when worklog provider is jira,
	// but allow Kendo tasks to post worklogs directly to Kendo.
	if !isJiraKey(worklogKey) && p.Cfg.Worklog.Provider == "jira" && worklogProvider != "kendo" {
		if p.FallbackIssue != "" {
			applyFallback()
		} else {
			resolved, err := resolveToFallbackIssue(p.Survey, p.Cfg)
			if err != nil {
				return nil, fmt.Errorf("resolve worklog target: %w", err)
			}
			worklogKey = resolved
		}
	}

	// Allow overriding the worklog key for Jira tasks via FallbackIssue.
	if p.FallbackIssue != "" && p.FallbackIssue != sr.TaskKey && isJiraKey(p.FallbackIssue) && isJiraKey(sr.TaskKey) {
		applyFallback()
	}

	var totalElapsed time.Duration

	// Post worklog for each segment
	for i, seg := range sr.Segments {
		elapsed := seg.EndTime.Sub(seg.StartTime)
		if elapsed < 0 {
			elapsed = 0
		}
		rounded := timer.RoundDuration(elapsed, p.RoundMinutes)
		totalElapsed += elapsed

		// Build description for this segment
		desc := seg.Comment
		if desc == "" {
			desc = p.Description
		}
		if len(sr.Segments) > 1 {
			desc = fmt.Sprintf("(%d/%d) %s", i+1, len(sr.Segments), desc)
		}

		// Post worklog
		if multi, ok := p.Jira.(*MultiTaskSource); ok && worklogProvider != "" {
			if err := multi.PostWorklogOn(ctx, worklogKey, rounded, desc, seg.StartTime, worklogProvider); err != nil {
				return nil, fmt.Errorf("post worklog: %w", err)
			}
		} else {
			if err := p.Jira.PostWorklog(ctx, worklogKey, rounded, desc, seg.StartTime); err != nil {
				return nil, fmt.Errorf("post worklog: %w", err)
			}
		}

	}

	result := &StopResult{
		TaskKey:      worklogKey,
		TotalElapsed: totalElapsed,
		Description:  p.Description,
		SegmentCount: len(sr.Segments),
	}

	if sr.Resumed != nil {
		result.ResumedKey = sr.Resumed.TaskKey
	}

	// Check remaining estimate if available
	if p.Estimate != nil {
		if multi, ok := p.Estimate.(*MultiTaskSource); ok && worklogProvider != "" {
			remaining, err := multi.GetEstimateOn(ctx, sr.TaskKey, worklogProvider)
			if err == nil {
				result.RemainingEstimate = remaining
				result.HasRemaining = true
			}
		} else if sr.TaskKey != "" {
			remaining, err := p.Estimate.GetEstimate(ctx, sr.TaskKey)
			if err == nil {
				result.RemainingEstimate = remaining
				result.HasRemaining = true
			}
		}
	}

	// Mark task as done if requested
	if p.Done && p.Completer != nil && sr.TaskKey != "" {
		if multi, ok := p.Completer.(*MultiTaskSource); ok && worklogProvider != "" {
			if err := multi.CompleteTaskOn(ctx, sr.TaskKey, worklogProvider); err != nil {
				return nil, fmt.Errorf("mark done: %w", err)
			}
		} else {
			if _, err := RunDone(ctx, DoneParams{
				TaskKey:   sr.TaskKey,
				Completer: p.Completer,
			}); err != nil {
				return nil, fmt.Errorf("mark done: %w", err)
			}
		}
		result.Done = true
	}

	return result, nil
}

// resolveGitHubToJira resolves a GitHub PR key to a Jira issue key. It first
// tries to extract a Jira key from the PR's branch name or body. If none is
// found, it prompts the user to pick from configured fallback issues.
func resolveGitHubToJira(ctx context.Context, resolver JiraKeyResolver, survey Surveyor, prKey string, cfg *config.Config) (string, error) {
	jiraKey, err := resolver.ResolveJiraKey(ctx, prKey)
	if err != nil {
		// API error — fall through to fallback prompt
		jiraKey = ""
	}

	if jiraKey != "" && survey != nil {
		// Let the user confirm or change the resolved key
		confirmed, err := survey.InputWithDefault(
			fmt.Sprintf("Jira issue for %s:", prKey), jiraKey)
		if err != nil {
			return "", err
		}
		return confirmed, nil
	}

	// No key found — prompt fallback
	if survey == nil {
		return "", fmt.Errorf("no Jira key found for %s and no interactive prompt available", prKey)
	}

	var fallbacks []string
	if cfg != nil {
		fallbacks = cfg.Worklog.FallbackIssues
	}
	return promptFallbackIssue(survey, fallbacks)
}

// PrintStopResult writes the stop result to the given writer.
func PrintStopResult(w io.Writer, result *StopResult) {
	fmt.Fprintf(w, "Timer stopped: %s\n", formatElapsed(result.TotalElapsed))
	fmt.Fprintf(w, "Worklog added to %s\n", result.TaskKey)
	if result.SegmentCount > 1 {
		fmt.Fprintf(w, "%d segments posted\n", result.SegmentCount)
	}
	if result.Done {
		fmt.Fprintf(w, "Marked %s as done\n", result.TaskKey)
	} else if result.HasRemaining {
		if result.RemainingEstimate > 0 {
			fmt.Fprintf(w, "%s has %s remaining — will be rescheduled on next sync.\n",
				result.TaskKey, formatElapsed(result.RemainingEstimate))
		} else {
			fmt.Fprintf(w, "Warning: %s has no time remaining but is not completed.\n", result.TaskKey)
			fmt.Fprintf(w, "  Use 'fylla task done %s' to complete, or 'fylla task estimate %s' to add time.\n",
				result.TaskKey, result.TaskKey)
		}
	}
	if result.ResumedKey != "" {
		fmt.Fprintf(w, "Resumed timer for %s\n", result.ResumedKey)
	}
}

func resolveToFallbackIssue(survey Surveyor, cfg *config.Config) (string, error) {
	var fallbacks []string
	if cfg != nil {
		fallbacks = cfg.Worklog.FallbackIssues
	}
	return promptFallbackIssue(survey, fallbacks)
}
