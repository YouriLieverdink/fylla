package scheduler

import (
	"fmt"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/task"
)

// defaultEstimate is used when a task has no remaining estimate.
const defaultEstimate = time.Hour

// Allocation represents a task assigned to a time slot.
type Allocation struct {
	Task   task.Task
	Start  time.Time
	End    time.Time
	AtRisk bool
}

// UnscheduledTask wraps a task that could not be scheduled with a reason.
type UnscheduledTask struct {
	Task   task.Task
	Reason string
}

// AllocateConfig holds parameters for the allocation algorithm.
type AllocateConfig struct {
	MinTaskDurationMinutes int
	BufferMinutes          int
	SnapMinutes            []int
}

// Allocate assigns sorted tasks to available free slots using a first-fit algorithm.
// Tasks are processed in order (highest priority first) and assigned to the first
// slot that fits their duration.
//
// slotsByProject maps project keys to their available slots. The empty string key ""
// holds default slots for projects without specific rules. When a task is allocated,
// that time is consumed globally across all project slot lists.
func Allocate(tasks []ScoredTask, slotsByProject map[string][]calendar.Slot, cfg AllocateConfig) ([]Allocation, []UnscheduledTask) {
	minDur := time.Duration(cfg.MinTaskDurationMinutes) * time.Minute
	buffer := time.Duration(cfg.BufferMinutes) * time.Minute

	var consumed []allocRange
	var allocations []Allocation
	var unscheduled []UnscheduledTask

	for _, st := range tasks {
		estimate := st.Task.RemainingEstimate
		if estimate <= 0 {
			estimate = defaultEstimate
		}

		slots := projectSlots(slotsByProject, st.Task.Project)
		available := availableSlots(slots, consumed, minDur)
		available = snapSlotStarts(available, cfg.SnapMinutes, minDur)

		// Filter out slots that start before the task's not-before date
		if st.Task.NotBefore != nil {
			available = filterSlotsNotBefore(available, *st.Task.NotBefore)
		}

		if len(available) == 0 {
			unscheduled = append(unscheduled, UnscheduledTask{
				Task:   st.Task,
				Reason: "no available slots",
			})
			continue
		}

		remaining := estimate
		var taskAllocs []Allocation

		for _, slot := range available {
			if remaining <= 0 {
				break
			}

			slotDur := slot.End.Sub(slot.Start)

			if remaining <= slotDur {
				// Task fits entirely in this slot
				alloc := Allocation{
					Task:  st.Task,
					Start: slot.Start,
					End:   slot.Start.Add(remaining),
				}
				taskAllocs = append(taskAllocs, alloc)
				consumed = append(consumed, allocRange{start: alloc.Start, end: alloc.End.Add(buffer)})
				remaining = 0
				break
			}

			// NoSplit: task must fit in a single slot, skip if it doesn't
			if st.Task.NoSplit {
				continue
			}

			// Task doesn't fit entirely — check if splitting is viable.
			// The remainder after using this slot must be >= minDur.
			if remaining-slotDur < minDur {
				continue
			}

			// Split: use the entire slot for part of the task
			alloc := Allocation{
				Task:  st.Task,
				Start: slot.Start,
				End:   slot.End,
			}
			taskAllocs = append(taskAllocs, alloc)
			consumed = append(consumed, allocRange{start: slot.Start, end: slot.End.Add(buffer)})
			remaining -= slotDur
		}

		if remaining > 0 {
			reason := "not enough time"
			if st.Task.NoSplit {
				reason = "no slot large enough (no-split)"
			}
			unscheduled = append(unscheduled, UnscheduledTask{
				Task:   st.Task,
				Reason: reason,
			})
			continue
		}

		// Split label: annotate each part when a task is split across slots.
		if len(taskAllocs) > 1 {
			total := len(taskAllocs)
			for i := range taskAllocs {
				taskAllocs[i].Task.Summary = fmt.Sprintf("%s (%d/%d)", taskAllocs[i].Task.Summary, i+1, total)
			}
		}

		// At-risk detection: task's last block ends after end-of-day on its due date.
		// Due dates from providers are date-only (midnight), so compare against
		// end-of-day to avoid false positives for tasks scheduled on their due date.
		if st.Task.DueDate != nil && len(taskAllocs) > 0 {
			lastEnd := taskAllocs[len(taskAllocs)-1].End
			dueEnd := time.Date(st.Task.DueDate.Year(), st.Task.DueDate.Month(), st.Task.DueDate.Day()+1, 0, 0, 0, 0, st.Task.DueDate.Location())
			if lastEnd.After(dueEnd) {
				for i := range taskAllocs {
					taskAllocs[i].AtRisk = true
				}
			}
		}

		allocations = append(allocations, taskAllocs...)
	}

	return allocations, unscheduled
}

func projectSlots(slotsByProject map[string][]calendar.Slot, project string) []calendar.Slot {
	if slots, ok := slotsByProject[project]; ok {
		return slots
	}
	return slotsByProject[""]
}

type allocRange struct {
	start, end time.Time
}

// filterSlotsNotBefore removes slots that start before the given time.
// Slots that span the boundary are trimmed to start at notBefore.
func filterSlotsNotBefore(slots []calendar.Slot, notBefore time.Time) []calendar.Slot {
	var result []calendar.Slot
	for _, s := range slots {
		if !s.End.After(notBefore) {
			continue
		}
		if s.Start.Before(notBefore) {
			s.Start = notBefore
		}
		result = append(result, s)
	}
	return result
}

// availableSlots returns slots with consumed ranges removed, filtered by minDur.
func availableSlots(slots []calendar.Slot, consumed []allocRange, minDur time.Duration) []calendar.Slot {
	var result []calendar.Slot
	for _, slot := range slots {
		remaining := subtractConsumedRanges(slot, consumed)
		for _, r := range remaining {
			if r.End.Sub(r.Start) >= minDur {
				result = append(result, r)
			}
		}
	}
	return result
}

// subtractConsumedRanges removes consumed time ranges from a slot, returning remaining pieces.
func subtractConsumedRanges(slot calendar.Slot, ranges []allocRange) []calendar.Slot {
	current := []calendar.Slot{slot}
	for _, r := range ranges {
		var next []calendar.Slot
		for _, s := range current {
			if !r.start.Before(s.End) || !r.end.After(s.Start) {
				next = append(next, s)
				continue
			}
			if r.start.After(s.Start) {
				next = append(next, calendar.Slot{Start: s.Start, End: r.start})
			}
			if r.end.Before(s.End) {
				next = append(next, calendar.Slot{Start: r.end, End: s.End})
			}
		}
		current = next
	}
	return current
}

// snapSlotStarts snaps each slot's start time forward to the nearest allowed minute.
// If snapMinutes is empty, no snapping is applied.
func snapSlotStarts(slots []calendar.Slot, snapMinutes []int, minDur time.Duration) []calendar.Slot {
	if len(snapMinutes) == 0 {
		return slots
	}
	var result []calendar.Slot
	for _, s := range slots {
		snapped := snapForward(s.Start, snapMinutes)
		if snapped.Before(s.End) && s.End.Sub(snapped) >= minDur {
			result = append(result, calendar.Slot{Start: snapped, End: s.End})
		}
	}
	return result
}

// snapForward rounds a time forward to the nearest allowed minute within an hour.
func snapForward(t time.Time, snapMinutes []int) time.Time {
	min := t.Minute()
	// Find the smallest snap minute >= current minute
	for _, sm := range snapMinutes {
		if sm >= min {
			return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), sm, 0, 0, t.Location())
		}
	}
	// No snap minute found in this hour — go to first snap minute of next hour
	next := time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, snapMinutes[0], 0, 0, t.Location())
	return next
}
