package commands

import "github.com/spf13/cobra"

func newEstimateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "estimate TASK-KEY DURATION",
		Short: "Set or adjust remaining estimate on a Jira task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
