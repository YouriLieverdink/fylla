package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iruoy/fylla/internal/config"
)

// ConfigShowParams holds inputs for the config show command.
type ConfigShowParams struct {
	ConfigPath string
}

// RunConfigShow loads and returns the config as YAML text.
func RunConfigShow(p ConfigShowParams) (string, error) {
	cfg, err := config.LoadFrom(p.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(data), nil
}

// PrintConfigShow writes the config YAML to the given writer.
func PrintConfigShow(w io.Writer, yamlText string) {
	fmt.Fprint(w, yamlText)
}

// ConfigEditParams holds inputs for the config edit command.
type ConfigEditParams struct {
	ConfigPath string
	Editor     string
}

// ResolveEditor returns the editor to use, falling back to "vi".
func ResolveEditor(editor string) string {
	if editor != "" {
		return editor
	}
	if env := os.Getenv("EDITOR"); env != "" {
		return env
	}
	return "vi"
}

// ConfigSetParams holds inputs for the config set command.
type ConfigSetParams struct {
	ConfigPath string
	Key        string
	Value      string
}

// RunConfigSet sets a value at a dotted key path in the config file.
func RunConfigSet(p ConfigSetParams) (*config.Config, error) {
	return config.SetIn(p.ConfigPath, p.Key, p.Value)
}

// PrintConfigSet writes the set confirmation to the given writer.
func PrintConfigSet(w io.Writer, key, value string) {
	fmt.Fprintf(w, "Set %s = %s\n", key, value)
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigSetCmd())

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Load(); err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			path, err := config.DefaultPath()
			if err != nil {
				return err
			}

			editor := ResolveEditor("")
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return config.KeyPaths(), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.Set(args[0], args[1])
			if err != nil {
				return fmt.Errorf("set config: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}
