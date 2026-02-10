package commands

import "github.com/spf13/cobra"

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start TASK-KEY",
		Short: "Start timer for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
