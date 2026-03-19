package commands

import (
	"fmt"
	"io"
	"os"

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
