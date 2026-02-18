package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/web"
	"github.com/spf13/cobra"
)

type apiEvent struct {
	TaskKey         string `json:"taskKey,omitempty"`
	Project         string `json:"project,omitempty"`
	Section         string `json:"section,omitempty"`
	Summary         string `json:"summary"`
	Start           string `json:"start"`
	End             string `json:"end"`
	AtRisk          bool   `json:"atRisk,omitempty"`
	IsCalendarEvent bool   `json:"isCalendarEvent,omitempty"`
}

type apiTask struct {
	Key       string  `json:"key"`
	Summary   string  `json:"summary"`
	Priority  int     `json:"priority"`
	DueDate   string  `json:"dueDate,omitempty"`
	Estimate  string  `json:"estimate"`
	IssueType string  `json:"issueType,omitempty"`
	Score     float64 `json:"score"`
	Project   string  `json:"project,omitempty"`
	Section   string  `json:"section,omitempty"`
	UpNext    bool    `json:"upNext,omitempty"`
}

type apiAllocation struct {
	TaskKey string `json:"taskKey"`
	Summary string `json:"summary"`
	Project string `json:"project,omitempty"`
	Section string `json:"section,omitempty"`
	Start   string `json:"start"`
	End     string `json:"end"`
	AtRisk  bool   `json:"atRisk,omitempty"`
}

type apiUnscheduled struct {
	TaskKey  string `json:"taskKey"`
	Summary  string `json:"summary"`
	Estimate string `json:"estimate"`
	Reason   string `json:"reason"`
}

type apiScheduleResponse struct {
	Allocations []apiAllocation  `json:"allocations"`
	AtRisk      []apiAllocation  `json:"atRisk"`
	Unscheduled []apiUnscheduled `json:"unscheduled"`
}

type apiStatusResponse struct {
	Providers     []string                    `json:"providers"`
	BusinessHours []config.BusinessHoursConfig `json:"businessHours"`
	WindowDays    int                         `json:"windowDays"`
	BufferMinutes int                         `json:"bufferMinutes"`
}

func serveHandleToday(cal CalendarClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		result, err := RunToday(r.Context(), TodayParams{
			Cal: cal,
			Now: now,
		})
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		events := make([]apiEvent, len(result.Events))
		for i, e := range result.Events {
			events[i] = apiEvent{
				TaskKey:         e.TaskKey,
				Project:         e.Project,
				Section:         e.Section,
				Summary:         e.Summary,
				Start:           e.Start.Format(time.RFC3339),
				End:             e.End.Format(time.RFC3339),
				AtRisk:          e.AtRisk,
				IsCalendarEvent: e.IsCalendarEvent,
			}
		}

		web.WriteJSON(w, events)
	}
}

func serveHandleTasks(fetcher TaskFetcher, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := serveDefaultQuery(cfg)

		result, err := RunList(r.Context(), ListParams{
			Tasks: fetcher,
			Cfg:   cfg,
			Query: query,
			Now:   time.Now(),
		})
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		tasks := make([]apiTask, len(result.Tasks))
		for i, st := range result.Tasks {
			var dueDate string
			if st.Task.DueDate != nil {
				dueDate = st.Task.DueDate.Format("2006-01-02")
			}
			estimate := serveFormatDuration(st.Task.RemainingEstimate)
			tasks[i] = apiTask{
				Key:       st.Task.Key,
				Summary:   st.Task.Summary,
				Priority:  st.Task.Priority,
				DueDate:   dueDate,
				Estimate:  estimate,
				IssueType: st.Task.IssueType,
				Score:     st.Score,
				Project:   st.Task.Project,
				Section:   st.Task.Section,
				UpNext:    st.Task.UpNext,
			}
		}

		web.WriteJSON(w, tasks)
	}
}

func serveHandleSchedule(fetcher TaskFetcher, cal CalendarClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		query := serveDefaultQuery(cfg)

		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 0, cfg.Scheduling.WindowDays-1).Add(24*time.Hour - time.Nanosecond)

		result, err := RunSync(r.Context(), SyncParams{
			Cal:    cal,
			Tasks:  fetcher,
			Cfg:    cfg,
			Query:  query,
			Now:    now,
			Start:  start,
			End:    end,
			DryRun: true,
		})
		if err != nil {
			web.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := apiScheduleResponse{
			Allocations: make([]apiAllocation, len(result.Allocations)),
			AtRisk:      make([]apiAllocation, len(result.AtRisk)),
			Unscheduled: make([]apiUnscheduled, len(result.Unscheduled)),
		}

		for i, a := range result.Allocations {
			resp.Allocations[i] = apiAllocation{
				TaskKey: a.Task.Key,
				Summary: a.Task.Summary,
				Project: a.Task.Project,
				Section: a.Task.Section,
				Start:   a.Start.Format(time.RFC3339),
				End:     a.End.Format(time.RFC3339),
				AtRisk:  a.AtRisk,
			}
		}
		for i, a := range result.AtRisk {
			resp.AtRisk[i] = apiAllocation{
				TaskKey: a.Task.Key,
				Summary: a.Task.Summary,
				Project: a.Task.Project,
				Section: a.Task.Section,
				Start:   a.Start.Format(time.RFC3339),
				End:     a.End.Format(time.RFC3339),
				AtRisk:  a.AtRisk,
			}
		}
		for i, u := range result.Unscheduled {
			resp.Unscheduled[i] = apiUnscheduled{
				TaskKey:  u.Task.Key,
				Summary:  u.Task.Summary,
				Estimate: serveFormatDuration(u.Task.RemainingEstimate),
				Reason:   u.Reason,
			}
		}

		web.WriteJSON(w, resp)
	}
}

func serveHandleStatus(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiStatusResponse{
			Providers:     cfg.ActiveProviders(),
			BusinessHours: cfg.BusinessHours,
			WindowDays:    cfg.Scheduling.WindowDays,
			BufferMinutes: cfg.Scheduling.BufferMinutes,
		})
	}
}

func serveDefaultQuery(cfg *config.Config) string {
	providers := cfg.ActiveProviders()
	switch providers[0] {
	case "todoist":
		return cfg.Todoist.DefaultFilter
	default:
		return cfg.Jira.DefaultJQL
	}
}

func serveFormatDuration(d time.Duration) string {
	if d <= 0 {
		return "\u2014"
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

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a local web dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			source, cfg, err := loadTaskSource()
			if err != nil {
				return err
			}

			cal, err := loadCalendarClient(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			port, _ := cmd.Flags().GetInt("port")

			var fetcher TaskFetcher
			if ms, ok := source.(*MultiTaskSource); ok {
				fetcher = &multiFetcher{
					queries: buildProviderQueries(cfg, "", ""),
					sources: ms.sources,
				}
			} else {
				fetcher = source
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			srv := web.NewServer(web.Handlers{
				Today:    serveHandleToday(cal),
				Tasks:    serveHandleTasks(fetcher, cfg),
				Schedule: serveHandleSchedule(fetcher, cal, cfg),
				Status:   serveHandleStatus(cfg),
			}, port)

			return srv.Run(ctx)
		},
	}

	cmd.Flags().IntP("port", "p", 8002, "Port to listen on")

	return cmd
}
