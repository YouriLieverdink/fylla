package commands

import "github.com/spf13/cobra"

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log TASK-KEY DURATION DESCRIPTION",
		Short: "Create manual worklog in Jira",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
