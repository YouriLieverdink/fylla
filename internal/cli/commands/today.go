package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// TodayParams holds all inputs for the today command.
type TodayParams struct {
	Cal CalendarClient
	Now time.Time
}

// TodayResult holds the output of a today operation.
type TodayResult struct {
	Events []FyllaEvent
}

// readTodayEvents reads Fylla events and source calendar events for today,
// merges them into a sorted timeline.
func readTodayEvents(ctx context.Context, cal CalendarClient, now time.Time) ([]FyllaEvent, error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24*time.Hour - time.Nanosecond)

	fyllaEvents, err := cal.FetchFyllaEvents(ctx, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("fetch fylla events: %w", err)
	}

	var events []FyllaEvent
	for _, e := range fyllaEvents {
		parsed := calendar.ParseTitle(e.Title)
		events = append(events, FyllaEvent{
			TaskKey: calendar.TaskKeyFromDescription(e.Description),
			Project: parsed.Project,
			Section: parsed.Section,
			Summary: parsed.Summary,
			Start:   e.Start,
			End:     e.End,
			AtRisk:  parsed.AtRisk,
		})
	}

	calEvents, err := cal.FetchEvents(ctx, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar events: %w", err)
	}

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

// RunToday reads today's schedule from the calendar.
func RunToday(ctx context.Context, p TodayParams) (*TodayResult, error) {
	events, err := readTodayEvents(ctx, p.Cal, p.Now)
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
			projectPrefix := fe.Project
			if fe.Section != "" {
				projectPrefix = projectPrefix + " / " + fe.Section
			}
			taskLabel = "[" + projectPrefix + "] " + taskLabel
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
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			now := time.Now()
			result, err := RunToday(cmd.Context(), TodayParams{
				Cal: cal,
				Now: now,
			})
			if err != nil {
				return err
			}

			PrintTodayResult(cmd.OutOrStdout(), result, now)
			return nil
		},
	}

	return cmd
}
