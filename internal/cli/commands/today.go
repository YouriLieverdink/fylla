package commands

import (
	"context"
	"fmt"
	"io"
	"time"

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

// RunToday fetches all Fylla events scheduled for today.
func RunToday(ctx context.Context, p TodayParams) (*TodayResult, error) {
	startOfDay := time.Date(p.Now.Year(), p.Now.Month(), p.Now.Day(), 0, 0, 0, 0, p.Now.Location())
	endOfDay := startOfDay.Add(24*time.Hour - time.Nanosecond)

	events, err := p.Cal.FetchEvents(ctx, startOfDay, endOfDay)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}

	var fyllaEvents []FyllaEvent
	for _, e := range events {
		if fe, ok := parseFyllaEvent(e); ok {
			fyllaEvents = append(fyllaEvents, fe)
		}
	}

	return &TodayResult{Events: fyllaEvents}, nil
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

		prefix := ""
		if fe.AtRisk {
			prefix = "[LATE] "
		}

		fmt.Fprintf(w, "%s%s – %s  %s%s: %s%s\n",
			marker,
			fe.Start.Format("15:04"),
			fe.End.Format("15:04"),
			prefix,
			fe.TaskKey,
			fe.Summary,
			suffix,
		)
	}
}

func newTodayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show all Fylla tasks scheduled for today",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
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
}
