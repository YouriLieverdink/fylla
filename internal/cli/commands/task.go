package commands

import "github.com/spf13/cobra"

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}

	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newEstimateCmd())
	cmd.AddCommand(newDueDateCmd())
	cmd.AddCommand(newPriorityCmd())

	return cmd
}
