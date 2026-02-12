package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// TaskCompleter abstracts marking a task as done for testing.
type TaskCompleter interface {
	CompleteTask(ctx context.Context, taskKey string) error
}

// DoneParams holds inputs for the done command.
type DoneParams struct {
	TaskKey   string
	Completer TaskCompleter
}

// DoneResult holds the output of a done operation.
type DoneResult struct {
	TaskKey string
}

// RunDone marks a task as complete using the configured source.
func RunDone(ctx context.Context, p DoneParams) (*DoneResult, error) {
	if err := p.Completer.CompleteTask(ctx, p.TaskKey); err != nil {
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
		Use:   "done TASK-KEY",
		Short: "Mark a task as done",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			result, err := RunDone(cmd.Context(), DoneParams{
				TaskKey:   args[0],
				Completer: source,
			})
			if err != nil {
				return err
			}

			PrintDoneResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
