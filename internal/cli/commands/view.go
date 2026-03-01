package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
)

// ViewParams holds inputs for the view command.
type ViewParams struct {
	TaskKey string
	Source  TaskSource
}

// ViewResult holds the output of a view operation.
type ViewResult struct {
	Key       string
	Summary   string
	Priority  int
	Estimate  time.Duration
	DueDate   *time.Time
	NotBefore *time.Time
	UpNext    bool
	NoSplit   bool
}

// RunView fetches task details from the provider.
func RunView(ctx context.Context, p ViewParams) (*ViewResult, error) {
	summary, err := p.Source.GetSummary(ctx, p.TaskKey)
	if err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}

	est, err := p.Source.GetEstimate(ctx, p.TaskKey)
	if err != nil {
		return nil, fmt.Errorf("get estimate: %w", err)
	}

	due, err := p.Source.GetDueDate(ctx, p.TaskKey)
	if err != nil {
		return nil, fmt.Errorf("get due date: %w", err)
	}

	pri, err := p.Source.GetPriority(ctx, p.TaskKey)
	if err != nil {
		return nil, fmt.Errorf("get priority: %w", err)
	}

	// Extract constraints from summary
	cleaned, notBefore, _, upNext, noSplit := task.ExtractConstraints(summary, time.Now(), due)

	return &ViewResult{
		Key:       p.TaskKey,
		Summary:   cleaned,
		Priority:  pri,
		Estimate:  est,
		DueDate:   due,
		NotBefore: notBefore,
		UpNext:    upNext,
		NoSplit:   noSplit,
	}, nil
}

// PrintViewResult writes the task details to the given writer.
func PrintViewResult(w io.Writer, result *ViewResult) {
	fmt.Fprintf(w, "Key:       %s\n", result.Key)
	fmt.Fprintf(w, "Summary:   %s\n", result.Summary)

	if name, ok := priorityLevelNames[result.Priority]; ok {
		fmt.Fprintf(w, "Priority:  %s\n", name)
	}

	if result.Estimate > 0 {
		fmt.Fprintf(w, "Estimate:  %s\n", formatDuration(result.Estimate))
	} else {
		fmt.Fprintf(w, "Estimate:  none\n")
	}

	if result.DueDate != nil {
		fmt.Fprintf(w, "Due:       %s\n", result.DueDate.Format("Mon Jan 2, 2006"))
	} else {
		fmt.Fprintf(w, "Due:       none\n")
	}

	if result.NotBefore != nil {
		fmt.Fprintf(w, "Not Before: %s\n", result.NotBefore.Format("Mon Jan 2, 2006"))
	}

	if result.UpNext {
		fmt.Fprintf(w, "Up Next:   yes\n")
	}
	if result.NoSplit {
		fmt.Fprintf(w, "No Split:  yes\n")
	}
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view TASK-KEY",
		Short: "View task details",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			result, err := RunView(cmd.Context(), ViewParams{
				TaskKey: args[0],
				Source:  source,
			})
			if err != nil {
				return err
			}

			PrintViewResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
