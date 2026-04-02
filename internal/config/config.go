package config

import (
	"fmt"
	"time"
)

// Config represents the fylla configuration file.
type Config struct {
	Providers     []string                         `yaml:"providers"`
	Jira          JiraConfig                       `yaml:"jira"`
	Todoist       TodoistConfig                    `yaml:"todoist"`
	GitHub        GitHubConfig                     `yaml:"github"`
	Local         LocalConfig                      `yaml:"local"`
	Kendo         KendoConfig                      `yaml:"kendo"`
	Calendar      CalendarConfig                   `yaml:"calendar"`
	Scheduling    SchedulingConfig                 `yaml:"scheduling"`
	BusinessHours []BusinessHoursConfig            `yaml:"businessHours"`
	ProjectRules  map[string][]BusinessHoursConfig `yaml:"projectRules"`
	Weights       WeightsConfig                    `yaml:"weights"`
	Worklog       WorklogConfig                    `yaml:"worklog"`
	Efficiency    EfficiencyConfig                 `yaml:"efficiency"`
}

// ActiveProviders returns the list of configured providers.
// It returns Providers if set, and defaults to ["jira"].
func (c *Config) ActiveProviders() []string {
	if len(c.Providers) > 0 {
		return c.Providers
	}
	return []string{"jira"}
}

// JiraConfig holds Jira connection settings.
type JiraConfig struct {
	Credentials     string            `yaml:"credentials"`
	URL             string            `yaml:"url"`
	Email           string            `yaml:"email"`
	DefaultJQL      string            `yaml:"defaultJql"`
	DefaultProject  string            `yaml:"defaultProject"`
	DoneTransitions map[string]string `yaml:"doneTransitions"`
}

// TodoistConfig holds Todoist connection settings.
type TodoistConfig struct {
	Credentials    string `yaml:"credentials"`
	DefaultFilter  string `yaml:"defaultFilter"`
	DefaultProject string `yaml:"defaultProject"`
}

// GitHubConfig holds GitHub PR review settings.
type GitHubConfig struct {
	Credentials  string   `yaml:"credentials"`
	DefaultQuery string   `yaml:"defaultQuery"`
	Repos        []string `yaml:"repos"`
}

// KendoConfig holds Kendo connection settings.
type KendoConfig struct {
	Credentials    string `yaml:"credentials"`
	URL            string `yaml:"url"`
	DefaultFilter  string `yaml:"defaultFilter"`
	DefaultProject string `yaml:"defaultProject"`
	DoneLane       string `yaml:"doneLane"`
}

// LocalConfig holds local task provider settings.
type LocalConfig struct {
	StorePath      string `yaml:"storePath"`
	DefaultFilter  string `yaml:"defaultFilter"`
	DefaultProject string `yaml:"defaultProject"`
}

// CalendarConfig holds Google Calendar settings.
type CalendarConfig struct {
	Credentials     string   `yaml:"credentials"`
	SourceCalendars []string `yaml:"sourceCalendars"`
	FyllaCalendar   string   `yaml:"fyllaCalendar"`
}

// SchedulingConfig holds scheduling parameters.
type SchedulingConfig struct {
	WindowDays             int   `yaml:"windowDays"`
	MinTaskDurationMinutes int   `yaml:"minTaskDurationMinutes"`
	MaxTaskDurationMinutes int   `yaml:"maxTaskDurationMinutes"`
	BufferMinutes          int   `yaml:"bufferMinutes"`
	TravelBufferMinutes    int   `yaml:"travelBufferMinutes"`
	SnapMinutes            []int `yaml:"snapMinutes"`
}

// BusinessHoursConfig holds default business hours.
type BusinessHoursConfig struct {
	Start    string `yaml:"start"`
	End      string `yaml:"end"`
	WorkDays []int  `yaml:"workDays"`
}

// WorklogConfig holds worklog-related settings.
type WorklogConfig struct {
	Provider       string   `yaml:"provider"`
	FallbackIssues []string `yaml:"fallbackIssues"`
	RoundMinutes   int      `yaml:"roundMinutes"`
}

// EfficiencyConfig holds efficiency tracking settings.
type EfficiencyConfig struct {
	WeeklyHours float64 `yaml:"weeklyHours"`
	DailyHours  float64 `yaml:"dailyHours"`
	Target      float64 `yaml:"target"` // 0.0–1.0, e.g. 0.7 = 70%
}

// WeightsConfig holds sorting algorithm weights.
type WeightsConfig struct {
	Priority  float64            `yaml:"priority"`
	DueDate   float64            `yaml:"dueDate"`
	Estimate  float64            `yaml:"estimate"`
	Age       float64            `yaml:"age"`
	UpNext    float64            `yaml:"upNext"`
	TypeBonus map[string]float64 `yaml:"typeBonus"`
}

// Validate checks config invariants and returns an error if any are violated.
func (c *Config) Validate() error {
	// Validate providers if set
	if len(c.Providers) > 0 {
		seen := make(map[string]bool)
		for _, p := range c.Providers {
			switch p {
			case "jira", "todoist", "github", "local", "kendo":
			default:
				return fmt.Errorf("unknown provider %q (must be 'jira', 'todoist', 'github', 'local', or 'kendo')", p)
			}
			if seen[p] {
				return fmt.Errorf("duplicate provider %q", p)
			}
			seen[p] = true
		}
	}

	// Weights must sum to 1.0 (with float tolerance)
	sum := c.Weights.Priority + c.Weights.DueDate + c.Weights.Estimate + c.Weights.Age
	if sum < 0.99 || sum > 1.01 {
		return fmt.Errorf("weights must sum to 1.0, got %.2f", sum)
	}

	// Business hours
	if len(c.BusinessHours) == 0 {
		return fmt.Errorf("businessHours: at least one entry is required")
	}
	for i, bh := range c.BusinessHours {
		if err := validateBusinessHours(bh, fmt.Sprintf("businessHours[%d]", i)); err != nil {
			return err
		}
	}

	// Project rules
	for name, windows := range c.ProjectRules {
		for i, bh := range windows {
			if err := validateBusinessHours(bh, fmt.Sprintf("projectRules.%s[%d]", name, i)); err != nil {
				return err
			}
		}
	}

	// Efficiency
	e := c.Efficiency
	if e.WeeklyHours != 0 || e.DailyHours != 0 || e.Target != 0 {
		if e.WeeklyHours < 0 {
			return fmt.Errorf("efficiency.weeklyHours must be positive")
		}
		if e.DailyHours < 0 {
			return fmt.Errorf("efficiency.dailyHours must be positive")
		}
		if e.Target < 0 || e.Target > 1 {
			return fmt.Errorf("efficiency.target must be between 0.0 and 1.0")
		}
	}

	// Scheduling
	if c.Scheduling.WindowDays <= 0 {
		return fmt.Errorf("scheduling.windowDays must be positive")
	}
	if c.Scheduling.MinTaskDurationMinutes <= 0 {
		return fmt.Errorf("scheduling.minTaskDurationMinutes must be positive")
	}

	return nil
}

func validateBusinessHours(bh BusinessHoursConfig, prefix string) error {
	startH, startM, err := parseHHMM(bh.Start)
	if err != nil {
		return fmt.Errorf("%s.start: %w", prefix, err)
	}
	endH, endM, err := parseHHMM(bh.End)
	if err != nil {
		return fmt.Errorf("%s.end: %w", prefix, err)
	}
	if startH*60+startM >= endH*60+endM {
		return fmt.Errorf("%s.start must be before end", prefix)
	}
	for _, d := range bh.WorkDays {
		if d < 1 || d > 7 {
			return fmt.Errorf("%s.workDays: invalid day %d (must be 1-7)", prefix, d)
		}
	}
	return nil
}

func parseHHMM(s string) (int, int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, fmt.Errorf("invalid time format %q (expected HH:MM)", s)
	}
	h := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[3]-'0')*10 + int(s[4]-'0')
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time format %q (expected HH:MM)", s)
	}
	// Verify all characters are digits (except the colon)
	for i, c := range s {
		if i == 2 {
			continue
		}
		if c < '0' || c > '9' {
			return 0, 0, fmt.Errorf("invalid time format %q (expected HH:MM)", s)
		}
	}
	return h, m, nil
}

// DailyTargetFor computes the total working duration for a given weekday
// by summing all business hour windows that include that day.
// WorkDays use ISO numbering (1=Monday..7=Sunday), while time.Weekday
// uses Go's convention (0=Sunday..6=Saturday).
func DailyTargetFor(windows []BusinessHoursConfig, weekday time.Weekday) time.Duration {
	// Convert Go weekday to ISO: Sun=0 → 7, Mon=1 → 1, etc.
	iso := int(weekday)
	if iso == 0 {
		iso = 7
	}

	var total time.Duration
	for _, w := range windows {
		active := false
		for _, d := range w.WorkDays {
			if d == iso {
				active = true
				break
			}
		}
		if !active {
			continue
		}
		startH, startM, err := parseHHMM(w.Start)
		if err != nil {
			continue
		}
		endH, endM, err := parseHHMM(w.End)
		if err != nil {
			continue
		}
		total += time.Duration(endH*60+endM-startH*60-startM) * time.Minute
	}
	return total
}

// BusinessHoursFor returns the business hours for a project key.
// If a project-specific rule exists, it is returned.
// Otherwise, the default business hours are returned.
func (c *Config) BusinessHoursFor(projectKey string) []BusinessHoursConfig {
	if windows, ok := c.ProjectRules[projectKey]; ok {
		return windows
	}
	return c.BusinessHours
}
