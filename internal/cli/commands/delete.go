package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// TaskDeleter abstracts permanently deleting a task for testing.
type TaskDeleter interface {
	DeleteTask(ctx context.Context, taskKey string) error
}

// DeleteParams holds inputs for the delete command.
type DeleteParams struct {
	TaskKey string
	Deleter TaskDeleter
}

// DeleteResult holds the output of a delete operation.
type DeleteResult struct {
	TaskKey string
}

// RunDelete permanently deletes a task using the configured source.
func RunDelete(ctx context.Context, p DeleteParams) (*DeleteResult, error) {
	if err := p.Deleter.DeleteTask(ctx, p.TaskKey); err != nil {
		return nil, fmt.Errorf("delete task: %w", err)
	}
	return &DeleteResult{TaskKey: p.TaskKey}, nil
}

// PrintDeleteResult writes the delete confirmation to the given writer.
func PrintDeleteResult(w io.Writer, result *DeleteResult) {
	fmt.Fprintf(w, "Deleted %s\n", result.TaskKey)
}

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete TASK-KEY",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			result, err := RunDelete(cmd.Context(), DeleteParams{
				TaskKey: args[0],
				Deleter: source,
			})
			if err != nil {
				return err
			}

			PrintDeleteResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
