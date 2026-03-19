package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/iruoy/fylla/internal/task"
)

// TaskCompleter abstracts marking a task as done for testing.
type TaskCompleter interface {
	CompleteTask(ctx context.Context, taskKey string) error
}

// ProviderTaskCompleter extends TaskCompleter with provider-aware completion.
type ProviderTaskCompleter interface {
	TaskCompleter
	CompleteTaskOn(ctx context.Context, taskKey, provider string) error
}

// DoneParams holds inputs for the done command.
type DoneParams struct {
	TaskKey   string
	Provider  string
	Completer TaskCompleter
}

// DoneResult holds the output of a done operation.
type DoneResult struct {
	TaskKey string
}

// RunDone marks a task as complete using the configured source.
// If the key has a recurrence instance suffix (@YYYY-MM-DD), it is stripped
// so the original task key is used for the provider call.
func RunDone(ctx context.Context, p DoneParams) (*DoneResult, error) {
	key, _ := task.StripInstanceSuffix(p.TaskKey)
	var err error
	if p.Provider != "" {
		if pc, ok := p.Completer.(ProviderTaskCompleter); ok {
			err = pc.CompleteTaskOn(ctx, key, p.Provider)
		} else {
			err = p.Completer.CompleteTask(ctx, key)
		}
	} else {
		err = p.Completer.CompleteTask(ctx, key)
	}
	if err != nil {
		return nil, fmt.Errorf("complete task: %w", err)
	}
	return &DoneResult{TaskKey: p.TaskKey}, nil
}

// PrintDoneResult writes the done confirmation to the given writer.
func PrintDoneResult(w io.Writer, result *DoneResult) {
	fmt.Fprintf(w, "Marked %s as done\n", result.TaskKey)
}
