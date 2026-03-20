package commands

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
	"github.com/iruoy/fylla/internal/timer"
	"github.com/iruoy/fylla/internal/tui"
	"github.com/iruoy/fylla/internal/tui/msg"
)

// RunServe starts the interactive TUI dashboard.
func RunServe(ctx context.Context) error {
	source, cfg, err := loadTaskSource()
	if err != nil {
		return err
	}

	cal, err := loadCalendarClient(ctx, cfg)
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

	query := serveDefaultQuery(cfg)

	return tui.Run(tui.Deps{
		CB:               buildCallbacks(ctx, cal, fetcher, source, cfg, cfgPath, query),
		DailyHours:       cfg.Efficiency.DailyHours,
		WeeklyHours:      cfg.Efficiency.WeeklyHours,
		EfficiencyTarget: cfg.Efficiency.Target,
	})
}

func serveDefaultQuery(cfg *config.Config) string {
	providers := cfg.ActiveProviders()
	switch providers[0] {
	case "todoist":
		return cfg.Todoist.DefaultFilter
	case "kendo":
		return cfg.Kendo.DefaultFilter
	default:
		return cfg.Jira.DefaultJQL
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
					Provider:        e.Provider,
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
					Provider:  st.Task.Provider,
					Summary:   st.Task.Summary,
					Priority:  st.Task.Priority,
					DueDate:   st.Task.DueDate,
					Estimate:  st.Task.RemainingEstimate,
					IssueType: st.Task.IssueType,
					Score:     st.Score,
					Project:   st.Task.Project,
					Section:   st.Task.Section,
					Status:       st.Task.Status,
					UpNext:       st.Task.UpNext,
					NoSplit:      st.Task.NoSplit,
					NotBefore:    st.Task.NotBefore,
					NotBeforeRaw: st.Task.NotBeforeRaw,
				}
			}
			return tasks, nil
		},
		DoneTask: func(taskKey, provider string) error {
			_, err := RunDone(ctx, DoneParams{TaskKey: taskKey, Provider: provider, Completer: source})
			return err
		},
		DeleteTask: func(taskKey, provider string) error {
			_, err := RunDelete(ctx, DeleteParams{TaskKey: taskKey, Provider: provider, Deleter: source})
			return err
		},
		StartTimer: func(taskKey, project, section string) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return err
			}
			return RunStart(StartParams{TaskKey: taskKey, Project: project, Section: section, TimerPath: path, Now: time.Now()})
		},
		InterruptTimer: func() error {
			path, err := timer.DefaultPath()
			if err != nil {
				return err
			}
			return timer.Interrupt(time.Now(), path)
		},
		TimerStatus: func() (*tui.TimerStatusInfo, error) {
			path, err := timer.DefaultPath()
			if err != nil {
				return nil, err
			}
			result, err := RunStatus(StatusParams{TimerPath: path, Now: time.Now()})
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, nil
			}
			summary, _ := source.GetSummary(ctx, result.TaskKey)
			project, section := result.Project, result.Section
			if project == "" {
				if tasks, err := fetcher.FetchTasks(ctx, query); err == nil {
					for _, t := range tasks {
						if t.Key == result.TaskKey {
							project = t.Project
							section = t.Section
							break
						}
					}
				}
			}
			info := &tui.TimerStatusInfo{
				TaskKey:      result.TaskKey,
				Summary:      summary,
				Project:      project,
				Section:      section,
				Comment:      result.Comment,
				Elapsed:      result.Elapsed,
				TotalElapsed: result.TotalElapsed,
				Running:      true,
			}
			for _, s := range result.Segments {
				info.Segments = append(info.Segments, tui.TimerSegmentInfo{Duration: s.Duration, Comment: s.Comment})
			}
			for _, p := range result.Paused {
				info.Paused = append(info.Paused, tui.PausedTimerInfo{
					TaskKey:      p.TaskKey,
					Project:      p.Project,
					SegmentCount: p.SegmentCount,
				})
			}
			return info, nil
		},
		SaveTimerComment: func(comment string) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return err
			}
			return timer.SetComment(comment, path)
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
		AddTask: func(provider, summary, project, section, issueType, description, estimate, dueDate, priority, parent string) (string, string, error) {
			var creator TaskCreator = source
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					creator = &providerCreator{ms: ms, provider: provider}
				}
			}
			result, err := RunAdd(ctx, AddParams{
				Summary:     summary,
				Project:     project,
				Section:     section,
				IssueType:   issueType,
				Description: description,
				Estimate:    estimate,
				DueDate:     dueDate,
				Priority:    priority,
				Parent:      parent,
				Inline:      true,
				Creator:     creator,
			})
			if err != nil {
				return "", "", err
			}
			return result.Key, result.Summary, nil
		},
		EditTask: func(params tui.EditTaskParams) error {
			ep := EditParams{
				TaskKey:  params.TaskKey,
				Provider: params.Provider,
				Summary:  params.Summary,
				Estimate: params.Estimate,
				Due:      params.Due,
				Priority: params.Priority,
				Parent:   params.Parent,
				Section:  params.Section,
				Source:   source,
			}
			if params.NotBefore != "" {
				ep.NotBefore = params.NotBefore
			} else if params.HadNotBefore {
				ep.NoNotBefore = true
			}
			if params.Due == "" && params.HadDue {
				ep.NoDue = true
			}
			if params.Estimate == "" && params.HadEstimate {
				ep.NoEstimate = true
			}
			if params.Priority == "" && params.HadPriority {
				ep.NoPriority = true
			}
			if params.Parent == "" && params.HadParent {
				ep.NoParent = true
			}
			if params.Section == "" && params.HadSection {
				ep.NoSection = true
			}
			if params.UpNext != nil {
				if *params.UpNext {
					ep.UpNext = true
				} else {
					ep.NoUpNext = true
				}
			}
			if params.NoSplit != nil {
				if *params.NoSplit {
					ep.NoSplit = true
				} else {
					ep.NoNoSplit = true
				}
			}
			_, err := RunEdit(ctx, ep)
			return err
		},
		AbortTimer: func() (string, string, error) {
			path, err := timer.DefaultPath()
			if err != nil {
				return "", "", err
			}
			result, err := RunAbort(AbortParams{TimerPath: path, Now: time.Now()})
			if err != nil {
				return "", "", err
			}
			return result.TaskKey, result.ResumedKey, nil
		},
		StopTimer: func(description string, done bool, fallbackIssue string) (string, time.Duration, string, error) {
			path, err := timer.DefaultPath()
			if err != nil {
				return "", 0, "", err
			}
			var resolver JiraKeyResolver
			if r, ok := source.(JiraKeyResolver); ok {
				resolver = r
			}
			result, err := RunStop(ctx, StopParams{
				TimerPath:     path,
				RoundMinutes:  5,
				Now:           time.Now(),
				Description:   description,
				Jira:          source,
				Cal:           cal,
				Estimate:      source,
				Cfg:           cfg,
				Resolver:      resolver,
				Completer:     source,
				Done:          done,
				FallbackIssue: fallbackIssue,
			})
			if err != nil {
				return "", 0, "", err
			}
			return result.TaskKey, result.TotalElapsed, result.ResumedKey, nil
		},
		ListSections: func(provider, project string) ([]string, error) {
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					if src, ok := ms.sources[provider]; ok {
						if sl, ok := src.(SectionLister); ok {
							return sl.ListSections(ctx, project)
						}
					}
					return nil, nil
				}
			}
			if sl, ok := source.(SectionLister); ok {
				return sl.ListSections(ctx, project)
			}
			return nil, nil
		},
		ListProjects: func(provider string) ([]string, error) {
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					if src, ok := ms.sources[provider]; ok {
						if pl, ok := src.(ProjectLister); ok {
							return pl.ListProjects(ctx)
						}
					}
					return nil, nil
				}
			}
			if pl, ok := source.(ProjectLister); ok {
				return pl.ListProjects(ctx)
			}
			return nil, nil
		},
		ListLanes: func(provider, project string) ([]string, error) {
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					if src, ok := ms.sources[provider]; ok {
						if ll, ok := src.(LaneLister); ok {
							return ll.ListLanes(ctx, project)
						}
					}
					return nil, nil
				}
			}
			if ll, ok := source.(LaneLister); ok {
				return ll.ListLanes(ctx, project)
			}
			return nil, nil
		},
		ListIssueTypes: func(provider, project string) ([]string, error) {
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					if src, ok := ms.sources[provider]; ok {
						if il, ok := src.(IssueTypeLister); ok {
							return il.ListIssueTypes(ctx, project)
						}
					}
					return nil, nil
				}
			}
			if il, ok := source.(IssueTypeLister); ok {
				return il.ListIssueTypes(ctx, project)
			}
			return nil, nil
		},
		ListEpics: func(project string) ([]msg.EpicOption, error) {
			var el EpicLister
			if e, ok := source.(EpicLister); ok {
				el = e
			} else if ms, ok := source.(*MultiTaskSource); ok {
				for _, name := range []string{"jira", "kendo"} {
					if src, ok := ms.sources[name]; ok {
						if e, ok := src.(EpicLister); ok {
							el = e
							break
						}
					}
				}
			}
			if el == nil {
				return nil, nil
			}
			epics, err := el.ListEpics(ctx, project)
			if err != nil {
				return nil, err
			}
			options := make([]msg.EpicOption, len(epics))
			for i, e := range epics {
				options[i] = msg.EpicOption{
					Key:   e.Key,
					Label: fmt.Sprintf("%s — %s", e.Key, e.Summary),
				}
			}
			return options, nil
		},
		GetParent: func(taskKey string) (string, error) {
			var pg ParentGetter
			if g, ok := source.(ParentGetter); ok {
				pg = g
			} else if ms, ok := source.(*MultiTaskSource); ok {
				// ParentGetter is only supported by Jira, so route directly
				routed := ms.routeTo(taskKey)
				if g, ok := routed.(ParentGetter); ok {
					pg = g
				}
			}
			if pg == nil {
				return "", nil
			}
			return pg.GetParent(ctx, taskKey)
		},
		Provider: func() string {
			return cfg.ActiveProviders()[0]
		},
		Providers: func() []string {
			return cfg.ActiveProviders()
		},
		SnoozeTask: func(taskKey, target string) error {
			_, err := RunSnooze(ctx, SnoozeParams{
				TaskKey: taskKey,
				Target:  target,
				Source:  source,
			})
			return err
		},
		ViewTask: func(taskKey string) (*msg.ViewResult, error) {
			result, err := RunView(ctx, ViewParams{
				TaskKey: taskKey,
				Source:  source,
			})
			if err != nil {
				return nil, err
			}
			return &msg.ViewResult{
				Key:       result.Key,
				Summary:   result.Summary,
				Priority:  result.Priority,
				Estimate:  result.Estimate,
				DueDate:   result.DueDate,
				NotBefore: result.NotBefore,
				UpNext:    result.UpNext,
				NoSplit:   result.NoSplit,
			}, nil
		},
		LoadWorklogs: func(weekView bool, date time.Time) ([]msg.WorklogEntry, error) {
			var since, until time.Time
			if weekView {
				weekday := int(date.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				since = time.Date(date.Year(), date.Month(), date.Day()-weekday+1, 0, 0, 0, 0, date.Location())
				until = since.AddDate(0, 0, 6)
			} else {
				since = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
				until = since
			}
			var wf WorklogFetcher
			if f, ok := source.(WorklogFetcher); ok {
				wf = f
			} else if ms, ok := source.(*MultiTaskSource); ok {
				provider := cfg.Worklog.Provider
				if provider == "" {
					provider = cfg.ActiveProviders()[0]
				}
				if src, ok := ms.sources[provider]; ok {
					if f, ok := src.(WorklogFetcher); ok {
						wf = f
					}
				}
			}
			if wf == nil {
				return nil, fmt.Errorf("no worklog provider available")
			}
			allEntries, err := wf.FetchWorklogs(ctx, since, until)
			if err != nil {
				return nil, err
			}
			entries := make([]msg.WorklogEntry, len(allEntries))
			for i, e := range allEntries {
				entries[i] = msg.WorklogEntry{
					ID:           e.ID,
					IssueKey:     e.IssueKey,
					Provider:     e.Provider,
					IssueSummary: e.IssueSummary,
					Description:  e.Description,
					Started:      e.Started,
					TimeSpent:    e.TimeSpent,
				}
			}
			return entries, nil
		},
		UpdateWorklog: func(issueKey, worklogID, provider string, timeSpent time.Duration, description string, started time.Time) error {
			var wu WorklogUpdater
			if u, ok := source.(WorklogUpdater); ok {
				wu = u
			} else if ms, ok := source.(*MultiTaskSource); ok {
				routed := ms.routeToWithProvider(issueKey, provider)
				if u, ok := routed.(WorklogUpdater); ok {
					wu = u
				}
			}
			if wu == nil {
				return fmt.Errorf("no worklog updater available")
			}
			return wu.UpdateWorklog(ctx, issueKey, worklogID, timeSpent, description, started)
		},
		DeleteWorklog: func(issueKey, worklogID, provider string) error {
			var wd WorklogDeleter
			if d, ok := source.(WorklogDeleter); ok {
				wd = d
			} else if ms, ok := source.(*MultiTaskSource); ok {
				routed := ms.routeToWithProvider(issueKey, provider)
				if d, ok := routed.(WorklogDeleter); ok {
					wd = d
				}
			}
			if wd == nil {
				return fmt.Errorf("no worklog deleter available")
			}
			return wd.DeleteWorklog(ctx, issueKey, worklogID)
		},
		AddWorklog: func(issueKey, provider string, timeSpent time.Duration, description string, started time.Time) error {
			if ms, ok := source.(*MultiTaskSource); ok && provider != "" {
				return ms.PostWorklogOn(ctx, issueKey, timeSpent, description, started, provider)
			}
			return source.PostWorklog(ctx, issueKey, timeSpent, description, started)
		},
		FallbackIssues: func() []tui.FallbackIssue {
			keys := cfg.Worklog.FallbackIssues
			issues := make([]tui.FallbackIssue, len(keys))
			var wg sync.WaitGroup
			for i, k := range keys {
				wg.Add(1)
				go func(idx int, key string) {
					defer wg.Done()
					summary, _ := source.GetSummary(ctx, key)
					issues[idx] = tui.FallbackIssue{Key: key, Summary: summary}
				}(i, k)
			}
			wg.Wait()
			return issues
		},
		ListTransitions: func(taskKey, provider string) ([]string, error) {
			var tl TransitionLister
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					if src, ok := ms.sources[provider]; ok {
						if l, ok := src.(TransitionLister); ok {
							tl = l
						}
					}
				}
			}
			if tl == nil {
				if l, ok := source.(TransitionLister); ok {
					tl = l
				} else if ms, ok := source.(*MultiTaskSource); ok {
					routed := ms.routeToWithProvider(taskKey, provider)
					if l, ok := routed.(TransitionLister); ok {
						tl = l
					}
				}
			}
			if tl == nil {
				return nil, fmt.Errorf("provider does not support transitions")
			}
			return tl.ListTransitions(ctx, taskKey)
		},
		MoveTask: func(taskKey, provider, target string) error {
			var tr Transitioner
			if provider != "" {
				if ms, ok := source.(*MultiTaskSource); ok {
					if src, ok := ms.sources[provider]; ok {
						if t, ok := src.(Transitioner); ok {
							tr = t
						}
					}
				}
			}
			if tr == nil {
				if t, ok := source.(Transitioner); ok {
					tr = t
				} else if ms, ok := source.(*MultiTaskSource); ok {
					routed := ms.routeToWithProvider(taskKey, provider)
					if t, ok := routed.(Transitioner); ok {
						tr = t
					}
				}
			}
			if tr == nil {
				return fmt.Errorf("provider does not support transitions")
			}
			return tr.TransitionTask(ctx, taskKey, target)
		},
	}
}

type providerCreator struct {
	ms       *MultiTaskSource
	provider string
}

func (p *providerCreator) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	return p.ms.CreateTaskOn(ctx, p.provider, input)
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
	for _, ev := range r.Events {
		if ev.Transparency == "transparent" || ev.IsOOO() || ev.AllDay {
			continue
		}
		if calendar.TaskKeyFromDescription(ev.Description) != "" {
			continue
		}
		result.CalendarEvents = append(result.CalendarEvents, msg.CalendarEvent{
			Summary: ev.Title,
			Start:   ev.Start,
			End:     ev.End,
		})
	}
	return result
}
