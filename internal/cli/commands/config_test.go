package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfigYAML = `jira:
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"
calendar:
  sourceCalendars: [primary]
  fyllaCalendar: fylla
scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  bufferMinutes: 15
businessHours:
  - start: "09:00"
    end: "17:00"
    workDays: [1, 2, 3, 4, 5]
projectRules:
  ADMIN:
    - start: "09:00"
      end: "10:00"
      workDays: [1, 2, 3, 4, 5]
weights:
  priority: 0.45
  dueDate: 0.30
  estimate: 0.15
  age: 0.10
`

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(testConfigYAML), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}

func TestCLI020_config_show(t *testing.T) {
	t.Run("displays all config sections", func(t *testing.T) {
		path := writeTestConfig(t)
		yamlText, err := RunConfigShow(ConfigShowParams{ConfigPath: path})
		if err != nil {
			t.Fatalf("RunConfigShow: %v", err)
		}

		sections := []string{"jira:", "calendar:", "scheduling:", "businessHours:", "projectRules:", "weights:"}
		for _, section := range sections {
			if !strings.Contains(yamlText, section) {
				t.Errorf("output missing section %q", section)
			}
		}
	})

	t.Run("displays config values correctly", func(t *testing.T) {
		path := writeTestConfig(t)
		yamlText, err := RunConfigShow(ConfigShowParams{ConfigPath: path})
		if err != nil {
			t.Fatalf("RunConfigShow: %v", err)
		}

		checks := []string{
			"windowDays: 5",
			"bufferMinutes: 15",
			"sourceCalendars:",
			"fyllaCalendar: fylla",
		}
		for _, check := range checks {
			if !strings.Contains(yamlText, check) {
				t.Errorf("output missing %q", check)
			}
		}
	})

	t.Run("PrintConfigShow writes to writer", func(t *testing.T) {
		var buf bytes.Buffer
		PrintConfigShow(&buf, "jira:\n  url: test\n")
		if !strings.Contains(buf.String(), "jira:") {
			t.Errorf("output = %q, want to contain jira:", buf.String())
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := RunConfigShow(ConfigShowParams{ConfigPath: "/nonexistent/config.yaml"})
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestCLI021_config_edit(t *testing.T) {
	t.Run("uses EDITOR env variable", func(t *testing.T) {
		result := ResolveEditor("")
		// When EDITOR is not set, defaults to "vi"
		// We can't easily test the env variable in parallel, so test the explicit override
		if result != "vi" && result != os.Getenv("EDITOR") {
			t.Errorf("ResolveEditor('') = %q, want 'vi' or $EDITOR", result)
		}
	})

	t.Run("explicit editor overrides env", func(t *testing.T) {
		result := ResolveEditor("vim")
		if result != "vim" {
			t.Errorf("ResolveEditor('vim') = %q, want 'vim'", result)
		}
	})

	t.Run("falls back to vi when no editor set", func(t *testing.T) {
		orig := os.Getenv("EDITOR")
		os.Unsetenv("EDITOR")
		defer func() {
			if orig != "" {
				os.Setenv("EDITOR", orig)
			}
		}()

		result := ResolveEditor("")
		if result != "vi" {
			t.Errorf("ResolveEditor('') = %q, want 'vi'", result)
		}
	})

	t.Run("respects EDITOR environment variable", func(t *testing.T) {
		orig := os.Getenv("EDITOR")
		os.Setenv("EDITOR", "nano")
		defer func() {
			if orig != "" {
				os.Setenv("EDITOR", orig)
			} else {
				os.Unsetenv("EDITOR")
			}
		}()

		result := ResolveEditor("")
		if result != "nano" {
			t.Errorf("ResolveEditor('') = %q, want 'nano'", result)
		}
	})
}

func TestCLI022_config_set(t *testing.T) {
	t.Run("sets scheduling.windowDays to 7", func(t *testing.T) {
		path := writeTestConfig(t)
		cfg, err := RunConfigSet(ConfigSetParams{
			ConfigPath: path,
			Key:        "scheduling.windowDays",
			Value:      "7",
		})
		if err != nil {
			t.Fatalf("RunConfigSet: %v", err)
		}

		if cfg.Scheduling.WindowDays != 7 {
			t.Errorf("windowDays = %d, want 7", cfg.Scheduling.WindowDays)
		}
	})

	t.Run("verify updated value persists via show", func(t *testing.T) {
		path := writeTestConfig(t)
		_, err := RunConfigSet(ConfigSetParams{
			ConfigPath: path,
			Key:        "scheduling.windowDays",
			Value:      "7",
		})
		if err != nil {
			t.Fatalf("RunConfigSet: %v", err)
		}

		yamlText, err := RunConfigShow(ConfigShowParams{ConfigPath: path})
		if err != nil {
			t.Fatalf("RunConfigShow: %v", err)
		}
		if !strings.Contains(yamlText, "windowDays: 7") {
			t.Errorf("show output missing 'windowDays: 7', got:\n%s", yamlText)
		}
	})

	t.Run("PrintConfigSet shows confirmation", func(t *testing.T) {
		var buf bytes.Buffer
		PrintConfigSet(&buf, "scheduling.windowDays", "7")
		out := buf.String()
		if !strings.Contains(out, "scheduling.windowDays") {
			t.Errorf("output = %q, want to contain key", out)
		}
		if !strings.Contains(out, "7") {
			t.Errorf("output = %q, want to contain value", out)
		}
	})

	t.Run("returns error for invalid key", func(t *testing.T) {
		path := writeTestConfig(t)
		_, err := RunConfigSet(ConfigSetParams{
			ConfigPath: path,
			Key:        "nonexistent.key",
			Value:      "value",
		})
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})

	t.Run("sets string value", func(t *testing.T) {
		path := writeTestConfig(t)
		cfg, err := RunConfigSet(ConfigSetParams{
			ConfigPath: path,
			Key:        "jira.email",
			Value:      "new@example.com",
		})
		if err != nil {
			t.Fatalf("RunConfigSet: %v", err)
		}

		if cfg.Jira.Email != "new@example.com" {
			t.Errorf("email = %q, want 'new@example.com'", cfg.Jira.Email)
		}
	})
}
