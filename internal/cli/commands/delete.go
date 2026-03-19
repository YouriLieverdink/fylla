package commands

import (
	"context"
	"fmt"
	"io"
)

// TaskDeleter abstracts permanently deleting a task for testing.
type TaskDeleter interface {
	DeleteTask(ctx context.Context, taskKey string) error
}

// ProviderTaskDeleter extends TaskDeleter with provider-aware deletion.
type ProviderTaskDeleter interface {
	TaskDeleter
	DeleteTaskOn(ctx context.Context, taskKey, provider string) error
}

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
	var err error
	if p.Provider != "" {
		if pd, ok := p.Deleter.(ProviderTaskDeleter); ok {
			err = pd.DeleteTaskOn(ctx, p.TaskKey, p.Provider)
		} else {
			err = p.Deleter.DeleteTask(ctx, p.TaskKey)
		}
	} else {
		err = p.Deleter.DeleteTask(ctx, p.TaskKey)
	}
	if err != nil {
		return nil, fmt.Errorf("delete task: %w", err)
	}
	return &DeleteResult{TaskKey: p.TaskKey}, nil
}

// PrintDeleteResult writes the delete confirmation to the given writer.
func PrintDeleteResult(w io.Writer, result *DeleteResult) {
	fmt.Fprintf(w, "Deleted %s\n", result.TaskKey)
}
