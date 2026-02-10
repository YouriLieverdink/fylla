package commands

import "github.com/spf13/cobra"

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}

	cmd.AddCommand(newAuthJiraCmd())
	cmd.AddCommand(newAuthGoogleCmd())

	return cmd
}

func newAuthJiraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jira",
		Short: "Configure Jira authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().String("url", "", "Jira instance URL")
	cmd.Flags().String("email", "", "Jira email address")
	cmd.Flags().String("token", "", "Jira API token")

	return cmd
}

func newAuthGoogleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "google",
		Short: "Authenticate with Google Calendar via OAuth",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}
