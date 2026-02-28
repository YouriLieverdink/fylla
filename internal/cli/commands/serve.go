package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/timer"
	"github.com/iruoy/fylla/internal/tui"
	"github.com/iruoy/fylla/internal/tui/msg"
	"github.com/spf13/cobra"
)

func serveDefaultQuery(cfg *config.Config) string {
	providers := cfg.ActiveProviders()
	switch providers[0] {
	case "todoist":
		return cfg.Todoist.DefaultFilter
	default:
		return cfg.Jira.DefaultJQL
	}
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the interactive TUI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			var fetcher TaskFetcher
			if ms, ok := source.(*MultiTaskSource); ok {
				fetcher = &multiFetcher{
					queries: buildProviderQueries(cfg, "", ""),
					sources: ms.sources,
				}
			} else {
				fetcher = source
			}

			cfgPath, err := config.DefaultPath()
			if err != nil {
				return fmt.Errorf("config path: %w", err)
			}

			ctx := cmd.Context()
			query := serveDefaultQuery(cfg)

			return tui.Run(tui.Deps{
				CB: buildCallbacks(ctx, cal, fetcher, source, cfg, cfgPath, query),
			})
		},
	}
}

func buildCallbacks(ctx context.Context, cal CalendarClient, fetcher TaskFetcher, source TaskSource, cfg *config.Config, cfgPath, query string) tui.Callbacks {
	return tui.Callbacks{
		LoadToday: func() ([]msg.FyllaEvent, error) {
			result, err := RunToday(ctx, TodayParams{Cal: cal, Now: time.Now()})
			if err != nil {
				return nil, err
			}
			events := make([]msg.FyllaEvent, len(result.Events))
			for i, e := range result.Events {
				events[i] = msg.FyllaEvent{
					TaskKey:         e.TaskKey,
					Project:         e.Project,
					Section:         e.Section,
					Summary:         e.Summary,
					Start:           e.Start,
					End:             e.End,
					AtRisk:          e.AtRisk,
					IsCalendarEvent: e.IsCalendarEvent,
				}
			}
			return events, nil
		},
		LoadTasks: func() ([]msg.ScoredTask, error) {
			result, err := RunList(ctx, ListParams{
				Tasks: fetcher,
				Cfg:   cfg,
				Query: query,
				Now:   time.Now(),
			})
			if err != nil {
				return nil, err
			}
			tasks := make([]msg.ScoredTask, len(result.Tasks))
			for i, st := range result.Tasks {
				tasks[i] = msg.ScoredTask{
					Key:       st.Task.Key,
					Summary:   st.Task.Summary,
					Priority:  st.Task.Priority,
					DueDate:   st.Task.DueDate,
					Estimate:  st.Task.RemainingEstimate,
					IssueType: st.Task.IssueType,
					Score:     st.Score,
					Project:   st.Task.Project,
					Section:   st.Task.Section,
					UpNext:    st.Task.UpNext,
				}
			}
			return tasks, nil
		},
		DoneTask: func(taskKey string) error {
			_, err := RunDone(ctx, DoneParams{TaskKey: taskKey, Completer: source})
			return err
		},
		DeleteTask: func(taskKey string) error {
			_, err := RunDelete(ctx, DeleteParams{TaskKey: taskKey, Deleter: source})
			return err
		},
		StartTimer: func(taskKey string) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return err
			}
			_, err = RunStart(StartParams{TaskKey: taskKey, TimerPath: path, Now: time.Now()})
			return err
		},
		TimerStatus: func() (string, time.Duration, bool, error) {
			path, err := timer.DefaultPath()
			if err != nil {
				return "", 0, false, err
			}
			result, err := RunStatus(StatusParams{TimerPath: path, Now: time.Now()})
			if err != nil {
				return "", 0, false, err
			}
			if result == nil {
				return "", 0, false, nil
			}
			return result.TaskKey, result.Elapsed, true, nil
		},
		SyncPreview: func() (*msg.SyncResult, error) {
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end := start.AddDate(0, 0, cfg.Scheduling.WindowDays-1).Add(24*time.Hour - time.Nanosecond)
			result, err := RunSync(ctx, SyncParams{
				Cal: cal, Tasks: fetcher, Cfg: cfg, Query: query,
				Now: now, Start: start, End: end, DryRun: true,
			})
			if err != nil {
				return nil, err
			}
			return convertSyncResult(result), nil
		},
		SyncApply: func(force bool) (*msg.SyncResult, error) {
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end := start.AddDate(0, 0, cfg.Scheduling.WindowDays-1).Add(24*time.Hour - time.Nanosecond)
			result, err := RunSync(ctx, SyncParams{
				Cal: cal, Tasks: fetcher, Cfg: cfg, Query: query,
				Now: now, Start: start, End: end, Force: force,
			})
			if err != nil {
				return nil, err
			}
			return convertSyncResult(result), nil
		},
		ClearEvents: func() (int, error) {
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end := start.AddDate(0, 0, cfg.Scheduling.WindowDays-1).Add(24*time.Hour - time.Nanosecond)
			result, err := RunClear(ctx, ClearParams{Cal: cal, Start: start, End: end})
			if err != nil {
				return 0, err
			}
			return result.Count, nil
		},
		LoadConfig: func() (string, error) {
			return RunConfigShow(ConfigShowParams{ConfigPath: cfgPath})
		},
		SetConfig: func(key, value string) error {
			_, err := RunConfigSet(ConfigSetParams{ConfigPath: cfgPath, Key: key, Value: value})
			return err
		},
		AddTask: func(summary, project, issueType, estimate, dueDate, priority string) (string, string, error) {
			result, err := RunAdd(ctx, AddParams{
				Summary:   summary,
				Project:   project,
				IssueType: issueType,
				Estimate:  estimate,
				DueDate:   dueDate,
				Priority:  priority,
				Inline:    true,
				Creator:   source,
			})
			if err != nil {
				return "", "", err
			}
			return result.Key, result.Summary, nil
		},
		EditTask: func(taskKey, estimate, due, priority string) error {
			_, err := RunEdit(ctx, EditParams{
				TaskKey:  taskKey,
				Estimate: estimate,
				Due:      due,
				Priority: priority,
				Source:   source,
			})
			return err
		},
		StopTimer: func(description string) (string, time.Duration, error) {
			path, err := timer.DefaultPath()
			if err != nil {
				return "", 0, err
			}
			result, err := RunStop(ctx, StopParams{
				TimerPath:    path,
				RoundMinutes: 5,
				Now:          time.Now(),
				Description:  description,
				Jira:         source,
				Cal:          cal,
				Estimate:     source,
				Cfg:          cfg,
			})
			if err != nil {
				return "", 0, err
			}
			return result.TaskKey, result.Elapsed, nil
		},
	}
}

func convertSyncResult(r *SyncResult) *msg.SyncResult {
	result := &msg.SyncResult{
		Created:   r.Created,
		Updated:   r.Updated,
		Deleted:   r.Deleted,
		Unchanged: r.Unchanged,
	}
	for _, a := range r.Allocations {
		result.Allocations = append(result.Allocations, msg.Allocation{
			TaskKey: a.Task.Key, Summary: a.Task.Summary,
			Project: a.Task.Project, Section: a.Task.Section,
			Start: a.Start, End: a.End, AtRisk: a.AtRisk,
		})
	}
	for _, a := range r.AtRisk {
		result.AtRisk = append(result.AtRisk, msg.Allocation{
			TaskKey: a.Task.Key, Summary: a.Task.Summary,
			Project: a.Task.Project, Section: a.Task.Section,
			Start: a.Start, End: a.End, AtRisk: a.AtRisk,
		})
	}
	for _, u := range r.Unscheduled {
		result.Unscheduled = append(result.Unscheduled, msg.UnscheduledTask{
			TaskKey: u.Task.Key, Summary: u.Task.Summary,
			Project: u.Task.Project, Section: u.Task.Section,
			Estimate: u.Task.RemainingEstimate, Reason: u.Reason,
		})
	}
	return result
}
