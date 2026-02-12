package task

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	estimateRe = regexp.MustCompile(`\[(\d+h)?(\d+m)?\]`)
	dueDateRe  = regexp.MustCompile(`\{(\d{4}-\d{2}-\d{2})\}`)
	spacesRe   = regexp.MustCompile(`\s{2,}`)
)

// ParseTitleEstimate extracts a duration like [2h], [30m], or [1h30m] from text.
// Returns the parsed duration and the text with the match removed.
// Returns 0 and the original text if no match is found.
func ParseTitleEstimate(text string) (time.Duration, string) {
	match := estimateRe.FindStringSubmatch(text)
	if match == nil {
		return 0, text
	}
	if match[1] == "" && match[2] == "" {
		return 0, text
	}

	var d time.Duration
	if match[1] != "" {
		h, _ := strconv.Atoi(strings.TrimSuffix(match[1], "h"))
		d += time.Duration(h) * time.Hour
	}
	if match[2] != "" {
		m, _ := strconv.Atoi(strings.TrimSuffix(match[2], "m"))
		d += time.Duration(m) * time.Minute
	}

	cleaned := strings.TrimSpace(spacesRe.ReplaceAllString(estimateRe.ReplaceAllString(text, ""), " "))
	return d, cleaned
}

// ParseTitleDueDate extracts a date like {2025-02-15} from text.
// Returns the parsed date and the text with the match removed.
// Returns nil and the original text if no match is found.
func ParseTitleDueDate(text string) (*time.Time, string) {
	match := dueDateRe.FindStringSubmatch(text)
	if match == nil {
		return nil, text
	}

	t, err := time.Parse("2006-01-02", match[1])
	if err != nil {
		return nil, text
	}

	cleaned := strings.TrimSpace(spacesRe.ReplaceAllString(dueDateRe.ReplaceAllString(text, ""), " "))
	return &t, cleaned
}

// SetTitleEstimate replaces or appends an estimate bracket in the text.
func SetTitleEstimate(text string, d time.Duration) string {
	bracket := formatBracketDuration(d)
	if estimateRe.MatchString(text) {
		return strings.TrimSpace(estimateRe.ReplaceAllString(text, bracket))
	}
	return text + " " + bracket
}

func formatBracketDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return "[" + strconv.Itoa(h) + "h" + strconv.Itoa(m) + "m]"
	}
	if h > 0 {
		return "[" + strconv.Itoa(h) + "h]"
	}
	return "[" + strconv.Itoa(m) + "m]"
}
