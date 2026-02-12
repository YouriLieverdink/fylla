package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
)

// CalendarClient abstracts calendar operations for testing.
type CalendarClient interface {
	FetchEvents(ctx context.Context, start, end time.Time) ([]calendar.Event, error)
	DeleteFyllaEvents(ctx context.Context, start, end time.Time) error
	CreateEvent(ctx context.Context, input calendar.CreateEventInput) error
}

// TaskFetcher abstracts task fetching for testing.
type TaskFetcher interface {
	FetchTasks(ctx context.Context, query string) ([]task.Task, error)
}

// SyncParams holds all inputs for the sync process.
type SyncParams struct {
	Cal    CalendarClient
	Tasks  TaskFetcher
	Cfg    *config.Config
	Query  string
	Now    time.Time
	Start  time.Time
	End    time.Time
	DryRun bool
}

// SyncResult holds the output of a sync operation.
type SyncResult struct {
	Allocations []scheduler.Allocation
	AtRisk      []scheduler.Allocation
}

// SyncFlags holds the parsed CLI flags for the sync command.
type SyncFlags struct {
	DryRun bool
	JQL    string
	Filter string
	Days   int
	From   string
	To     string
}

// BuildSyncParams computes SyncParams from CLI flags and config.
func BuildSyncParams(flags SyncFlags, cfg *config.Config, now time.Time) (query string, start, end time.Time, dryRun bool, err error) {
	dryRun = flags.DryRun

	// Query: source-specific flag/default
	switch cfg.Source {
	case "todoist":
		query = flags.Filter
		if query == "" {
			query = cfg.Todoist.DefaultFilter
		}
	default:
		query = flags.JQL
		if query == "" {
			query = cfg.Jira.DefaultJQL
		}
	}

	// Date range: --from/--to take precedence over --days over config windowDays
	if flags.From != "" && flags.To != "" {
		start, err = time.Parse("2006-01-02", flags.From)
		if err != nil {
			return "", time.Time{}, time.Time{}, false, fmt.Errorf("parse --from: %w", err)
		}
		end, err = time.Parse("2006-01-02", flags.To)
		if err != nil {
			return "", time.Time{}, time.Time{}, false, fmt.Errorf("parse --to: %w", err)
		}
		// Set end to end of day
		end = end.Add(24*time.Hour - time.Nanosecond)
	} else {
		days := cfg.Scheduling.WindowDays
		if flags.Days > 0 {
			days = flags.Days
		}
		start = now
		end = now.AddDate(0, 0, days)
	}

	return query, start, end, dryRun, nil
}

// PrintSyncResult writes the sync result to the given writer.
func PrintSyncResult(w io.Writer, result *SyncResult, dryRun bool) {
	if dryRun {
		fmt.Fprintln(w, "Dry run — no events created.")
		fmt.Fprintln(w)
	}

	if len(result.Allocations) == 0 {
		fmt.Fprintln(w, "No tasks to schedule.")
		return
	}

	fmt.Fprintf(w, "Scheduled %d event(s):\n", len(result.Allocations))
	for _, alloc := range result.Allocations {
		prefix := ""
		if alloc.AtRisk {
			prefix = "[LATE] "
		}
		fmt.Fprintf(w, "  %s%s: %s  %s – %s\n",
			prefix,
			alloc.Task.Key,
			alloc.Task.Summary,
			alloc.Start.Format("Mon 15:04"),
			alloc.End.Format("15:04"),
		)
	}

	if len(result.AtRisk) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "At-risk tasks:")
		for _, ar := range result.AtRisk {
			dueStr := "no due date"
			if ar.Task.DueDate != nil {
				dueStr = "due " + ar.Task.DueDate.Format("Jan 2")
			}
			fmt.Fprintf(w, "  %s: %s (%s)\n", ar.Task.Key, ar.Task.Summary, dueStr)
		}
	}
}

// RunSync executes the full sync process:
//  1. Delete existing [Fylla] events from Google Calendar
//  2. Fetch tasks using the configured source
//  3. Sort tasks by composite score
//  4. Fetch Google Calendar events within scheduling window
//  5. Find free slots per project respecting time windows
//  6. Allocate tasks to slots using first-fit algorithm
//  7. Create fresh [Fylla] calendar events
//  8. Return at-risk warnings
func RunSync(ctx context.Context, p SyncParams) (*SyncResult, error) {
	// Step 1: Delete existing [Fylla] events (skip on dry-run)
	if !p.DryRun {
		if err := p.Cal.DeleteFyllaEvents(ctx, p.Start, p.End); err != nil {
			return nil, fmt.Errorf("delete fylla events: %w", err)
		}
	}

	// Step 2: Fetch tasks
	tasks, err := p.Tasks.FetchTasks(ctx, p.Query)
	if err != nil {
		return nil, fmt.Errorf("fetch tasks: %w", err)
	}

	// Step 3: Sort by composite score
	sorted := scheduler.SortTasks(tasks, p.Cfg.Weights, p.Cfg.TypeScores, p.Now)

	// Step 4: Fetch Google Calendar events
	events, err := p.Cal.FetchEvents(ctx, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar events: %w", err)
	}

	// Step 5: Find free slots per project
	slotsByProject := make(map[string][]calendar.Slot)

	defaultSlots, err := calendar.FindFreeSlots(
		p.Now, p.Start, p.End, events,
		p.Cfg.BusinessHours,
		p.Cfg.Scheduling.BufferMinutes,
		p.Cfg.Scheduling.MinTaskDurationMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("find default slots: %w", err)
	}
	slotsByProject[""] = defaultSlots

	for project := range p.Cfg.ProjectRules {
		hours := p.Cfg.BusinessHoursFor(project)
		slots, err := calendar.FindFreeSlots(
			p.Now, p.Start, p.End, events,
			hours,
			p.Cfg.Scheduling.BufferMinutes,
			p.Cfg.Scheduling.MinTaskDurationMinutes,
		)
		if err != nil {
			return nil, fmt.Errorf("find slots for project %s: %w", project, err)
		}
		slotsByProject[project] = slots
	}

	// Step 6: Allocate tasks to slots
	allocations := scheduler.Allocate(sorted, slotsByProject, scheduler.AllocateConfig{
		MinTaskDurationMinutes: p.Cfg.Scheduling.MinTaskDurationMinutes,
	})

	// Step 7: Create calendar events (skip on dry-run)
	if !p.DryRun {
		for _, alloc := range allocations {
			if err := p.Cal.CreateEvent(ctx, calendar.CreateEventInput{
				TaskKey: alloc.Task.Key,
				Summary: alloc.Task.Summary,
				Start:   alloc.Start,
				End:     alloc.End,
				AtRisk:  alloc.AtRisk,
			}); err != nil {
				return nil, fmt.Errorf("create event for %s: %w", alloc.Task.Key, err)
			}
		}
	}

	// Step 8: Collect at-risk warnings
	var atRisk []scheduler.Allocation
	seen := make(map[string]bool)
	for _, alloc := range allocations {
		if alloc.AtRisk && !seen[alloc.Task.Key] {
			atRisk = append(atRisk, alloc)
			seen[alloc.Task.Key] = true
		}
	}

	return &SyncResult{
		Allocations: allocations,
		AtRisk:      atRisk,
	}, nil
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Schedule tasks into Google Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			credFile, _ := cmd.Flags().GetString("client-credentials")
			if credFile == "" {
				credFile = cfg.Calendar.ClientCredentials
			}
			if credFile == "" {
				return fmt.Errorf("set calendar.clientCredentials in config or pass --client-credentials")
			}

			oauthCfg, err := calendar.OAuthConfigFromFile(credFile)
			if err != nil {
				return fmt.Errorf("load client credentials: %w", err)
			}

			tokenPath, err := calendar.TokenPath()
			if err != nil {
				return err
			}

			token, err := calendar.CachedToken(cmd.Context(), oauthCfg, tokenPath)
			if err != nil {
				return fmt.Errorf("google auth: %w", err)
			}

			baseURL := cfg.Jira.URL
			if baseURL == "" {
				baseURL = "https://todoist.com"
			}
			cal, err := calendar.NewGoogleClient(cmd.Context(), oauthCfg, token,
				cfg.Calendar.SourceCalendar, cfg.Calendar.FyllaCalendar, baseURL)
			if err != nil {
				return err
			}

			now := time.Now()
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			jql, _ := cmd.Flags().GetString("jql")
			filter, _ := cmd.Flags().GetString("filter")
			days, _ := cmd.Flags().GetInt("days")
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")

			query, start, end, dryRun, err := BuildSyncParams(SyncFlags{
				DryRun: dryRun,
				JQL:    jql,
				Filter: filter,
				Days:   days,
				From:   from,
				To:     to,
			}, cfg, now)
			if err != nil {
				return err
			}

			result, err := RunSync(cmd.Context(), SyncParams{
				Cal:    cal,
				Tasks:  source,
				Cfg:    cfg,
				Query:  query,
				Now:    now,
				Start:  start,
				End:    end,
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}

			PrintSyncResult(cmd.OutOrStdout(), result, dryRun)
			return nil
		},
	}

	cmd.Flags().String("client-credentials", "", "Path to Google OAuth client credentials JSON file")
	cmd.Flags().Bool("dry-run", false, "Preview schedule without creating events")
	cmd.Flags().String("jql", "", "Custom JQL query override (Jira source)")
	cmd.Flags().String("filter", "", "Custom filter override (Todoist source)")
	cmd.Flags().Int("days", 0, "Override scheduling window (days)")
	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD)")

	return cmd
}
