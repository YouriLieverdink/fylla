package commands

import (
	"context"
	"fmt"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// OAuthAuthenticator abstracts the Google OAuth flow for testing.
type OAuthAuthenticator interface {
	CachedToken(ctx context.Context, cfg *oauth2.Config, tokenPath string) (*oauth2.Token, error)
}

// defaultOAuthAuthenticator delegates to the calendar package.
type defaultOAuthAuthenticator struct{}

func (d defaultOAuthAuthenticator) CachedToken(ctx context.Context, cfg *oauth2.Config, tokenPath string) (*oauth2.Token, error) {
	return calendar.CachedToken(ctx, cfg, tokenPath)
}

// AuthJiraParams holds inputs for the Jira auth operation.
type AuthJiraParams struct {
	URL             string
	Email           string
	Token           string
	ConfigPath      string
	CredentialsPath string
}

// RunAuthJira stores Jira credentials: url and email in config, token in credentials.
func RunAuthJira(p AuthJiraParams) error {
	cfg, err := config.LoadFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg.Jira.URL = p.URL
	cfg.Jira.Email = p.Email

	if err := config.SaveTo(cfg, p.ConfigPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	creds, err := config.LoadCredentialsFrom(p.CredentialsPath)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	creds.JiraToken = p.Token
	if err := config.SaveCredentialsTo(creds, p.CredentialsPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	return nil
}

// AuthGoogleParams holds inputs for the Google auth operation.
type AuthGoogleParams struct {
	ClientCredentialsPath string
	TokenPath             string
	Auth                  OAuthAuthenticator
}

// RunAuthGoogle runs the Google OAuth flow and caches the token.
func RunAuthGoogle(ctx context.Context, p AuthGoogleParams) error {
	oauthCfg, err := calendar.OAuthConfigFromFile(p.ClientCredentialsPath)
	if err != nil {
		return fmt.Errorf("load client credentials: %w", err)
	}

	_, err = p.Auth.CachedToken(ctx, oauthCfg, p.TokenPath)
	if err != nil {
		return fmt.Errorf("google auth: %w", err)
	}

	return nil
}

// AuthTodoistParams holds inputs for the Todoist auth operation.
type AuthTodoistParams struct {
	Token           string
	CredentialsPath string
}

// RunAuthTodoist stores the Todoist API token in credentials.
func RunAuthTodoist(p AuthTodoistParams) error {
	creds, err := config.LoadCredentialsFrom(p.CredentialsPath)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	creds.TodoistToken = p.Token
	if err := config.SaveCredentialsTo(creds, p.CredentialsPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	return nil
}

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}

	cmd.AddCommand(newAuthJiraCmd())
	cmd.AddCommand(newAuthGoogleCmd())
	cmd.AddCommand(newAuthTodoistCmd())

	return cmd
}

func newAuthJiraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jira",
		Short: "Configure Jira authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			email, _ := cmd.Flags().GetString("email")
			token, _ := cmd.Flags().GetString("token")

			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}

			// Fall back to existing config values for url and email
			if url == "" || email == "" {
				cfg, err := config.LoadFrom(cfgPath)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if url == "" {
					url = cfg.Jira.URL
				}
				if email == "" {
					email = cfg.Jira.Email
				}
			}

			if url == "" {
				return fmt.Errorf("--url is required (no existing value in config)")
			}
			if email == "" {
				return fmt.Errorf("--email is required (no existing value in config)")
			}
			if token == "" {
				return fmt.Errorf("--token is required")
			}

			credPath, err := config.CredentialsPath()
			if err != nil {
				return err
			}

			if err := RunAuthJira(AuthJiraParams{
				URL:             url,
				Email:           email,
				Token:           token,
				ConfigPath:      cfgPath,
				CredentialsPath: credPath,
			}); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Jira credentials stored successfully.")
			return nil
		},
	}

	cmd.Flags().String("url", "", "Jira instance URL")
	cmd.Flags().String("email", "", "Jira email address")
	cmd.Flags().String("token", "", "Jira API token")

	return cmd
}

func newAuthTodoistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todoist",
		Short: "Configure Todoist authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			if token == "" {
				return fmt.Errorf("--token is required")
			}

			credPath, err := config.CredentialsPath()
			if err != nil {
				return err
			}

			if err := RunAuthTodoist(AuthTodoistParams{
				Token:           token,
				CredentialsPath: credPath,
			}); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Todoist credentials stored successfully.")
			return nil
		},
	}

	cmd.Flags().String("token", "", "Todoist API token")

	return cmd
}

func newAuthGoogleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Authenticate with Google Calendar via OAuth",
		RunE: func(cmd *cobra.Command, args []string) error {
			credFile, _ := cmd.Flags().GetString("client-credentials")
			if credFile == "" {
				cfg, err := config.Load()
				if err == nil && cfg.Calendar.ClientCredentials != "" {
					credFile = cfg.Calendar.ClientCredentials
				}
			}
			if credFile == "" {
				return fmt.Errorf("set calendar.clientCredentials in config or pass --client-credentials")
			}

			tokenPath, err := calendar.TokenPath()
			if err != nil {
				return err
			}

			if err := RunAuthGoogle(cmd.Context(), AuthGoogleParams{
				ClientCredentialsPath: credFile,
				TokenPath:             tokenPath,
				Auth:                  defaultOAuthAuthenticator{},
			}); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Google authentication successful.")
			return nil
		},
	}

	cmd.Flags().String("client-credentials", "", "Path to Google OAuth client credentials JSON file")

	return cmd
}
