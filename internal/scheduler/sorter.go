package scheduler

import (
	"math"
	"sort"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
)

// ScoredTask pairs a task with its computed composite score.
type ScoredTask struct {
	Task  task.Task
	Score float64
}

// SortTasks scores and sorts tasks by descending composite score.
// UpNext tasks are sorted first (by score among themselves), followed by regular tasks.
// The now parameter is used for relative date calculations.
func SortTasks(tasks []task.Task, cfg config.WeightsConfig, typeScores map[string]int, now time.Time) []ScoredTask {
	scored := make([]ScoredTask, len(tasks))
	for i, t := range tasks {
		scored[i] = ScoredTask{
			Task:  t,
			Score: CompositeScore(t, cfg, typeScores, now),
		}
	}

	var upnext, regular []ScoredTask
	for _, st := range scored {
		if st.Task.UpNext {
			upnext = append(upnext, st)
		} else {
			regular = append(regular, st)
		}
	}

	sort.SliceStable(upnext, func(i, j int) bool {
		return upnext[i].Score > upnext[j].Score
	})
	sort.SliceStable(regular, func(i, j int) bool {
		return regular[i].Score > regular[j].Score
	})

	return append(upnext, regular...)
}

// CompositeScore calculates the weighted composite score for a task.
func CompositeScore(t task.Task, w config.WeightsConfig, typeScores map[string]int, now time.Time) float64 {
	score := w.Priority*PriorityScore(t.Priority) +
		w.DueDate*DueDateScore(t.DueDate, now) +
		w.Estimate*EstimateScore(t.RemainingEstimate) +
		w.IssueType*IssueTypeScore(t.IssueType, typeScores) +
		w.Age*AgeScore(t.Created, now)

	score += CrunchBoost(t.DueDate, now)

	return score
}

// PriorityScore maps Jira priority (1-5) to a 0-100 score.
// Highest(1)=100, High(2)=80, Medium(3)=60, Low(4)=40, Lowest(5)=20.
func PriorityScore(priority int) float64 {
	if priority < 1 {
		priority = 3
	}
	if priority > 5 {
		priority = 5
	}
	return float64(120 - 20*priority)
}

// DueDateScore scores based on days until due: 0 days=100, 30+ days=0.
// Tasks without a due date score 0.
func DueDateScore(dueDate *time.Time, now time.Time) float64 {
	if dueDate == nil {
		return 0
	}
	days := dueDate.Sub(now).Hours() / 24
	if days <= 0 {
		return 100
	}
	if days >= 30 {
		return 0
	}
	return 100 * (1 - days/30)
}

// EstimateScore scores smaller tasks higher (quick wins).
// 0 or unset estimate → 0 score. Uses inverse relationship capped at 8h.
func EstimateScore(estimate time.Duration) float64 {
	if estimate <= 0 {
		return 0
	}
	hours := estimate.Hours()
	if hours >= 8 {
		return 0
	}
	return 100 * (1 - hours/8)
}

// IssueTypeScore returns the configured score for an issue type.
// Returns 0 if the type is not found in the map.
func IssueTypeScore(issueType string, typeScores map[string]int) float64 {
	if s, ok := typeScores[issueType]; ok {
		return float64(s)
	}
	return 0
}

// AgeScore gives older tasks a slight boost.
// 30+ day old tasks get max score (100), new tasks get 0.
func AgeScore(created time.Time, now time.Time) float64 {
	days := now.Sub(created).Hours() / 24
	if days <= 0 {
		return 0
	}
	if days >= 30 {
		return 100
	}
	return 100 * days / 30
}

// CrunchBoost adds extra priority for tasks due within 3 days.
// Returns a flat 20-point bonus scaled by urgency.
func CrunchBoost(dueDate *time.Time, now time.Time) float64 {
	if dueDate == nil {
		return 0
	}
	days := dueDate.Sub(now).Hours() / 24
	if days < 0 || days > 3 {
		return 0
	}
	return 20 * (1 - days/3)
}

// Round is a helper to round floats for test comparisons.
func Round(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}
