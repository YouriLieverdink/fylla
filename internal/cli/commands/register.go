package commands

import "github.com/spf13/cobra"

// Register adds all subcommands to the root command.
func Register(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newTaskCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newTodayCmd())
	rootCmd.AddCommand(newNextCmd())
	rootCmd.AddCommand(newClearCmd())
	rootCmd.AddCommand(newTimerCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newWorklogCmd())
	rootCmd.AddCommand(newReportCmd())
}
