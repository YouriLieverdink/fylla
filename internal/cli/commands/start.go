package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
)

// StartParams holds inputs for the start command.
type StartParams struct {
	TaskKey  string
	Project  string
	Section  string
	Provider string
	Summary  string
	// WorklogTarget is a pre-selected worklog destination for tasks whose own
	// provider isn't the worklog provider (set by the TUI's upfront picker).
	WorklogTarget string
	TimerPath     string
	Now           time.Time
}

// RunStart begins a timer for the specified task.
func RunStart(p StartParams) error {
	return timer.Start(p.TaskKey, p.Project, p.Section, p.Provider, p.Summary, p.WorklogTarget, p.Now, p.TimerPath)
}

// PrintStartResult writes the start confirmation to the given writer.
func PrintStartResult(w io.Writer, taskKey string) {
	fmt.Fprintf(w, "Started timer for %s\n", taskKey)
}
