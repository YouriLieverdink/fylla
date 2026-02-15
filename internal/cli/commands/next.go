package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// NextParams holds all inputs for the next command.
type NextParams struct {
	Cal CalendarClient
	Now time.Time
}

// FyllaEvent represents a scheduled Fylla task event or a calendar event.
type FyllaEvent struct {
	TaskKey         string
	Project         string
	Summary         string
	Start           time.Time
	End             time.Time
	AtRisk          bool
	IsCalendarEvent bool
}

// NextResult holds the output of a next operation.
type NextResult struct {
	Current *FyllaEvent
	Next    *FyllaEvent
}

// RunNext finds the current or next upcoming Fylla task for today.
func RunNext(ctx context.Context, p NextParams) (*NextResult, error) {
	events, err := readTodayEvents(ctx, p.Cal, p.Now)
	if err != nil {
		return nil, err
	}

	var result NextResult
	for _, fe := range events {
		if !p.Now.Before(fe.Start) && p.Now.Before(fe.End) {
			current := fe
			result.Current = &current
			continue
		}
		if fe.Start.After(p.Now) && result.Next == nil {
			next := fe
			result.Next = &next
		}
	}

	return &result, nil
}

// PrintNextResult writes the next task info to the given writer.
func PrintNextResult(w io.Writer, result *NextResult, now time.Time) {
	if result.Current == nil && result.Next == nil {
		fmt.Fprintln(w, "No more Fylla tasks today.")
		return
	}

	if result.Current != nil {
		if result.Current.IsCalendarEvent {
			fmt.Fprintf(w, "Current: %s (until %s)\n",
				result.Current.Summary,
				result.Current.End.Format("15:04"),
			)
		} else {
			prefix := ""
			if result.Current.AtRisk {
				prefix = "[LATE] "
			}
			taskLabel := result.Current.TaskKey
			if result.Current.Project != "" {
				taskLabel = "[" + result.Current.Project + "] " + taskLabel
			}
			fmt.Fprintf(w, "Current: %s%s: %s (until %s)\n",
				prefix,
				taskLabel,
				result.Current.Summary,
				result.Current.End.Format("15:04"),
			)
		}
	}

	if result.Next != nil {
		until := result.Next.Start.Sub(now)
		minutes := int(until.Minutes())

		if result.Next.IsCalendarEvent {
			if minutes < 60 {
				fmt.Fprintf(w, "Next:    %s (starts in %dm)\n",
					result.Next.Summary,
					minutes,
				)
			} else {
				fmt.Fprintf(w, "Next:    %s (%s – %s)\n",
					result.Next.Summary,
					result.Next.Start.Format("15:04"),
					result.Next.End.Format("15:04"),
				)
			}
		} else {
			prefix := ""
			if result.Next.AtRisk {
				prefix = "[LATE] "
			}
			taskLabel := result.Next.TaskKey
			if result.Next.Project != "" {
				taskLabel = "[" + result.Next.Project + "] " + taskLabel
			}
			if minutes < 60 {
				fmt.Fprintf(w, "Next:    %s%s: %s (starts in %dm)\n",
					prefix,
					taskLabel,
					result.Next.Summary,
					minutes,
				)
			} else {
				fmt.Fprintf(w, "Next:    %s%s: %s (%s – %s)\n",
					prefix,
					taskLabel,
					result.Next.Summary,
					result.Next.Start.Format("15:04"),
					result.Next.End.Format("15:04"),
				)
			}
		}
	}
}

func newNextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show the current or next scheduled task",
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
			result, err := RunNext(cmd.Context(), NextParams{
				Cal: cal,
				Now: now,
			})
			if err != nil {
				return err
			}

			PrintNextResult(cmd.OutOrStdout(), result, now)
			return nil
		},
	}

	return cmd
}
