package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// EstimateGetter abstracts fetching the current remaining estimate from Jira.
type EstimateGetter interface {
	GetEstimate(ctx context.Context, issueKey string) (time.Duration, error)
}

// EstimateUpdater abstracts updating the remaining estimate in Jira.
type EstimateUpdater interface {
	UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error
}

// EstimateParams holds inputs for the estimate command.
type EstimateParams struct {
	TaskKey  string
	Duration string // raw duration string, e.g. "4h", "+2h", "-1h"
	Jira     EstimateUpdater
	Getter   EstimateGetter
}

// EstimateResult holds the output of an estimate operation.
type EstimateResult struct {
	TaskKey  string
	Duration time.Duration
}

// RunEstimate sets or adjusts the remaining estimate on a Jira issue.
func RunEstimate(ctx context.Context, p EstimateParams) (*EstimateResult, error) {
	raw := strings.TrimSpace(p.Duration)
	if raw == "" {
		return nil, fmt.Errorf("duration is required")
	}

	var final time.Duration

	if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		// Relative adjustment
		sign := raw[0]
		dur, err := ParseDuration(raw[1:])
		if err != nil {
			return nil, fmt.Errorf("parse adjustment: %w", err)
		}

		current, err := p.Getter.GetEstimate(ctx, p.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("get current estimate: %w", err)
		}

		if sign == '+' {
			final = current + dur
		} else {
			final = current - dur
			if final < 0 {
				final = 0
			}
		}
	} else {
		// Absolute value
		dur, err := ParseDuration(raw)
		if err != nil {
			return nil, err
		}
		final = dur
	}

	if err := p.Jira.UpdateEstimate(ctx, p.TaskKey, final); err != nil {
		return nil, err
	}

	return &EstimateResult{
		TaskKey:  p.TaskKey,
		Duration: final,
	}, nil
}

// PrintEstimateResult writes the estimate confirmation to the given writer.
func PrintEstimateResult(w io.Writer, result *EstimateResult) {
	fmt.Fprintf(w, "Remaining estimate for %s set to %s\n", result.TaskKey, formatElapsed(result.Duration))
}

func newEstimateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "estimate TASK-KEY DURATION",
		Short: "Set or adjust remaining estimate on a Jira task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, _, err := loadJiraClient()
			if err != nil {
				return err
			}

			result, err := RunEstimate(cmd.Context(), EstimateParams{
				TaskKey:  args[0],
				Duration: args[1],
				Jira:     client,
				Getter:   client,
			})
			if err != nil {
				return err
			}

			PrintEstimateResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
