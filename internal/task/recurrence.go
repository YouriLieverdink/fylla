package task

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	recurrenceRe = regexp.MustCompile(`(?i)\(every\s+([^)]+)\)`)
	dayNames     = map[string]int{
		"monday": 1, "mon": 1,
		"tuesday": 2, "tue": 2,
		"wednesday": 3, "wed": 3,
		"thursday": 4, "thu": 4,
		"friday": 5, "fri": 5,
		"saturday": 6, "sat": 6,
		"sunday": 7, "sun": 7,
	}
	weekdayDays = []int{1, 2, 3, 4, 5}
)

// ExtractRecurrence parses an "(every ...)" clause from a summary string.
// Returns the cleaned summary and a Recurrence if found.
func ExtractRecurrence(summary string) (string, *Recurrence) {
	m := recurrenceRe.FindStringSubmatch(summary)
	if m == nil {
		return summary, nil
	}

	rec := parseRecurrenceSpec(m[1])
	if rec == nil {
		return summary, nil
	}

	cleaned := strings.TrimSpace(spacesRe.ReplaceAllString(
		recurrenceRe.ReplaceAllString(summary, ""), " "))
	return cleaned, rec
}

func parseRecurrenceSpec(spec string) *Recurrence {
	spec = strings.TrimSpace(spec)
	lower := strings.ToLower(spec)
	words := strings.Fields(lower)

	if len(words) == 0 {
		return nil
	}

	// "every day"
	if lower == "day" {
		return &Recurrence{Freq: "daily"}
	}

	// "every weekday"
	if lower == "weekday" {
		return &Recurrence{Freq: "weekly", Days: weekdayDays}
	}

	// "every month"
	if lower == "month" {
		return &Recurrence{Freq: "monthly"}
	}

	// "every 2 weeks on Friday"
	if len(words) >= 2 && words[0] == "2" && words[1] == "weeks" {
		rec := &Recurrence{Freq: "biweekly"}
		// Parse optional "on <days>"
		for i := 2; i < len(words); i++ {
			if words[i] == "on" {
				continue
			}
			if d, ok := dayNames[words[i]]; ok {
				rec.Days = append(rec.Days, d)
			}
		}
		return rec
	}

	// "every Monday Wednesday Friday" or "every Monday"
	var days []int
	for _, w := range words {
		if d, ok := dayNames[w]; ok {
			days = append(days, d)
		}
	}
	if len(days) > 0 {
		return &Recurrence{Freq: "weekly", Days: days}
	}

	return nil
}

// ExpandRecurring generates task instances for a recurring task within a time window.
// Each instance gets a key of "originalKey@YYYY-MM-DD", its own NotBefore set to the
// instance date, and Recurrence set to nil (so it won't be re-expanded).
func ExpandRecurring(t Task, windowStart, windowEnd time.Time) []Task {
	if t.Recurrence == nil {
		return []Task{t}
	}

	dates := generateDates(t.Recurrence, windowStart, windowEnd)
	if len(dates) == 0 {
		return nil
	}

	instances := make([]Task, 0, len(dates))
	for _, d := range dates {
		inst := t
		inst.Key = fmt.Sprintf("%s@%s", t.Key, d.Format("2006-01-02"))
		nb := d
		inst.NotBefore = &nb
		due := d
		inst.DueDate = &due
		inst.Recurrence = nil
		instances = append(instances, inst)
	}
	return instances
}

// ExpandAll replaces recurring tasks with instances within the window.
// Non-recurring tasks are passed through unchanged.
func ExpandAll(tasks []Task, start, end time.Time) []Task {
	var result []Task
	for _, t := range tasks {
		if t.Recurrence != nil {
			result = append(result, ExpandRecurring(t, start, end)...)
		} else {
			result = append(result, t)
		}
	}
	return result
}

func generateDates(rec *Recurrence, start, end time.Time) []time.Time {
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	var dates []time.Time

	switch rec.Freq {
	case "daily":
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			if matchesDay(d, rec.Days) {
				dates = append(dates, d)
			}
		}

	case "weekly":
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			if matchesDay(d, rec.Days) {
				dates = append(dates, d)
			}
		}

	case "biweekly":
		// Find the first matching day on or after start, then skip every other week
		first := time.Time{}
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			if matchesDay(d, rec.Days) {
				if first.IsZero() {
					first = d
					dates = append(dates, d)
				} else {
					weeksSince := int(d.Sub(first).Hours()/24) / 7
					if weeksSince%2 == 0 {
						dates = append(dates, d)
					}
				}
			}
		}

	case "monthly":
		// Generate one date per month on the same day as the start
		day := startDate.Day()
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 1, 0) {
			candidate := time.Date(d.Year(), d.Month(), day, 0, 0, 0, 0, d.Location())
			if candidate.Month() != d.Month() {
				// Day doesn't exist in this month (e.g., Feb 31); skip
				continue
			}
			if !candidate.Before(startDate) && !candidate.After(endDate) {
				dates = append(dates, candidate)
			}
		}
	}

	return dates
}

func matchesDay(d time.Time, days []int) bool {
	if len(days) == 0 {
		return true
	}
	iso := isoWeekday(d)
	for _, day := range days {
		if day == iso {
			return true
		}
	}
	return false
}

func isoWeekday(d time.Time) int {
	w := int(d.Weekday())
	if w == 0 {
		return 7
	}
	return w
}

// StripInstanceSuffix removes the @YYYY-MM-DD suffix from an expanded recurring task key.
// Returns the original key and true if a suffix was found, or the key unchanged and false.
func StripInstanceSuffix(key string) (string, bool) {
	if len(key) > 11 && key[len(key)-11] == '@' {
		return key[:len(key)-11], true
	}
	return key, false
}
