package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultPath returns the default config file path (~/.config/fylla/config.yaml).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "fylla", "config.yaml"), nil
}

// LoadFrom reads and parses a YAML config file at the given path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// Load reads the config from the default path, creating it from defaults if missing.
func Load() (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}
		if err := os.WriteFile(path, defaultConfigYAML, 0644); err != nil {
			return nil, fmt.Errorf("write default config: %w", err)
		}
	}
	return LoadFrom(path)
}

// SaveTo marshals the config and writes it to the given path.
func SaveTo(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Save writes the config to the default path.
func Save(cfg *Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return SaveTo(cfg, path)
}

// SetIn reads a YAML file, sets a dotted key path to the given value, writes back,
// and returns the parsed Config.
func SetIn(path, key, value string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("invalid config document")
	}

	parts := strings.Split(key, ".")
	if err := setNode(doc.Content[0], parts, value); err != nil {
		return nil, fmt.Errorf("set %s: %w", key, err)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		return nil, fmt.Errorf("parse updated config: %w", err)
	}
	return &cfg, nil
}

// Set updates a value at the given dotted key path in the default config file.
func Set(key, value string) (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return SetIn(path, key, value)
}

func setNode(node *yaml.Node, parts []string, value string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node")
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		if keyNode.Value == parts[0] {
			if len(parts) == 1 {
				applyValue(valNode, value)
				return nil
			}
			return setNode(valNode, parts[1:], value)
		}
	}

	return fmt.Errorf("key %q not found", parts[0])
}

func applyValue(node *yaml.Node, value string) {
	if n, err := strconv.Atoi(value); err == nil {
		node.Kind = yaml.ScalarNode
		node.Tag = "!!int"
		node.Value = strconv.Itoa(n)
		node.Content = nil
		return
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		node.Kind = yaml.ScalarNode
		node.Tag = "!!float"
		node.Value = strconv.FormatFloat(f, 'f', -1, 64)
		node.Content = nil
		return
	}
	if b, err := strconv.ParseBool(value); err == nil {
		node.Kind = yaml.ScalarNode
		node.Tag = "!!bool"
		node.Value = strconv.FormatBool(b)
		node.Content = nil
		return
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		items := strings.Split(inner, ",")
		node.Kind = yaml.SequenceNode
		node.Tag = "!!seq"
		node.Value = ""
		node.Content = nil
		for _, item := range items {
			item = strings.TrimSpace(item)
			child := &yaml.Node{Kind: yaml.ScalarNode, Value: item}
			applyValue(child, item)
			node.Content = append(node.Content, child)
		}
		return
	}
	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	node.Value = value
	node.Content = nil
}
