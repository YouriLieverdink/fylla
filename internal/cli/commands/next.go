package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/spf13/cobra"
)

// NextParams holds all inputs for the next command.
type NextParams struct {
	Cal           CalendarClient
	Now           time.Time
	FyllaCalendar string
}

// FyllaEvent represents a parsed Fylla calendar event.
type FyllaEvent struct {
	TaskKey string
	Summary string
	Start   time.Time
	End     time.Time
	AtRisk  bool
}

// NextResult holds the output of a next operation.
type NextResult struct {
	Current *FyllaEvent
	Next    *FyllaEvent
}

// RunNext finds the current or next upcoming Fylla task for today.
func RunNext(ctx context.Context, p NextParams) (*NextResult, error) {
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

	var result NextResult
	for _, fe := range fyllaEvents {
		if !p.Now.Before(fe.Start) && p.Now.Before(fe.End) {
			result.Current = &FyllaEvent{
				TaskKey: fe.TaskKey,
				Summary: fe.Summary,
				Start:   fe.Start,
				End:     fe.End,
				AtRisk:  fe.AtRisk,
			}
			continue
		}
		if fe.Start.After(p.Now) && result.Next == nil {
			result.Next = &FyllaEvent{
				TaskKey: fe.TaskKey,
				Summary: fe.Summary,
				Start:   fe.Start,
				End:     fe.End,
				AtRisk:  fe.AtRisk,
			}
		}
	}

	return &result, nil
}

// parseFyllaEvent extracts task info from a calendar event with Fylla prefix.
func parseFyllaEvent(e calendar.Event) (FyllaEvent, bool) {
	title := e.Title
	atRisk := false

	if strings.HasPrefix(title, "[LATE] [Fylla] ") {
		title = strings.TrimPrefix(title, "[LATE] [Fylla] ")
		atRisk = true
	} else if strings.HasPrefix(title, "[Fylla] ") {
		title = strings.TrimPrefix(title, "[Fylla] ")
	} else {
		return FyllaEvent{}, false
	}

	// Parse "TASK-KEY: Summary"
	idx := strings.Index(title, ": ")
	if idx < 0 {
		return FyllaEvent{}, false
	}

	return FyllaEvent{
		TaskKey: title[:idx],
		Summary: title[idx+2:],
		Start:   e.Start,
		End:     e.End,
		AtRisk:  atRisk,
	}, true
}

// PrintNextResult writes the next task info to the given writer.
func PrintNextResult(w io.Writer, result *NextResult, now time.Time) {
	if result.Current == nil && result.Next == nil {
		fmt.Fprintln(w, "No more Fylla tasks today.")
		return
	}

	if result.Current != nil {
		prefix := ""
		if result.Current.AtRisk {
			prefix = "[LATE] "
		}
		fmt.Fprintf(w, "Current: %s%s: %s (until %s)\n",
			prefix,
			result.Current.TaskKey,
			result.Current.Summary,
			result.Current.End.Format("15:04"),
		)
	}

	if result.Next != nil {
		prefix := ""
		if result.Next.AtRisk {
			prefix = "[LATE] "
		}
		until := result.Next.Start.Sub(now)
		minutes := int(until.Minutes())
		if minutes < 60 {
			fmt.Fprintf(w, "Next:    %s%s: %s (starts in %dm)\n",
				prefix,
				result.Next.TaskKey,
				result.Next.Summary,
				minutes,
			)
		} else {
			fmt.Fprintf(w, "Next:    %s%s: %s (%s – %s)\n",
				prefix,
				result.Next.TaskKey,
				result.Next.Summary,
				result.Next.Start.Format("15:04"),
				result.Next.End.Format("15:04"),
			)
		}
	}
}

func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Show the current or next scheduled task",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadTaskSource()
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
				cfg.Calendar.SourceCalendars, cfg.Calendar.FyllaCalendar, baseURL, cfg.Source)
			if err != nil {
				return err
			}

			now := time.Now()
			result, err := RunNext(cmd.Context(), NextParams{
				Cal:           cal,
				Now:           now,
				FyllaCalendar: cfg.Calendar.FyllaCalendar,
			})
			if err != nil {
				return err
			}

			PrintNextResult(cmd.OutOrStdout(), result, now)
			return nil
		},
	}
}
