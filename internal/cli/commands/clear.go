package commands

import (
	"context"
	"fmt"
	"io"
	"time"
)

// ClearParams holds inputs for the clear command.
type ClearParams struct {
	Cal    CalendarClient
	Start  time.Time
	End    time.Time
	DryRun bool
}

// ClearResult holds the output of a clear operation.
type ClearResult struct {
	Count  int
	DryRun bool
}

// RunClear removes all Fylla-managed events from Google Calendar.
func RunClear(ctx context.Context, p ClearParams) (*ClearResult, error) {
	if p.DryRun {
		events, err := p.Cal.FetchFyllaEvents(ctx, p.Start, p.End)
		if err != nil {
			return nil, fmt.Errorf("fetch fylla events: %w", err)
		}
		return &ClearResult{Count: len(events), DryRun: true}, nil
	}

	if err := p.Cal.DeleteFyllaEvents(ctx, p.Start, p.End); err != nil {
		return nil, fmt.Errorf("delete fylla events: %w", err)
	}

	return &ClearResult{DryRun: false}, nil
}

// PrintClearResult writes the clear result to the given writer.
func PrintClearResult(w io.Writer, result *ClearResult) {
	if result.DryRun {
		fmt.Fprintf(w, "Would remove %d Fylla event(s).\n", result.Count)
		return
	}
	fmt.Fprintln(w, "All Fylla events removed.")
}
