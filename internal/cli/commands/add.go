package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/iruoy/fylla/internal/jira"
	"github.com/spf13/cobra"
)

// IssueCreator abstracts Jira issue creation for testing.
type IssueCreator interface {
	CreateIssue(ctx context.Context, input jira.CreateIssueInput) (string, error)
}

// AddParams holds inputs for the add command.
type AddParams struct {
	Project     string
	IssueType   string
	Summary     string
	Description string
	Estimate    string // raw duration string
	Priority    string
	Quick       bool
	Jira        IssueCreator
}

// AddResult holds the output of an add operation.
type AddResult struct {
	Key     string
	Summary string
}

// BuildCreateInput converts AddParams into a Jira CreateIssueInput,
// applying defaults for quick mode.
func BuildCreateInput(p AddParams) (jira.CreateIssueInput, error) {
	input := jira.CreateIssueInput{
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
			return jira.CreateIssueInput{}, fmt.Errorf("parse estimate: %w", err)
		}
		input.Estimate = dur
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
	if !p.Quick {
		if p.Priority == "" {
			fields = append(fields, "priority")
		}
	}
	return fields
}

// RunAdd creates a new issue in Jira using the collected parameters.
func RunAdd(ctx context.Context, p AddParams) (*AddResult, error) {
	input, err := BuildCreateInput(p)
	if err != nil {
		return nil, err
	}

	key, err := p.Jira.CreateIssue(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
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
		Short: "Create a new Jira task interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().Bool("quick", false, "Quick mode - only essential fields")
	cmd.Flags().String("project", "", "Pre-select project")

	return cmd
}
