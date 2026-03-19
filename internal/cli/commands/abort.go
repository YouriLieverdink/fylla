package commands

import (
	"fmt"
	"io"

	"github.com/iruoy/fylla/internal/timer"
)

// AbortParams holds inputs for the abort command.
type AbortParams struct {
	TimerPath string
}

// AbortResult holds the output of an abort operation.
type AbortResult struct {
	TaskKey string
}

// RunAbort aborts the running timer without logging work.
func RunAbort(p AbortParams) (*AbortResult, error) {
	s, err := timer.Abort(p.TimerPath)
	if err != nil {
		return nil, err
	}
	return &AbortResult{TaskKey: s.TaskKey}, nil
}

// PrintAbortResult writes the abort result to the given writer.
func PrintAbortResult(w io.Writer, result *AbortResult) {
	fmt.Fprintf(w, "Timer aborted for %s\n", result.TaskKey)
}
