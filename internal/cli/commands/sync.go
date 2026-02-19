package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/iruoy/fylla/internal/task"
	"github.com/spf13/cobra"
)

const (
	maxLabelWidth   = 40
	maxSummaryWidth = 40
)

// CalendarClient abstracts calendar operations for testing.
type CalendarClient interface {
	FetchEvents(ctx context.Context, start, end time.Time) ([]calendar.Event, error)
	FetchFyllaEvents(ctx context.Context, start, end time.Time) ([]calendar.Event, error)
	DeleteFyllaEvents(ctx context.Context, start, end time.Time) error
	CreateEvent(ctx context.Context, input calendar.CreateEventInput) error
	UpdateEvent(ctx context.Context, eventID string, input calendar.CreateEventInput) error
	DeleteEvent(ctx context.Context, eventID string) error
}

// TaskFetcher abstracts task fetching for testing.
type TaskFetcher interface {
	FetchTasks(ctx context.Context, query string) ([]task.Task, error)
}

// SyncParams holds all inputs for the sync process.
type SyncParams struct {
	Cal      CalendarClient
	Tasks    TaskFetcher
	Cfg      *config.Config
	Query    string
	Now      time.Time
	Start    time.Time
	End      time.Time
	DryRun   bool
	Force    bool
	Progress io.Writer
}

// SyncResult holds the output of a sync operation.
type SyncResult struct {
	Allocations []scheduler.Allocation
	AtRisk      []scheduler.Allocation
	Unscheduled []scheduler.UnscheduledTask
	Events      []calendar.Event
	Created     int
	Updated     int
	Deleted     int
	Unchanged   int
}

// SyncFlags holds the parsed CLI flags for the sync command.
type SyncFlags struct {
	DryRun bool
	JQL    string
	Filter string
	Days   int
	From   string
	To     string
}

// BuildSyncParams computes SyncParams from CLI flags and config.
func BuildSyncParams(flags SyncFlags, cfg *config.Config, now time.Time) (query string, start, end time.Time, dryRun bool, err error) {
	dryRun = flags.DryRun

	// Query: use first provider's default as the single query for backward compat
	providers := cfg.ActiveProviders()
	switch providers[0] {
	case "todoist":
		query = flags.Filter
		if query == "" {
			query = cfg.Todoist.DefaultFilter
		}
	default:
		query = flags.JQL
		if query == "" {
			query = cfg.Jira.DefaultJQL
		}
	}

	// Date range: --from/--to take precedence over --days over config windowDays
	if flags.From != "" && flags.To != "" {
		start, err = time.Parse("2006-01-02", flags.From)
		if err != nil {
			return "", time.Time{}, time.Time{}, false, fmt.Errorf("parse --from: %w", err)
		}
		end, err = time.Parse("2006-01-02", flags.To)
		if err != nil {
			return "", time.Time{}, time.Time{}, false, fmt.Errorf("parse --to: %w", err)
		}
		// Set end to end of day
		end = end.Add(24*time.Hour - time.Nanosecond)
	} else {
		days := cfg.Scheduling.WindowDays
		if flags.Days > 0 {
			days = flags.Days
		}
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, days-1).Add(24*time.Hour - time.Nanosecond)
	}

	return query, start, end, dryRun, nil
}

// timelineEntry represents a single row in the dry-run timeline output.
type timelineEntry struct {
	start   time.Time
	end     time.Time
	isEvent bool   // true = calendar event, false = scheduled task
	key     string // task key (empty for events)
	summary string
	atRisk  bool
	project string
	section string
}

func truncateString(s string, maxWidth int) string {
	if displayWidth(s) <= maxWidth {
		return s
	}
	// Walk runes, tracking display width, and cut before exceeding maxWidth.
	// Reserve 1 column for the ellipsis.
	runes := []rune(s)
	w := 0
	prev := 0
	cutAt := 0
	for i, r := range runes {
		rw := runewidth.RuneWidth(r)
		if r == 0xFE0F && prev == 1 {
			rw = 1
		}
		if r >= 0xFE00 && r <= 0xFE0F {
			prev = 0
		} else {
			prev = runewidth.RuneWidth(r)
		}
		if w+rw > maxWidth-1 {
			cutAt = i
			break
		}
		w += rw
		cutAt = i + 1
	}
	return string(runes[:cutAt]) + "…"
}

func padRight(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func capWidth(n, max int) int {
	if n > max {
		return max
	}
	return n
}

func syncTaskLabel(key, project, section string, verbose bool) string {
	if !verbose {
		return key
	}
	return buildTaskLabel(key, project, section, maxLabelWidth)
}

func buildTaskLabel(key, project, section string, maxTotal int) string {
	if project == "" {
		return key
	}
	prefix := project
	if section != "" {
		prefix = prefix + " / " + section
	}
	full := "[" + prefix + "] " + key
	keyWidth := displayWidth(key)
	if displayWidth(full) <= maxTotal || maxTotal <= keyWidth {
		return full
	}
	// Truncate prefix to fit: "[prefix…] " + key
	// overhead: [ ] + space = 3 display cols, plus … = 4
	available := maxTotal - keyWidth - 4
	if available <= 0 {
		return key
	}
	return "[" + truncateString(prefix, available) + "] " + key
}

// PrintSyncResult writes the sync result to the given writer.
// When verbose is true, task labels include the [Project / Section] prefix.
func PrintSyncResult(w io.Writer, result *SyncResult, dryRun, verbose bool) {
	if dryRun {
		fmt.Fprintln(w, "Dry run — no events created.")
		fmt.Fprintln(w)
		printDryRunTimeline(w, result, verbose)
	} else {
		printAppliedResult(w, result, verbose)
	}

	if len(result.AtRisk) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "At-risk tasks:")
		for _, ar := range result.AtRisk {
			dueStr := "no due date"
			if ar.Task.DueDate != nil {
				dueStr = "due " + ar.Task.DueDate.Format("Jan 2")
			}
			taskLabel := syncTaskLabel(ar.Task.Key, ar.Task.Project, ar.Task.Section, verbose)
			fmt.Fprintf(w, "  %s: %s (%s)\n", taskLabel, ar.Task.Summary, dueStr)
		}
	}

	if len(result.Unscheduled) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Could not schedule %d task(s):\n", len(result.Unscheduled))

		maxKey := 0
		maxSummary := 0
		for _, u := range result.Unscheduled {
			taskLabel := syncTaskLabel(u.Task.Key, u.Task.Project, u.Task.Section, verbose)
			if w := displayWidth(taskLabel); w > maxKey {
				maxKey = w
			}
			if w := displayWidth(u.Task.Summary); w > maxSummary {
				maxSummary = w
			}
		}
		maxSummary = capWidth(maxSummary, maxSummaryWidth)
		for _, u := range result.Unscheduled {
			est := formatDuration(u.Remaining)
			taskLabel := syncTaskLabel(u.Task.Key, u.Task.Project, u.Task.Section, verbose)
			fmt.Fprintf(w, "  %s  %s  %5s  — %s\n",
				padRight(taskLabel, maxKey),
				padRight(truncateString(u.Task.Summary, maxSummary), maxSummary),
				est,
				u.Reason,
			)
		}
	}
}

func printDryRunTimeline(w io.Writer, result *SyncResult, verbose bool) {
	if len(result.Allocations) == 0 && len(result.Unscheduled) == 0 {
		fmt.Fprintln(w, "No tasks to schedule.")
		return
	}

	// Build timeline entries from allocations and busy calendar events.
	var entries []timelineEntry
	for _, alloc := range result.Allocations {
		entries = append(entries, timelineEntry{
			start:   alloc.Start,
			end:     alloc.End,
			key:     alloc.Task.Key,
			summary: alloc.Task.Summary,
			atRisk:  alloc.AtRisk,
			project: alloc.Task.Project,
			section: alloc.Task.Section,
		})
	}
	for _, ev := range result.Events {
		if ev.Transparency == "transparent" || ev.IsOOO() || ev.AllDay {
			continue
		}
		if calendar.TaskKeyFromDescription(ev.Description) != "" {
			continue
		}
		entries = append(entries, timelineEntry{
			start:   ev.Start,
			end:     ev.End,
			isEvent: true,
			summary: ev.Title,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].start.Before(entries[j].start)
	})

	// Compute column widths.
	maxKey := 0
	maxSummary := 0
	for _, e := range entries {
		label := "•"
		if !e.isEvent {
			label = syncTaskLabel(e.key, e.project, e.section, verbose)
		}
		if w := displayWidth(label); w > maxKey {
			maxKey = w
		}
		if w := displayWidth(e.summary); w > maxSummary {
			maxSummary = w
		}
	}
	maxSummary = capWidth(maxSummary, maxSummaryWidth)

	fmt.Fprintf(w, "Scheduled %d event(s):\n", len(result.Allocations))

	// Group by day and print.
	var currentDay string
	for _, e := range entries {
		day := e.start.Format("Mon Jan 2")
		if day != currentDay {
			fmt.Fprintf(w, "\n  %s\n", day)
			currentDay = day
		}

		label := "•"
		if !e.isEvent {
			label = syncTaskLabel(e.key, e.project, e.section, verbose)
		}

		dur := formatDuration(e.end.Sub(e.start))
		suffix := ""
		if e.atRisk {
			suffix = "  ⚠️"
		}

		fmt.Fprintf(w, "    %s  %s  %5s  %s – %s%s\n",
			padRight(label, maxKey),
			padRight(truncateString(e.summary, maxSummary), maxSummary),
			dur,
			e.start.Format("15:04"),
			e.end.Format("15:04"),
			suffix,
		)
	}
}

func printAppliedResult(w io.Writer, result *SyncResult, verbose bool) {
	if len(result.Allocations) == 0 {
		fmt.Fprintln(w, "No tasks to schedule.")
		return
	}

	maxKey := 0
	maxSummary := 0
	for _, alloc := range result.Allocations {
		taskLabel := syncTaskLabel(alloc.Task.Key, alloc.Task.Project, alloc.Task.Section, verbose)
		if w := displayWidth(taskLabel); w > maxKey {
			maxKey = w
		}
		if w := displayWidth(alloc.Task.Summary); w > maxSummary {
			maxSummary = w
		}
	}
	maxSummary = capWidth(maxSummary, maxSummaryWidth)

	indexWidth := len(fmt.Sprintf("%d", len(result.Allocations)))

	fmt.Fprintf(w, "Scheduled %d event(s):\n", len(result.Allocations))
	for i, alloc := range result.Allocations {
		suffix := ""
		if alloc.AtRisk {
			suffix = "  ⚠️"
		}
		taskLabel := syncTaskLabel(alloc.Task.Key, alloc.Task.Project, alloc.Task.Section, verbose)
		est := formatDuration(alloc.End.Sub(alloc.Start))
		fmt.Fprintf(w, "  %*d. %s  %s  %5s  %s – %s%s\n",
			indexWidth, i+1,
			padRight(taskLabel, maxKey),
			padRight(truncateString(alloc.Task.Summary, maxSummary), maxSummary),
			est,
			alloc.Start.Format("Mon Jan 2 15:04"),
			alloc.End.Format("15:04"),
			suffix,
		)
	}

	if result.Created > 0 || result.Updated > 0 || result.Deleted > 0 || result.Unchanged > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Changes: %d created, %d updated, %d deleted, %d unchanged.\n",
			result.Created, result.Updated, result.Deleted, result.Unchanged)
	}
}

func progress(w io.Writer, format string, args ...interface{}) {
	if w == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	// Overwrite the current line with the new message.
	fmt.Fprintf(w, "\r\033[K%s", msg)
}

func progressClear(w io.Writer) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "\r\033[K")
}

// desiredEvent represents a calendar event that the scheduler wants to exist.
type desiredEvent struct {
	TaskKey string
	Project string
	Section string
	Summary string
	Start   time.Time
	End     time.Time
	AtRisk  bool
}

// RunSync executes the full sync process:
//  1. Fetch tasks using the configured source
//  2. Sort tasks by composite score
//  3. Fetch Google Calendar events within scheduling window
//  4. Find free slots per project respecting time windows
//  5. Allocate tasks to slots using first-fit algorithm
//  6. Reconcile desired schedule against existing Fylla events (or force-recreate)
//  7. Return at-risk warnings
func RunSync(ctx context.Context, p SyncParams) (*SyncResult, error) {
	// Step 1: Fetch tasks
	progress(p.Progress, "Fetching tasks...")
	tasks, err := p.Tasks.FetchTasks(ctx, p.Query)
	if err != nil {
		return nil, fmt.Errorf("fetch tasks: %w", err)
	}

	// Step 2: Sort by composite score
	progress(p.Progress, "Sorting %d tasks...", len(tasks))
	sorted := scheduler.SortTasks(tasks, p.Cfg.Weights, p.Now)

	// Step 3: Fetch Google Calendar events
	progress(p.Progress, "Reading calendar...")
	events, err := p.Cal.FetchEvents(ctx, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar events: %w", err)
	}

	// Step 4: Find free slots per project
	progress(p.Progress, "Finding free slots...")
	slotsByProject := make(map[string][]calendar.Slot)

	defaultSlots, err := calendar.FindFreeSlots(
		p.Now, p.Start, p.End, events,
		p.Cfg.BusinessHours,
		p.Cfg.Scheduling.BufferMinutes,
		p.Cfg.Scheduling.MinTaskDurationMinutes,
		p.Cfg.Scheduling.SnapMinutes,
		p.Cfg.Scheduling.TravelBufferMinutes,
	)
	if err != nil {
		return nil, fmt.Errorf("find default slots: %w", err)
	}
	slotsByProject[""] = defaultSlots

	for project := range p.Cfg.ProjectRules {
		hours := p.Cfg.BusinessHoursFor(project)
		slots, err := calendar.FindFreeSlots(
			p.Now, p.Start, p.End, events,
			hours,
			p.Cfg.Scheduling.BufferMinutes,
			p.Cfg.Scheduling.MinTaskDurationMinutes,
			p.Cfg.Scheduling.SnapMinutes,
			p.Cfg.Scheduling.TravelBufferMinutes,
		)
		if err != nil {
			return nil, fmt.Errorf("find slots for project %s: %w", project, err)
		}
		slotsByProject[project] = slots
	}

	// Step 5: Allocate tasks to slots
	progress(p.Progress, "Scheduling %d tasks into available slots...", len(sorted))
	allocations, unscheduled := scheduler.Allocate(sorted, slotsByProject, scheduler.AllocateConfig{
		MinTaskDurationMinutes: p.Cfg.Scheduling.MinTaskDurationMinutes,
		BufferMinutes:          p.Cfg.Scheduling.BufferMinutes,
		SnapMinutes:            p.Cfg.Scheduling.SnapMinutes,
	})

	// Step 6: Apply schedule to calendar
	// Cleanup covers all past Fylla events, not just the scheduling window.
	cleanupStart := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var created, updated, deleted, unchanged int
	if !p.DryRun {
		if p.Force {
			// Force mode: delete future events, then create fresh
			progress(p.Progress, "Clearing previous schedule...")
			if err := p.Cal.DeleteFyllaEvents(ctx, p.Now, p.End); err != nil {
				return nil, fmt.Errorf("delete fylla events: %w", err)
			}
			progress(p.Progress, "Creating %d calendar events...", len(allocations))
			for _, alloc := range allocations {
				if err := p.Cal.CreateEvent(ctx, calendar.CreateEventInput{
					TaskKey: alloc.Task.Key,
					Project: alloc.Task.Project,
					Section: alloc.Task.Section,
					Summary: alloc.Task.Summary,
					Start:   alloc.Start,
					End:     alloc.End,
					AtRisk:  alloc.AtRisk,
				}); err != nil {
					return nil, fmt.Errorf("create event for %s: %w", alloc.Task.Key, err)
				}
			}
			created = len(allocations)
		} else {
			// Incremental mode: reconcile desired vs existing
			progress(p.Progress, "Fetching existing Fylla events...")
			existing, err := p.Cal.FetchFyllaEvents(ctx, cleanupStart, p.End)
			if err != nil {
				return nil, fmt.Errorf("fetch fylla events: %w", err)
			}

			desired := make([]desiredEvent, len(allocations))
			for i, alloc := range allocations {
				desired[i] = desiredEvent{
					TaskKey: alloc.Task.Key,
					Project: alloc.Task.Project,
					Section: alloc.Task.Section,
					Summary: alloc.Task.Summary,
					Start:   alloc.Start,
					End:     alloc.End,
					AtRisk:  alloc.AtRisk,
				}
			}

			created, updated, deleted, unchanged, err = reconcile(ctx, p.Cal, existing, desired, p.Now, p.Progress)
			if err != nil {
				return nil, fmt.Errorf("reconcile events: %w", err)
			}
		}
	}

	// Step 7: Collect at-risk warnings
	var atRisk []scheduler.Allocation
	seen := make(map[string]bool)
	for _, alloc := range allocations {
		if alloc.AtRisk && !seen[alloc.Task.Key] {
			atRisk = append(atRisk, alloc)
			seen[alloc.Task.Key] = true
		}
	}

	progressClear(p.Progress)

	return &SyncResult{
		Allocations: allocations,
		AtRisk:      atRisk,
		Unscheduled: unscheduled,
		Events:      events,
		Created:     created,
		Updated:     updated,
		Deleted:     deleted,
		Unchanged:   unchanged,
	}, nil
}

// reconcile compares desired events against existing Fylla events and applies
// the minimal set of changes. Events are matched by task key and chronological
// order (to handle split tasks that produce multiple events per key).
func reconcile(ctx context.Context, cal CalendarClient, existing []calendar.Event, desired []desiredEvent, now time.Time, prog io.Writer) (created, updated, deleted, unchanged int, err error) {
	// Track which existing events are matched.
	matchedExisting := make(map[string]bool)

	// Preserve past events — don't reconcile events that have already ended.
	existingByKey := make(map[string][]calendar.Event)
	for _, ev := range existing {
		key := calendar.TaskKeyFromDescription(ev.Description)
		if key == "" {
			continue
		}
		if !ev.End.After(now) {
			// Past event — mark as matched so it won't be deleted.
			matchedExisting[ev.ID] = true
			continue
		}
		existingByKey[key] = append(existingByKey[key], ev)
	}

	// Group desired events by task key, preserving order.
	type indexedDesired struct {
		event desiredEvent
		index int
	}
	desiredByKey := make(map[string][]indexedDesired)
	for i, d := range desired {
		desiredByKey[d.TaskKey] = append(desiredByKey[d.TaskKey], indexedDesired{event: d, index: i})
	}

	// Match desired events against existing by key + position.
	for key, dList := range desiredByKey {
		eList := existingByKey[key]
		for i, d := range dList {
			if i < len(eList) {
				ev := eList[i]
				matchedExisting[ev.ID] = true
				input := calendar.CreateEventInput{
					TaskKey: d.event.TaskKey,
					Project: d.event.Project,
					Section: d.event.Section,
					Summary: d.event.Summary,
					Start:   d.event.Start,
					End:     d.event.End,
					AtRisk:  d.event.AtRisk,
				}
				if eventsMatch(ev, d.event) {
					unchanged++
				} else {
					if err := cal.UpdateEvent(ctx, ev.ID, input); err != nil {
						return created, updated, deleted, unchanged, fmt.Errorf("update event %s: %w", ev.ID, err)
					}
					updated++
				}
			} else {
				// New event — no existing match.
				if err := cal.CreateEvent(ctx, calendar.CreateEventInput{
					TaskKey: d.event.TaskKey,
					Project: d.event.Project,
					Section: d.event.Section,
					Summary: d.event.Summary,
					Start:   d.event.Start,
					End:     d.event.End,
					AtRisk:  d.event.AtRisk,
				}); err != nil {
					return created, updated, deleted, unchanged, fmt.Errorf("create event for %s: %w", d.event.TaskKey, err)
				}
				created++
			}
		}
	}

	// Delete existing events that are no longer desired.
	for _, ev := range existing {
		if matchedExisting[ev.ID] {
			continue
		}
		key := calendar.TaskKeyFromDescription(ev.Description)
		if key == "" {
			continue
		}
		if err := cal.DeleteEvent(ctx, ev.ID); err != nil {
			return created, updated, deleted, unchanged, fmt.Errorf("delete event %s: %w", ev.ID, err)
		}
		deleted++
	}

	progress(prog, "Reconciled: %d created, %d updated, %d deleted, %d unchanged.",
		created, updated, deleted, unchanged)

	return created, updated, deleted, unchanged, nil
}

// eventsMatch returns true if an existing calendar event matches the desired state.
func eventsMatch(existing calendar.Event, desired desiredEvent) bool {
	return existing.Start.Equal(desired.Start) && existing.End.Equal(desired.End) &&
		calendar.BuildTitleWithSection(desired.Project, desired.Section, desired.Summary, desired.AtRisk) == existing.Title &&
		calendar.TaskKeyFromDescription(existing.Description) == desired.TaskKey
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Schedule tasks into Google Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			now := time.Now()
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			force, _ := cmd.Flags().GetBool("force")
			jql, _ := cmd.Flags().GetString("jql")
			filter, _ := cmd.Flags().GetString("filter")
			days, _ := cmd.Flags().GetInt("days")
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")

			query, start, end, dryRun, err := BuildSyncParams(SyncFlags{
				DryRun: dryRun,
				JQL:    jql,
				Filter: filter,
				Days:   days,
				From:   from,
				To:     to,
			}, cfg, now)
			if err != nil {
				return err
			}

			// Use multiFetcher for multi-provider, or the source directly
			var fetcher TaskFetcher
			if ms, ok := source.(*MultiTaskSource); ok {
				fetcher = &multiFetcher{
					queries: buildProviderQueries(cfg, jql, filter),
					sources: ms.sources,
				}
			} else {
				fetcher = source
			}

			result, err := RunSync(cmd.Context(), SyncParams{
				Cal:      cal,
				Tasks:    fetcher,
				Cfg:      cfg,
				Query:    query,
				Now:      now,
				Start:    start,
				End:      end,
				DryRun:   dryRun,
				Force:    force,
				Progress: cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			PrintSyncResult(cmd.OutOrStdout(), result, dryRun, verbose)
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Preview schedule without creating events")
	cmd.Flags().Bool("force", false, "Delete all events and recreate (skip incremental sync)")
	cmd.Flags().BoolP("verbose", "v", false, "Show project and section in task labels")
	cmd.Flags().String("jql", "", "Custom JQL query override (Jira source)")
	cmd.Flags().String("filter", "", "Custom filter override (Todoist source)")
	cmd.Flags().Int("days", 0, "Number of days to schedule (1 = today only)")
	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD)")

	return cmd
}
