package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCFG001_DefaultPathAndAutoCreate(t *testing.T) {
	t.Run("DefaultPath ends with fylla/config.yaml", func(t *testing.T) {
		path, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath() error: %v", err)
		}
		if filepath.Base(path) != "config.yaml" {
			t.Errorf("expected config.yaml, got %s", filepath.Base(path))
		}
		parent := filepath.Base(filepath.Dir(path))
		if parent != "fylla" {
			t.Errorf("expected fylla dir, got %s", parent)
		}
	})

	t.Run("LoadFrom missing file returns error", func(t *testing.T) {
		_, err := LoadFrom(filepath.Join(t.TempDir(), "nonexistent.yaml"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("dir auto-created when writing defaults", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sub", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, defaultConfigYAML, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		cfg, err := LoadFrom(path)
		if err != nil {
			t.Fatalf("LoadFrom: %v", err)
		}
		if cfg.Jira.URL == "" {
			t.Error("expected non-empty Jira URL from defaults")
		}
	})
}

func TestCFG002_JiraConfig(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Jira.URL != "https://company.atlassian.net" {
		t.Errorf("URL = %q", cfg.Jira.URL)
	}
	if cfg.Jira.Email != "you@example.com" {
		t.Errorf("Email = %q", cfg.Jira.Email)
	}
	if cfg.Jira.DefaultJQL != "assignee = currentUser() AND status = 'To Do'" {
		t.Errorf("DefaultJQL = %q", cfg.Jira.DefaultJQL)
	}
}

func TestCFG003_CalendarConfig(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Calendar.SourceCalendar != "primary" {
		t.Errorf("SourceCalendar = %q", cfg.Calendar.SourceCalendar)
	}
	if cfg.Calendar.FyllaCalendar != "fylla" {
		t.Errorf("FyllaCalendar = %q", cfg.Calendar.FyllaCalendar)
	}
}

func TestCFG004_SchedulingConfig(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.Scheduling.WindowDays != 5 {
		t.Errorf("WindowDays = %d", cfg.Scheduling.WindowDays)
	}
	if cfg.Scheduling.MinTaskDurationMinutes != 25 {
		t.Errorf("MinTaskDurationMinutes = %d", cfg.Scheduling.MinTaskDurationMinutes)
	}
	if cfg.Scheduling.BufferMinutes != 15 {
		t.Errorf("BufferMinutes = %d", cfg.Scheduling.BufferMinutes)
	}
}

func TestCFG005_BusinessHours(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if cfg.BusinessHours.Start != "09:00" {
		t.Errorf("Start = %q", cfg.BusinessHours.Start)
	}
	if cfg.BusinessHours.End != "17:00" {
		t.Errorf("End = %q", cfg.BusinessHours.End)
	}
	expectedDays := []int{1, 2, 3, 4, 5}
	if len(cfg.BusinessHours.WorkDays) != len(expectedDays) {
		t.Fatalf("WorkDays len = %d", len(cfg.BusinessHours.WorkDays))
	}
	for i, d := range expectedDays {
		if cfg.BusinessHours.WorkDays[i] != d {
			t.Errorf("WorkDays[%d] = %d, want %d", i, cfg.BusinessHours.WorkDays[i], d)
		}
	}
}

func TestCFG006_BusinessHoursFor(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	t.Run("project rule exists", func(t *testing.T) {
		bh := cfg.BusinessHoursFor("ADMIN")
		if bh.Start != "09:00" || bh.End != "10:00" {
			t.Errorf("ADMIN hours = %s-%s", bh.Start, bh.End)
		}
	})

	t.Run("unknown project returns default", func(t *testing.T) {
		bh := cfg.BusinessHoursFor("UNKNOWN")
		if bh.Start != "09:00" || bh.End != "17:00" {
			t.Errorf("default hours = %s-%s", bh.Start, bh.End)
		}
	})
}

func TestCFG007_Weights(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"Priority", cfg.Weights.Priority, 0.40},
		{"DueDate", cfg.Weights.DueDate, 0.30},
		{"Estimate", cfg.Weights.Estimate, 0.15},
		{"IssueType", cfg.Weights.IssueType, 0.10},
		{"Age", cfg.Weights.Age, 0.05},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestCFG008_TypeScores(t *testing.T) {
	path := writeTestConfig(t)
	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	expected := map[string]int{"Bug": 100, "Task": 70, "Story": 50}
	for k, want := range expected {
		t.Run(k, func(t *testing.T) {
			got, ok := cfg.TypeScores[k]
			if !ok {
				t.Fatalf("missing key %q", k)
			}
			if got != want {
				t.Errorf("%s = %d, want %d", k, got, want)
			}
		})
	}
}

func TestCFG009_Credentials(t *testing.T) {
	t.Run("path separate from config", func(t *testing.T) {
		cfgPath, _ := DefaultPath()
		credPath, _ := CredentialsPath()
		if cfgPath == credPath {
			t.Error("credentials path should differ from config path")
		}
		if filepath.Ext(credPath) != ".json" {
			t.Errorf("expected .json extension, got %s", filepath.Ext(credPath))
		}
	})

	t.Run("missing file returns empty credentials", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		creds, err := LoadCredentialsFrom(path)
		if err != nil {
			t.Fatalf("LoadCredentialsFrom: %v", err)
		}
		if creds.JiraToken != "" || creds.GoogleOAuthToken != "" {
			t.Error("expected empty credentials")
		}
	})

	t.Run("round-trip save and load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		original := &Credentials{
			JiraToken:        "jira-secret",
			GoogleOAuthToken: "google-secret",
		}
		if err := SaveCredentialsTo(original, path); err != nil {
			t.Fatalf("SaveCredentialsTo: %v", err)
		}

		loaded, err := LoadCredentialsFrom(path)
		if err != nil {
			t.Fatalf("LoadCredentialsFrom: %v", err)
		}
		if loaded.JiraToken != original.JiraToken {
			t.Errorf("JiraToken = %q, want %q", loaded.JiraToken, original.JiraToken)
		}
		if loaded.GoogleOAuthToken != original.GoogleOAuthToken {
			t.Errorf("GoogleOAuthToken = %q, want %q", loaded.GoogleOAuthToken, original.GoogleOAuthToken)
		}
	})

	t.Run("file permissions are 0600", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		if err := SaveCredentialsTo(&Credentials{}, path); err != nil {
			t.Fatalf("SaveCredentialsTo: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("permissions = %o, want 0600", perm)
		}
	})
}

func TestSet_ScalarValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name:  "int value",
			key:   "scheduling.windowDays",
			value: "10",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Scheduling.WindowDays != 10 {
					t.Errorf("WindowDays = %d, want 10", cfg.Scheduling.WindowDays)
				}
			},
		},
		{
			name:  "float value",
			key:   "weights.priority",
			value: "0.55",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Weights.Priority != 0.55 {
					t.Errorf("Priority = %v, want 0.55", cfg.Weights.Priority)
				}
			},
		},
		{
			name:  "string value",
			key:   "jira.email",
			value: "new@example.com",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Jira.Email != "new@example.com" {
					t.Errorf("Email = %q, want new@example.com", cfg.Jira.Email)
				}
			},
		},
		{
			name:  "nested key path",
			key:   "calendar.fyllaCalendar",
			value: "work",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Calendar.FyllaCalendar != "work" {
					t.Errorf("FyllaCalendar = %q, want work", cfg.Calendar.FyllaCalendar)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestConfig(t)
			cfg, err := SetIn(path, tt.key, tt.value)
			if err != nil {
				t.Fatalf("SetIn: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestSet_KeyNotFound(t *testing.T) {
	path := writeTestConfig(t)
	_, err := SetIn(path, "nonexistent.key", "value")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestValidate(t *testing.T) {
	validConfig := func() Config {
		return Config{
			Source: "jira",
			Scheduling: SchedulingConfig{
				WindowDays:             5,
				MinTaskDurationMinutes: 25,
				BufferMinutes:          15,
			},
			BusinessHours: BusinessHoursConfig{
				Start:    "09:00",
				End:      "17:00",
				WorkDays: []int{1, 2, 3, 4, 5},
			},
			Weights: WeightsConfig{
				Priority:  0.40,
				DueDate:   0.30,
				Estimate:  0.15,
				IssueType: 0.10,
				Age:       0.05,
			},
		}
	}

	t.Run("valid config passes", func(t *testing.T) {
		cfg := validConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty source is valid", func(t *testing.T) {
		cfg := validConfig()
		cfg.Source = ""
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("todoist source is valid", func(t *testing.T) {
		cfg := validConfig()
		cfg.Source = "todoist"
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid source", func(t *testing.T) {
		cfg := validConfig()
		cfg.Source = "trello"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid source")
		}
	})

	t.Run("weights sum too low", func(t *testing.T) {
		cfg := validConfig()
		cfg.Weights.Priority = 0.10
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for weights sum")
		}
	})

	t.Run("weights sum too high", func(t *testing.T) {
		cfg := validConfig()
		cfg.Weights.Priority = 0.80
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for weights sum")
		}
	})

	t.Run("invalid business hours start format", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours.Start = "9:00"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid start time")
		}
	})

	t.Run("invalid business hours end format", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours.End = "25:00"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid end time")
		}
	})

	t.Run("business hours start after end", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours.Start = "17:00"
		cfg.BusinessHours.End = "09:00"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for start after end")
		}
	})

	t.Run("invalid work day", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours.WorkDays = []int{0, 1, 2}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid work day 0")
		}
	})

	t.Run("work day 8 is invalid", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours.WorkDays = []int{1, 8}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for work day 8")
		}
	})

	t.Run("windowDays zero", func(t *testing.T) {
		cfg := validConfig()
		cfg.Scheduling.WindowDays = 0
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for zero windowDays")
		}
	})

	t.Run("minTaskDurationMinutes negative", func(t *testing.T) {
		cfg := validConfig()
		cfg.Scheduling.MinTaskDurationMinutes = -1
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for negative minTaskDurationMinutes")
		}
	})

	t.Run("invalid project rule", func(t *testing.T) {
		cfg := validConfig()
		cfg.ProjectRules = map[string]ProjectRule{
			"BAD": {Start: "abc", End: "17:00", WorkDays: []int{1}},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid project rule")
		}
	})

	t.Run("project rule start after end", func(t *testing.T) {
		cfg := validConfig()
		cfg.ProjectRules = map[string]ProjectRule{
			"BAD": {Start: "18:00", End: "09:00", WorkDays: []int{1}},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for project rule start after end")
		}
	})
}

func TestKeyPaths(t *testing.T) {
	paths := KeyPaths()
	if len(paths) == 0 {
		t.Fatal("KeyPaths returned empty slice")
	}

	// Verify known leaf paths are present
	expected := []string{
		"source",
		"jira.url",
		"jira.email",
		"jira.defaultJql",
		"todoist.defaultFilter",
		"calendar.sourceCalendar",
		"calendar.fyllaCalendar",
		"scheduling.windowDays",
		"scheduling.minTaskDurationMinutes",
		"scheduling.bufferMinutes",
		"businessHours.start",
		"businessHours.end",
		"businessHours.workDays",
		"weights.priority",
		"weights.dueDate",
		"weights.estimate",
		"weights.issueType",
		"weights.age",
	}
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}
	for _, e := range expected {
		if !pathSet[e] {
			t.Errorf("expected key path %q not found in %v", e, paths)
		}
	}

	// Verify paths are sorted
	for i := 1; i < len(paths); i++ {
		if paths[i] < paths[i-1] {
			t.Errorf("paths not sorted: %q before %q", paths[i-1], paths[i])
		}
	}

	// Verify no bare section names (mapping nodes should not appear as paths)
	for _, p := range paths {
		switch p {
		case "jira", "todoist", "calendar", "scheduling", "businessHours", "weights":
			t.Errorf("section name %q should not be a leaf path", p)
		}
	}
}

// writeTestConfig writes the default config YAML to a temp file and returns its path.
func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, defaultConfigYAML, 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}
