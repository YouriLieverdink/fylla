package commands

import "github.com/spf13/cobra"

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show currently running task and elapsed time",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
