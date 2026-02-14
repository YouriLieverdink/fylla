package config

import "fmt"

// Config represents the fylla configuration file.
type Config struct {
	Source        string                 `yaml:"source"`
	Jira          JiraConfig             `yaml:"jira"`
	Todoist       TodoistConfig          `yaml:"todoist"`
	Calendar      CalendarConfig         `yaml:"calendar"`
	Scheduling    SchedulingConfig       `yaml:"scheduling"`
	BusinessHours BusinessHoursConfig    `yaml:"businessHours"`
	ProjectRules  map[string]ProjectRule `yaml:"projectRules"`
	Weights       WeightsConfig          `yaml:"weights"`
	TypeScores    map[string]int         `yaml:"typeScores"`
}

// JiraConfig holds Jira connection settings.
type JiraConfig struct {
	URL            string `yaml:"url"`
	Email          string `yaml:"email"`
	DefaultJQL     string `yaml:"defaultJql"`
	DefaultProject string `yaml:"defaultProject"`
}

// TodoistConfig holds Todoist connection settings.
type TodoistConfig struct {
	DefaultFilter  string `yaml:"defaultFilter"`
	DefaultProject string `yaml:"defaultProject"`
}

// CalendarConfig holds Google Calendar settings.
type CalendarConfig struct {
	SourceCalendars   []string `yaml:"sourceCalendars"`
	FyllaCalendar     string   `yaml:"fyllaCalendar"`
	ClientCredentials string   `yaml:"clientCredentials"`
}

// SchedulingConfig holds scheduling parameters.
type SchedulingConfig struct {
	WindowDays             int   `yaml:"windowDays"`
	MinTaskDurationMinutes int   `yaml:"minTaskDurationMinutes"`
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

// ProjectRule holds project-specific scheduling rules.
type ProjectRule struct {
	Start    string `yaml:"start"`
	End      string `yaml:"end"`
	WorkDays []int  `yaml:"workDays"`
}

// WeightsConfig holds sorting algorithm weights.
type WeightsConfig struct {
	Priority  float64 `yaml:"priority"`
	DueDate   float64 `yaml:"dueDate"`
	Estimate  float64 `yaml:"estimate"`
	IssueType float64 `yaml:"issueType"`
	Age       float64 `yaml:"age"`
}

// Validate checks config invariants and returns an error if any are violated.
func (c *Config) Validate() error {
	// Source must be jira, todoist, or empty (defaults to jira)
	switch c.Source {
	case "", "jira", "todoist":
	default:
		return fmt.Errorf("source must be 'jira' or 'todoist', got %q", c.Source)
	}

	// Weights must sum to 1.0 (with float tolerance)
	sum := c.Weights.Priority + c.Weights.DueDate + c.Weights.Estimate + c.Weights.IssueType + c.Weights.Age
	if sum < 0.99 || sum > 1.01 {
		return fmt.Errorf("weights must sum to 1.0, got %.2f", sum)
	}

	// Business hours
	if err := validateBusinessHours(c.BusinessHours, "businessHours"); err != nil {
		return err
	}

	// Project rules
	for name, rule := range c.ProjectRules {
		bh := BusinessHoursConfig{Start: rule.Start, End: rule.End, WorkDays: rule.WorkDays}
		if err := validateBusinessHours(bh, fmt.Sprintf("projectRules.%s", name)); err != nil {
			return err
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

// BusinessHoursFor returns the business hours for a project key.
// If a project-specific rule exists, it is returned as a BusinessHoursConfig.
// Otherwise, the default business hours are returned.
func (c *Config) BusinessHoursFor(projectKey string) BusinessHoursConfig {
	if rule, ok := c.ProjectRules[projectKey]; ok {
		return BusinessHoursConfig{
			Start:    rule.Start,
			End:      rule.End,
			WorkDays: rule.WorkDays,
		}
	}
	return c.BusinessHours
}
