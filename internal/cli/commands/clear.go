package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
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

func newClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove all Fylla events from Google Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")

			start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)

			if from != "" {
				start, err = time.Parse("2006-01-02", from)
				if err != nil {
					return fmt.Errorf("parse --from: %w", err)
				}
			}
			if to != "" {
				end, err = time.Parse("2006-01-02", to)
				if err != nil {
					return fmt.Errorf("parse --to: %w", err)
				}
				end = end.Add(24*time.Hour - time.Nanosecond)
			}

			result, err := RunClear(cmd.Context(), ClearParams{
				Cal:    cal,
				Start:  start,
				End:    end,
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}

			PrintClearResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Preview what would be removed without deleting")
	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD)")

	return cmd
}
