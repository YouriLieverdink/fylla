package calendar

import (
	"fmt"
	"time"

	"github.com/iruoy/fylla/internal/config"
)

// Slot represents a free time slot available for scheduling.
type Slot struct {
	Start time.Time
	End   time.Time
}

// Duration returns the length of the slot.
func (s Slot) Duration() time.Duration {
	return s.End.Sub(s.Start)
}

// FindFreeSlots finds available scheduling slots within the given time range.
// It respects business hours, work days, OOO events, buffer between events,
// and the current time (no scheduling in the past).
//
// Parameters:
//   - now: current time; today's slots start from max(now, business hours start)
//   - rangeStart: beginning of the scheduling window
//   - rangeEnd: end of the scheduling window
//   - events: existing calendar events (meetings, OOO, etc.)
//   - hours: business hours configuration to apply
//   - bufferMinutes: gap to leave between busy events and free slots
//   - minDurationMinutes: minimum slot size to return
//   - travelBufferMinutes: extra buffer before events that have a location
func FindFreeSlots(
	now time.Time,
	rangeStart, rangeEnd time.Time,
	events []Event,
	hours []config.BusinessHoursConfig,
	bufferMinutes int,
	minDurationMinutes int,
	snapMinutes []int,
	travelBufferMinutes int,
) ([]Slot, error) {
	type parsedWindow struct {
		start      timeOfDay
		end        timeOfDay
		workDaySet map[time.Weekday]bool
	}

	var windows []parsedWindow
	for i, h := range hours {
		s, err := parseTimeOfDay(h.Start)
		if err != nil {
			return nil, fmt.Errorf("parse business hours[%d] start: %w", i, err)
		}
		e, err := parseTimeOfDay(h.End)
		if err != nil {
			return nil, fmt.Errorf("parse business hours[%d] end: %w", i, err)
		}
		wds := make(map[time.Weekday]bool, len(h.WorkDays))
		for _, d := range h.WorkDays {
			wds[time.Weekday(d%7)] = true
		}
		windows = append(windows, parsedWindow{start: s, end: e, workDaySet: wds})
	}

	oooRanges := collectOOORanges(events)
	busy := collectBusyRanges(events, bufferMinutes, travelBufferMinutes)

	buffer := time.Duration(bufferMinutes) * time.Minute
	minDur := time.Duration(minDurationMinutes) * time.Minute

	var slots []Slot

	current := rangeStart
	for !current.After(rangeEnd) && !dateOf(current).After(dateOf(rangeEnd)) {
		loc := current.Location()
		y, m, d := current.Date()

		for _, w := range windows {
			if !w.workDaySet[current.Weekday()] {
				continue
			}

			windowStart := time.Date(y, m, d, w.start.hour, w.start.minute, 0, 0, loc)
			windowEnd := time.Date(y, m, d, w.end.hour, w.end.minute, 0, 0, loc)

			// SLOT-006: today's slots start from now (plus buffer)
			if sameDate(current, now) {
				earliest := now.Add(buffer)
				if earliest.After(windowStart) {
					windowStart = earliest
				}
			}

			if windowStart.After(windowEnd) || windowStart.Equal(windowEnd) {
				continue
			}

			// SLOT-005/007: block OOO ranges
			if isFullyBlocked(windowStart, windowEnd, oooRanges) {
				continue
			}

			daySlots := subtractBusy(windowStart, windowEnd, busy, oooRanges, minDur)
			slots = append(slots, daySlots...)
		}

		current = nextDay(current)
	}

	slots = snapSlotStarts(slots, snapMinutes, minDur)

	return slots, nil
}

// snapSlotStarts snaps each slot's start time forward to the nearest allowed minute.
func snapSlotStarts(slots []Slot, snapMinutes []int, minDur time.Duration) []Slot {
	if len(snapMinutes) == 0 {
		return slots
	}
	var result []Slot
	for _, s := range slots {
		snapped := snapForward(s.Start, snapMinutes)
		if snapped.Before(s.End) && s.End.Sub(snapped) >= minDur {
			result = append(result, Slot{Start: snapped, End: s.End})
		}
	}
	return result
}

func snapForward(t time.Time, snapMinutes []int) time.Time {
	min := t.Minute()
	for _, sm := range snapMinutes {
		if sm >= min {
			return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), sm, 0, 0, t.Location())
		}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, snapMinutes[0], 0, 0, t.Location())
}

type timeOfDay struct {
	hour, minute int
}

func parseTimeOfDay(s string) (timeOfDay, error) {
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return timeOfDay{}, fmt.Errorf("invalid time format %q, expected HH:MM", s)
	}
	return timeOfDay{hour: h, minute: m}, nil
}

type timeRange struct {
	start, end time.Time
}

func collectOOORanges(events []Event) []timeRange {
	var ranges []timeRange
	for _, e := range events {
		if e.IsOOO() {
			ranges = append(ranges, timeRange{start: e.Start, end: e.End})
		}
	}
	return ranges
}

func collectBusyRanges(events []Event, bufferMinutes, travelBufferMinutes int) []timeRange {
	buffer := time.Duration(bufferMinutes) * time.Minute
	travelBuffer := time.Duration(travelBufferMinutes) * time.Minute
	var ranges []timeRange
	for _, e := range events {
		if e.IsOOO() {
			continue // OOO handled separately
		}
		start := e.Start
		if e.Location != "" {
			start = start.Add(-travelBuffer)
		}
		ranges = append(ranges, timeRange{
			start: start,
			end:   e.End.Add(buffer),
		})
	}
	return ranges
}

func isFullyBlocked(start, end time.Time, oooRanges []timeRange) bool {
	for _, ooo := range oooRanges {
		if !ooo.start.After(start) && !ooo.end.Before(end) {
			return true
		}
	}
	return false
}

func subtractBusy(windowStart, windowEnd time.Time, busy, oooRanges []timeRange, minDur time.Duration) []Slot {
	// Merge all blocking ranges
	var blocks []timeRange
	blocks = append(blocks, busy...)
	blocks = append(blocks, oooRanges...)

	// Filter to ranges that overlap our window
	var relevant []timeRange
	for _, b := range blocks {
		if b.start.Before(windowEnd) && b.end.After(windowStart) {
			relevant = append(relevant, b)
		}
	}

	// Sort by start time
	sortRanges(relevant)

	var slots []Slot
	cursor := windowStart

	for _, b := range relevant {
		blockStart := b.start
		blockEnd := b.end

		if blockStart.After(cursor) {
			slotEnd := blockStart
			if slotEnd.After(windowEnd) {
				slotEnd = windowEnd
			}
			if slotEnd.Sub(cursor) >= minDur {
				slots = append(slots, Slot{Start: cursor, End: slotEnd})
			}
		}

		if blockEnd.After(cursor) {
			cursor = blockEnd
		}
	}

	// Remaining time after last block
	if cursor.Before(windowEnd) && windowEnd.Sub(cursor) >= minDur {
		slots = append(slots, Slot{Start: cursor, End: windowEnd})
	}

	return slots
}

func sortRanges(ranges []timeRange) {
	for i := 1; i < len(ranges); i++ {
		for j := i; j > 0 && ranges[j].start.Before(ranges[j-1].start); j-- {
			ranges[j], ranges[j-1] = ranges[j-1], ranges[j]
		}
	}
}

func nextDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func dateOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
