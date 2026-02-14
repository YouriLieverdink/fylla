package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
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
				cfg.Calendar.SourceCalendar, cfg.Calendar.FyllaCalendar, baseURL)
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
