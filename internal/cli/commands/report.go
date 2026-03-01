package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// ReportParams holds inputs for the report command.
type ReportParams struct {
	Cal  CalendarClient
	Cfg  *config.Config
	Now  time.Time
	Days int
}

// ReportResult holds the output of a report operation.
type ReportResult struct {
	Start          time.Time
	End            time.Time
	TasksDone      int
	TaskTime       time.Duration
	MeetingTime    time.Duration
	TotalEvents    int
	UpcomingAtRisk []string
}

// RunReport generates a summary of calendar activity.
func RunReport(ctx context.Context, p ReportParams) (*ReportResult, error) {
	start := time.Date(p.Now.Year(), p.Now.Month(), p.Now.Day(), 0, 0, 0, 0, p.Now.Location())
	end := start.AddDate(0, 0, p.Days).Add(-time.Nanosecond)

	events, err := p.Cal.FetchEvents(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}

	fyllaEvents, err := p.Cal.FetchFyllaEvents(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("fetch fylla events: %w", err)
	}

	result := &ReportResult{
		Start:       start,
		End:         end,
		TotalEvents: len(events),
	}

	// Count task time and done tasks from fylla events
	for _, ev := range fyllaEvents {
		dur := ev.End.Sub(ev.Start)
		result.TaskTime += dur

		parsed := calendar.ParseTitle(ev.Title)
		if parsed.Done {
			result.TasksDone++
		}
	}

	// Count meeting time (non-fylla, non-transparent, non-allday, non-OOO)
	fyllaEventIDs := make(map[string]bool)
	for _, ev := range fyllaEvents {
		fyllaEventIDs[ev.ID] = true
	}
	for _, ev := range events {
		if fyllaEventIDs[ev.ID] {
			continue
		}
		if ev.Transparency == "transparent" || ev.AllDay || ev.IsOOO() {
			continue
		}
		result.MeetingTime += ev.End.Sub(ev.Start)
	}

	return result, nil
}

// PrintReportResult writes the report to the given writer.
func PrintReportResult(w io.Writer, result *ReportResult) {
	if result.Start.Format("2006-01-02") == result.End.Format("2006-01-02") {
		fmt.Fprintf(w, "Report for %s\n", result.Start.Format("Mon Jan 2, 2006"))
	} else {
		fmt.Fprintf(w, "Report for %s — %s\n",
			result.Start.Format("Mon Jan 2"),
			result.End.Format("Mon Jan 2, 2006"))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Tasks completed:  %d\n", result.TasksDone)
	fmt.Fprintf(w, "  Time on tasks:    %s\n", formatDuration(result.TaskTime))
	fmt.Fprintf(w, "  Meeting time:     %s\n", formatDuration(result.MeetingTime))
	fmt.Fprintf(w, "  Total events:     %d\n", result.TotalEvents)
}

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show activity summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			days, _ := cmd.Flags().GetInt("days")

			result, err := RunReport(cmd.Context(), ReportParams{
				Cal:  cal,
				Cfg:  cfg,
				Now:  time.Now(),
				Days: days,
			})
			if err != nil {
				return err
			}

			PrintReportResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().Int("days", 1, "Number of days to include (1 = today)")

	return cmd
}
