package commands

import "github.com/spf13/cobra"

// Register adds all subcommands to the root command.
func Register(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newTaskCmd())
	rootCmd.AddCommand(newScheduleCmd())
	rootCmd.AddCommand(newTimerCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newInitCmd())
}
