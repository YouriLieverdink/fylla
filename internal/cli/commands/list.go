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
func PrintListResult(w io.Writer, result *ListResult, verbose bool) {
	if len(result.Tasks) == 0 {
		fmt.Fprintln(w, "No tasks found.")
		return
	}

	// Compute column widths for alignment.
	maxKey := 0
	maxSummary := 0
	for _, st := range result.Tasks {
		if len(st.Task.Key) > maxKey {
			maxKey = len(st.Task.Key)
		}
		if len(st.Task.Summary) > maxSummary {
			maxSummary = len(st.Task.Summary)
		}
	}

	indexWidth := len(fmt.Sprintf("%d", len(result.Tasks)))

	fmt.Fprintf(w, "%d task(s):\n", len(result.Tasks))
	for i, st := range result.Tasks {
		fmt.Fprintf(w, "  %*d. %-*s  %-*s  %5.1f\n",
			indexWidth, i+1,
			maxKey, st.Task.Key,
			maxSummary, st.Task.Summary,
			st.Score)

		if verbose {
			details := formatTaskDetails(st)
			if details != "" {
				fmt.Fprintf(w, "  %*s  %-*s  %s\n",
					indexWidth, "",
					maxKey, "",
					details)
			}
		}
	}
}

func formatTaskDetails(st scheduler.ScoredTask) string {
	var parts []string
	if st.Task.IssueType != "" {
		parts = append(parts, st.Task.IssueType)
	}
	parts = append(parts, formatDuration(st.Task.RemainingEstimate))
	if st.Task.DueDate != nil {
		parts = append(parts, "Due: "+st.Task.DueDate.Format("Jan 2"))
	}
	if name, ok := priorityLevelNames[st.Task.Priority]; ok && st.Task.Priority != 0 {
		parts = append(parts, "Priority: "+name)
	}
	if st.Task.NotBefore != nil {
		parts = append(parts, "Not Before: "+st.Task.NotBefore.Format("Jan 2"))
	}
	if st.Task.UpNext {
		parts = append(parts, "Up Next")
	}
	if st.Task.NoSplit {
		parts = append(parts, "No Split")
	}
	return strings.Join(parts, " | ")
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

			// Use multiFetcher for multi-provider, or the source directly
			var fetcher TaskFetcher
			var query string
			if ms, ok := source.(*MultiTaskSource); ok {
				fetcher = &multiFetcher{
					queries: buildProviderQueries(cfg, jql, filter),
					sources: ms.sources,
				}
			} else {
				fetcher = source
				providers := cfg.ActiveProviders()
				switch providers[0] {
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
			}

			result, err := RunList(cmd.Context(), ListParams{
				Tasks: fetcher,
				Cfg:   cfg,
				Query: query,
				Now:   time.Now(),
			})
			if err != nil {
				return err
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			PrintListResult(cmd.OutOrStdout(), result, verbose)
			return nil
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show detailed task metadata on a second line")
	cmd.Flags().String("jql", "", "Custom JQL query override (Jira source)")
	cmd.Flags().String("filter", "", "Custom filter override (Todoist source)")

	return cmd
}
