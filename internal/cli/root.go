package cli

import (
	"github.com/iruoy/fylla/internal/cli/commands"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root cobra command for fylla.
func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fylla",
		Short: "Fylla - Fill your calendar with what matters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunServe(cmd.Context())
		},
	}
}
