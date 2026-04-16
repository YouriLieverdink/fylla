package scheduler

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/task"
)

// ScoreBreakdown holds the individual components of a composite score.
type ScoreBreakdown struct {
	PriorityRaw      float64
	PriorityWeight   float64
	PriorityWeighted float64
	PriorityReason   string
	DueDateRaw       float64
	DueDateWeight    float64
	DueDateWeighted  float64
	DueDateReason    string
	EstimateRaw      float64
	EstimateWeight   float64
	EstimateWeighted float64
	EstimateReason   string
	AgeRaw           float64
	AgeWeight        float64
	AgeWeighted      float64
	AgeReason        string
	CrunchBoost      float64
	CrunchReason     string
	TypeBonus        float64
	TypeBonusReason  string
	UpNextBoost      float64
	NotBeforeMult    float64
	NotBeforeReason  string
	Total            float64
}

// ScoredTask pairs a task with its computed composite score.
type ScoredTask struct {
	Task      task.Task
	Score     float64
	Breakdown ScoreBreakdown
}

// SortTasks scores and sorts tasks by descending composite score.
// The now parameter is used for relative date calculations.
func SortTasks(tasks []task.Task, cfg config.WeightsConfig, now time.Time) []ScoredTask {
	scored := make([]ScoredTask, len(tasks))
	for i, t := range tasks {
		bd := CompositeScoreBreakdown(t, cfg, now)
		scored[i] = ScoredTask{
			Task:      t,
			Score:     bd.Total,
			Breakdown: bd,
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	return scored
}

// CompositeScore calculates the weighted composite score for a task.
func CompositeScore(t task.Task, w config.WeightsConfig, now time.Time) float64 {
	score := w.Priority*PriorityScore(t.Priority) +
		w.DueDate*DueDateScore(t.DueDate, now) +
		w.Estimate*EstimateScore(t.RemainingEstimate) +
		w.Age*AgeScore(t.Created, now)

	score += CrunchBoost(t.DueDate, now)
	score += TypeBonus(t.IssueType, w.TypeBonus)

	if t.UpNext {
		score += w.UpNext
	} else {
		score *= NotBeforePenalty(t.NotBefore, now)
	}

	return score
}

// CompositeScoreBreakdown calculates the weighted composite score and returns
// the individual components for display.
func CompositeScoreBreakdown(t task.Task, w config.WeightsConfig, now time.Time) ScoreBreakdown {
	bd := ScoreBreakdown{
		PriorityRaw:      PriorityScore(t.Priority),
		PriorityWeight:   w.Priority,
		PriorityWeighted: w.Priority * PriorityScore(t.Priority),
		PriorityReason:   priorityReason(t.Priority),
		DueDateRaw:       DueDateScore(t.DueDate, now),
		DueDateWeight:    w.DueDate,
		DueDateWeighted:  w.DueDate * DueDateScore(t.DueDate, now),
		DueDateReason:    dueDateReason(t.DueDate, now),
		EstimateRaw:      EstimateScore(t.RemainingEstimate),
		EstimateWeight:   w.Estimate,
		EstimateWeighted: w.Estimate * EstimateScore(t.RemainingEstimate),
		EstimateReason:   estimateReason(t.RemainingEstimate),
		AgeRaw:           AgeScore(t.Created, now),
		AgeWeight:        w.Age,
		AgeWeighted:      w.Age * AgeScore(t.Created, now),
		AgeReason:        ageReason(t.Created, now),
		CrunchBoost:      CrunchBoost(t.DueDate, now),
		CrunchReason:     crunchReason(t.DueDate, now),
		TypeBonus:        TypeBonus(t.IssueType, w.TypeBonus),
		TypeBonusReason:  typeBonusReason(t.IssueType, w.TypeBonus),
		NotBeforeMult:    1.0,
	}

	score := bd.PriorityWeighted + bd.DueDateWeighted + bd.EstimateWeighted + bd.AgeWeighted
	score += bd.CrunchBoost
	score += bd.TypeBonus

	if t.UpNext {
		bd.UpNextBoost = w.UpNext
		score += bd.UpNextBoost
	} else {
		bd.NotBeforeMult = NotBeforePenalty(t.NotBefore, now)
		bd.NotBeforeReason = notBeforeReason(t.NotBefore, now)
		score *= bd.NotBeforeMult
	}

	bd.Total = score
	return bd
}

// PriorityScore maps priority (1-5) to a 0-100 score.
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
// Overdue tasks (days <= 0) receive the maximum 20-point boost.
func CrunchBoost(dueDate *time.Time, now time.Time) float64 {
	if dueDate == nil {
		return 0
	}
	days := dueDate.Sub(now).Hours() / 24
	if days > 3 {
		return 0
	}
	if days <= 0 {
		return 20
	}
	return 20 * (1 - days/3)
}

// TypeBonus returns the configured flat bonus for the given issue type, or 0.
func TypeBonus(issueType string, bonuses map[string]float64) float64 {
	if len(bonuses) == 0 {
		return 0
	}
	return bonuses[issueType]
}

// NotBeforePenalty returns a multiplier (0.2 to 1.0) based on how far away
// the NotBefore date is. Tasks actionable now get 1.0, tasks 7+ days away
// get 0.2, with linear interpolation in between.
func NotBeforePenalty(notBefore *time.Time, now time.Time) float64 {
	if notBefore == nil || !notBefore.After(now) {
		return 1.0
	}
	days := notBefore.Sub(now).Hours() / 24
	if days >= 7 {
		return 0.2
	}
	return 1.0 - 0.8*days/7
}

// Round is a helper to round floats for test comparisons.
func Round(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}

var priorityNames = [6]string{"", "Highest", "High", "Medium", "Low", "Lowest"}

func priorityReason(priority int) string {
	if priority < 1 || priority > 5 {
		return "unset"
	}
	return priorityNames[priority]
}

func dueDateReason(dueDate *time.Time, now time.Time) string {
	if dueDate == nil {
		return "no due date"
	}
	days := int(math.Ceil(dueDate.Sub(now).Hours() / 24))
	if days < 0 {
		return fmt.Sprintf("%d days overdue", -days)
	}
	if days == 0 {
		return "due today"
	}
	if days == 1 {
		return "due tomorrow"
	}
	return fmt.Sprintf("due in %d days", days)
}

func estimateReason(estimate time.Duration) string {
	if estimate <= 0 {
		return "no estimate"
	}
	h := estimate.Hours()
	if h < 1 {
		return fmt.Sprintf("%.0fm", estimate.Minutes())
	}
	return fmt.Sprintf("%.1fh", h)
}

func ageReason(created time.Time, now time.Time) string {
	days := int(now.Sub(created).Hours() / 24)
	if days <= 0 {
		return "created today"
	}
	if days == 1 {
		return "1 day old"
	}
	return fmt.Sprintf("%d days old", days)
}

func crunchReason(dueDate *time.Time, now time.Time) string {
	if dueDate == nil {
		return "no due date"
	}
	days := dueDate.Sub(now).Hours() / 24
	if days > 3 {
		return "due in >3 days"
	}
	if days <= 0 {
		return "overdue"
	}
	return fmt.Sprintf("due in %.1f days", days)
}

func typeBonusReason(issueType string, bonuses map[string]float64) string {
	if issueType == "" {
		return "no type"
	}
	if len(bonuses) == 0 || bonuses[issueType] == 0 {
		return issueType
	}
	return issueType
}

func notBeforeReason(notBefore *time.Time, now time.Time) string {
	if notBefore == nil || !notBefore.After(now) {
		return "actionable now"
	}
	days := int(math.Ceil(notBefore.Sub(now).Hours() / 24))
	if days == 1 {
		return "starts tomorrow"
	}
	return fmt.Sprintf("starts in %d days", days)
}
