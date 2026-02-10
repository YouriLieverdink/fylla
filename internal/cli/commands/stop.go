package commands

import "github.com/spf13/cobra"

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop timer and log work to Jira",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().StringP("description", "d", "", "Work description (skips interactive prompt)")

	return cmd
}
