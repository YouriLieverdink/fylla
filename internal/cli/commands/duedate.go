package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tj/go-naturaldate"
)

// DueDateParams holds inputs for the due date command.
type DueDateParams struct {
	TaskKey string
	Date    string // raw date string, e.g. "2025-02-15", "+7d", "-3d"
	Jira    DueDateUpdater
	Getter  DueDateGetter
}

// DueDateResult holds the output of a due date operation.
type DueDateResult struct {
	TaskKey string
	DueDate time.Time
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var relativeDaysRe = regexp.MustCompile(`^(\d+)d$`)

// ParseDate parses a date string in YYYY-MM-DD format, or falls back
// to natural language parsing (e.g. "Friday", "next Monday", "tomorrow").
func ParseDate(s string) (time.Time, error) {
	if dateRe.MatchString(s) {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date %q: %w", s, err)
		}
		return d, nil
	}
	d, err := naturaldate.Parse(s, time.Now(), naturaldate.WithDirection(naturaldate.Future))
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse date %q: %w", s, err)
	}
	return d, nil
}

// parseRelativeDays parses a relative day offset like "7d".
func parseRelativeDays(s string) (int, error) {
	matches := relativeDaysRe.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid relative offset %q (expected format: 7d)", s)
	}
	return strconv.Atoi(matches[1])
}

// RunDueDate sets or adjusts the due date on a Jira issue.
func RunDueDate(ctx context.Context, p DueDateParams) (*DueDateResult, error) {
	raw := strings.TrimSpace(p.Date)
	if raw == "" {
		return nil, fmt.Errorf("date is required")
	}

	var final time.Time

	if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		sign := raw[0]
		days, err := parseRelativeDays(raw[1:])
		if err != nil {
			return nil, fmt.Errorf("parse adjustment: %w", err)
		}

		current, err := p.Getter.GetDueDate(ctx, p.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("get current due date: %w", err)
		}

		base := time.Now()
		if current != nil {
			base = *current
		}

		if sign == '+' {
			final = base.AddDate(0, 0, days)
		} else {
			final = base.AddDate(0, 0, -days)
		}
	} else {
		d, err := ParseDate(raw)
		if err != nil {
			return nil, err
		}
		final = d
	}

	if err := p.Jira.UpdateDueDate(ctx, p.TaskKey, final); err != nil {
		return nil, err
	}

	return &DueDateResult{
		TaskKey: p.TaskKey,
		DueDate: final,
	}, nil
}

// PrintDueDateResult writes the due date confirmation to the given writer.
func PrintDueDateResult(w io.Writer, result *DueDateResult) {
	fmt.Fprintf(w, "Due date for %s set to %s\n", result.TaskKey, result.DueDate.Format("2006-01-02"))
}

