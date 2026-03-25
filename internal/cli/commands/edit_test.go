package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// editMock extends mockSource with configurable return values for edit tests.
type editMock struct {
	mockSource
	estimateVal    time.Duration
	estimateErr    error
	dueDateVal     *time.Time
	dueDateErr     error
	priorityVal    int
	priorityErr    error
	updateErr      error
	removeDueErr   error
	getSummaryErr  error
	updateSumErr   error
	updatedEst     time.Duration
	updatedDue     time.Time
	updatedPri     int
	removedDue     bool
	updatedParent  string
	parentUpdated  bool
}

func (m *editMock) GetEstimate(_ context.Context, _ string) (time.Duration, error) {
	return m.estimateVal, m.estimateErr
}

func (m *editMock) UpdateEstimate(_ context.Context, _ string, d time.Duration) error {
	m.updatedEst = d
	return m.updateErr
}

func (m *editMock) GetDueDate(_ context.Context, _ string) (*time.Time, error) {
	return m.dueDateVal, m.dueDateErr
}

func (m *editMock) UpdateDueDate(_ context.Context, _ string, d time.Time) error {
	m.updatedDue = d
	return m.updateErr
}

func (m *editMock) GetPriority(_ context.Context, _ string) (int, error) {
	return m.priorityVal, m.priorityErr
}

func (m *editMock) UpdatePriority(_ context.Context, _ string, p int) error {
	m.updatedPri = p
	return m.updateErr
}

func (m *editMock) RemoveDueDate(_ context.Context, _ string) error {
	m.removedDue = true
	return m.removeDueErr
}

func (m *editMock) GetSummary(_ context.Context, _ string) (string, error) {
	return m.summary, m.getSummaryErr
}

func (m *editMock) UpdateSummary(_ context.Context, _ string, s string) error {
	m.updatedSummary = s
	return m.updateSumErr
}

func (m *editMock) UpdateParent(_ context.Context, _ string, parent string) error {
	m.updatedParent = parent
	m.parentUpdated = true
	return m.updateErr
}

func TestRunEdit_SingleFlags(t *testing.T) {
	ctx := context.Background()

	t.Run("estimate only", func(t *testing.T) {
		m := &editMock{}
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			Estimate: "4h",
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EstimateResult == nil {
			t.Fatal("expected estimate result")
		}
		if result.EstimateResult.Duration != 4*time.Hour {
			t.Errorf("estimate = %v, want 4h", result.EstimateResult.Duration)
		}
		if m.updatedEst != 4*time.Hour {
			t.Errorf("updatedEst = %v, want 4h", m.updatedEst)
		}
	})

	t.Run("due only", func(t *testing.T) {
		m := &editMock{}
		result, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			Due:     "2025-06-15",
			Source:  m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.DueDateResult == nil {
			t.Fatal("expected due date result")
		}
		if result.DueDateResult.DueDate.Format("2006-01-02") != "2025-06-15" {
			t.Errorf("due date = %v, want 2025-06-15", result.DueDateResult.DueDate)
		}
	})

	t.Run("no-due only", func(t *testing.T) {
		m := &editMock{}
		result, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			NoDue:   true,
			Source:  m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.DueDateRemoved {
			t.Fatal("expected due date removed")
		}
		if !m.removedDue {
			t.Fatal("expected RemoveDueDate to be called")
		}
	})

	t.Run("priority only", func(t *testing.T) {
		m := &editMock{priorityVal: 3}
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			Priority: "High",
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.PriorityResult == nil {
			t.Fatal("expected priority result")
		}
		if result.PriorityResult.Priority != 2 {
			t.Errorf("priority = %d, want 2 (High)", result.PriorityResult.Priority)
		}
	})
}

func TestRunEdit_UpNext(t *testing.T) {
	ctx := context.Background()

	t.Run("add upnext to summary", func(t *testing.T) {
		m := &editMock{}
		m.summary = "Do the thing"
		result, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			UpNext:  true,
			Source:  m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.UpNextSet {
			t.Fatal("expected UpNextSet")
		}
		if m.updatedSummary != "Do the thing (upnext)" {
			t.Errorf("summary = %q, want %q", m.updatedSummary, "Do the thing (upnext)")
		}
	})

	t.Run("upnext already present is idempotent", func(t *testing.T) {
		m := &editMock{}
		m.summary = "Do the thing (upnext)"
		result, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			UpNext:  true,
			Source:  m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.UpNextSet {
			t.Fatal("expected UpNextSet")
		}
		// Should not have called UpdateSummary since it's already there
		if m.updatedSummary != "" {
			t.Errorf("should not update summary when upnext already present, got %q", m.updatedSummary)
		}
	})

	t.Run("remove upnext from summary", func(t *testing.T) {
		m := &editMock{}
		m.summary = "Do the thing (upnext)"
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			NoUpNext: true,
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.UpNextRemoved {
			t.Fatal("expected UpNextRemoved")
		}
		if m.updatedSummary != "Do the thing" {
			t.Errorf("summary = %q, want %q", m.updatedSummary, "Do the thing")
		}
	})

	t.Run("remove upnext when not present is idempotent", func(t *testing.T) {
		m := &editMock{}
		m.summary = "Do the thing"
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			NoUpNext: true,
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.UpNextRemoved {
			t.Fatal("expected UpNextRemoved")
		}
		if m.updatedSummary != "" {
			t.Errorf("should not update summary when upnext not present, got %q", m.updatedSummary)
		}
	})
}

func TestRunEdit_MultiFlag(t *testing.T) {
	ctx := context.Background()

	t.Run("estimate and priority together", func(t *testing.T) {
		m := &editMock{priorityVal: 3}
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			Estimate: "2h",
			Priority: "High",
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EstimateResult == nil {
			t.Fatal("expected estimate result")
		}
		if result.PriorityResult == nil {
			t.Fatal("expected priority result")
		}
	})

	t.Run("all flags together", func(t *testing.T) {
		m := &editMock{priorityVal: 3}
		m.summary = "Do the thing"
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			Estimate: "4h",
			Due:      "2025-06-15",
			Priority: "High",
			UpNext:   true,
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.EstimateResult == nil {
			t.Fatal("expected estimate result")
		}
		if result.DueDateResult == nil {
			t.Fatal("expected due date result")
		}
		if result.PriorityResult == nil {
			t.Fatal("expected priority result")
		}
		if !result.UpNextSet {
			t.Fatal("expected UpNextSet")
		}
	})
}

func TestRunEdit_ErrorPropagation(t *testing.T) {
	ctx := context.Background()

	t.Run("estimate error", func(t *testing.T) {
		m := &editMock{updateErr: fmt.Errorf("api error")}
		_, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			Estimate: "4h",
			Source:   m,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "estimate") {
			t.Errorf("error should mention estimate: %v", err)
		}
	})

	t.Run("due date error", func(t *testing.T) {
		m := &editMock{updateErr: fmt.Errorf("api error")}
		_, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			Due:     "2025-06-15",
			Source:  m,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "due date") {
			t.Errorf("error should mention due date: %v", err)
		}
	})

	t.Run("remove due date error", func(t *testing.T) {
		m := &editMock{removeDueErr: fmt.Errorf("api error")}
		_, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			NoDue:   true,
			Source:  m,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "remove due date") {
			t.Errorf("error should mention remove due date: %v", err)
		}
	})

	t.Run("priority error", func(t *testing.T) {
		m := &editMock{priorityVal: 3, updateErr: fmt.Errorf("api error")}
		_, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			Priority: "High",
			Source:   m,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "priority") {
			t.Errorf("error should mention priority: %v", err)
		}
	})

	t.Run("get summary error", func(t *testing.T) {
		m := &editMock{getSummaryErr: fmt.Errorf("api error")}
		_, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			UpNext:  true,
			Source:  m,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "get summary") {
			t.Errorf("error should mention get summary: %v", err)
		}
	})

	t.Run("update summary error", func(t *testing.T) {
		m := &editMock{updateSumErr: fmt.Errorf("api error")}
		m.summary = "Do the thing"
		_, err := RunEdit(ctx, EditParams{
			TaskKey: "PROJ-1",
			UpNext:  true,
			Source:  m,
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "update summary") {
			t.Errorf("error should mention update summary: %v", err)
		}
	})
}

func TestPrintEditResult(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		var buf bytes.Buffer
		PrintEditResult(&buf, &EditResult{
			TaskKey:        "PROJ-1",
			EstimateResult: &EstimateResult{TaskKey: "PROJ-1", Duration: 4 * time.Hour},
			DueDateResult:  &DueDateResult{TaskKey: "PROJ-1", DueDate: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)},
			PriorityResult: &PriorityResult{TaskKey: "PROJ-1", Priority: 2, Name: "High"},
			UpNextSet:      true,
		})
		output := buf.String()
		if !strings.Contains(output, "Remaining estimate") {
			t.Errorf("missing estimate output: %s", output)
		}
		if !strings.Contains(output, "Due date") {
			t.Errorf("missing due date output: %s", output)
		}
		if !strings.Contains(output, "Priority") {
			t.Errorf("missing priority output: %s", output)
		}
		if !strings.Contains(output, "marked as up next") {
			t.Errorf("missing up next output: %s", output)
		}
	})

	t.Run("due date removed", func(t *testing.T) {
		var buf bytes.Buffer
		PrintEditResult(&buf, &EditResult{
			TaskKey:        "PROJ-1",
			DueDateRemoved: true,
		})
		output := buf.String()
		if !strings.Contains(output, "removed") {
			t.Errorf("missing removed output: %s", output)
		}
	})

	t.Run("unmarked up next", func(t *testing.T) {
		var buf bytes.Buffer
		PrintEditResult(&buf, &EditResult{
			TaskKey:       "PROJ-1",
			UpNextRemoved: true,
		})
		output := buf.String()
		if !strings.Contains(output, "unmarked as up next") {
			t.Errorf("missing unmarked output: %s", output)
		}
	})

	t.Run("cleared fields", func(t *testing.T) {
		var buf bytes.Buffer
		PrintEditResult(&buf, &EditResult{
			TaskKey:         "PROJ-1",
			EstimateRemoved: true,
			PriorityRemoved: true,
			ParentRemoved:   true,
		})
		output := buf.String()
		if !strings.Contains(output, "Estimate for PROJ-1 removed") {
			t.Errorf("missing estimate removed output: %s", output)
		}
		if !strings.Contains(output, "Priority for PROJ-1 removed") {
			t.Errorf("missing priority removed output: %s", output)
		}
		if !strings.Contains(output, "parent removed") {
			t.Errorf("missing parent removed output: %s", output)
		}
	})
}

func TestRunEdit_ClearFields(t *testing.T) {
	ctx := context.Background()

	t.Run("no-estimate clears estimate", func(t *testing.T) {
		m := &editMock{}
		result, err := RunEdit(ctx, EditParams{
			TaskKey:    "PROJ-1",
			NoEstimate: true,
			Source:     m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.EstimateRemoved {
			t.Fatal("expected EstimateRemoved")
		}
		if m.updatedEst != 0 {
			t.Errorf("expected estimate set to 0, got %v", m.updatedEst)
		}
	})

	t.Run("no-priority clears priority", func(t *testing.T) {
		m := &editMock{}
		result, err := RunEdit(ctx, EditParams{
			TaskKey:    "PROJ-1",
			NoPriority: true,
			Source:     m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.PriorityRemoved {
			t.Fatal("expected PriorityRemoved")
		}
		if m.updatedPri != 0 {
			t.Errorf("expected priority set to 0, got %d", m.updatedPri)
		}
	})

	t.Run("no-parent clears parent", func(t *testing.T) {
		m := &editMock{}
		result, err := RunEdit(ctx, EditParams{
			TaskKey:  "PROJ-1",
			NoParent: true,
			Source:   m,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.ParentRemoved {
			t.Fatal("expected ParentRemoved")
		}
		if !m.parentUpdated {
			t.Fatal("expected UpdateParent to be called")
		}
		if m.updatedParent != "" {
			t.Errorf("expected parent set to empty, got %q", m.updatedParent)
		}
	})
}

