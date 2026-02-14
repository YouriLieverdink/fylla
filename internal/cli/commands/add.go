package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
)

// defaultProject returns the configured default project for the given provider.
func defaultProject(cfg *config.Config, provider string) string {
	switch provider {
	case "todoist":
		return cfg.Todoist.DefaultProject
	default:
		return cfg.Jira.DefaultProject
	}
}

// TaskCreator abstracts task creation for testing.
type TaskCreator interface {
	CreateTask(ctx context.Context, input task.CreateInput) (string, error)
}

// ProjectLister returns available project names.
type ProjectLister interface {
	ListProjects(ctx context.Context) ([]string, error)
}

// AddParams holds inputs for the add command.
type AddParams struct {
	Project     string
	IssueType   string
	Summary     string
	Description string
	Estimate    string // raw duration string
	DueDate     string // raw date string, e.g. "2025-02-15"
	Priority    string
	Inline      bool // true when args were provided on the command line
	Creator     TaskCreator
	Projects    ProjectLister
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
		Summary:     p.Summary,
		Description: p.Description,
		IssueType:   p.IssueType,
		Priority:    p.Priority,
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
	if provider == "jira" && p.IssueType == "" {
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

	// Append constraints back to summary using ISO dates for reliable re-parsing
	if parsed.NotBefore != nil {
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

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [inline task description]",
		Short: "Create a new task",
		Long: `Create a new task interactively or with inline attributes.

Inline syntax:
  fylla task add 'Write the docs [30m] (due Friday priority:p1 not before Monday upnext nosplit)'

The estimate goes in [brackets]. Attributes inside (parentheses) are extracted
and used for scheduling.

Extracted attributes inside ():
  due <date>          Due date (natural language or YYYY-MM-DD)
  not before <date>   Earliest scheduling date (natural language or YYYY-MM-DD)
  priority:<level>    Priority (p1=Highest, p2=High, p3=Medium, p4=Low, p5=Lowest)
  upnext              Schedule before other tasks
  nosplit             Prevent splitting across multiple slots`,
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			providerFlag, _ := cmd.Flags().GetString("provider")
			if providerFlag == "" {
				providerFlag = cfg.ActiveProviders()[0]
			}

			project, _ := cmd.Flags().GetString("project")
			if project == "" {
				project = defaultProject(cfg, providerFlag)
			}

			// Route creation to the specified provider when using MultiTaskSource
			var creator TaskCreator
			if ms, ok := source.(*MultiTaskSource); ok {
				if src, exists := ms.sources[providerFlag]; exists {
					creator = src
				} else {
					creator = source
				}
			} else {
				creator = source
			}

			p := AddParams{
				Project: project,
				Creator: creator,
			}
			if pl, ok := creator.(ProjectLister); ok {
				p.Projects = pl
			}

			// If positional args are provided, parse them as inline input
			if len(args) > 0 {
				parsed := task.ParseInput(strings.Join(args, " "), time.Now())
				applyParsedInput(&p, parsed)
				p.Inline = true
			}

			// Default IssueType to "Task" for Jira when not provided
		if providerFlag == "jira" && p.IssueType == "" {
			p.IssueType = "Task"
		}

		for _, field := range RequiredFields(p, providerFlag) {
				switch field {
				case "project":
					if p.Projects != nil {
						names, err := p.Projects.ListProjects(cmd.Context())
						if err == nil && len(names) > 0 {
							prompt := &survey.Select{
								Message: "Project:",
								Options: names,
							}
							if err := survey.AskOne(prompt, &p.Project); err != nil {
								return fmt.Errorf("prompt project: %w", err)
							}
							break
						}
					}
					prompt := &survey.Input{Message: "Project key:"}
					if err := survey.AskOne(prompt, &p.Project); err != nil {
						return fmt.Errorf("prompt project: %w", err)
					}
				case "issueType":
					prompt := &survey.Select{
						Message: "Issue type:",
						Options: []string{"Task", "Bug", "Story", "Epic"},
					}
					if err := survey.AskOne(prompt, &p.IssueType); err != nil {
						return fmt.Errorf("prompt issue type: %w", err)
					}
				case "summary":
					var raw string
					prompt := &survey.Input{Message: "Summary:"}
					if err := survey.AskOne(prompt, &raw); err != nil {
						return fmt.Errorf("prompt summary: %w", err)
					}
					parsed := task.ParseInput(raw, time.Now())
					applyParsedInput(&p, parsed)
				case "description":
					prompt := &survey.Input{Message: "Description:"}
					if err := survey.AskOne(prompt, &p.Description); err != nil {
						return fmt.Errorf("prompt description: %w", err)
					}
				case "estimate":
					prompt := &survey.Input{Message: "Estimate (e.g. 2h, 30m, 1h30m):"}
					if err := survey.AskOne(prompt, &p.Estimate); err != nil {
						return fmt.Errorf("prompt estimate: %w", err)
					}
				case "dueDate":
					prompt := &survey.Input{Message: "Due date (YYYY-MM-DD or natural language):"}
					if err := survey.AskOne(prompt, &p.DueDate); err != nil {
						return fmt.Errorf("prompt due date: %w", err)
					}
				case "priority":
					prompt := &survey.Select{
						Message: "Priority:",
						Options: []string{"Highest", "High", "Medium", "Low", "Lowest"},
						Default: "Medium",
					}
					if err := survey.AskOne(prompt, &p.Priority); err != nil {
						return fmt.Errorf("prompt priority: %w", err)
					}
				}
			}

			result, err := RunAdd(cmd.Context(), p)
			if err != nil {
				return err
			}

			PrintAddResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().String("project", "", "Pre-select project")
	cmd.Flags().String("provider", "", "Provider to create the task on (defaults to first configured)")

	return cmd
}
