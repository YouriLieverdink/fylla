package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReadPointer returns the profile name stored in the pointer file, or the
// empty string if the file does not exist.
func ReadPointer() (string, error) {
	path, err := PointerPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read pointer: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WritePointer writes the given profile name to the pointer file.
func WritePointer(name string) error {
	path, err := PointerPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create root dir: %w", err)
	}
	return os.WriteFile(path, []byte(name+"\n"), 0644)
}

// ListProfiles returns the sorted names of all profile directories under
// RootDir/profiles/. An empty slice is returned if the directory does not
// exist.
func ListProfiles() ([]string, error) {
	root, err := RootDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, profilesDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// ProfileExists reports whether the profile directory for name exists.
func ProfileExists(name string) (bool, error) {
	dir, err := ProfileDirFor(name)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat profile: %w", err)
	}
	return info.IsDir(), nil
}

// CreateProfile creates a new profile directory for name. If from is empty,
// the profile is seeded from the default config template. Otherwise the
// contents of the from profile are copied.
func CreateProfile(name, from string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	exists, err := ProfileExists(name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("profile %q already exists", name)
	}
	dir, err := ProfileDirFor(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}
	if from == "" {
		cfgPath := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(cfgPath, defaultConfigYAML, 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		return nil
	}
	fromExists, err := ProfileExists(from)
	if err != nil {
		return err
	}
	if !fromExists {
		return fmt.Errorf("source profile %q not found", from)
	}
	src, err := ProfileDirFor(from)
	if err != nil {
		return err
	}
	return copyDir(src, dir)
}

// DeleteProfile removes the profile directory for name. It refuses to
// delete the active profile unless force is true.
func DeleteProfile(name string, force bool) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	exists, err := ProfileExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("profile %q not found", name)
	}
	if !force {
		active, _ := ReadPointer()
		if active == name {
			return fmt.Errorf("profile %q is current; run 'fylla profile use <other>' first, or pass --force", name)
		}
	}
	dir, err := ProfileDirFor(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// ResolveProfile returns the active profile name, applying the precedence:
// flag > FYLLA_PROFILE env > pointer file > DefaultProfileName.
// An explicit flag or env value referencing a non-existent profile is a
// hard error. A stale pointer falls back to DefaultProfileName when it
// exists.
func ResolveProfile(flag string) (string, error) {
	if flag != "" {
		if err := ValidateProfileName(flag); err != nil {
			return "", err
		}
		exists, err := ProfileExists(flag)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", fmt.Errorf("profile %q not found", flag)
		}
		return flag, nil
	}
	if env := os.Getenv("FYLLA_PROFILE"); env != "" {
		if err := ValidateProfileName(env); err != nil {
			return "", fmt.Errorf("FYLLA_PROFILE: %w", err)
		}
		exists, err := ProfileExists(env)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", fmt.Errorf("profile %q (from FYLLA_PROFILE) not found", env)
		}
		return env, nil
	}
	name, err := ReadPointer()
	if err != nil {
		return "", err
	}
	if name != "" {
		if err := ValidateProfileName(name); err != nil {
			return "", fmt.Errorf("pointer file %q: %w", name, err)
		}
		exists, err := ProfileExists(name)
		if err != nil {
			return "", err
		}
		if exists {
			return name, nil
		}
		defExists, err := ProfileExists(DefaultProfileName)
		if err != nil {
			return "", err
		}
		if defExists {
			return DefaultProfileName, nil
		}
		return "", fmt.Errorf("pointer profile %q not found and %q does not exist", name, DefaultProfileName)
	}
	return DefaultProfileName, nil
}

// BootstrapActiveProfile ensures the active profile directory and its
// config.yaml exist, seeding from the default template if missing. Callers
// should invoke this after SetActiveProfile.
func BootstrapActiveProfile() error {
	dir, err := ProfileDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, defaultConfigYAML, 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}
	return nil
}

// MigrateLegacyLayout moves legacy top-level state files (config.yaml,
// timer.json, *_credentials.json) into profiles/default/ on first run.
// Returns true if migration was performed.
func MigrateLegacyLayout() (bool, error) {
	root, err := RootDir()
	if err != nil {
		return false, err
	}
	profilesRoot := filepath.Join(root, profilesDir)
	if _, err := os.Stat(profilesRoot); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat profiles dir: %w", err)
	}

	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read root: %w", err)
	}

	var toMove []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "config.yaml" || name == "timer.json" || name == pointerFile {
			toMove = append(toMove, name)
			continue
		}
		if strings.HasSuffix(name, "_credentials.json") || name == "google_credentials.json" {
			toMove = append(toMove, name)
		}
	}
	if len(toMove) == 0 {
		return false, nil
	}

	dest := filepath.Join(profilesRoot, DefaultProfileName)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return false, fmt.Errorf("create default profile dir: %w", err)
	}
	for _, name := range toMove {
		if name == pointerFile {
			continue
		}
		src := filepath.Join(root, name)
		dst := filepath.Join(dest, name)
		if err := os.Rename(src, dst); err != nil {
			return false, fmt.Errorf("move %s: %w", name, err)
		}
	}
	if err := WritePointer(DefaultProfileName); err != nil {
		return false, err
	}
	return true, nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read src: %w", err)
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(sp, dp); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sp, dp); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s: %w", src, err)
	}
	return out.Close()
}
