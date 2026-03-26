package commands

import (
	"context"
	"fmt"
	"io"
)

// DeleteParams holds inputs for the delete command.
type DeleteParams struct {
	TaskKey  string
	Provider string
	Deleter  TaskDeleter
}

// DeleteResult holds the output of a delete operation.
type DeleteResult struct {
	TaskKey string
}

// RunDelete permanently deletes a task using the configured source.
func RunDelete(ctx context.Context, p DeleteParams) (*DeleteResult, error) {
	if err := routedSource(p.Deleter, p.Provider).DeleteTask(ctx, p.TaskKey); err != nil {
		return nil, fmt.Errorf("delete task: %w", err)
	}
	return &DeleteResult{TaskKey: p.TaskKey}, nil
}

// PrintDeleteResult writes the delete confirmation to the given writer.
func PrintDeleteResult(w io.Writer, result *DeleteResult) {
	fmt.Fprintf(w, "Deleted %s\n", result.TaskKey)
}
