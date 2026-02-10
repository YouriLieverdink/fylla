package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// LogParams holds inputs for the log command.
type LogParams struct {
	TaskKey     string
	Duration    time.Duration
	Description string
	Jira        WorklogPoster
}

// RunLog creates a manual worklog in Jira.
func RunLog(ctx context.Context, p LogParams) error {
	return p.Jira.PostWorklog(ctx, p.TaskKey, p.Duration, p.Description)
}

// PrintLogResult writes the log confirmation to the given writer.
func PrintLogResult(w io.Writer, taskKey string, duration time.Duration) {
	fmt.Fprintf(w, "Worklog added to %s: %s\n", taskKey, formatElapsed(duration))
}

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

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log TASK-KEY DURATION DESCRIPTION",
		Short: "Create manual worklog in Jira",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
