package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/spf13/cobra"
)

// ListParams holds inputs for the list operation.
type ListParams struct {
	Tasks TaskFetcher
	Cfg   *config.Config
	Query string
	Now   time.Time
}

// ListResult holds the output of a list operation.
type ListResult struct {
	Tasks []scheduler.ScoredTask
}

// RunList fetches and sorts tasks without scheduling.
func RunList(ctx context.Context, p ListParams) (*ListResult, error) {
	tasks, err := p.Tasks.FetchTasks(ctx, p.Query)
	if err != nil {
		return nil, fmt.Errorf("fetch tasks: %w", err)
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
		var parts []string
		if st.Task.IssueType != "" {
			parts = append(parts, st.Task.IssueType)
		}
		parts = append(parts, formatDuration(st.Task.RemainingEstimate))
		if st.Task.DueDate != nil {
			parts = append(parts, "due "+st.Task.DueDate.Format("Jan 2"))
		}
		fmt.Fprintf(w, "  %d. %s: %s [%s] (score: %.1f)\n",
			i+1, st.Task.Key, st.Task.Summary,
			strings.Join(parts, ", "), st.Score)
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
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			jql, _ := cmd.Flags().GetString("jql")
			filter, _ := cmd.Flags().GetString("filter")

			var query string
			switch cfg.Source {
			case "todoist":
				query = filter
				if query == "" {
					query = cfg.Todoist.DefaultFilter
				}
			default:
				query = jql
				if query == "" {
					query = cfg.Jira.DefaultJQL
				}
			}

			result, err := RunList(cmd.Context(), ListParams{
				Tasks: source.(TaskFetcher),
				Cfg:   cfg,
				Query: query,
				Now:   time.Now(),
			})
			if err != nil {
				return err
			}

			PrintListResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	cmd.Flags().String("jql", "", "Custom JQL query override (Jira source)")
	cmd.Flags().String("filter", "", "Custom filter override (Todoist source)")

	return cmd
}
