package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var upnextRe = regexp.MustCompile(`(?i)\bupnext\b`)

// EditParams holds inputs for the edit command.
type EditParams struct {
	TaskKey   string
	Estimate  string
	Due       string
	NoDue     bool
	Priority  string
	UpNext    bool
	NoUpNext  bool
	Source    TaskSource
}

// EditResult holds the output of an edit operation.
type EditResult struct {
	TaskKey         string
	EstimateResult  *EstimateResult
	DueDateResult   *DueDateResult
	DueDateRemoved  bool
	PriorityResult  *PriorityResult
	UpNextSet       bool
	UpNextRemoved   bool
}

// RunEdit applies one or more edits to a task.
func RunEdit(ctx context.Context, p EditParams) (*EditResult, error) {
	result := &EditResult{TaskKey: p.TaskKey}

	if p.Estimate != "" {
		r, err := RunEstimate(ctx, EstimateParams{
			TaskKey:  p.TaskKey,
			Duration: p.Estimate,
			Jira:     p.Source,
			Getter:   p.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("estimate: %w", err)
		}
		result.EstimateResult = r
	}

	if p.Due != "" {
		r, err := RunDueDate(ctx, DueDateParams{
			TaskKey: p.TaskKey,
			Date:    p.Due,
			Jira:    p.Source,
			Getter:  p.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("due date: %w", err)
		}
		result.DueDateResult = r
	}

	if p.NoDue {
		if err := p.Source.RemoveDueDate(ctx, p.TaskKey); err != nil {
			return nil, fmt.Errorf("remove due date: %w", err)
		}
		result.DueDateRemoved = true
	}

	if p.Priority != "" {
		r, err := RunPriority(ctx, PriorityParams{
			TaskKey:  p.TaskKey,
			Priority: p.Priority,
			Updater:  p.Source,
			Getter:   p.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("priority: %w", err)
		}
		result.PriorityResult = r
	}

	if p.UpNext || p.NoUpNext {
		summary, err := p.Source.GetSummary(ctx, p.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("get summary: %w", err)
		}

		hasUpNext := upnextRe.MatchString(summary)

		if p.UpNext && !hasUpNext {
			summary = strings.TrimSpace(summary) + " upnext"
			if err := p.Source.UpdateSummary(ctx, p.TaskKey, summary); err != nil {
				return nil, fmt.Errorf("update summary: %w", err)
			}
			result.UpNextSet = true
		} else if p.UpNext && hasUpNext {
			// Already set, no-op but report success
			result.UpNextSet = true
		} else if p.NoUpNext && hasUpNext {
			summary = strings.TrimSpace(upnextRe.ReplaceAllString(summary, ""))
			// Collapse multiple spaces
			summary = strings.Join(strings.Fields(summary), " ")
			if err := p.Source.UpdateSummary(ctx, p.TaskKey, summary); err != nil {
				return nil, fmt.Errorf("update summary: %w", err)
			}
			result.UpNextRemoved = true
		} else if p.NoUpNext && !hasUpNext {
			// Already absent, no-op but report success
			result.UpNextRemoved = true
		}
	}

	return result, nil
}

// PrintEditResult writes the edit confirmation to the given writer.
func PrintEditResult(w io.Writer, result *EditResult) {
	if result.EstimateResult != nil {
		PrintEstimateResult(w, result.EstimateResult)
	}
	if result.DueDateResult != nil {
		PrintDueDateResult(w, result.DueDateResult)
	}
	if result.DueDateRemoved {
		fmt.Fprintf(w, "Due date for %s removed\n", result.TaskKey)
	}
	if result.PriorityResult != nil {
		PrintPriorityResult(w, result.PriorityResult)
	}
	if result.UpNextSet {
		fmt.Fprintf(w, "%s marked as up next\n", result.TaskKey)
	}
	if result.UpNextRemoved {
		fmt.Fprintf(w, "%s unmarked as up next\n", result.TaskKey)
	}
}

func newEditCmd() *cobra.Command {
	var (
		estimate string
		due      string
		noDue    bool
		priority string
		upNext   bool
		noUpNext bool
	)

	cmd := &cobra.Command{
		Use:   "edit TASK-KEY",
		Short: "Edit task properties",
		Long:  "Set or adjust estimate, due date, priority, and up-next status on a task",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if estimate == "" && due == "" && !noDue && priority == "" && !upNext && !noUpNext {
				return fmt.Errorf("at least one flag is required (--estimate, --due, --no-due, --priority, --up-next, --no-up-next)")
			}
			if due != "" && noDue {
				return fmt.Errorf("--due and --no-due are mutually exclusive")
			}
			if upNext && noUpNext {
				return fmt.Errorf("--up-next and --no-up-next are mutually exclusive")
			}

			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			result, err := RunEdit(cmd.Context(), EditParams{
				TaskKey:  args[0],
				Estimate: estimate,
				Due:      due,
				NoDue:    noDue,
				Priority: priority,
				UpNext:   upNext,
				NoUpNext: noUpNext,
				Source:   source,
			})
			if err != nil {
				return err
			}

			PrintEditResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().StringVarP(&estimate, "estimate", "e", "", "set estimate (e.g. 4h, +2h, -1h)")
	cmd.Flags().StringVarP(&due, "due", "d", "", "set due date (YYYY-MM-DD, natural language, +7d, -3d)")
	cmd.Flags().BoolVar(&noDue, "no-due", false, "remove due date")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "set priority (Highest, High, Medium, Low, Lowest, 1-5, +1, -1)")
	cmd.Flags().BoolVar(&upNext, "up-next", false, "mark as up next")
	cmd.Flags().BoolVar(&noUpNext, "no-up-next", false, "unmark as up next")

	return cmd
}
