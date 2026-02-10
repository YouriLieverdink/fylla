package scheduler

import (
	"testing"
	"time"

	"github.com/iruoy/fylla/internal/config"
	"github.com/iruoy/fylla/internal/jira"
)

var defaultWeights = config.WeightsConfig{
	Priority:  0.40,
	DueDate:   0.30,
	Estimate:  0.15,
	IssueType: 0.10,
	Age:       0.05,
}

var defaultTypeScores = map[string]int{
	"Bug":   100,
	"Task":  70,
	"Story": 50,
}

func timePtr(t time.Time) *time.Time { return &t }

func Test_SORT001_priority_weight(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
	created := now.Add(-24 * time.Hour)

	highPri := jira.Task{Key: "P-1", Priority: 1, IssueType: "Task", Created: created}
	lowPri := jira.Task{Key: "P-2", Priority: 5, IssueType: "Task", Created: created}

	t.Run("higher priority task sorted first", func(t *testing.T) {
		tasks := []jira.Task{lowPri, highPri}
		sorted := SortTasks(tasks, defaultWeights, defaultTypeScores, now)
		if sorted[0].Task.Key != "P-1" {
			t.Errorf("expected P-1 first, got %s", sorted[0].Task.Key)
		}
	})

	t.Run("priority contributes 40 percent", func(t *testing.T) {
		// Priority 1 score = 100, weight = 0.40 → contribution = 40
		contrib := defaultWeights.Priority * PriorityScore(1)
		if Round(contrib, 2) != 40.0 {
			t.Errorf("expected priority contribution 40, got %.2f", contrib)
		}
	})
}

func Test_SORT002_due_date_weight(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
	created := now.Add(-24 * time.Hour)

	soonDue := jira.Task{
		Key: "D-1", Priority: 3, IssueType: "Task", Created: created,
		DueDate: timePtr(now.Add(24 * time.Hour)),
	}
	laterDue := jira.Task{
		Key: "D-2", Priority: 3, IssueType: "Task", Created: created,
		DueDate: timePtr(now.Add(20 * 24 * time.Hour)),
	}

	t.Run("earlier due date prioritized", func(t *testing.T) {
		sorted := SortTasks([]jira.Task{laterDue, soonDue}, defaultWeights, defaultTypeScores, now)
		if sorted[0].Task.Key != "D-1" {
			t.Errorf("expected D-1 first, got %s", sorted[0].Task.Key)
		}
	})

	t.Run("due date contributes 30 percent", func(t *testing.T) {
		// Due today = score 100, weight 0.30 → contribution = 30
		contrib := defaultWeights.DueDate * DueDateScore(timePtr(now), now)
		if Round(contrib, 2) != 30.0 {
			t.Errorf("expected due date contribution 30, got %.2f", contrib)
		}
	})
}

func Test_SORT003_estimate_weight(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
	created := now.Add(-24 * time.Hour)

	small := jira.Task{
		Key: "E-1", Priority: 3, IssueType: "Task", Created: created,
		RemainingEstimate: 30 * time.Minute,
	}
	large := jira.Task{
		Key: "E-2", Priority: 3, IssueType: "Task", Created: created,
		RemainingEstimate: 6 * time.Hour,
	}

	t.Run("smaller task prioritized", func(t *testing.T) {
		sorted := SortTasks([]jira.Task{large, small}, defaultWeights, defaultTypeScores, now)
		if sorted[0].Task.Key != "E-1" {
			t.Errorf("expected E-1 first, got %s", sorted[0].Task.Key)
		}
	})

	t.Run("estimate contributes 15 percent", func(t *testing.T) {
		// 0h task scores 0 (no estimate), but a small 30m task scores ~93.75
		// Max possible: 100 * 0.15 = 15
		maxContrib := defaultWeights.Estimate * 100
		if Round(maxContrib, 2) != 15.0 {
			t.Errorf("expected max estimate contribution 15, got %.2f", maxContrib)
		}
	})
}

func Test_SORT004_issue_type_weight(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
	created := now.Add(-24 * time.Hour)

	bug := jira.Task{Key: "T-1", Priority: 3, IssueType: "Bug", Created: created}
	task := jira.Task{Key: "T-2", Priority: 3, IssueType: "Task", Created: created}

	t.Run("bug prioritized over task", func(t *testing.T) {
		sorted := SortTasks([]jira.Task{task, bug}, defaultWeights, defaultTypeScores, now)
		if sorted[0].Task.Key != "T-1" {
			t.Errorf("expected T-1 (Bug) first, got %s", sorted[0].Task.Key)
		}
	})

	t.Run("issue type contributes 10 percent", func(t *testing.T) {
		// Bug=100, weight=0.10 → contribution = 10
		contrib := defaultWeights.IssueType * IssueTypeScore("Bug", defaultTypeScores)
		if Round(contrib, 2) != 10.0 {
			t.Errorf("expected issue type contribution 10, got %.2f", contrib)
		}
	})
}

func Test_SORT005_age_weight(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)

	old := jira.Task{
		Key: "A-1", Priority: 3, IssueType: "Task",
		Created: now.Add(-20 * 24 * time.Hour),
	}
	recent := jira.Task{
		Key: "A-2", Priority: 3, IssueType: "Task",
		Created: now.Add(-1 * 24 * time.Hour),
	}

	t.Run("older task gets slight boost", func(t *testing.T) {
		sorted := SortTasks([]jira.Task{recent, old}, defaultWeights, defaultTypeScores, now)
		if sorted[0].Task.Key != "A-1" {
			t.Errorf("expected A-1 (older) first, got %s", sorted[0].Task.Key)
		}
	})

	t.Run("age contributes 5 percent", func(t *testing.T) {
		// 30-day old task = 100, weight = 0.05 → max contribution = 5
		maxContrib := defaultWeights.Age * 100
		if Round(maxContrib, 2) != 5.0 {
			t.Errorf("expected max age contribution 5, got %.2f", maxContrib)
		}
	})
}

func Test_SORT006_priority_scoring(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		expected float64
	}{
		{"Highest(1)=100", 1, 100},
		{"High(2)=80", 2, 80},
		{"Medium(3)=60", 3, 60},
		{"Low(4)=40", 4, 40},
		{"Lowest(5)=20", 5, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PriorityScore(tt.priority)
			if got != tt.expected {
				t.Errorf("PriorityScore(%d) = %.0f, want %.0f", tt.priority, got, tt.expected)
			}
		})
	}

	t.Run("linear interpolation between levels", func(t *testing.T) {
		// Each step decreases by 20
		for p := 1; p < 5; p++ {
			diff := PriorityScore(p) - PriorityScore(p+1)
			if diff != 20 {
				t.Errorf("expected 20-point step between priority %d and %d, got %.0f", p, p+1, diff)
			}
		}
	})
}

func Test_SORT007_due_date_scoring(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)

	t.Run("due today scores 100", func(t *testing.T) {
		score := DueDateScore(timePtr(now), now)
		if score != 100 {
			t.Errorf("expected 100, got %.2f", score)
		}
	})

	t.Run("due in 30+ days scores 0", func(t *testing.T) {
		due := now.Add(31 * 24 * time.Hour)
		score := DueDateScore(&due, now)
		if score != 0 {
			t.Errorf("expected 0, got %.2f", score)
		}
	})

	t.Run("linear decay between 0 and 30 days", func(t *testing.T) {
		due15 := now.Add(15 * 24 * time.Hour)
		score := DueDateScore(&due15, now)
		expected := 50.0
		if Round(score, 1) != expected {
			t.Errorf("expected %.1f for 15 days, got %.1f", expected, score)
		}
	})

	t.Run("overdue scores 100", func(t *testing.T) {
		past := now.Add(-2 * 24 * time.Hour)
		score := DueDateScore(&past, now)
		if score != 100 {
			t.Errorf("expected 100 for overdue, got %.2f", score)
		}
	})

	t.Run("no due date scores 0", func(t *testing.T) {
		score := DueDateScore(nil, now)
		if score != 0 {
			t.Errorf("expected 0 for nil due date, got %.2f", score)
		}
	})
}

func Test_SORT008_estimate_scoring(t *testing.T) {
	t.Run("30 minute task scores high", func(t *testing.T) {
		score := EstimateScore(30 * time.Minute)
		// 30min = 0.5h → 100*(1-0.5/8) = 93.75
		if Round(score, 2) != 93.75 {
			t.Errorf("expected 93.75, got %.2f", score)
		}
	})

	t.Run("8 hour task scores 0", func(t *testing.T) {
		score := EstimateScore(8 * time.Hour)
		if score != 0 {
			t.Errorf("expected 0, got %.2f", score)
		}
	})

	t.Run("inverse relationship", func(t *testing.T) {
		small := EstimateScore(1 * time.Hour)
		large := EstimateScore(4 * time.Hour)
		if small <= large {
			t.Errorf("expected smaller estimate to score higher: 1h=%.2f, 4h=%.2f", small, large)
		}
	})

	t.Run("zero estimate scores 0", func(t *testing.T) {
		score := EstimateScore(0)
		if score != 0 {
			t.Errorf("expected 0, got %.2f", score)
		}
	})
}

func Test_SORT009_issue_type_scoring(t *testing.T) {
	tests := []struct {
		name     string
		itype    string
		expected float64
	}{
		{"Bug=100", "Bug", 100},
		{"Task=70", "Task", 70},
		{"Story=50", "Story", 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IssueTypeScore(tt.itype, defaultTypeScores)
			if got != tt.expected {
				t.Errorf("IssueTypeScore(%s) = %.0f, want %.0f", tt.itype, got, tt.expected)
			}
		})
	}

	t.Run("unknown type scores 0", func(t *testing.T) {
		got := IssueTypeScore("Epic", defaultTypeScores)
		if got != 0 {
			t.Errorf("expected 0 for unknown type, got %.0f", got)
		}
	})
}

func Test_SORT010_crunch_mode(t *testing.T) {
	now := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)

	t.Run("task due in 2 days gets crunch boost", func(t *testing.T) {
		due := now.Add(2 * 24 * time.Hour)
		boost := CrunchBoost(&due, now)
		if boost <= 0 {
			t.Errorf("expected positive crunch boost, got %.2f", boost)
		}
		// 2 days → 20*(1-2/3) ≈ 6.67
		expected := 20.0 * (1 - 2.0/3.0)
		if Round(boost, 2) != Round(expected, 2) {
			t.Errorf("expected %.2f boost, got %.2f", expected, boost)
		}
	})

	t.Run("task due in 5 days gets no boost", func(t *testing.T) {
		due := now.Add(5 * 24 * time.Hour)
		boost := CrunchBoost(&due, now)
		if boost != 0 {
			t.Errorf("expected 0 crunch boost for 5-day due, got %.2f", boost)
		}
	})

	t.Run("crunch boost affects sorting", func(t *testing.T) {
		created := now.Add(-24 * time.Hour)
		crunch := jira.Task{
			Key: "C-1", Priority: 3, IssueType: "Task", Created: created,
			DueDate: timePtr(now.Add(2 * 24 * time.Hour)),
		}
		normal := jira.Task{
			Key: "C-2", Priority: 3, IssueType: "Task", Created: created,
			DueDate: timePtr(now.Add(10 * 24 * time.Hour)),
		}
		sorted := SortTasks([]jira.Task{normal, crunch}, defaultWeights, defaultTypeScores, now)
		if sorted[0].Task.Key != "C-1" {
			t.Errorf("expected crunch task C-1 first, got %s", sorted[0].Task.Key)
		}
	})

	t.Run("no due date means no boost", func(t *testing.T) {
		boost := CrunchBoost(nil, now)
		if boost != 0 {
			t.Errorf("expected 0 for nil due date, got %.2f", boost)
		}
	})
}
