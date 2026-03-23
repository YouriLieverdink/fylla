package scheduler

import (
	"fmt"
	"sort"
	"time"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
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
// Remaining holds the unscheduled portion of the estimate (equals the full
// estimate when no partial allocation was possible).
type UnscheduledTask struct {
	Task      task.Task
	Reason    string
	Remaining time.Duration
}

// AllocateConfig holds parameters for the allocation algorithm.
type AllocateConfig struct {
	MinTaskDurationMinutes int
	MaxTaskDurationMinutes int
	BufferMinutes          int
	SnapMinutes            []int
	Weights                config.WeightsConfig
	Now                    time.Time
}

// Allocate assigns sorted tasks to available free slots using a first-fit algorithm.
// Tasks are processed in order (highest priority first) and assigned to the first
// slot that fits their duration.
//
// slotsByProject maps project keys to their available slots. The empty string key ""
// holds default slots for projects without specific rules. When a task is allocated,
// that time is consumed globally across all project slot lists.
//
// When MaxTaskDurationMinutes is set, the allocator caps each chunk to that duration
// and re-queues the remainder with a halved score, allowing other tasks to interleave.
func Allocate(tasks []ScoredTask, slotsByProject map[string][]calendar.Slot, cfg AllocateConfig) ([]Allocation, []UnscheduledTask) {
	minDur := time.Duration(cfg.MinTaskDurationMinutes) * time.Minute
	buffer := time.Duration(cfg.BufferMinutes) * time.Minute
	var maxDur time.Duration
	if cfg.MaxTaskDurationMinutes > 0 {
		maxDur = time.Duration(cfg.MaxTaskDurationMinutes) * time.Minute
	}

	queue := make([]ScoredTask, len(tasks))
	copy(queue, tasks)

	var consumed []allocRange
	var allocations []Allocation
	var unscheduled []UnscheduledTask
	taskAllocs := map[string][]int{} // key → indices into allocations

	for len(queue) > 0 {
		st := queue[0]
		queue = queue[1:]

		estimate := st.Task.RemainingEstimate
		if estimate <= 0 {
			estimate = defaultEstimate
		}

		slots := projectSlots(slotsByProject, st.Task.Project)
		available := availableSlots(slots, consumed, 0)
		available = snapSlotStarts(available, cfg.SnapMinutes, 0)

		hadSlots := len(available) > 0
		if st.Task.NotBefore != nil {
			available = filterSlotsNotBefore(available, *st.Task.NotBefore)
		}

		if len(available) == 0 {
			reason := "no available slots"
			if hadSlots && st.Task.NotBefore != nil {
				reason = "starts after scheduling window"
			}
			unscheduled = append(unscheduled, UnscheduledTask{
				Task:      st.Task,
				Reason:    reason,
				Remaining: estimate,
			})
			continue
		}

		if maxDur > 0 {
			placed := false
			for _, slot := range available {
				slotDur := slot.End.Sub(slot.Start)

				effectiveDur := slotDur
				if effectiveDur > maxDur {
					effectiveDur = maxDur
				}

				if effectiveDur < minDur {
					continue
				}

				if estimate <= effectiveDur {
					alloc := Allocation{Task: st.Task, Start: slot.Start, End: slot.Start.Add(estimate)}
					idx := len(allocations)
					allocations = append(allocations, alloc)
					consumed = append(consumed, allocRange{start: alloc.Start, end: alloc.End.Add(buffer)})
					taskAllocs[st.Task.Key] = append(taskAllocs[st.Task.Key], idx)
					placed = true
					break
				}

				if st.Task.NoSplit {
					continue
				}

				alloc := Allocation{Task: st.Task, Start: slot.Start, End: slot.Start.Add(effectiveDur)}
				idx := len(allocations)
				allocations = append(allocations, alloc)
				consumed = append(consumed, allocRange{start: alloc.Start, end: alloc.End.Add(buffer)})
				taskAllocs[st.Task.Key] = append(taskAllocs[st.Task.Key], idx)

				remainder := st
				remainder.Task.RemainingEstimate = estimate - effectiveDur
				remainder.Score = recalcScore(remainder.Task, cfg.Weights, cfg.Now)
				queue = insertSorted(queue, remainder)
				placed = true
				break
			}

			if !placed {
				reason := "not enough time"
				if st.Task.NoSplit {
					reason = "no slot large enough (no-split)"
				}
				unscheduled = append(unscheduled, UnscheduledTask{
					Task:      st.Task,
					Reason:    reason,
					Remaining: estimate,
				})
			}
		} else {
			remaining := estimate
			var localAllocs []Allocation

			for _, slot := range available {
				if remaining <= 0 {
					break
				}

				slotDur := slot.End.Sub(slot.Start)

				if slotDur < minDur && remaining == estimate {
					continue
				}

				if remaining <= slotDur {
					alloc := Allocation{Task: st.Task, Start: slot.Start, End: slot.Start.Add(remaining)}
					localAllocs = append(localAllocs, alloc)
					consumed = append(consumed, allocRange{start: alloc.Start, end: alloc.End.Add(buffer)})
					remaining = 0
					break
				}

				if st.Task.NoSplit {
					continue
				}

				alloc := Allocation{Task: st.Task, Start: slot.Start, End: slot.End}
				localAllocs = append(localAllocs, alloc)
				consumed = append(consumed, allocRange{start: slot.Start, end: slot.End.Add(buffer)})
				remaining -= slotDur
			}

			if remaining > 0 {
				reason := "not enough time"
				if st.Task.NoSplit {
					reason = "no slot large enough (no-split)"
				}
				unscheduled = append(unscheduled, UnscheduledTask{
					Task:      st.Task,
					Reason:    reason,
					Remaining: remaining,
				})
				if len(localAllocs) == 0 {
					continue
				}
			}

			if len(localAllocs) > 1 {
				total := len(localAllocs)
				for i := range localAllocs {
					localAllocs[i].Task.Summary = fmt.Sprintf("%s (%d/%d)", localAllocs[i].Task.Summary, i+1, total)
				}
			}

			if st.Task.DueDate != nil && len(localAllocs) > 0 {
				lastEnd := localAllocs[len(localAllocs)-1].End
				dueEnd := time.Date(st.Task.DueDate.Year(), st.Task.DueDate.Month(), st.Task.DueDate.Day()+1, 0, 0, 0, 0, st.Task.DueDate.Location())
				if lastEnd.After(dueEnd) {
					for i := range localAllocs {
						localAllocs[i].AtRisk = true
					}
				}
			}

			allocations = append(allocations, localAllocs...)
		}
	}

	// When maxDur is active, apply split labels and at-risk detection across interleaved allocations
	if maxDur > 0 {
		for key, indices := range taskAllocs {
			if len(indices) > 1 {
				for i, idx := range indices {
					allocations[idx].Task.Summary = fmt.Sprintf("%s (%d/%d)",
						allocations[idx].Task.Summary, i+1, len(indices))
				}
			}

			// At-risk detection on the last allocation for this task
			lastIdx := indices[len(indices)-1]
			t := allocations[lastIdx].Task
			if t.DueDate != nil {
				lastEnd := allocations[lastIdx].End
				dueEnd := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day()+1, 0, 0, 0, 0, t.DueDate.Location())
				if lastEnd.After(dueEnd) {
					for _, idx := range indices {
						allocations[idx].AtRisk = true
					}
				}
			}
			_ = key
		}
	}

	return allocations, unscheduled
}

// recalcScore recomputes the composite score for a task remainder after
// splitting. It halves the priority and age components to encourage
// interleaving, while preserving due-date urgency (due-date score and
// crunch boost) at full strength so tasks aren't pushed past their deadline.
func recalcScore(t task.Task, w config.WeightsConfig, now time.Time) float64 {
	base := w.Priority*PriorityScore(t.Priority) + w.Age*AgeScore(t.Created, now)
	urgent := w.DueDate*DueDateScore(t.DueDate, now) + CrunchBoost(t.DueDate, now)
	estimate := w.Estimate * EstimateScore(t.RemainingEstimate)

	score := base*0.5 + urgent + estimate
	if t.UpNext {
		score += w.UpNext
	} else {
		score *= NotBeforePenalty(t.NotBefore, now)
	}
	return score
}

// insertSorted inserts a ScoredTask into a queue sorted by descending score.
func insertSorted(queue []ScoredTask, st ScoredTask) []ScoredTask {
	i := sort.Search(len(queue), func(i int) bool {
		return queue[i].Score < st.Score
	})
	queue = append(queue, ScoredTask{})
	copy(queue[i+1:], queue[i:])
	queue[i] = st
	return queue
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
