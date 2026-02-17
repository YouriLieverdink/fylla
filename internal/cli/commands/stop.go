package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iruoy/fylla/internal/timer"
	"github.com/spf13/cobra"
)

// WorklogPoster abstracts Jira worklog posting for testing.
type WorklogPoster interface {
	PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error
}

// StopParams holds inputs for the stop command.
type StopParams struct {
	TimerPath    string
	RoundMinutes int
	Now          time.Time
	Description  string
	Jira         WorklogPoster
}

// StopResult holds the output of a stop operation.
type StopResult struct {
	TaskKey     string
	Elapsed     time.Duration
	Rounded     time.Duration
	Description string
}

// RunStop stops the timer, posts the worklog to Jira, and returns the result.
func RunStop(ctx context.Context, p StopParams) (*StopResult, error) {
	sr, err := timer.Stop(p.Now, p.RoundMinutes, p.TimerPath)
	if err != nil {
		return nil, err
	}

	if err := p.Jira.PostWorklog(ctx, sr.TaskKey, sr.Rounded, p.Description, sr.StartTime); err != nil {
		return nil, fmt.Errorf("post worklog: %w", err)
	}

	return &StopResult{
		TaskKey:     sr.TaskKey,
		Elapsed:     sr.Elapsed,
		Rounded:     sr.Rounded,
		Description: p.Description,
	}, nil
}

// PrintStopResult writes the stop result to the given writer.
func PrintStopResult(w io.Writer, result *StopResult) {
	fmt.Fprintf(w, "Timer stopped: %s\n", formatElapsed(result.Rounded))
	fmt.Fprintf(w, "Worklog added to %s\n", result.TaskKey)
}

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop timer and log work",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			description, _ := cmd.Flags().GetString("description")
			if description == "" {
				prompt := &survey.Input{Message: "Work description:"}
				if err := survey.AskOne(prompt, &description); err != nil {
					return fmt.Errorf("prompt description: %w", err)
				}
			}

			timerPath, err := timer.DefaultPath()
			if err != nil {
				return fmt.Errorf("timer path: %w", err)
			}

			result, err := RunStop(cmd.Context(), StopParams{
				TimerPath:    timerPath,
				RoundMinutes: 5,
				Now:          time.Now(),
				Description:  description,
				Jira:         source,
			})
			if err != nil {
				return err
			}

			PrintStopResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().StringP("description", "d", "", "Work description (skips interactive prompt)")

	return cmd
}
