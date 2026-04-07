package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
)

// defaultProject returns the configured default project for the given provider.
func defaultProject(cfg *config.Config, provider string) string {
	switch provider {
	case "todoist":
		return cfg.Todoist.DefaultProject
	case "local":
		return cfg.Local.DefaultProject
	default:
		return cfg.Jira.DefaultProject
	}
}

// AddParams holds inputs for the add command.
type AddParams struct {
	Project     string
	Section     string
	IssueType   string
	Summary     string
	Description string
	Estimate    string // raw duration string
	DueDate     string // raw date string, e.g. "2025-02-15"
	Priority    string
	Parent      string
	SprintID    *int   // Sprint/iteration ID (Kendo)
	Lane        string // Board column / lane name (Kendo)
	Inline      bool   // true when args were provided on the command line
	Creator     TaskCreator
	Projects    ProjectLister
	Sections    SectionLister
	Epics       EpicLister
}

// AddResult holds the output of an add operation.
type AddResult struct {
	Key     string
	Summary string
}

// BuildCreateInput converts AddParams into a task.CreateInput,
// applying sensible defaults when fields are empty.
func BuildCreateInput(p AddParams) (task.CreateInput, error) {
	input := task.CreateInput{
		Project:     p.Project,
		Section:     p.Section,
		Summary:     p.Summary,
		Description: p.Description,
		IssueType:   p.IssueType,
		Priority:    p.Priority,
		Parent:      p.Parent,
		SprintID:    p.SprintID,
		Lane:        p.Lane,
	}

	if input.Priority == "" {
		input.Priority = "Medium"
	}

	if p.Estimate != "" {
		dur, err := ParseDuration(p.Estimate)
		if err != nil {
			return task.CreateInput{}, fmt.Errorf("parse estimate: %w", err)
		}
		input.Estimate = dur
	}

	if p.DueDate != "" {
		d, err := ParseDate(p.DueDate)
		if err != nil {
			return task.CreateInput{}, fmt.Errorf("parse due date: %w", err)
		}
		input.DueDate = &d
	}

	return input, nil
}

// RequiredFields returns the list of field names that need prompting.
// In inline mode (args provided), only project is prompted if missing.
// In interactive mode (no args), all empty fields are prompted.
// The provider parameter controls provider-specific fields (e.g. issueType for Jira only).
func RequiredFields(p AddParams, provider string) []string {
	var fields []string
	if p.Project == "" {
		fields = append(fields, "project")
	}
	if p.Inline {
		return fields
	}
	if (provider == "jira" || provider == "kendo") && p.IssueType == "" {
		fields = append(fields, "issueType")
	}
	if p.Summary == "" {
		fields = append(fields, "summary")
	}
	if p.Description == "" {
		fields = append(fields, "description")
	}
	if p.Estimate == "" {
		fields = append(fields, "estimate")
	}
	if p.DueDate == "" {
		fields = append(fields, "dueDate")
	}
	if p.Priority == "" {
		fields = append(fields, "priority")
	}
	if (provider == "todoist" || provider == "local") && p.Section == "" {
		fields = append(fields, "section")
	}
	if provider == "jira" && p.Parent == "" {
		fields = append(fields, "parent")
	}
	return fields
}

// RunAdd creates a new task using the collected parameters.
func RunAdd(ctx context.Context, p AddParams) (*AddResult, error) {
	input, err := BuildCreateInput(p)
	if err != nil {
		return nil, err
	}

	key, err := p.Creator.CreateTask(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return &AddResult{
		Key:     key,
		Summary: p.Summary,
	}, nil
}

// PrintAddResult writes the add confirmation to the given writer.
func PrintAddResult(w io.Writer, result *AddResult) {
	fmt.Fprintf(w, "Created %s: %s\n", result.Key, result.Summary)
}

// applyParsedInput populates AddParams from a ParsedInput result.
func applyParsedInput(p *AddParams, parsed task.ParsedInput) {
	summary := parsed.Summary

	// Append constraints back to summary; prefer raw relative offset for dynamic resolution
	if parsed.NotBeforeRaw != "" {
		summary += " not before " + parsed.NotBeforeRaw
	} else if parsed.NotBefore != nil {
		summary += " not before " + parsed.NotBefore.Format("2006-01-02")
	}
	if parsed.UpNext {
		summary += " upnext"
	}
	if parsed.NoSplit {
		summary += " nosplit"
	}

	if summary != "" {
		p.Summary = strings.TrimSpace(summary)
	}
	if parsed.Estimate > 0 {
		p.Estimate = formatEstimate(parsed.Estimate)
	}
	if parsed.DueDate != nil {
		p.DueDate = parsed.DueDate.Format("2006-01-02")
	}
	if parsed.Priority != "" {
		p.Priority = parsed.Priority
	}
}

func formatEstimate(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}
