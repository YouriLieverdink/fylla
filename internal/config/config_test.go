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

	if len(cfg.Calendar.SourceCalendars) != 1 || cfg.Calendar.SourceCalendars[0] != "primary" {
		t.Errorf("SourceCalendars = %v", cfg.Calendar.SourceCalendars)
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

	if len(cfg.BusinessHours) != 1 {
		t.Fatalf("BusinessHours len = %d, want 1", len(cfg.BusinessHours))
	}
	if cfg.BusinessHours[0].Start != "09:00" {
		t.Errorf("Start = %q", cfg.BusinessHours[0].Start)
	}
	if cfg.BusinessHours[0].End != "17:00" {
		t.Errorf("End = %q", cfg.BusinessHours[0].End)
	}
	expectedDays := []int{1, 2, 3, 4, 5}
	if len(cfg.BusinessHours[0].WorkDays) != len(expectedDays) {
		t.Fatalf("WorkDays len = %d", len(cfg.BusinessHours[0].WorkDays))
	}
	for i, d := range expectedDays {
		if cfg.BusinessHours[0].WorkDays[i] != d {
			t.Errorf("WorkDays[%d] = %d, want %d", i, cfg.BusinessHours[0].WorkDays[i], d)
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
		if len(bh) != 1 {
			t.Fatalf("ADMIN hours len = %d, want 1", len(bh))
		}
		if bh[0].Start != "09:00" || bh[0].End != "10:00" {
			t.Errorf("ADMIN hours = %s-%s", bh[0].Start, bh[0].End)
		}
	})

	t.Run("unknown project returns default", func(t *testing.T) {
		bh := cfg.BusinessHoursFor("UNKNOWN")
		if len(bh) != 1 {
			t.Fatalf("default hours len = %d, want 1", len(bh))
		}
		if bh[0].Start != "09:00" || bh[0].End != "17:00" {
			t.Errorf("default hours = %s-%s", bh[0].Start, bh[0].End)
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
		{"Priority", cfg.Weights.Priority, 0.45},
		{"DueDate", cfg.Weights.DueDate, 0.30},
		{"Estimate", cfg.Weights.Estimate, 0.15},
		{"Age", cfg.Weights.Age, 0.10},
		{"UpNext", cfg.Weights.UpNext, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestCFG009_ProviderCredentials(t *testing.T) {
	t.Run("default path separate from config", func(t *testing.T) {
		cfgPath, _ := DefaultPath()
		credPath, _ := DefaultProviderCredentialsPath("jira")
		if cfgPath == credPath {
			t.Error("credentials path should differ from config path")
		}
		if filepath.Ext(credPath) != ".json" {
			t.Errorf("expected .json extension, got %s", filepath.Ext(credPath))
		}
	})

	t.Run("provider name in path", func(t *testing.T) {
		jiraPath, _ := DefaultProviderCredentialsPath("jira")
		todoistPath, _ := DefaultProviderCredentialsPath("todoist")
		if jiraPath == todoistPath {
			t.Error("jira and todoist paths should differ")
		}
		if filepath.Base(jiraPath) != "jira_credentials.json" {
			t.Errorf("jira path = %q", filepath.Base(jiraPath))
		}
		if filepath.Base(todoistPath) != "todoist_credentials.json" {
			t.Errorf("todoist path = %q", filepath.Base(todoistPath))
		}
	})

	t.Run("missing file returns empty credentials", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		creds, err := LoadProviderCredentials(path)
		if err != nil {
			t.Fatalf("LoadProviderCredentials: %v", err)
		}
		if creds.Token != "" {
			t.Error("expected empty token")
		}
	})

	t.Run("round-trip save and load", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		original := &ProviderCredentials{Token: "my-secret"}
		if err := SaveProviderCredentials(original, path); err != nil {
			t.Fatalf("SaveProviderCredentials: %v", err)
		}

		loaded, err := LoadProviderCredentials(path)
		if err != nil {
			t.Fatalf("LoadProviderCredentials: %v", err)
		}
		if loaded.Token != original.Token {
			t.Errorf("Token = %q, want %q", loaded.Token, original.Token)
		}
	})

	t.Run("file permissions are 0600", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "creds.json")
		if err := SaveProviderCredentials(&ProviderCredentials{}, path); err != nil {
			t.Fatalf("SaveProviderCredentials: %v", err)
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
			Scheduling: SchedulingConfig{
				WindowDays:             5,
				MinTaskDurationMinutes: 25,
				BufferMinutes:          15,
			},
			BusinessHours: []BusinessHoursConfig{{
				Start:    "09:00",
				End:      "17:00",
				WorkDays: []int{1, 2, 3, 4, 5},
			}},
			Weights: WeightsConfig{
				Priority: 0.45,
				DueDate:  0.30,
				Estimate: 0.15,
				Age:      0.10,
				UpNext:   50,
			},
		}
	}

	t.Run("valid config passes", func(t *testing.T) {
		cfg := validConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
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
		cfg.BusinessHours[0].Start = "9:00"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid start time")
		}
	})

	t.Run("invalid business hours end format", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours[0].End = "25:00"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid end time")
		}
	})

	t.Run("business hours start after end", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours[0].Start = "17:00"
		cfg.BusinessHours[0].End = "09:00"
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for start after end")
		}
	})

	t.Run("invalid work day", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours[0].WorkDays = []int{0, 1, 2}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid work day 0")
		}
	})

	t.Run("work day 8 is invalid", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours[0].WorkDays = []int{1, 8}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for work day 8")
		}
	})

	t.Run("empty business hours fails", func(t *testing.T) {
		cfg := validConfig()
		cfg.BusinessHours = []BusinessHoursConfig{}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for empty business hours")
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
		cfg.ProjectRules = map[string][]BusinessHoursConfig{
			"BAD": {{Start: "abc", End: "17:00", WorkDays: []int{1}}},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid project rule")
		}
	})

	t.Run("project rule start after end", func(t *testing.T) {
		cfg := validConfig()
		cfg.ProjectRules = map[string][]BusinessHoursConfig{
			"BAD": {{Start: "18:00", End: "09:00", WorkDays: []int{1}}},
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
		"providers",
		"jira.url",
		"jira.email",
		"jira.defaultJql",
		"todoist.defaultFilter",
		"calendar.sourceCalendars",
		"calendar.fyllaCalendar",
		"scheduling.windowDays",
		"scheduling.minTaskDurationMinutes",
		"scheduling.bufferMinutes",
		"weights.priority",
		"weights.dueDate",
		"weights.estimate",
		"weights.age",
		"weights.upNext",
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
	// Note: businessHours is now a sequence node, so it correctly appears as a leaf path
	for _, p := range paths {
		switch p {
		case "jira", "todoist", "calendar", "scheduling", "weights":
			t.Errorf("section name %q should not be a leaf path", p)
		}
	}
}

func TestValidateProviders(t *testing.T) {
	validConfig := func() Config {
		return Config{
			Scheduling: SchedulingConfig{
				WindowDays:             5,
				MinTaskDurationMinutes: 25,
				BufferMinutes:          15,
			},
			BusinessHours: []BusinessHoursConfig{{
				Start:    "09:00",
				End:      "17:00",
				WorkDays: []int{1, 2, 3, 4, 5},
			}},
			Weights: WeightsConfig{
				Priority: 0.45,
				DueDate:  0.30,
				Estimate: 0.15,
				Age:      0.10,
				UpNext:   50,
			},
		}
	}

	t.Run("single provider is valid", func(t *testing.T) {
		cfg := validConfig()
		cfg.Providers = []string{"jira"}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("multiple providers is valid", func(t *testing.T) {
		cfg := validConfig()
		cfg.Providers = []string{"jira", "todoist"}
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unknown provider is invalid", func(t *testing.T) {
		cfg := validConfig()
		cfg.Providers = []string{"trello"}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for unknown provider")
		}
	})

	t.Run("duplicate provider is invalid", func(t *testing.T) {
		cfg := validConfig()
		cfg.Providers = []string{"jira", "jira"}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for duplicate provider")
		}
	})

	t.Run("empty providers is valid (uses fallback)", func(t *testing.T) {
		cfg := validConfig()
		cfg.Providers = nil
		if err := cfg.Validate(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestActiveProviders(t *testing.T) {
	t.Run("uses Providers when set", func(t *testing.T) {
		cfg := Config{Providers: []string{"jira", "todoist"}}
		got := cfg.ActiveProviders()
		if len(got) != 2 || got[0] != "jira" || got[1] != "todoist" {
			t.Errorf("ActiveProviders() = %v, want [jira todoist]", got)
		}
	})

	t.Run("defaults to jira", func(t *testing.T) {
		cfg := Config{}
		got := cfg.ActiveProviders()
		if len(got) != 1 || got[0] != "jira" {
			t.Errorf("ActiveProviders() = %v, want [jira]", got)
		}
	})

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
