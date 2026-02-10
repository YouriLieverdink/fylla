package cli

import (
	"github.com/iruoy/fylla/internal/cli/commands"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root cobra command for fylla.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "fylla",
		Short: "Fylla - Fill your calendar with what matters",
		Long:  "A CLI tool that pulls Jira tasks, sorts them by priority rules, and schedules them into free slots on Google Calendar.",
	}

	commands.Register(rootCmd)

	return rootCmd
}
