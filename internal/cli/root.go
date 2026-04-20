package cli

import (
	"fmt"
	"os"

	"github.com/iruoy/fylla/internal/cli/commands"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root cobra command for fylla.
func NewRootCmd() *cobra.Command {
	var profileFlag string

	cmd := &cobra.Command{
		Use:   "fylla",
		Short: "Fylla - Fill your calendar with what matters",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			migrated, err := config.MigrateLegacyLayout()
			if err != nil {
				return fmt.Errorf("migrate legacy layout: %w", err)
			}
			if migrated {
				fmt.Fprintln(os.Stderr, "fylla: migrated existing config into profiles/default/")
			}
			// Bootstrap default profile on fresh installs so that
			// ResolveProfile below does not fail when the pointer is absent.
			if name := config.ActiveProfile(); name == config.DefaultProfileName {
				exists, err := config.ProfileExists(config.DefaultProfileName)
				if err != nil {
					return err
				}
				if !exists {
					// Temporarily point at default and bootstrap.
					config.SetActiveProfile(config.DefaultProfileName)
					if err := config.BootstrapActiveProfile(); err != nil {
						return err
					}
					if err := config.WritePointer(config.DefaultProfileName); err != nil {
						return err
					}
				}
			}

			name, err := config.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			config.SetActiveProfile(name)
			return config.BootstrapActiveProfile()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.RunServe(cmd.Context())
		},
	}

	cmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "profile name (overrides FYLLA_PROFILE and pointer file)")
	cmd.AddCommand(commands.NewProfileCmd())
	cmd.AddCommand(commands.NewAuthCmd())
	return cmd
}
