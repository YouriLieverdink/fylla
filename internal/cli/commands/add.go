package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
)

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
	Quick       bool
	Creator     TaskCreator
	Projects    ProjectLister
}

// AddResult holds the output of an add operation.
type AddResult struct {
	Key     string
	Summary string
}

// BuildCreateInput converts AddParams into a task.CreateInput,
// applying defaults for quick mode.
func BuildCreateInput(p AddParams) (task.CreateInput, error) {
	input := task.CreateInput{
		Project:     p.Project,
		Summary:     p.Summary,
		IssueType:   p.IssueType,
		Description: p.Description,
		Priority:    p.Priority,
	}

	if p.Quick {
		if input.IssueType == "" {
			input.IssueType = "Task"
		}
		if input.Priority == "" {
			input.Priority = "Medium"
		}
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

// RequiredFields returns the list of field names that need prompting
// based on which values are already set and whether quick mode is active.
func RequiredFields(p AddParams) []string {
	var fields []string
	if p.Project == "" {
		fields = append(fields, "project")
	}
	if !p.Quick {
		if p.IssueType == "" {
			fields = append(fields, "issueType")
		}
	}
	fields = append(fields, "summary")
	if !p.Quick {
		fields = append(fields, "description")
	}
	fields = append(fields, "estimate")
	fields = append(fields, "dueDate")
	if !p.Quick {
		if p.Priority == "" {
			fields = append(fields, "priority")
		}
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

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new task interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			quick, _ := cmd.Flags().GetBool("quick")
			project, _ := cmd.Flags().GetString("project")

			p := AddParams{
				Project: project,
				Quick:   quick,
				Creator: source,
			}
			if pl, ok := source.(ProjectLister); ok {
				p.Projects = pl
			}

			for _, field := range RequiredFields(p) {
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
					prompt := &survey.Input{Message: "Summary:"}
					if err := survey.AskOne(prompt, &p.Summary); err != nil {
						return fmt.Errorf("prompt summary: %w", err)
					}
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
					prompt := &survey.Input{Message: "Due date (YYYY-MM-DD):"}
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

	cmd.Flags().Bool("quick", false, "Quick mode - only essential fields")
	cmd.Flags().String("project", "", "Pre-select project")

	return cmd
}
