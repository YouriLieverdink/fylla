package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/jira"
	"github.com/iruoy/fylla/internal/scheduler"
	"github.com/spf13/cobra"
)

// CalendarClient abstracts calendar operations for testing.
type CalendarClient interface {
	FetchEvents(ctx context.Context, start, end time.Time) ([]calendar.Event, error)
	DeleteFyllaEvents(ctx context.Context, start, end time.Time) error
	CreateEvent(ctx context.Context, input calendar.CreateEventInput) error
}

// JiraFetcher abstracts Jira task fetching for testing.
type JiraFetcher interface {
	FetchTasks(ctx context.Context, jql string) ([]jira.Task, error)
}

// SyncParams holds all inputs for the sync process.
type SyncParams struct {
	Cal    CalendarClient
	Jira   JiraFetcher
	Cfg    *config.Config
	JQL    string
	Now    time.Time
	Start  time.Time
	End    time.Time
	DryRun bool
}

// SyncResult holds the output of a sync operation.
type SyncResult struct {
	Allocations []scheduler.Allocation
	AtRisk      []scheduler.Allocation
}

// RunSync executes the full sync process:
//  1. Delete existing [Fylla] events from Google Calendar
//  2. Fetch Jira tasks using JQL
//  3. Sort tasks by composite score
//  4. Fetch Google Calendar events within scheduling window
//  5. Find free slots per project respecting time windows
//  6. Allocate tasks to slots using first-fit algorithm
//  7. Create fresh [Fylla] calendar events
//  8. Return at-risk warnings
func RunSync(ctx context.Context, p SyncParams) (*SyncResult, error) {
	// Step 1: Delete existing [Fylla] events (skip on dry-run)
	if !p.DryRun {
		if err := p.Cal.DeleteFyllaEvents(ctx, p.Start, p.End); err != nil {
			return nil, fmt.Errorf("delete fylla events: %w", err)
		}
	}

	// Step 2: Fetch Jira tasks
	tasks, err := p.Jira.FetchTasks(ctx, p.JQL)
	if err != nil {
		return nil, fmt.Errorf("fetch jira tasks: %w", err)
	}

	// Step 3: Sort by composite score
	sorted := scheduler.SortTasks(tasks, p.Cfg.Weights, p.Cfg.TypeScores, p.Now)

	// Step 4: Fetch Google Calendar events
	events, err := p.Cal.FetchEvents(ctx, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar events: %w", err)
	}

	// Step 5: Find free slots per project
	slotsByProject := make(map[string][]calendar.Slot)

	defaultSlots, err := calendar.FindFreeSlots(
		p.Now, p.Start, p.End, events,
		p.Cfg.BusinessHours,
		p.Cfg.Scheduling.BufferMinutes,
		p.Cfg.Scheduling.MinTaskDurationMinutes,
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
		)
		if err != nil {
			return nil, fmt.Errorf("find slots for project %s: %w", project, err)
		}
		slotsByProject[project] = slots
	}

	// Step 6: Allocate tasks to slots
	allocations := scheduler.Allocate(sorted, slotsByProject, scheduler.AllocateConfig{
		MinTaskDurationMinutes: p.Cfg.Scheduling.MinTaskDurationMinutes,
	})

	// Step 7: Create calendar events (skip on dry-run)
	if !p.DryRun {
		for _, alloc := range allocations {
			if err := p.Cal.CreateEvent(ctx, calendar.CreateEventInput{
				TaskKey: alloc.Task.Key,
				Summary: alloc.Task.Summary,
				Start:   alloc.Start,
				End:     alloc.End,
				AtRisk:  alloc.AtRisk,
			}); err != nil {
				return nil, fmt.Errorf("create event for %s: %w", alloc.Task.Key, err)
			}
		}
	}

	// Step 8: Collect at-risk warnings
	var atRisk []scheduler.Allocation
	seen := make(map[string]bool)
	for _, alloc := range allocations {
		if alloc.AtRisk && !seen[alloc.Task.Key] {
			atRisk = append(atRisk, alloc)
			seen[alloc.Task.Key] = true
		}
	}

	return &SyncResult{
		Allocations: allocations,
		AtRisk:      atRisk,
	}, nil
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Schedule Jira tasks into Google Calendar",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Preview schedule without creating events")
	cmd.Flags().String("jql", "", "Custom JQL query override")
	cmd.Flags().Int("days", 0, "Override scheduling window (days)")
	cmd.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End date (YYYY-MM-DD)")

	return cmd
}
