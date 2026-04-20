package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// NewProfileCmd returns the `fylla profile` command tree.
func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage configuration profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileCurrentCmd())
	cmd.AddCommand(newProfileUseCmd())
	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileDeleteCmd())
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles (current marked with *)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(cmd.OutOrStdout())
		},
	}
}

func runProfileList(w io.Writer) error {
	names, err := config.ListProfiles()
	if err != nil {
		return err
	}
	current, _ := config.ReadPointer()
	if len(names) == 0 {
		fmt.Fprintln(w, "no profiles found")
		return nil
	}
	for _, n := range names {
		prefix := "  "
		if n == current {
			prefix = "* "
		}
		fmt.Fprintf(w, "%s%s\n", prefix, n)
	}
	return nil
}

func newProfileCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the current profile name",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := config.ReadPointer()
			if err != nil {
				return err
			}
			if name == "" {
				name = config.DefaultProfileName
			}
			fmt.Fprintln(cmd.OutOrStdout(), name)
			return nil
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the current profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := config.ValidateProfileName(name); err != nil {
				return err
			}
			exists, err := config.ProfileExists(name)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("profile %q not found", name)
			}
			if err := config.WritePointer(name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to profile %q\n", name)
			return nil
		},
	}
}

func newProfileCreateCmd() *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile (empty template by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := config.CreateProfile(name, from); err != nil {
				return err
			}
			if from == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Created profile %q from template\n", name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Created profile %q from %q\n", name, from)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "copy contents from an existing profile")
	return cmd
}

func newProfileDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a profile and all its state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete profile %q and all its credentials? [y/N] ", name)
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				if strings.ToLower(strings.TrimSpace(line)) != "y" {
					fmt.Fprintln(cmd.ErrOrStderr(), "Aborted")
					return nil
				}
			}
			if err := config.DeleteProfile(name, force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted profile %q\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation and allow deleting the current profile")
	return cmd
}
