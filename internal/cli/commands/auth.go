package commands

import (
	"fmt"
	"strings"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// NewAuthCmd returns the `fylla auth` command tree.
func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate fylla with a task or calendar provider",
		Long:  "Authentication commands write credentials into the profile given by --profile.",
	}
	cmd.AddCommand(newAuthKendoCmd())
	cmd.AddCommand(newAuthTodoistCmd())
	cmd.AddCommand(newAuthGitHubCmd())
	cmd.AddCommand(newAuthGoogleCmd())
	return cmd
}

// requireExplicitProfile returns an error if --profile was not passed on
// the command line. Environment/pointer resolution is rejected to force
// callers to be explicit about which profile receives credentials.
func requireExplicitProfile(cmd *cobra.Command) error {
	flag := cmd.Root().PersistentFlags().Lookup("profile")
	if flag == nil || !flag.Changed {
		return fmt.Errorf("--profile is required for auth commands (pass it explicitly to target a specific profile)")
	}
	return nil
}

func newAuthKendoCmd() *cobra.Command {
	var url, token string
	cmd := &cobra.Command{
		Use:   "kendo",
		Short: "Store a Kendo API token for the profile given by --profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireExplicitProfile(cmd); err != nil {
				return err
			}
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			if url == "" {
				return fmt.Errorf("--url is required")
			}
			if err := saveProviderToken("kendo", token); err != nil {
				return err
			}
			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}
			if _, err := config.SetIn(cfgPath, "kendo.url", url); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored Kendo credentials for profile %q\n", config.ActiveProfile())
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Kendo base URL (e.g. https://yourapp.kendo.dev)")
	cmd.Flags().StringVar(&token, "token", "", "Kendo API token")
	return cmd
}

func newAuthTodoistCmd() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "todoist",
		Short: "Store a Todoist API token for the profile given by --profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireExplicitProfile(cmd); err != nil {
				return err
			}
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			if err := saveProviderToken("todoist", token); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored Todoist credentials for profile %q\n", config.ActiveProfile())
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Todoist API token")
	return cmd
}

func newAuthGitHubCmd() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Store a GitHub personal access token for the profile given by --profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireExplicitProfile(cmd); err != nil {
				return err
			}
			if token == "" {
				return fmt.Errorf("--token is required")
			}
			if err := saveProviderToken("github", token); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored GitHub credentials for profile %q\n", config.ActiveProfile())
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "GitHub personal access token")
	return cmd
}

func newAuthGoogleCmd() *cobra.Command {
	var clientCreds string
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Run the Google OAuth flow and store a calendar token for the profile given by --profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireExplicitProfile(cmd); err != nil {
				return err
			}
			if clientCreds == "" {
				return fmt.Errorf("--client-credentials is required (path to Google OAuth client JSON)")
			}
			oauthCfg, err := calendar.OAuthConfigFromFile(clientCreds)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Opening browser for Google authorization...")
			token, err := calendar.Authenticate(cmd.Context(), oauthCfg)
			if err != nil {
				return fmt.Errorf("google auth: %w", err)
			}
			creds := calendar.NewGoogleCredentials(oauthCfg, token)
			path, err := calendar.TokenPath()
			if err != nil {
				return err
			}
			if err := calendar.SaveGoogleCredentials(creds, path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored Google calendar credentials for profile %q\n", config.ActiveProfile())
			return nil
		},
	}
	cmd.Flags().StringVar(&clientCreds, "client-credentials", "", "path to Google OAuth client JSON")
	return cmd
}

// saveProviderToken writes a ProviderCredentials{Token: token} into the
// active profile's credential file for the given provider.
func saveProviderToken(provider, token string) error {
	path, err := config.DefaultProviderCredentialsPath(provider)
	if err != nil {
		return err
	}
	creds := &config.ProviderCredentials{Token: strings.TrimSpace(token)}
	return config.SaveProviderCredentials(creds, path)
}
