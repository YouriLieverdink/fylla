package task

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tj/go-naturaldate"
)

var (
	estimateRe = regexp.MustCompile(`\[(\d+h)?(\d+m)?\]`)
	dueDateRe  = regexp.MustCompile(`\{(\d{4}-\d{2}-\d{2})\}`)
	spacesRe   = regexp.MustCompile(`\s{2,}`)
	attrsRe    = regexp.MustCompile(`\(([^)]+)\)`)
	priorityRe = regexp.MustCompile(`(?i)\bpriority:(\S+)`)
	descRe     = regexp.MustCompile(`(?i)\bdesc:(.+)`)
	isoDateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

var priorityAliases = map[string]string{
	"critical": "Highest", "p1": "Highest", "highest": "Highest",
	"high": "High", "p2": "High",
	"medium": "Medium", "p3": "Medium",
	"low": "Low", "p4": "Low",
	"lowest": "Lowest", "p5": "Lowest",
}

// ParsedInput holds the result of parsing a free-form task input string.
type ParsedInput struct {
	Summary     string
	Estimate    time.Duration
	DueDate     *time.Time
	Priority    string
	Description string
}

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

// ParseInput parses a free-form task input string, extracting inline attributes.
// Attributes are placed inside parentheses:
//
//	Write the docs [30m] (due Friday priority:critical not before Monday upnext nosplit)
//
// Only due, priority, and desc are extracted. Everything else (not before, upnext,
// nosplit, etc.) is left in the title as scheduling hints.
// The ref time is used as the reference point for natural language date parsing.
func ParseInput(text string, ref time.Time) ParsedInput {
	var result ParsedInput

	// 1. Extract [duration] from the full text
	result.Estimate, text = ParseTitleEstimate(text)

	// 2. Extract (...) attributes block
	if m := attrsRe.FindStringSubmatch(text); m != nil {
		attrs := m[1]
		text = strings.TrimSpace(spacesRe.ReplaceAllString(attrsRe.ReplaceAllString(text, ""), " "))

		// Parse extracted attributes; remaining text goes back into the title
		var remaining string
		remaining = parseAttrs(&result, attrs, ref)
		if remaining != "" {
			text = strings.TrimSpace(text + " " + remaining)
		}
	}

	// 3. Remaining text = summary
	result.Summary = strings.TrimSpace(spacesRe.ReplaceAllString(text, " "))

	return result
}

// parseAttrs parses the content inside the (...) attributes block.
// Extracts due, priority:, and desc:. Returns any remaining text (not before,
// upnext, nosplit, etc.) to be appended back to the title.
func parseAttrs(result *ParsedInput, attrs string, ref time.Time) string {
	// Extract priority:X
	if m := priorityRe.FindStringSubmatch(attrs); m != nil {
		if alias, ok := priorityAliases[strings.ToLower(m[1])]; ok {
			result.Priority = alias
		}
		attrs = strings.TrimSpace(priorityRe.ReplaceAllString(attrs, ""))
	}

	// Extract desc:... (greedy, must be extracted before due to avoid consuming date text)
	if m := descRe.FindStringSubmatch(attrs); m != nil {
		result.Description = strings.TrimSpace(m[1])
		attrs = strings.TrimSpace(descRe.ReplaceAllString(attrs, ""))
	}

	// Extract "due <date>" — try progressively longer word sequences after "due"
	attrs, result.DueDate = extractDueClause(attrs, ref)

	return strings.TrimSpace(attrs)
}

// extractDueClause extracts "due <date>" from the attributes text.
// It tries progressively longer word sequences to find the shortest valid date.
func extractDueClause(text string, ref time.Time) (string, *time.Time) {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "due ")
	if idx == -1 {
		return text, nil
	}
	// Verify word boundary at start
	if idx > 0 && text[idx-1] != ' ' {
		return text, nil
	}

	afterKeyword := idx + 4 // len("due ")
	rest := strings.TrimSpace(text[afterKeyword:])
	if rest == "" {
		return text, nil
	}

	words := strings.Fields(rest)

	// Try progressively longer word sequences (1 word, 2 words, etc.)
	for n := 1; n <= len(words); n++ {
		candidate := strings.Join(words[:n], " ")
		parsed, err := parseNaturalDate(candidate, ref)
		if err != nil {
			continue
		}
		if parsed.Equal(ref) {
			continue
		}
		// Found a valid date — remove "due" + consumed words
		remaining := strings.Join(words[n:], " ")
		cleaned := text[:idx] + remaining
		return strings.TrimSpace(spacesRe.ReplaceAllString(cleaned, " ")), &parsed
	}

	return text, nil
}

// parseNaturalDate tries ISO format first, then falls back to natural language parsing.
func parseNaturalDate(s string, ref time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if isoDateRe.MatchString(s) {
		return time.Parse("2006-01-02", s)
	}
	t, err := naturaldate.Parse(s, ref, naturaldate.WithDirection(naturaldate.Future))
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse date %q: %w", s, err)
	}
	return t, nil
}

