package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/spf13/cobra"
)

// ListParams holds inputs for the list operation.
type ListParams struct {
	Jira JiraFetcher
	Cfg  *config.Config
	JQL  string
	Now  time.Time
}

// ListResult holds the output of a list operation.
type ListResult struct {
	Tasks []scheduler.ScoredTask
}

// RunList fetches and sorts tasks without scheduling.
func RunList(ctx context.Context, p ListParams) (*ListResult, error) {
	tasks, err := p.Jira.FetchTasks(ctx, p.JQL)
	if err != nil {
		return nil, fmt.Errorf("fetch jira tasks: %w", err)
	}

	sorted := scheduler.SortTasks(tasks, p.Cfg.Weights, p.Cfg.TypeScores, p.Now)

	return &ListResult{Tasks: sorted}, nil
}

// PrintListResult writes the sorted task list to the given writer.
func PrintListResult(w io.Writer, result *ListResult) {
	if len(result.Tasks) == 0 {
		fmt.Fprintln(w, "No tasks found.")
		return
	}

	fmt.Fprintf(w, "%d task(s):\n", len(result.Tasks))
	for i, st := range result.Tasks {
		dueStr := ""
		if st.Task.DueDate != nil {
			dueStr = fmt.Sprintf("  due %s", st.Task.DueDate.Format("Jan 2"))
		}
		est := formatDuration(st.Task.RemainingEstimate)
		fmt.Fprintf(w, "  %d. %s: %s  [%s %s%s]  (score: %.1f)\n",
			i+1, st.Task.Key, st.Task.Summary,
			st.Task.IssueType, est, dueStr, st.Score)
	}
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "no estimate"
	}
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

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show sorted tasks without scheduling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().String("jql", "", "Custom JQL query override")

	return cmd
}
