package commands

import "github.com/spf13/cobra"

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Schedule Jira tasks into Google Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Preview schedule without creating events")
	cmd.Flags().String("jql", "", "Custom JQL query override")
	cmd.Flags().Int("days", 0, "Override scheduling window (days)")
	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD)")

	return cmd
}
