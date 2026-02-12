package commands

import "github.com/spf13/cobra"

// Register adds all subcommands to the root command.
func Register(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newEstimateCmd())
	rootCmd.AddCommand(newAddCmd())
	rootCmd.AddCommand(newDoneCmd())
	rootCmd.AddCommand(newNextCmd())
}
