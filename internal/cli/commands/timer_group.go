package commands

import "github.com/spf13/cobra"

func newTimerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timer",
		Short: "Track time on tasks",
	}

	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newAbortCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLogCmd())
	cmd.AddCommand(newCommentCmd())

	return cmd
}
