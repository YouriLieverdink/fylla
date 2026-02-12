package commands

import "github.com/spf13/cobra"

func newScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Calendar scheduling",
	}

	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newNextCmd())

	return cmd
}
