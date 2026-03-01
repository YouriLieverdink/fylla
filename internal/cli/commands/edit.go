package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
)

var (
	upnextRe    = regexp.MustCompile(`(?i)\bupnext\b`)
	nosplitRe   = regexp.MustCompile(`(?i)\bnosplit\b`)
	notBeforeRe = regexp.MustCompile(`(?i)\bnot before \S+`)
)

// EditParams holds inputs for the edit command.
type EditParams struct {
	TaskKey     string
	Summary     string
	Estimate    string
	Due         string
	NoDue       bool
	Priority    string
	UpNext      bool
	NoUpNext    bool
	NoSplit     bool
	NoNoSplit   bool
	NotBefore   string
	NoNotBefore bool
	Source      TaskSource
}

// EditResult holds the output of an edit operation.
type EditResult struct {
	TaskKey          string
	EstimateResult   *EstimateResult
	DueDateResult    *DueDateResult
	DueDateRemoved   bool
	PriorityResult   *PriorityResult
	UpNextSet        bool
	UpNextRemoved    bool
	NoSplitSet       bool
	NoSplitRemoved   bool
	NotBeforeSet     bool
	NotBeforeRemoved bool
	SummaryUpdated   bool
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

	// Summary-keyword operations (upnext, nosplit, not before, direct summary)
	// need to work on the same summary to avoid race conditions.
	needsSummaryUpdate := p.UpNext || p.NoUpNext || p.NoSplit || p.NoNoSplit ||
		p.NotBefore != "" || p.NoNotBefore || p.Summary != ""

	if needsSummaryUpdate {
		summary, err := p.Source.GetSummary(ctx, p.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("get summary: %w", err)
		}

		// Strip bracket estimate (e.g. [2h]) so keyword operations don't lose it
		titleEst, summary := task.ParseTitleEstimate(summary)

		changed := false

		// Handle direct summary update: replace the base summary
		// (strip existing keywords, set new base, then re-apply keywords below)
		if p.Summary != "" {
			// Strip constraint keywords from current summary to get current base
			baseSummary := stripKeywords(summary)
			if baseSummary != p.Summary {
				// Replace base summary while preserving keywords
				summary = p.Summary + extractKeywordSuffix(summary)
				changed = true
				result.SummaryUpdated = true
			}
		}

		// UpNext
		hasUpNext := upnextRe.MatchString(summary)
		if p.UpNext && !hasUpNext {
			summary = strings.TrimSpace(summary) + " upnext"
			changed = true
			result.UpNextSet = true
		} else if p.UpNext && hasUpNext {
			result.UpNextSet = true
		} else if p.NoUpNext && hasUpNext {
			summary = strings.TrimSpace(upnextRe.ReplaceAllString(summary, ""))
			summary = strings.Join(strings.Fields(summary), " ")
			changed = true
			result.UpNextRemoved = true
		} else if p.NoUpNext && !hasUpNext {
			result.UpNextRemoved = true
		}

		// NoSplit
		hasNoSplit := nosplitRe.MatchString(summary)
		if p.NoSplit && !hasNoSplit {
			summary = strings.TrimSpace(summary) + " nosplit"
			changed = true
			result.NoSplitSet = true
		} else if p.NoSplit && hasNoSplit {
			result.NoSplitSet = true
		} else if p.NoNoSplit && hasNoSplit {
			summary = strings.TrimSpace(nosplitRe.ReplaceAllString(summary, ""))
			summary = strings.Join(strings.Fields(summary), " ")
			changed = true
			result.NoSplitRemoved = true
		} else if p.NoNoSplit && !hasNoSplit {
			result.NoSplitRemoved = true
		}

		// NotBefore
		hasNotBefore := notBeforeRe.MatchString(summary)
		if p.NotBefore != "" {
			if hasNotBefore {
				summary = strings.TrimSpace(notBeforeRe.ReplaceAllString(summary, ""))
				summary = strings.Join(strings.Fields(summary), " ")
			}
			summary = strings.TrimSpace(summary) + " not before " + p.NotBefore
			changed = true
			result.NotBeforeSet = true
		} else if p.NoNotBefore && hasNotBefore {
			summary = strings.TrimSpace(notBeforeRe.ReplaceAllString(summary, ""))
			summary = strings.Join(strings.Fields(summary), " ")
			changed = true
			result.NotBeforeRemoved = true
		}

		if changed {
			// Re-add bracket estimate if one was present
			if titleEst > 0 {
				summary = task.SetTitleEstimate(summary, titleEst)
			}
			if err := p.Source.UpdateSummary(ctx, p.TaskKey, summary); err != nil {
				return nil, fmt.Errorf("update summary: %w", err)
			}
		}
	}

	return result, nil
}

// stripKeywords removes constraint keywords (upnext, nosplit, not before <date>) from a summary.
func stripKeywords(summary string) string {
	s := upnextRe.ReplaceAllString(summary, "")
	s = nosplitRe.ReplaceAllString(s, "")
	s = notBeforeRe.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// extractKeywordSuffix returns the keyword portion of a summary.
func extractKeywordSuffix(summary string) string {
	var parts []string
	if upnextRe.MatchString(summary) {
		parts = append(parts, "upnext")
	}
	if nosplitRe.MatchString(summary) {
		parts = append(parts, "nosplit")
	}
	if loc := notBeforeRe.FindString(summary); loc != "" {
		parts = append(parts, loc)
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
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
	if result.SummaryUpdated {
		fmt.Fprintf(w, "%s summary updated\n", result.TaskKey)
	}
	if result.UpNextSet {
		fmt.Fprintf(w, "%s marked as up next\n", result.TaskKey)
	}
	if result.UpNextRemoved {
		fmt.Fprintf(w, "%s unmarked as up next\n", result.TaskKey)
	}
	if result.NoSplitSet {
		fmt.Fprintf(w, "%s marked as no-split\n", result.TaskKey)
	}
	if result.NoSplitRemoved {
		fmt.Fprintf(w, "%s unmarked as no-split\n", result.TaskKey)
	}
	if result.NotBeforeSet {
		fmt.Fprintf(w, "%s not-before date set\n", result.TaskKey)
	}
	if result.NotBeforeRemoved {
		fmt.Fprintf(w, "%s not-before date removed\n", result.TaskKey)
	}
}

func newEditCmd() *cobra.Command {
	var (
		estimate    string
		due         string
		noDue       bool
		priority    string
		upNext      bool
		noUpNext    bool
		noSplit     bool
		noNoSplit   bool
		notBefore   string
		noNotBefore bool
		summary     string
	)

	cmd := &cobra.Command{
		Use:   "edit TASK-KEY",
		Short: "Edit task properties",
		Long:  "Set or adjust estimate, due date, priority, up-next, no-split, not-before, and summary on a task",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if estimate == "" && due == "" && !noDue && priority == "" && !upNext && !noUpNext &&
				!noSplit && !noNoSplit && notBefore == "" && !noNotBefore && summary == "" {
				return fmt.Errorf("at least one flag is required")
			}
			if due != "" && noDue {
				return fmt.Errorf("--due and --no-due are mutually exclusive")
			}
			if upNext && noUpNext {
				return fmt.Errorf("--up-next and --no-up-next are mutually exclusive")
			}
			if noSplit && noNoSplit {
				return fmt.Errorf("--no-split and --no-no-split are mutually exclusive")
			}
			if notBefore != "" && noNotBefore {
				return fmt.Errorf("--not-before and --no-not-before are mutually exclusive")
			}

			source, _, err := loadTaskSource()
			if err != nil {
				return err
			}

			result, err := RunEdit(cmd.Context(), EditParams{
				TaskKey:     args[0],
				Summary:     summary,
				Estimate:    estimate,
				Due:         due,
				NoDue:       noDue,
				Priority:    priority,
				UpNext:      upNext,
				NoUpNext:    noUpNext,
				NoSplit:     noSplit,
				NoNoSplit:   noNoSplit,
				NotBefore:   notBefore,
				NoNotBefore: noNotBefore,
				Source:      source,
			})
			if err != nil {
				return err
			}

			PrintEditResult(cmd.OutOrStdout(), result)
			maybeAutoResync(cmd.Context(), cmd.ErrOrStderr())
			return nil
		},
	}

	cmd.Flags().StringVarP(&estimate, "estimate", "e", "", "set estimate (e.g. 4h, +2h, -1h)")
	cmd.Flags().StringVarP(&due, "due", "d", "", "set due date (YYYY-MM-DD, natural language, +7d, -3d)")
	cmd.Flags().BoolVar(&noDue, "no-due", false, "remove due date")
	cmd.Flags().StringVarP(&priority, "priority", "p", "", "set priority (Highest, High, Medium, Low, Lowest, 1-5, +1, -1)")
	cmd.Flags().BoolVar(&upNext, "up-next", false, "mark as up next")
	cmd.Flags().BoolVar(&noUpNext, "no-up-next", false, "unmark as up next")
	cmd.Flags().BoolVar(&noSplit, "no-split", false, "mark as no-split")
	cmd.Flags().BoolVar(&noNoSplit, "no-no-split", false, "unmark as no-split")
	cmd.Flags().StringVar(&notBefore, "not-before", "", "set not-before date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&noNotBefore, "no-not-before", false, "remove not-before date")
	cmd.Flags().StringVarP(&summary, "summary", "s", "", "set summary text")

	return cmd
}
