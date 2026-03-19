package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
)

// StartParams holds inputs for the start command.
type StartParams struct {
	TaskKey   string
	Project   string
	Section   string
	Provider  string
	TimerPath string
	Now       time.Time
}

// RunStart begins a timer for the specified task.
func RunStart(p StartParams) (*timer.State, error) {
	return timer.Start(p.TaskKey, p.Project, p.Section, p.Provider, p.Now, p.TimerPath)
}

// PrintStartResult writes the start confirmation to the given writer.
func PrintStartResult(w io.Writer, state *timer.State) {
	fmt.Fprintf(w, "Started timer for %s\n", state.TaskKey)
}
