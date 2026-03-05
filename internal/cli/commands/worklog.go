package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// WorklogParams holds all inputs for the worklog command.
type WorklogParams struct {
	Cal      CalendarClient
	Jira     WorklogPoster
	Cfg      *config.Config
	Survey   Surveyor
	Date     time.Time
	W        io.Writer
	Resolver JiraKeyResolver
}

// WorklogEntry represents a single worklog to be posted.
type WorklogEntry struct {
	TaskKey     string
	Duration    time.Duration
	Description string
	Started     time.Time
}

// WorklogResult holds the output of a worklog operation.
type WorklogResult struct {
	Entries []WorklogEntry
	Target  time.Duration
	Posted  int
	Errors  int
}

// RunWorklog interactively builds and posts worklogs from today's calendar events.
func RunWorklog(ctx context.Context, p WorklogParams) (*WorklogResult, error) {
	target := config.DailyTargetFor(p.Cfg.BusinessHours, p.Date.Weekday())

	events, err := readTodayEvents(ctx, p.Cal, p.Date)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	if target == 0 {
		fmt.Fprintln(p.W, "Non-work day (daily target is 0h).")
		if len(events) == 0 {
			return &WorklogResult{Target: target}, nil
		}
		fmt.Fprintln(p.W, "Events found — you can still log them.")
	}

	var entries []WorklogEntry
	var totalLogged time.Duration

	// Walk each event
	for _, fe := range events {
		eventDur := fe.End.Sub(fe.Start)
		durStr := formatElapsed(eventDur)

		if fe.IsCalendarEvent {
			// Calendar event (meeting)
			fmt.Fprintf(p.W, "\n  %s – %s  %s (%s)\n",
				fe.Start.Format("15:04"), fe.End.Format("15:04"), fe.Summary, durStr)

			rawDur, err := p.Survey.InputWithDefault("Duration:", durStr)
			if err != nil {
				return nil, fmt.Errorf("prompt duration: %w", err)
			}
			dur, err := ParseDuration(rawDur)
			if err != nil {
				fmt.Fprintf(p.W, "  Skipping (invalid duration: %s)\n", rawDur)
				continue
			}
			if dur == 0 {
				fmt.Fprintln(p.W, "  Skipping (0 duration)")
				continue
			}

			// Pick Jira issue for this meeting
			issueKey, err := promptFallbackIssue(p.Survey, p.Cfg.Worklog.FallbackIssues)
			if err != nil {
				return nil, fmt.Errorf("prompt issue: %w", err)
			}

			entries = append(entries, WorklogEntry{
				TaskKey:     issueKey,
				Duration:    dur,
				Description: fe.Summary,
				Started:     fe.Start,
			})
			totalLogged += dur
		} else {
			// Fylla task
			fmt.Fprintf(p.W, "\n  %s – %s  %s: %s (%s)\n",
				fe.Start.Format("15:04"), fe.End.Format("15:04"), fe.TaskKey, fe.Summary, durStr)

			rawDur, err := p.Survey.InputWithDefault("Duration:", durStr)
			if err != nil {
				return nil, fmt.Errorf("prompt duration: %w", err)
			}
			dur, err := ParseDuration(rawDur)
			if err != nil {
				fmt.Fprintf(p.W, "  Skipping (invalid duration: %s)\n", rawDur)
				continue
			}
			if dur == 0 {
				fmt.Fprintln(p.W, "  Skipping (0 duration)")
				continue
			}

			taskKey := fe.TaskKey
			if isGitHubKey(fe.TaskKey) && p.Resolver != nil {
				resolved, err := resolveGitHubToJira(ctx, p.Resolver, p.Survey, fe.TaskKey, p.Cfg)
				if err != nil {
					return nil, fmt.Errorf("resolve jira key for %s: %w", fe.TaskKey, err)
				}
				taskKey = resolved
			}
			if isLocalKey(fe.TaskKey) {
				resolved, err := resolveToFallbackIssue(p.Survey, p.Cfg)
				if err != nil {
					return nil, fmt.Errorf("resolve worklog target for %s: %w", fe.TaskKey, err)
				}
				taskKey = resolved
			}

			entries = append(entries, WorklogEntry{
				TaskKey:     taskKey,
				Duration:    dur,
				Description: fe.Summary,
				Started:     fe.Start,
			})
			totalLogged += dur
		}
	}

	// Handle remainder
	if target > 0 && totalLogged < target {
		remaining := target - totalLogged
		fmt.Fprintf(p.W, "\nRemaining: %s (target: %s, logged: %s)\n",
			formatElapsed(remaining), formatElapsed(target), formatElapsed(totalLogged))

		issueKey, err := promptFallbackIssue(p.Survey, p.Cfg.Worklog.FallbackIssues)
		if err != nil {
			return nil, fmt.Errorf("prompt fallback issue: %w", err)
		}

		rawDur, err := p.Survey.InputWithDefault("Duration:", formatDurationCompact(remaining))
		if err != nil {
			return nil, fmt.Errorf("prompt fallback duration: %w", err)
		}
		dur, err := ParseDuration(rawDur)
		if err == nil && dur > 0 {
			// Calculate start time: after last entry or start of business hours
			started := p.Date
			if len(entries) > 0 {
				last := entries[len(entries)-1]
				started = last.Started.Add(last.Duration)
			}
			entries = append(entries, WorklogEntry{
				TaskKey:     issueKey,
				Duration:    dur,
				Description: "Fallback",
				Started:     started,
			})
			totalLogged += dur
		}
	} else if target > 0 && totalLogged > target {
		fmt.Fprintf(p.W, "\nWarning: logged %s exceeds daily target of %s\n",
			formatElapsed(totalLogged), formatElapsed(target))
	}

	if len(entries) == 0 {
		fmt.Fprintln(p.W, "\nNo worklogs to post.")
		return &WorklogResult{Target: target}, nil
	}

	// Show summary
	fmt.Fprintln(p.W, "\nWorklog summary:")
	maxKey := 0
	for _, e := range entries {
		if w := len(e.TaskKey); w > maxKey {
			maxKey = w
		}
	}
	for _, e := range entries {
		fmt.Fprintf(p.W, "  %-*s  %5s  %s  %s\n",
			maxKey, e.TaskKey,
			formatDurationCompact(e.Duration),
			e.Started.Format("15:04"),
			e.Description,
		)
	}
	fmt.Fprintf(p.W, "  Total: %s\n", formatElapsed(totalLogged))

	// Confirm
	confirm, err := p.Survey.Select("Post worklogs?", []string{"Yes", "No"})
	if err != nil {
		return nil, fmt.Errorf("prompt confirm: %w", err)
	}

	result := &WorklogResult{
		Entries: entries,
		Target:  target,
	}

	if confirm != "Yes" {
		return result, nil
	}

	// Post worklogs
	for _, e := range entries {
		if err := p.Jira.PostWorklog(ctx, e.TaskKey, e.Duration, e.Description, e.Started); err != nil {
			fmt.Fprintf(p.W, "  Failed to post %s: %v\n", e.TaskKey, err)
			result.Errors++
			continue
		}
		result.Posted++
	}

	fmt.Fprintf(p.W, "\nPosted %d worklog(s)", result.Posted)
	if result.Errors > 0 {
		fmt.Fprintf(p.W, " (%d failed)", result.Errors)
	}
	fmt.Fprintln(p.W, ".")

	return result, nil
}

// promptFallbackIssue asks the user to pick a Jira issue key from fallbacks or type one.
func promptFallbackIssue(s Surveyor, fallbacks []string) (string, error) {
	options := make([]string, len(fallbacks))
	copy(options, fallbacks)
	options = append(options, "other (type manually)")

	if len(fallbacks) == 0 {
		// No fallbacks configured — just ask for manual input
		return s.Input("Jira issue key:")
	}

	choice, err := s.Select("Jira issue:", options)
	if err != nil {
		return "", err
	}

	if choice == "other (type manually)" {
		return s.Input("Jira issue key:")
	}
	return choice, nil
}

// formatDurationCompact formats a duration as "1h30m", "2h", or "30m".
func formatDurationCompact(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

func newWorklogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worklog",
		Short: "Review and post worklogs from today's calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			dateStr, _ := cmd.Flags().GetString("date")
			date := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("parse --date: %w", err)
				}
				// Set to same time of day with local timezone
				now := time.Now()
				date = time.Date(parsed.Year(), parsed.Month(), parsed.Day(),
					now.Hour(), now.Minute(), now.Second(), 0, now.Location())
			}

			var resolver JiraKeyResolver
			if r, ok := source.(JiraKeyResolver); ok {
				resolver = r
			}

			result, err := RunWorklog(cmd.Context(), WorklogParams{
				Cal:      cal,
				Jira:     source,
				Cfg:      cfg,
				Survey:   defaultSurveyor{},
				Date:     date,
				W:        cmd.OutOrStdout(),
				Resolver: resolver,
			})
			if err != nil {
				return err
			}

			_ = result
			return nil
		},
	}

	cmd.Flags().String("date", "", "Date to review worklogs for (YYYY-MM-DD, default: today)")

	return cmd
}
