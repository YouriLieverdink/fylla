package commands

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// mockEstimateUpdater records UpdateEstimate calls.
type mockEstimateUpdater struct {
	calls []estimateCall
	err   error
}

type estimateCall struct {
	issueKey  string
	remaining time.Duration
}

func (m *mockEstimateUpdater) UpdateEstimate(_ context.Context, issueKey string, remaining time.Duration) error {
	m.calls = append(m.calls, estimateCall{issueKey, remaining})
	return m.err
}

// mockEstimateGetter returns a preset current estimate.
type mockEstimateGetter struct {
	estimate time.Duration
	err      error
}

func (m *mockEstimateGetter) GetEstimate(_ context.Context, _ string) (time.Duration, error) {
	return m.estimate, m.err
}

func TestCLI015_estimate_sets_remaining(t *testing.T) {
	t.Run("sets remaining estimate to 4h", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "4h",
			Updater:  updater,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		if len(updater.calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(updater.calls))
		}
		if updater.calls[0].issueKey != "PROJ-123" {
			t.Errorf("issueKey = %q, want PROJ-123", updater.calls[0].issueKey)
		}
		if updater.calls[0].remaining != 4*time.Hour {
			t.Errorf("remaining = %v, want 4h", updater.calls[0].remaining)
		}

		if result.TaskKey != "PROJ-123" {
			t.Errorf("result.TaskKey = %q, want PROJ-123", result.TaskKey)
		}
		if result.Duration != 4*time.Hour {
			t.Errorf("result.Duration = %v, want 4h", result.Duration)
		}
	})

	t.Run("sets remaining estimate to 30m", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-456",
			Duration: "30m",
			Updater:  updater,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		if updater.calls[0].remaining != 30*time.Minute {
			t.Errorf("remaining = %v, want 30m", updater.calls[0].remaining)
		}
		if result.Duration != 30*time.Minute {
			t.Errorf("result.Duration = %v, want 30m", result.Duration)
		}
	})

	t.Run("confirmation message displayed", func(t *testing.T) {
		var buf bytes.Buffer
		PrintEstimateResult(&buf, &EstimateResult{
			TaskKey:  "PROJ-123",
			Duration: 4 * time.Hour,
		})
		out := buf.String()
		if !strings.Contains(out, "PROJ-123") {
			t.Errorf("output = %q, want to contain PROJ-123", out)
		}
		if !strings.Contains(out, "4h") {
			t.Errorf("output = %q, want to contain 4h", out)
		}
	})

	t.Run("returns error for invalid duration", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		_, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "invalid",
			Updater:  updater,
		})
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})

	t.Run("returns error from updater", func(t *testing.T) {
		updater := &mockEstimateUpdater{err: fmt.Errorf("update error")}
		_, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "4h",
			Updater:  updater,
		})
		if err == nil {
			t.Fatal("expected error from updater")
		}
	})

	t.Run("sets combined duration 1h30m", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "1h30m",
			Updater:  updater,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}
		want := 1*time.Hour + 30*time.Minute
		if result.Duration != want {
			t.Errorf("result.Duration = %v, want %v", result.Duration, want)
		}
	})
}

func TestCLI016_estimate_relative_adjustments(t *testing.T) {
	t.Run("add 2h to current 4h estimate", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{estimate: 4 * time.Hour}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "+2h",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		if result.Duration != 6*time.Hour {
			t.Errorf("result.Duration = %v, want 6h", result.Duration)
		}
		if updater.calls[0].remaining != 6*time.Hour {
			t.Errorf("remaining = %v, want 6h", updater.calls[0].remaining)
		}
	})

	t.Run("subtract 1h from current 5h estimate", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{estimate: 5 * time.Hour}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "-1h",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		if result.Duration != 4*time.Hour {
			t.Errorf("result.Duration = %v, want 4h", result.Duration)
		}
	})

	t.Run("subtract more than current clamps to zero", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{estimate: 30 * time.Minute}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "-2h",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		if result.Duration != 0 {
			t.Errorf("result.Duration = %v, want 0", result.Duration)
		}
	})

	t.Run("add 30m to current 4h estimate", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{estimate: 4 * time.Hour}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "+30m",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		want := 4*time.Hour + 30*time.Minute
		if result.Duration != want {
			t.Errorf("result.Duration = %v, want %v", result.Duration, want)
		}
	})

	t.Run("returns error when getter fails", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{err: fmt.Errorf("fetch error")}
		_, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "+2h",
			Updater:  updater,
			Getter:   getter,
		})
		if err == nil {
			t.Fatal("expected error from getter")
		}
	})

	t.Run("returns error for invalid relative duration", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{estimate: 4 * time.Hour}
		_, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "+invalid",
			Updater:  updater,
			Getter:   getter,
		})
		if err == nil {
			t.Fatal("expected error for invalid relative duration")
		}
	})

	t.Run("subtract 30m from 1h estimate", func(t *testing.T) {
		updater := &mockEstimateUpdater{}
		getter := &mockEstimateGetter{estimate: 1 * time.Hour}
		result, err := RunEstimate(context.Background(), EstimateParams{
			TaskKey:  "PROJ-123",
			Duration: "-30m",
			Updater:  updater,
			Getter:   getter,
		})
		if err != nil {
			t.Fatalf("RunEstimate: %v", err)
		}

		if result.Duration != 30*time.Minute {
			t.Errorf("result.Duration = %v, want 30m", result.Duration)
		}
	})
}
