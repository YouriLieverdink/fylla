package commands

import (
	"context"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/github"
	"github.com/iruoy/fylla/internal/kendo"
	"github.com/iruoy/fylla/internal/local"
	"github.com/iruoy/fylla/internal/task"
	"github.com/iruoy/fylla/internal/todoist"
)

// TaskSource combines all task-related interfaces that every source must implement.
type TaskSource interface {
	TaskFetcher
	TaskCreator
	TaskCompleter
	TaskDeleter
	WorklogPoster
	EstimateGetter
	EstimateUpdater
	DueDateGetter
	DueDateUpdater
	DueDateRemover
	PriorityGetter
	PriorityUpdater
	SummaryGetter
	SummaryUpdater
}

// Compile-time checks that all clients satisfy TaskSource.
var (
	_ TaskSource = (*todoist.Client)(nil)
	_ TaskSource = (*github.Client)(nil)
	_ TaskSource = (*local.Client)(nil)
	_ TaskSource = (*kendo.Client)(nil)
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

// TaskCreator abstracts task creation for testing.
type TaskCreator interface {
	CreateTask(ctx context.Context, input task.CreateInput) (string, error)
}

// TaskCompleter abstracts marking a task as done for testing.
type TaskCompleter interface {
	CompleteTask(ctx context.Context, taskKey string) error
}

// TaskDeleter abstracts permanently deleting a task for testing.
type TaskDeleter interface {
	DeleteTask(ctx context.Context, taskKey string) error
}

// WorklogPoster abstracts worklog posting for testing.
type WorklogPoster interface {
	PostWorklog(ctx context.Context, issueKey string, timeSpent time.Duration, description string, started time.Time) error
}

// WorklogFetcher fetches worklogs from a provider.
type WorklogFetcher interface {
	FetchWorklogs(ctx context.Context, since, until time.Time, filter task.WorklogFilter) ([]task.WorklogEntry, error)
}

// WorklogUpdater updates a worklog entry.
type WorklogUpdater interface {
	UpdateWorklog(ctx context.Context, issueKey, worklogID string, timeSpent time.Duration, description string, started time.Time) error
}

// WorklogDeleter deletes a worklog entry.
type WorklogDeleter interface {
	DeleteWorklog(ctx context.Context, issueKey, worklogID string) error
}

// EstimateGetter abstracts fetching remaining estimate for testing.
type EstimateGetter interface {
	GetEstimate(ctx context.Context, issueKey string) (time.Duration, error)
}

// EstimateUpdater abstracts updating remaining estimate for testing.
type EstimateUpdater interface {
	UpdateEstimate(ctx context.Context, issueKey string, remaining time.Duration) error
}

// DueDateGetter abstracts fetching the due date of a task.
type DueDateGetter interface {
	GetDueDate(ctx context.Context, issueKey string) (*time.Time, error)
}

// DueDateUpdater abstracts setting the due date of a task.
type DueDateUpdater interface {
	UpdateDueDate(ctx context.Context, issueKey string, dueDate time.Time) error
}

// DueDateRemover abstracts clearing the due date from a task.
type DueDateRemover interface {
	RemoveDueDate(ctx context.Context, issueKey string) error
}

// DueStringUpdater is an optional interface for providers that accept a raw
// due-date string (recurrence or natural language) — currently Todoist.
type DueStringUpdater interface {
	UpdateDueDateString(ctx context.Context, issueKey string, dueString string) error
}

// PriorityGetter abstracts fetching the priority of a task.
type PriorityGetter interface {
	GetPriority(ctx context.Context, issueKey string) (int, error)
}

// PriorityUpdater abstracts updating the priority of a task.
type PriorityUpdater interface {
	UpdatePriority(ctx context.Context, issueKey string, priority int) error
}

// SummaryGetter abstracts fetching the raw summary/title of a task.
type SummaryGetter interface {
	GetSummary(ctx context.Context, issueKey string) (string, error)
}

// SummaryUpdater abstracts updating the summary/title of a task.
type SummaryUpdater interface {
	UpdateSummary(ctx context.Context, issueKey string, summary string) error
}

// EpicLister lists open epics from a provider, optionally scoped to a project.
type EpicLister interface {
	ListEpics(ctx context.Context, project string) ([]task.Epic, error)
}

// LaneLister lists lane names from a provider, optionally scoped to a project.
type LaneLister interface {
	ListLanes(ctx context.Context, project string) ([]string, error)
}

// SprintLister lists available sprints for a project.
type SprintLister interface {
	ListSprints(ctx context.Context, project string) ([]task.SprintOption, error)
}

// IssueTypeLister lists available issue types for a project.
type IssueTypeLister interface {
	ListIssueTypes(ctx context.Context, project string) ([]string, error)
}

// ProjectLister lists available projects.
type ProjectLister interface {
	ListProjects(ctx context.Context) ([]string, error)
}

// SectionLister lists available sections/components within a project.
type SectionLister interface {
	ListSections(ctx context.Context, project string) ([]string, error)
}

// ParentUpdater updates the parent of a task.
type ParentUpdater interface {
	UpdateParent(ctx context.Context, issueKey, parentKey string) error
}

// ParentGetter fetches the parent key of a task.
type ParentGetter interface {
	GetParent(ctx context.Context, issueKey string) (string, error)
}

// ProjectUpdater updates the project of a task.
type ProjectUpdater interface {
	UpdateProject(ctx context.Context, taskKey, project string) error
}

// SectionUpdater updates the section of a task.
type SectionUpdater interface {
	UpdateSection(ctx context.Context, taskKey, section string) error
}

// SprintUpdater updates the sprint of a task.
type SprintUpdater interface {
	UpdateSprint(ctx context.Context, issueKey string, sprintID *int) error
}

// TransitionLister lists available transitions/lanes for a task.
type TransitionLister interface {
	ListTransitions(ctx context.Context, taskKey string) ([]string, error)
}

// Transitioner transitions a task to a target status/lane.
type Transitioner interface {
	TransitionTask(ctx context.Context, taskKey, target string) error
}

// IssueKeyResolver resolves a non-native task key (e.g. GitHub PR) to a worklog-compatible issue key.
type IssueKeyResolver interface {
	ResolveIssueKey(ctx context.Context, taskKey string) (string, error)
}
