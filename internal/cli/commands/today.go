package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/spf13/cobra"
)

// TodayParams holds all inputs for the today command.
type TodayParams struct {
	Cal   CalendarClient
	Tasks TaskFetcher
	Cfg   *config.Config
	Query string
	Now   time.Time
}

// TodayResult holds the output of a today operation.
type TodayResult struct {
	Events []FyllaEvent
}

// allocateToday runs the fetch→sort→slots→allocate pipeline for today only.
// It fetches real calendar events to avoid scheduling over existing commitments.
func allocateToday(ctx context.Context, cal CalendarClient, tasks TaskFetcher, cfg *config.Config, query string, now time.Time) ([]FyllaEvent, error) {
	fetched, err := tasks.FetchTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("fetch tasks: %w", err)
	}

	sorted := scheduler.SortTasks(fetched, cfg.Weights, now)

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24*time.Hour - time.Nanosecond)

	// Fetch real calendar events to respect existing commitments
	var calEvents []calendar.Event
	if cal != nil {
		calEvents, err = cal.FetchEvents(ctx, startOfDay, endOfDay)
		if err != nil {
			return nil, fmt.Errorf("fetch calendar events: %w", err)
		}
	}

	slotsByProject := make(map[string][]calendar.Slot)

	defaultSlots, err := calendar.FindFreeSlots(
		now, startOfDay, endOfDay, calEvents,
		cfg.BusinessHours,
		cfg.Scheduling.BufferMinutes,
		cfg.Scheduling.MinTaskDurationMinutes,
		cfg.Scheduling.SnapMinutes,
		cfg.Scheduling.TravelBufferMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("find default slots: %w", err)
	}
	slotsByProject[""] = defaultSlots

	for project := range cfg.ProjectRules {
		hours := cfg.BusinessHoursFor(project)
		slots, err := calendar.FindFreeSlots(
			now, startOfDay, endOfDay, calEvents,
			hours,
			cfg.Scheduling.BufferMinutes,
			cfg.Scheduling.MinTaskDurationMinutes,
			cfg.Scheduling.SnapMinutes,
			cfg.Scheduling.TravelBufferMinutes,
		)
		if err != nil {
			return nil, fmt.Errorf("find slots for project %s: %w", project, err)
		}
		slotsByProject[project] = slots
	}

	allocations := scheduler.Allocate(sorted, slotsByProject, scheduler.AllocateConfig{
		MinTaskDurationMinutes: cfg.Scheduling.MinTaskDurationMinutes,
		BufferMinutes:          cfg.Scheduling.BufferMinutes,
		SnapMinutes:            cfg.Scheduling.SnapMinutes,
	})

	var events []FyllaEvent
	for _, alloc := range allocations {
		events = append(events, FyllaEvent{
			TaskKey: alloc.Task.Key,
			Project: alloc.Task.Project,
			Summary: alloc.Task.Summary,
			Start:   alloc.Start,
			End:     alloc.End,
			AtRisk:  alloc.AtRisk,
		})
	}

	// Merge real calendar events (exclude Fylla-created and all-day events)
	for _, e := range calEvents {
		if e.AllDay || calendar.TaskKeyFromDescription(e.Description) != "" {
			continue
		}
		events = append(events, FyllaEvent{
			Summary:         e.Title,
			Start:           e.Start,
			End:             e.End,
			IsCalendarEvent: true,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	return events, nil
}

// RunToday allocates tasks into today's business-hours slots.
func RunToday(ctx context.Context, p TodayParams) (*TodayResult, error) {
	events, err := allocateToday(ctx, p.Cal, p.Tasks, p.Cfg, p.Query, p.Now)
	if err != nil {
		return nil, err
	}
	return &TodayResult{Events: events}, nil
}

// PrintTodayResult writes the full day schedule to the given writer.
func PrintTodayResult(w io.Writer, result *TodayResult, now time.Time) {
	if len(result.Events) == 0 {
		fmt.Fprintln(w, "No Fylla tasks scheduled for today.")
		return
	}

	fmt.Fprintln(w, "Today's schedule:")
	for _, fe := range result.Events {
		isCurrent := !now.Before(fe.Start) && now.Before(fe.End)

		marker := "  "
		suffix := ""
		if isCurrent {
			marker = "> "
			suffix = "  (current)"
		}

		if fe.IsCalendarEvent {
			fmt.Fprintf(w, "%s%s – %s  %s%s\n",
				marker,
				fe.Start.Format("15:04"),
				fe.End.Format("15:04"),
				fe.Summary,
				suffix,
			)
			continue
		}

		prefix := ""
		if fe.AtRisk {
			prefix = "[LATE] "
		}

		taskLabel := fe.TaskKey
		if fe.Project != "" {
			taskLabel = "[" + fe.Project + "] " + taskLabel
		}

		fmt.Fprintf(w, "%s%s – %s  %s%s: %s%s\n",
			marker,
			fe.Start.Format("15:04"),
			fe.End.Format("15:04"),
			prefix,
			taskLabel,
			fe.Summary,
			suffix,
		)
	}
}

func newTodayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Show all Fylla tasks scheduled for today",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			jql, _ := cmd.Flags().GetString("jql")
			filter, _ := cmd.Flags().GetString("filter")

			var fetcher TaskFetcher
			var query string
			if ms, ok := source.(*MultiTaskSource); ok {
				fetcher = &multiFetcher{
					queries: buildProviderQueries(cfg, jql, filter),
					sources: ms.sources,
				}
			} else {
				fetcher = source
				providers := cfg.ActiveProviders()
				switch providers[0] {
				case "todoist":
					query = filter
					if query == "" {
						query = cfg.Todoist.DefaultFilter
					}
				default:
					query = jql
					if query == "" {
						query = cfg.Jira.DefaultJQL
					}
				}
			}

			now := time.Now()
			result, err := RunToday(cmd.Context(), TodayParams{
				Cal:   cal,
				Tasks: fetcher,
				Cfg:   cfg,
				Query: query,
				Now:   now,
			})
			if err != nil {
				return err
			}

			PrintTodayResult(cmd.OutOrStdout(), result, now)
			return nil
		},
	}

	cmd.Flags().String("jql", "", "Custom JQL query override (Jira source)")
	cmd.Flags().String("filter", "", "Custom filter override (Todoist source)")

	return cmd
}
