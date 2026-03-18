package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
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

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done TASK-KEY [TASK-KEY...]",
		Short: "Mark one or more tasks as done",
		Args:  cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				result, err := RunDone(cmd.Context(), DoneParams{
					TaskKey:   args[0],
					Completer: source,
				})
				if err != nil {
					return err
				}
				PrintDoneResult(cmd.OutOrStdout(), result)
			} else {
				ctx := cmd.Context()
				results := RunBatch(args, func(key string) error {
					_, err := RunDone(ctx, DoneParams{
						TaskKey:   key,
						Completer: source,
					})
					return err
				})
				PrintBatchResults(cmd.OutOrStdout(), results, "marked as done")
			}

			maybeAutoResync(cmd.Context(), cmd.ErrOrStderr())
			return nil
		},
	}
}
