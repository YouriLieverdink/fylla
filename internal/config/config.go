package config

import (
	"fmt"
	"time"
)

// Config represents the fylla configuration file.
type Config struct {
	Providers     []string                         `yaml:"providers"`
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
	Targets       []TargetConfig                   `yaml:"targets"`
}

// ActiveProviders returns the list of configured providers.
func (c *Config) ActiveProviders() []string {
	return c.Providers
}

// TodoistConfig holds Todoist connection settings.
type TodoistConfig struct {
	DefaultFilter  string `yaml:"defaultFilter"`
	DefaultProject string `yaml:"defaultProject"`
}

// GitHubConfig holds GitHub PR review settings.
type GitHubConfig struct {
	DefaultQueries []string `yaml:"defaultQueries"`
	Repos          []string `yaml:"repos"`
}

// KendoConfig holds Kendo connection settings.
type KendoConfig struct {
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
	SourceCalendars []string `yaml:"sourceCalendars"`
	FyllaCalendar   string   `yaml:"fyllaCalendar"`
}

// SchedulingConfig holds scheduling parameters.
type SchedulingConfig struct {
	WindowDays              int   `yaml:"windowDays"`
	MinTaskDurationMinutes  int   `yaml:"minTaskDurationMinutes"`
	MaxTaskDurationMinutes  int   `yaml:"maxTaskDurationMinutes"`
	BufferMinutes           int   `yaml:"bufferMinutes"`
	TravelBufferMinutes     int   `yaml:"travelBufferMinutes"`
	SnapMinutes             []int `yaml:"snapMinutes"`
	DefaultEstimateMinutes  int   `yaml:"defaultEstimateMinutes"`
	ProviderTimeoutSeconds  int   `yaml:"providerTimeoutSeconds"`
	TaskCacheTTLSeconds     int   `yaml:"taskCacheTTLSeconds"`
	PreviewTimeoutSeconds   int   `yaml:"previewTimeoutSeconds"`
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

// TargetScopeMe counts only the current user's worklog entries.
const TargetScopeMe = "me"

// TargetScopeAnyone counts all users' worklog entries.
const TargetScopeAnyone = "anyone"

// Recurring period values for TargetConfig.Period.
const (
	TargetPeriodMonthly  = "monthly"
	TargetPeriodWeekly   = "weekly"
	TargetPeriodBiweekly = "biweekly"
)

// TargetConfig defines a soft hour goal scoped to a single project.
//
// Period selects a recurring window that resolves against "now":
//   - monthly (default): calendar month
//   - weekly: 7-day window starting on the anchor's weekday (default Monday)
//   - biweekly: 14-day window aligned to Anchor (required)
//
// Set StartDate+EndDate (mutually exclusive with Period) to pin a fixed range.
type TargetConfig struct {
	Project   string  `yaml:"project"`
	Hours     float64 `yaml:"hours"`
	Scope     string  `yaml:"scope"`
	Period    string  `yaml:"period,omitempty"`
	Anchor    string  `yaml:"anchor,omitempty"`
	StartDate string  `yaml:"startDate,omitempty"`
	EndDate   string  `yaml:"endDate,omitempty"`
}

// ResolvePeriod returns the inclusive start and end dates for the target's
// current cycle relative to now.
func (t TargetConfig) ResolvePeriod(now time.Time) (time.Time, time.Time, error) {
	loc := now.Location()
	if t.StartDate != "" && t.EndDate != "" {
		s, err := time.ParseInLocation("2006-01-02", t.StartDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("startDate: %w", err)
		}
		e, err := time.ParseInLocation("2006-01-02", t.EndDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("endDate: %w", err)
		}
		return s, e, nil
	}

	day := dayOf(now)

	switch t.Period {
	case "", TargetPeriodMonthly:
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		last := first.AddDate(0, 1, -1)
		return first, last, nil

	case TargetPeriodWeekly:
		anchorWeekday := time.Monday
		if t.Anchor != "" {
			a, err := time.ParseInLocation("2006-01-02", t.Anchor, loc)
			if err != nil {
				return time.Time{}, time.Time{}, fmt.Errorf("anchor: %w", err)
			}
			anchorWeekday = a.Weekday()
		}
		offset := (int(day.Weekday()) - int(anchorWeekday) + 7) % 7
		start := day.AddDate(0, 0, -offset)
		end := start.AddDate(0, 0, 6)
		return start, end, nil

	case TargetPeriodBiweekly:
		if t.Anchor == "" {
			return time.Time{}, time.Time{}, fmt.Errorf("anchor: required for biweekly period")
		}
		a, err := time.ParseInLocation("2006-01-02", t.Anchor, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("anchor: %w", err)
		}
		anchor := dayOf(a)
		days := int(day.Sub(anchor) / (24 * time.Hour))
		// Floor toward negative infinity so cycles also resolve before the anchor.
		cycle := days / 14
		if days < 0 && days%14 != 0 {
			cycle--
		}
		start := anchor.AddDate(0, 0, cycle*14)
		end := start.AddDate(0, 0, 13)
		return start, end, nil
	}

	return time.Time{}, time.Time{}, fmt.Errorf("unknown period %q", t.Period)
}

// PeriodLabel returns a human-readable label for the target's current cycle.
func (t TargetConfig) PeriodLabel(now time.Time) string {
	if t.StartDate != "" && t.EndDate != "" {
		return t.StartDate + ".." + t.EndDate
	}
	switch t.Period {
	case "", TargetPeriodMonthly:
		return now.Format("January 2006")
	case TargetPeriodWeekly, TargetPeriodBiweekly:
		s, e, err := t.ResolvePeriod(now)
		if err != nil {
			return t.Period
		}
		return s.Format("Jan 2") + "–" + e.Format("Jan 2")
	}
	return t.Period
}

// dayOf truncates t to midnight in its location.
func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
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
	if len(c.Providers) == 0 {
		return fmt.Errorf("providers: at least one provider must be configured (set 'providers' in config.yaml)")
	}
	seen := make(map[string]bool)
	for _, p := range c.Providers {
		switch p {
		case "todoist", "github", "local", "kendo":
		default:
			return fmt.Errorf("unknown provider %q (must be 'todoist', 'github', 'local', or 'kendo')", p)
		}
		if seen[p] {
			return fmt.Errorf("duplicate provider %q", p)
		}
		seen[p] = true
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

	// Targets
	for i, t := range c.Targets {
		prefix := fmt.Sprintf("targets[%d]", i)
		if t.Project == "" {
			return fmt.Errorf("%s.project: required", prefix)
		}
		if t.Hours <= 0 {
			return fmt.Errorf("%s.hours: must be positive", prefix)
		}
		switch t.Scope {
		case "", TargetScopeMe, TargetScopeAnyone:
		default:
			return fmt.Errorf("%s.scope: must be %q or %q", prefix, TargetScopeMe, TargetScopeAnyone)
		}
		switch t.Period {
		case "", TargetPeriodMonthly, TargetPeriodWeekly, TargetPeriodBiweekly:
		default:
			return fmt.Errorf("%s.period: must be one of %q, %q, %q", prefix,
				TargetPeriodMonthly, TargetPeriodWeekly, TargetPeriodBiweekly)
		}
		hasStart := t.StartDate != ""
		hasEnd := t.EndDate != ""
		hasFixed := hasStart || hasEnd
		hasRecurring := t.Period != ""
		if hasStart != hasEnd {
			return fmt.Errorf("%s: startDate and endDate must be set together", prefix)
		}
		if hasFixed && hasRecurring {
			return fmt.Errorf("%s: use either period or startDate/endDate, not both", prefix)
		}
		if hasStart {
			s, err := time.Parse("2006-01-02", t.StartDate)
			if err != nil {
				return fmt.Errorf("%s.startDate: %w", prefix, err)
			}
			e, err := time.Parse("2006-01-02", t.EndDate)
			if err != nil {
				return fmt.Errorf("%s.endDate: %w", prefix, err)
			}
			if s.After(e) {
				return fmt.Errorf("%s: startDate must be on or before endDate", prefix)
			}
		}
		if t.Anchor != "" {
			if _, err := time.Parse("2006-01-02", t.Anchor); err != nil {
				return fmt.Errorf("%s.anchor: %w", prefix, err)
			}
		}
		if t.Period == TargetPeriodBiweekly && t.Anchor == "" {
			return fmt.Errorf("%s.anchor: required for biweekly period", prefix)
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
