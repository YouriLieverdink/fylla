package commands

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
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
		CB:                      buildCallbacks(ctx, cal, fetcher, source, cfg, cfgPath, query),
		DailyHours:              cfg.Efficiency.DailyHours,
		WeeklyHours:             cfg.Efficiency.WeeklyHours,
		EfficiencyTarget:        cfg.Efficiency.Target,
		WorklogProvider:         worklogProvider(cfg),
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
					Breakdown: mapBreakdown(st.Breakdown),
					Project:   st.Task.Project,
					Section:   st.Task.Section,
					Status:       st.Task.Status,
					UpNext:       st.Task.UpNext,
					NoSplit:      st.Task.NoSplit,
					NotBefore:    st.Task.NotBefore,
					NotBeforeRaw: st.Task.NotBeforeRaw,
					SprintID:     st.Task.SprintID,
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
		StartTimer: func(taskKey, project, section, provider string) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return err
			}
			return RunStart(StartParams{TaskKey: taskKey, Project: project, Section: section, Provider: provider, TimerPath: path, Now: time.Now()})
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
			summary, _ := routedSource(source, result.Provider).GetSummary(ctx, result.TaskKey)
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
				StartTime:    result.StartTime,
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
		SaveTimerStartTime: func(startTime time.Time) error {
			path, err := timer.DefaultPath()
			if err != nil {
				return err
			}
			return timer.SetStartTime(startTime, time.Now(), path)
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
		LoadConfig: func() (*config.Config, error) {
			return config.LoadFrom(cfgPath)
		},
		SetConfig: func(key, value string) error {
			_, err := RunConfigSet(ConfigSetParams{ConfigPath: cfgPath, Key: key, Value: value})
			return err
		},
		AddTask: func(provider, summary, project, section, issueType, lane, description, estimate, dueDate, priority, parent string, sprintID *int) (string, string, error) {
			var creator TaskCreator = source
			if provider != "" {
				creator = &providerCreator{source: source, provider: provider}
			}
			result, err := RunAdd(ctx, AddParams{
				Summary:     summary,
				Project:     project,
				Section:     section,
				IssueType:   issueType,
				Lane:        lane,
				Description: description,
				Estimate:    estimate,
				DueDate:     dueDate,
				Priority:    priority,
				Parent:      parent,
				SprintID:    sprintID,
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
			if params.SprintID != nil {
				ep.SprintID = params.SprintID
			} else if params.HadSprint {
				ep.NoSprint = true
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
		StopTimer: func(description string, done bool, fallbackIssue, fallbackProvider string) (string, time.Duration, string, error) {
			path, err := timer.DefaultPath()
			if err != nil {
				return "", 0, "", err
			}
			var resolver JiraKeyResolver
			if r, ok := source.(JiraKeyResolver); ok {
				resolver = r
			}
			result, err := RunStop(ctx, StopParams{
				TimerPath:        path,
				RoundMinutes:     cfg.Worklog.RoundMinutes,
				Now:              time.Now(),
				Description:      description,
				Jira:             source,
				Estimate:         source,
				Cfg:              cfg,
				Resolver:         resolver,
				Completer:        source,
				Done:             done,
				FallbackIssue:    fallbackIssue,
				FallbackProvider: fallbackProvider,
			})
			if err != nil {
				return "", 0, "", err
			}
			return result.TaskKey, result.TotalElapsed, result.ResumedKey, nil
		},
		ListSections: func(provider, project string) ([]string, error) {
			if sl, ok := routedSource(source, provider).(SectionLister); ok {
				return sl.ListSections(ctx, project)
			}
			return nil, nil
		},
		ListProjects: func(provider string) ([]string, error) {
			if pl, ok := routedSource(source, provider).(ProjectLister); ok {
				return pl.ListProjects(ctx)
			}
			return nil, nil
		},
		ListLanes: func(provider, project string) ([]string, error) {
			if ll, ok := routedSource(source, provider).(LaneLister); ok {
				return ll.ListLanes(ctx, project)
			}
			return nil, nil
		},
		ListIssueTypes: func(provider, project string) ([]string, error) {
			if il, ok := routedSource(source, provider).(IssueTypeLister); ok {
				return il.ListIssueTypes(ctx, project)
			}
			return nil, nil
		},
		ListSprints: func(provider, project string) ([]msg.SprintOption, error) {
			if sl, ok := routedSource(source, provider).(SprintLister); ok {
				return sl.ListSprints(ctx, project)
			}
			return nil, nil
		},
		ListEpics: func(provider, project string) ([]msg.EpicOption, error) {
			var el EpicLister
			if provider != "" {
				if e, ok := routedSource(source, provider).(EpicLister); ok {
					el = e
				}
			} else {
				// Try source directly, then fall back to jira/kendo providers.
				if e, ok := source.(EpicLister); ok {
					el = e
				} else {
					for _, name := range []string{"jira", "kendo"} {
						if e, ok := routedSource(source, name).(EpicLister); ok {
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
		GetParent: func(taskKey, provider string) (string, error) {
			if pg, ok := routedSourceFor(source, taskKey, provider).(ParentGetter); ok {
				return pg.GetParent(ctx, taskKey)
			}
			return "", nil
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
			routed := routedSource(source, worklogProvider(cfg))
			if wf, ok := routed.(WorklogFetcher); ok {
				return wf.FetchWorklogs(ctx, since, until)
			}
			return nil, fmt.Errorf("no worklog provider available")
		},
		UpdateWorklog: func(issueKey, worklogID, provider string, timeSpent time.Duration, description string, started time.Time) error {
			routed := routedSourceFor(source, issueKey, coalesce(provider, worklogProvider(cfg)))
			if wu, ok := routed.(WorklogUpdater); ok {
				return wu.UpdateWorklog(ctx, issueKey, worklogID, timeSpent, description, started)
			}
			return fmt.Errorf("no worklog updater available")
		},
		DeleteWorklog: func(issueKey, worklogID, provider string) error {
			routed := routedSourceFor(source, issueKey, coalesce(provider, worklogProvider(cfg)))
			if wd, ok := routed.(WorklogDeleter); ok {
				return wd.DeleteWorklog(ctx, issueKey, worklogID)
			}
			return fmt.Errorf("no worklog deleter available")
		},
		AddWorklog: func(issueKey, provider string, timeSpent time.Duration, description string, started time.Time) error {
			routed := routedSource(source, coalesce(provider, worklogProvider(cfg)))
			return routed.PostWorklog(ctx, issueKey, timeSpent, description, started)
		},
		FallbackIssues: func() []tui.FallbackIssue {
			keys := cfg.Worklog.FallbackIssues
			issues := make([]tui.FallbackIssue, len(keys))
			provider := cfg.Worklog.Provider
			if provider == "" {
				provider = cfg.ActiveProviders()[0]
			}
			routed := routedSource(source, provider)
			var wg sync.WaitGroup
			for i, k := range keys {
				wg.Add(1)
				go func(idx int, key string) {
					defer wg.Done()
					summary, _ := routed.GetSummary(ctx, key)
					issues[idx] = tui.FallbackIssue{Key: key, Summary: summary}
				}(i, k)
			}
			wg.Wait()
			return issues
		},
		ListTransitions: func(taskKey, provider string) ([]string, error) {
			if tl, ok := routedSourceFor(source, taskKey, provider).(TransitionLister); ok {
				return tl.ListTransitions(ctx, taskKey)
			}
			return nil, fmt.Errorf("provider does not support transitions")
		},
		MoveTask: func(taskKey, provider, target string) error {
			if tr, ok := routedSourceFor(source, taskKey, provider).(Transitioner); ok {
				return tr.TransitionTask(ctx, taskKey, target)
			}
			return fmt.Errorf("provider does not support transitions")
		},
		ResolveJiraKey: func(prKey string) (string, error) {
			if r, ok := source.(JiraKeyResolver); ok {
				return r.ResolveJiraKey(ctx, prKey)
			}
			return "", fmt.Errorf("no resolver available")
		},
		SearchAllTasks: func(search string) ([]msg.ScoredTask, error) {
			var allFetcher TaskFetcher
			if ms, ok := source.(*MultiTaskSource); ok {
				queries := buildSearchAllQueries(cfg, search)
				allFetcher = &multiFetcher{
					queries: queries,
					sources: ms.sources,
				}
			} else {
				allFetcher = source
			}
			result, err := RunList(ctx, ListParams{
				Tasks: allFetcher,
				Cfg:   cfg,
				Query: buildSearchAllQuery(cfg, search),
				Now:   time.Now(),
			})
			if err != nil {
				return nil, err
			}
			maxResults := 20
			if len(result.Tasks) > maxResults {
				result.Tasks = result.Tasks[:maxResults]
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
					Breakdown: mapBreakdown(st.Breakdown),
					Project:   st.Task.Project,
					Section:   st.Task.Section,
					Status:    st.Task.Status,
					UpNext:    st.Task.UpNext,
					NoSplit:   st.Task.NoSplit,
					SprintID:  st.Task.SprintID,
				}
			}
			return tasks, nil
		},
	}
}

type providerCreator struct {
	source   TaskSource
	provider string
}

func (p *providerCreator) CreateTask(ctx context.Context, input task.CreateInput) (string, error) {
	return routedSource(p.source, p.provider).CreateTask(ctx, input)
}

// worklogProvider returns the configured worklog provider name,
// falling back to the first active provider.
func worklogProvider(cfg *config.Config) string {
	if cfg.Worklog.Provider != "" {
		return cfg.Worklog.Provider
	}
	return cfg.ActiveProviders()[0]
}

// coalesce returns the first non-empty string.
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
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

func mapBreakdown(b scheduler.ScoreBreakdown) msg.ScoreBreakdown {
	return msg.ScoreBreakdown{
		PriorityRaw:      b.PriorityRaw,
		PriorityWeight:   b.PriorityWeight,
		PriorityWeighted: b.PriorityWeighted,
		PriorityReason:   b.PriorityReason,
		DueDateRaw:       b.DueDateRaw,
		DueDateWeight:    b.DueDateWeight,
		DueDateWeighted:  b.DueDateWeighted,
		DueDateReason:    b.DueDateReason,
		EstimateRaw:      b.EstimateRaw,
		EstimateWeight:   b.EstimateWeight,
		EstimateWeighted: b.EstimateWeighted,
		EstimateReason:   b.EstimateReason,
		AgeRaw:           b.AgeRaw,
		AgeWeight:        b.AgeWeight,
		AgeWeighted:      b.AgeWeighted,
		AgeReason:        b.AgeReason,
		CrunchBoost:      b.CrunchBoost,
		CrunchReason:     b.CrunchReason,
		TypeBonus:        b.TypeBonus,
		TypeBonusReason:  b.TypeBonusReason,
		UpNextBoost:      b.UpNextBoost,
		NotBeforeMult:    b.NotBeforeMult,
		NotBeforeReason:  b.NotBeforeReason,
		Total:            b.Total,
	}
}
