package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/spf13/cobra"
)

// Surveyor abstracts interactive prompts for testing.
type Surveyor interface {
	Select(message string, options []string) (string, error)
	Input(message string) (string, error)
	Password(message string) (string, error)
}

// defaultSurveyor uses AlecAivazis/survey for real interactive prompts.
type defaultSurveyor struct{}

func (d defaultSurveyor) Select(message string, options []string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
	}, &answer)
	return answer, err
}

func (d defaultSurveyor) Input(message string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Input{Message: message}, &answer)
	return answer, err
}

func (d defaultSurveyor) Password(message string) (string, error) {
	var answer string
	err := survey.AskOne(&survey.Password{Message: message}, &answer)
	return answer, err
}

// InitParams holds all inputs for the init command.
type InitParams struct {
	Survey          Surveyor
	Auth            OAuthAuthenticator
	ConfigPath      string
	CredentialsPath string
	TokenPath       string
}

// RunInit walks through first-time setup interactively.
func RunInit(ctx context.Context, w io.Writer, p InitParams) error {
	// Ensure config exists
	cfg, err := config.LoadFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 1. Source selection
	source, err := p.Survey.Select("Task source:", []string{"jira", "todoist"})
	if err != nil {
		return fmt.Errorf("source selection: %w", err)
	}
	cfg.Source = source
	if err := config.SaveTo(cfg, p.ConfigPath); err != nil {
		return fmt.Errorf("save source: %w", err)
	}
	fmt.Fprintf(w, "Source set to %s.\n", source)

	// 2. Source credentials
	switch source {
	case "jira":
		url, err := p.Survey.Input("Jira URL (e.g. https://company.atlassian.net):")
		if err != nil {
			return fmt.Errorf("jira url prompt: %w", err)
		}
		email, err := p.Survey.Input("Jira email:")
		if err != nil {
			return fmt.Errorf("jira email prompt: %w", err)
		}
		token, err := p.Survey.Password("Jira API token:")
		if err != nil {
			return fmt.Errorf("jira token prompt: %w", err)
		}
		if err := RunAuthJira(AuthJiraParams{
			URL:             url,
			Email:           email,
			Token:           token,
			ConfigPath:      p.ConfigPath,
			CredentialsPath: p.CredentialsPath,
		}); err != nil {
			return fmt.Errorf("auth jira: %w", err)
		}
		fmt.Fprintln(w, "Jira credentials stored.")
	case "todoist":
		token, err := p.Survey.Password("Todoist API token:")
		if err != nil {
			return fmt.Errorf("todoist token prompt: %w", err)
		}
		if err := RunAuthTodoist(AuthTodoistParams{
			Token:           token,
			CredentialsPath: p.CredentialsPath,
		}); err != nil {
			return fmt.Errorf("auth todoist: %w", err)
		}
		fmt.Fprintln(w, "Todoist credentials stored.")
	}

	// 3. Google Calendar
	credFile, err := p.Survey.Input("Path to Google client_credentials.json:")
	if err != nil {
		return fmt.Errorf("credentials path prompt: %w", err)
	}

	cfg, err = config.LoadFrom(p.ConfigPath)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	cfg.Calendar.ClientCredentials = credFile
	if err := config.SaveTo(cfg, p.ConfigPath); err != nil {
		return fmt.Errorf("save calendar config: %w", err)
	}

	if err := RunAuthGoogle(ctx, AuthGoogleParams{
		ClientCredentialsPath: credFile,
		TokenPath:             p.TokenPath,
		Auth:                  p.Auth,
	}); err != nil {
		return fmt.Errorf("auth google: %w", err)
	}
	fmt.Fprintln(w, "Google Calendar authenticated.")

	// 4. Next steps
	fmt.Fprintln(w, "\nSetup complete! Try:")
	fmt.Fprintln(w, "  fylla task list")
	fmt.Fprintln(w, "  fylla schedule sync")
	return nil
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive first-time setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}

			// Ensure default config exists
			if _, loadErr := config.Load(); loadErr != nil {
				return loadErr
			}

			credPath, err := config.CredentialsPath()
			if err != nil {
				return err
			}

			tokenPath, err := calendar.TokenPath()
			if err != nil {
				return err
			}

			return RunInit(cmd.Context(), cmd.OutOrStdout(), InitParams{
				Survey:          defaultSurveyor{},
				Auth:            defaultOAuthAuthenticator{},
				ConfigPath:      cfgPath,
				CredentialsPath: credPath,
				TokenPath:       tokenPath,
			})
		},
	}
}
