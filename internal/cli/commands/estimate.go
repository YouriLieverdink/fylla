package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var durationRe = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?$`)

// ParseDuration parses a human-friendly duration string like "2h", "30m", or "1h30m".
func ParseDuration(s string) (time.Duration, error) {
	matches := durationRe.FindStringSubmatch(s)
	if matches == nil || (matches[1] == "" && matches[2] == "") {
		return 0, fmt.Errorf("invalid duration %q (expected format: 2h, 30m, 1h30m)", s)
	}

	var d time.Duration
	if matches[1] != "" {
		h, _ := strconv.Atoi(matches[1])
		d += time.Duration(h) * time.Hour
	}
	if matches[2] != "" {
		m, _ := strconv.Atoi(matches[2])
		d += time.Duration(m) * time.Minute
	}
	return d, nil
}

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

