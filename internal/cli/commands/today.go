package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
)

// FyllaEvent represents a scheduled Fylla task event or a calendar event.
type FyllaEvent struct {
	TaskKey         string
	Provider        string
	Project         string
	Section         string
	Summary         string
	Start           time.Time
	End             time.Time
	AtRisk          bool
	IsCalendarEvent bool
}

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
		taskKey, provider := calendar.TaskKeyAndProviderFromDescription(e.Description)
		events = append(events, FyllaEvent{
			TaskKey:  taskKey,
			Provider: provider,
			Project:  parsed.Project,
			Section:  parsed.Section,
			Summary:  parsed.Summary,
			Start:    e.Start,
			End:      e.End,
			AtRisk:   parsed.AtRisk,
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
// When verbose is true, task labels include the [Project / Section] prefix.
func PrintTodayResult(w io.Writer, result *TodayResult, now time.Time, verbose bool) {
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

		taskLabel := syncTaskLabel(fe.TaskKey, fe.Project, fe.Section, verbose)

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
