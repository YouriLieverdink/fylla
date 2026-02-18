package prutil

import "time"

// EstimateFromLines derives a review time estimate from the total lines changed
// in a pull request (additions + deletions).
func EstimateFromLines(added, removed int) time.Duration {
	total := added + removed
	switch {
	case total < 50:
		return 15 * time.Minute
	case total < 200:
		return 30 * time.Minute
	case total < 500:
		return 45 * time.Minute
	case total < 1000:
		return 1 * time.Hour
	default:
		return 90 * time.Minute
	}
}
