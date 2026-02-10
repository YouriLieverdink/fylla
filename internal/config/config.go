package config

import "gopkg.in/yaml.v3"

// Config represents the fylla configuration file.
type Config struct {
	Jira          JiraConfig                `yaml:"jira"`
	Calendar      CalendarConfig            `yaml:"calendar"`
	Scheduling    SchedulingConfig          `yaml:"scheduling"`
	BusinessHours BusinessHoursConfig       `yaml:"businessHours"`
	ProjectRules  map[string]ProjectRule    `yaml:"projectRules"`
	Weights       WeightsConfig             `yaml:"weights"`
	TypeScores    map[string]int            `yaml:"typeScores"`
}

// JiraConfig holds Jira connection settings.
type JiraConfig struct {
	URL        string `yaml:"url"`
	Email      string `yaml:"email"`
	DefaultJQL string `yaml:"defaultJql"`
}

// CalendarConfig holds Google Calendar settings.
type CalendarConfig struct {
	SourceCalendar string `yaml:"sourceCalendar"`
	FyllaCalendar  string `yaml:"fyllaCalendar"`
}

// SchedulingConfig holds scheduling parameters.
type SchedulingConfig struct {
	WindowDays             int `yaml:"windowDays"`
	MinTaskDurationMinutes int `yaml:"minTaskDurationMinutes"`
	BufferMinutes          int `yaml:"bufferMinutes"`
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

// ensure yaml import is used
var _ = yaml.Node{}
