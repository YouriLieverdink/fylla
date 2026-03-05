package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

// WorklogPoster abstracts Jira worklog posting for testing.
type WorklogPoster interface {
	PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error
}

// StopParams holds inputs for the stop command.
type StopParams struct {
	TimerPath    string
	RoundMinutes int
	Now          time.Time
	Description  string
	Jira         WorklogPoster
	Cal          CalendarClient
	Estimate     EstimateGetter
	Cfg          *config.Config
	Resolver     JiraKeyResolver
	Survey       Surveyor
	Completer    TaskCompleter
	Done         bool
}

// StopResult holds the output of a stop operation.
type StopResult struct {
	TaskKey           string
	Elapsed           time.Duration
	Rounded           time.Duration
	Description       string
	CalendarUpdated   bool
	RemainingEstimate time.Duration
	HasRemaining      bool
	Done              bool
}

// RunStop stops the timer, posts the worklog to Jira, and returns the result.
func RunStop(ctx context.Context, p StopParams) (*StopResult, error) {
	sr, err := timer.Stop(p.Now, p.RoundMinutes, p.TimerPath)
	if err != nil {
		return nil, err
	}

	worklogKey := sr.TaskKey

	// Resolve GitHub PR keys to Jira issue keys for worklog posting.
	if isGitHubKey(sr.TaskKey) && p.Resolver != nil {
		resolved, err := resolveGitHubToJira(ctx, p.Resolver, p.Survey, sr.TaskKey, p.Cfg)
		if err != nil {
			return nil, fmt.Errorf("resolve jira key: %w", err)
		}
		worklogKey = resolved
	}

	// Resolve local task keys to a fallback issue for worklog posting.
	if isLocalKey(sr.TaskKey) {
		resolved, err := resolveToFallbackIssue(p.Survey, p.Cfg)
		if err != nil {
			return nil, fmt.Errorf("resolve worklog target: %w", err)
		}
		worklogKey = resolved
	}

	if err := p.Jira.PostWorklog(ctx, worklogKey, sr.Rounded, p.Description, sr.StartTime); err != nil {
		return nil, fmt.Errorf("post worklog: %w", err)
	}

	result := &StopResult{
		TaskKey:     worklogKey,
		Elapsed:     sr.Elapsed,
		Rounded:     sr.Rounded,
		Description: p.Description,
	}

	// Update calendar event if calendar is available
	if p.Cal != nil {
		if updated, err := updateCalendarEvent(ctx, p.Cal, sr.TaskKey, sr.StartTime, sr.Rounded); err == nil {
			result.CalendarUpdated = updated
		}
		// Gracefully ignore calendar errors
	}

	// Check remaining estimate if available
	if p.Estimate != nil {
		remaining, err := p.Estimate.GetEstimate(ctx, sr.TaskKey)
		if err == nil {
			result.RemainingEstimate = remaining
			result.HasRemaining = true
		}
	}

	// Mark task as done if requested
	if p.Done && p.Completer != nil {
		if _, err := RunDone(ctx, DoneParams{
			TaskKey:   sr.TaskKey,
			Completer: p.Completer,
		}); err != nil {
			return nil, fmt.Errorf("mark done: %w", err)
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

// updateCalendarEvent finds the calendar event for the task on the timer's start date
// and updates its end time + marks it as done.
func updateCalendarEvent(ctx context.Context, cal CalendarClient, taskKey string, startTime time.Time, rounded time.Duration) (bool, error) {
	startOfDay := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	endOfDay := startOfDay.Add(24*time.Hour - time.Nanosecond)

	events, err := cal.FetchFyllaEvents(ctx, startOfDay, endOfDay)
	if err != nil {
		return false, err
	}

	for _, ev := range events {
		key := calendar.TaskKeyFromDescription(ev.Description)
		if key != taskKey {
			continue
		}
		// Check overlap: event overlaps with timer start
		if ev.Start.After(startTime) || ev.End.Before(startTime) {
			continue
		}

		parsed := calendar.ParseTitle(ev.Title)
		newEnd := startTime.Add(rounded)

		if err := cal.UpdateEvent(ctx, ev.ID, calendar.CreateEventInput{
			TaskKey: taskKey,
			Project: parsed.Project,
			Section: parsed.Section,
			Summary: parsed.Summary,
			Start:   ev.Start,
			End:     newEnd,
			AtRisk:  parsed.AtRisk,
			Done:    true,
		}); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// PrintStopResult writes the stop result to the given writer.
func PrintStopResult(w io.Writer, result *StopResult) {
	fmt.Fprintf(w, "Timer stopped: %s\n", formatElapsed(result.Rounded))
	fmt.Fprintf(w, "Worklog added to %s\n", result.TaskKey)
	if result.CalendarUpdated {
		fmt.Fprintf(w, "Calendar event updated\n")
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
}

func resolveToFallbackIssue(survey Surveyor, cfg *config.Config) (string, error) {
	var fallbacks []string
	if cfg != nil {
		fallbacks = cfg.Worklog.FallbackIssues
	}
	return promptFallbackIssue(survey, fallbacks)
}

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop timer and log work",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			description, _ := cmd.Flags().GetString("description")
			done, _ := cmd.Flags().GetBool("done")
			if description == "" {
				prompt := &survey.Input{Message: "Work description:"}
				if err := survey.AskOne(prompt, &description); err != nil {
					return fmt.Errorf("prompt description: %w", err)
				}
			}

			timerPath, err := timer.DefaultPath()
			if err != nil {
				return fmt.Errorf("timer path: %w", err)
			}

			// Load calendar client (optional)
			var cal CalendarClient
			if cfg.Calendar.Credentials != "" {
				c, err := loadCalendarClient(cmd.Context(), cfg)
				if err == nil {
					cal = c
				}
			}

			// Resolve GitHub PR keys to Jira for worklog posting.
			var resolver JiraKeyResolver
			if r, ok := source.(JiraKeyResolver); ok {
				resolver = r
			}

			now := time.Now()
			result, err := RunStop(cmd.Context(), StopParams{
				TimerPath:    timerPath,
				RoundMinutes: 5,
				Now:          now,
				Description:  description,
				Jira:         source,
				Cal:          cal,
				Estimate:     source,
				Cfg:          cfg,
				Resolver:     resolver,
				Survey:       defaultSurveyor{},
				Completer:    source,
				Done:         done,
			})
			if err != nil {
				return err
			}

			PrintStopResult(cmd.OutOrStdout(), result)
			maybeAutoResync(cmd.Context(), cmd.ErrOrStderr())
			return nil
		},
	}

	cmd.Flags().StringP("description", "d", "", "Work description (skips interactive prompt)")
	cmd.Flags().BoolP("done", "D", false, "Mark the task as done after logging work")

	return cmd
}
