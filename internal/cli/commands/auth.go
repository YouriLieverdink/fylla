package commands

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// OAuthAuthenticator abstracts the Google OAuth flow for testing.
type OAuthAuthenticator interface {
	Authenticate(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error)
}

// defaultOAuthAuthenticator delegates to the calendar package.
type defaultOAuthAuthenticator struct{}

func (d defaultOAuthAuthenticator) Authenticate(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	return calendar.Authenticate(ctx, cfg)
}

// AuthJiraParams holds inputs for the Jira auth operation.
type AuthJiraParams struct {
	URL        string
	Email      string
	Token      string
	ConfigPath string
}

// RunAuthJira stores Jira credentials: url, email, and credential path in config; token in per-provider file.
func RunAuthJira(p AuthJiraParams) error {
	cfg, err := config.LoadFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	credPath := cfg.Jira.Credentials
	if credPath == "" {
		credPath = filepath.Join(filepath.Dir(p.ConfigPath), "jira_credentials.json")
	}

	if _, err := config.SetMultiIn(p.ConfigPath, map[string]string{
		"jira.url":         p.URL,
		"jira.email":       p.Email,
		"jira.credentials": credPath,
	}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if err := config.SaveProviderCredentials(&config.ProviderCredentials{Token: p.Token}, credPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	return nil
}

// AuthGoogleParams holds inputs for the Google auth operation.
type AuthGoogleParams struct {
	ClientFile      string // input: Google OAuth client credentials JSON
	CredentialsPath string // output: where to save combined google_credentials.json
	ConfigPath      string
	Auth            OAuthAuthenticator
}

// RunAuthGoogle runs the Google OAuth flow and saves client config + token together.
// When ConfigPath is set, the credentials path is saved to config.
func RunAuthGoogle(ctx context.Context, p AuthGoogleParams) error {
	// Try loading existing credentials and refreshing the token
	if p.CredentialsPath != "" {
		creds, err := calendar.LoadGoogleCredentials(p.CredentialsPath)
		if err == nil && creds.Token != nil {
			if refreshErr := calendar.EnsureValidToken(ctx, creds); refreshErr == nil {
				if saveErr := calendar.SaveGoogleCredentials(creds, p.CredentialsPath); saveErr != nil {
					return fmt.Errorf("save refreshed credentials: %w", saveErr)
				}
				return nil
			}
		}
	}

	// Need a client file to create new credentials
	if p.ClientFile == "" {
		return fmt.Errorf("no existing credentials and no --client-credentials provided; run 'fylla auth google --client-credentials path/to/client.json'")
	}

	oauthCfg, err := calendar.OAuthConfigFromFile(p.ClientFile)
	if err != nil {
		return fmt.Errorf("load client credentials: %w", err)
	}

	token, err := p.Auth.Authenticate(ctx, oauthCfg)
	if err != nil {
		return fmt.Errorf("google auth: %w", err)
	}

	creds := calendar.NewGoogleCredentials(oauthCfg, token)
	if err := calendar.SaveGoogleCredentials(creds, p.CredentialsPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	if p.ConfigPath != "" {
		if _, err := config.SetMultiIn(p.ConfigPath, map[string]string{
			"calendar.credentials": p.CredentialsPath,
		}); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	return nil
}

// AuthTodoistParams holds inputs for the Todoist auth operation.
type AuthTodoistParams struct {
	Token      string
	ConfigPath string
}

// RunAuthTodoist stores the Todoist API token in a per-provider credential file.
func RunAuthTodoist(p AuthTodoistParams) error {
	cfg, err := config.LoadFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	credPath := cfg.Todoist.Credentials
	if credPath == "" {
		credPath = filepath.Join(filepath.Dir(p.ConfigPath), "todoist_credentials.json")
	}

	if _, err := config.SetMultiIn(p.ConfigPath, map[string]string{
		"todoist.credentials": credPath,
	}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if err := config.SaveProviderCredentials(&config.ProviderCredentials{Token: p.Token}, credPath); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	return nil
}

// AuthGitHubParams holds inputs for the GitHub auth operation.
type AuthGitHubParams struct {
	Token      string
	ConfigPath string
}

// RunAuthGitHub stores the GitHub PAT in a per-provider credential file.
func RunAuthGitHub(p AuthGitHubParams) error {
	cfg, err := config.LoadFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	credPath := cfg.GitHub.Credentials
	if credPath == "" {
		credPath = filepath.Join(filepath.Dir(p.ConfigPath), "github_credentials.json")
	}

	if _, err := config.SetMultiIn(p.ConfigPath, map[string]string{
		"github.credentials": credPath,
	}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if err := config.SaveProviderCredentials(&config.ProviderCredentials{Token: p.Token}, credPath); err != nil {
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
	cmd.AddCommand(newAuthGitHubCmd())

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

			if err := RunAuthJira(AuthJiraParams{
				URL:        url,
				Email:      email,
				Token:      token,
				ConfigPath: cfgPath,
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

			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}

			if err := RunAuthTodoist(AuthTodoistParams{
				Token:      token,
				ConfigPath: cfgPath,
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
			clientFile, _ := cmd.Flags().GetString("client-credentials")

			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}

			// Determine output credentials path
			credPath := ""
			cfg, loadErr := config.LoadFrom(cfgPath)
			if loadErr == nil && cfg.Calendar.Credentials != "" {
				credPath = cfg.Calendar.Credentials
			}
			if credPath == "" {
				credPath, err = calendar.TokenPath()
				if err != nil {
					return err
				}
			}

			if err := RunAuthGoogle(cmd.Context(), AuthGoogleParams{
				ClientFile:      clientFile,
				CredentialsPath: credPath,
				ConfigPath:      cfgPath,
				Auth:            defaultOAuthAuthenticator{},
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

func newAuthGitHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Configure GitHub authentication",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, _ := cmd.Flags().GetString("token")
			if token == "" {
				return fmt.Errorf("--token is required")
			}

			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}

			if err := RunAuthGitHub(AuthGitHubParams{
				Token:      token,
				ConfigPath: cfgPath,
			}); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "GitHub credentials stored successfully.")
			return nil
		},
	}

	cmd.Flags().String("token", "", "GitHub personal access token")

	return cmd
}

