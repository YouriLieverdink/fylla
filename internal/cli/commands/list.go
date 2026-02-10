package commands

import "github.com/spf13/cobra"

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show sorted tasks without scheduling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
