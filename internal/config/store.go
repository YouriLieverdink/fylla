package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// RootDir returns the fylla config root directory following XDG conventions.
// This directory holds the profile pointer file and the profiles/ subdirectory.
func RootDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "fylla"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "fylla"), nil
}

// DefaultProfileName is the name of the profile created during migration
// and used as the bootstrap profile on fresh installs.
const DefaultProfileName = "default"

// pointerFile is the filename of the current-profile pointer under RootDir.
const pointerFile = "current"

// profilesDir is the subdirectory under RootDir holding per-profile state.
const profilesDir = "profiles"

// activeProfile holds the resolved profile name for the current process.
// It is set once at startup (by the CLI layer) via SetActiveProfile and
// read thereafter. When unset, ActiveProfile falls back to DefaultProfileName.
var activeProfile string

// profileNameRe restricts profile names to word characters only.
var profileNameRe = regexp.MustCompile(`^\w+$`)

// reservedProfileNames cannot be used as profile names to avoid collisions
// with other files under RootDir.
var reservedProfileNames = map[string]bool{
	"config":      true,
	"credentials": true,
	"current":     true,
	"profiles":    true,
}

// ValidateProfileName returns an error if name is not a legal profile name.
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("profile name cannot start with '.'")
	}
	if reservedProfileNames[name] {
		return fmt.Errorf("profile name %q is reserved", name)
	}
	if !profileNameRe.MatchString(name) {
		return fmt.Errorf("profile name %q invalid: only word characters (a-z A-Z 0-9 _) allowed", name)
	}
	return nil
}

// SetActiveProfile sets the process-wide active profile name.
// The name must have been validated by the caller.
func SetActiveProfile(name string) {
	activeProfile = name
}

// ActiveProfile returns the active profile name, falling back to
// DefaultProfileName if SetActiveProfile has not been called.
func ActiveProfile() string {
	if activeProfile == "" {
		return DefaultProfileName
	}
	return activeProfile
}

// ProfileDir returns the directory for the active profile, e.g.
// ~/.config/fylla/profiles/<active>/.
func ProfileDir() (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, profilesDir, ActiveProfile()), nil
}

// ProfileDirFor returns the directory for the given profile name.
func ProfileDirFor(name string) (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, profilesDir, name), nil
}

// PointerPath returns the path to the current-profile pointer file.
func PointerPath() (string, error) {
	root, err := RootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, pointerFile), nil
}

// DefaultPath returns the active profile's config.yaml path.
func DefaultPath() (string, error) {
	dir, err := ProfileDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
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

// EnsurePath creates the active profile's config.yaml from defaults if it
// does not exist, and returns its path. It does not parse or validate the
// file contents.
func EnsurePath() (string, error) {
	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create config dir: %w", err)
		}
		if err := os.WriteFile(path, defaultConfigYAML, 0644); err != nil {
			return "", fmt.Errorf("write default config: %w", err)
		}
	}
	return path, nil
}

// Load reads the config from the active profile path, creating it from
// defaults if missing.
func Load() (*Config, error) {
	path, err := EnsurePath()
	if err != nil {
		return nil, err
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

// Save writes the config to the active profile path.
func Save(cfg *Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return SaveTo(cfg, path)
}

// SetMultiIn reads a YAML file, applies multiple dotted-key updates in one
// read/write cycle, and returns the parsed Config. Missing intermediate mapping
// nodes and leaf keys are created automatically. Because it round-trips through
// the yaml.Node tree, formatting such as flow-style arrays and blank lines in
// the original file is preserved for keys that are not modified.
func SetMultiIn(path string, kvs map[string]string) (*Config, error) {
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

	for key, value := range kvs {
		parts := strings.Split(key, ".")
		if err := setOrCreateNode(doc.Content[0], parts, value); err != nil {
			return nil, fmt.Errorf("set %s: %w", key, err)
		}
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

// Set updates a value at the given dotted key path in the active profile's
// config file.
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

func setOrCreateNode(node *yaml.Node, parts []string, value string) error {
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
			return setOrCreateNode(valNode, parts[1:], value)
		}
	}

	// Key not found — create it
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: parts[0]}

	if len(parts) == 1 {
		valNode := &yaml.Node{Kind: yaml.ScalarNode}
		applyValue(valNode, value)
		node.Content = append(node.Content, keyNode, valNode)
		return nil
	}

	// Intermediate mapping node
	mapNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	node.Content = append(node.Content, keyNode, mapNode)
	return setOrCreateNode(mapNode, parts[1:], value)
}

// KeyPaths returns the dotted key paths that can be used with Set/SetIn.
// It walks the default config YAML to discover all settable leaf paths.
func KeyPaths() []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(defaultConfigYAML, &doc); err != nil {
		return nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	var paths []string
	collectKeyPaths(doc.Content[0], "", &paths)
	sort.Strings(paths)
	return paths
}

func collectKeyPaths(node *yaml.Node, prefix string, paths *[]string) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if val.Kind == yaml.MappingNode {
			collectKeyPaths(val, path, paths)
		} else {
			*paths = append(*paths, path)
		}
	}
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
		node.Style = yaml.FlowStyle
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
