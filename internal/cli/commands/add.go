package commands

import (
	"github.com/spf13/cobra"

	_ "github.com/AlecAivazis/survey/v2"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new Jira task interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().Bool("quick", false, "Quick mode - only essential fields")
	cmd.Flags().String("project", "", "Pre-select project")

	return cmd
}
