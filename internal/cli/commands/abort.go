package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/timer"
)

// AbortParams holds inputs for the abort command.
type AbortParams struct {
	TimerPath string
	Now       time.Time
}

// AbortResult holds the output of an abort operation.
type AbortResult struct {
	TaskKey    string
	ResumedKey string
}

// RunAbort aborts the running timer without logging work.
func RunAbort(p AbortParams) (*AbortResult, error) {
	r, err := timer.Abort(p.Now, p.TimerPath)
	if err != nil {
		return nil, err
	}
	result := &AbortResult{TaskKey: r.TaskKey}
	if r.Resumed != nil {
		result.ResumedKey = r.Resumed.TaskKey
	}
	return result, nil
}

// PrintAbortResult writes the abort result to the given writer.
func PrintAbortResult(w io.Writer, result *AbortResult) {
	fmt.Fprintf(w, "Timer aborted for %s\n", result.TaskKey)
	if result.ResumedKey != "" {
		fmt.Fprintf(w, "Resumed timer for %s\n", result.ResumedKey)
	}
}
