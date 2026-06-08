package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iruoy/fylla/internal/task"
)

var (
	upnextRe            = regexp.MustCompile(`(?i)\bupnext\b`)
	nosplitRe           = regexp.MustCompile(`(?i)\bnosplit\b`)
	notBeforeRe         = regexp.MustCompile(`(?i)\bnot before \S+`)
	dueRe               = regexp.MustCompile(`(?i)\bdue \S+`)
	emptyParensRe       = regexp.MustCompile(`\(\s*\)`)
	trailingOpenParenRe = regexp.MustCompile(`\(\s*$`)
)

// EditParams holds inputs for the edit command.
type EditParams struct {
	TaskKey     string
	Provider    string
	Summary     string
	Estimate    string
	Due         string
	NoDue       bool
	Priority    string
	UpNext      bool
	NoUpNext    bool
	NoSplit     bool
	NoNoSplit   bool
	NotBefore   string
	NoNotBefore bool
	Project     string
	NoProject   bool
	Parent      string
	NoParent    bool
	Section     string
	NoSection   bool
	NoEstimate  bool
	NoPriority  bool
	SprintID    *int
	NoSprint    bool
	Source      TaskSource

	// FullState signals that Summary plus the estimate and keyword flags fully
	// describe the desired title — so the batch path can compose it and update
	// in a single request without first reading the current title. The TUI sets
	// this (its edit form always submits the complete state); CLI partial edits
	// leave it false and fall back to reading the current title.
	FullState bool
}

// EditResult holds the output of an edit operation.
type EditResult struct {
	TaskKey          string
	EstimateResult   *EstimateResult
	DueDateResult    *DueDateResult
	DueDateRemoved   bool
	PriorityResult   *PriorityResult
	UpNextSet        bool
	UpNextRemoved    bool
	NoSplitSet       bool
	NoSplitRemoved   bool
	NotBeforeSet     bool
	NotBeforeRemoved bool
	SummaryUpdated   bool
	ProjectUpdated   bool
	ProjectRemoved   bool
	ParentUpdated    bool
	ParentRemoved    bool
	SectionUpdated   bool
	SectionRemoved   bool
	EstimateRemoved  bool
	PriorityRemoved  bool
}

// RunEdit applies one or more edits to a task.
func RunEdit(ctx context.Context, p EditParams) (*EditResult, error) {
	// Resolve to the correct provider-specific source when provider is known.
	p.Source = routedSource(p.Source, p.Provider)

	// Fast path: when every requested edit maps onto a single combined provider
	// update (currently Todoist), issue one request instead of one per field.
	if bu, ok := p.Source.(BatchUpdater); ok && canBatchEdit(p) {
		return runBatchEdit(ctx, bu, p)
	}

	result := &EditResult{TaskKey: p.TaskKey}

	if p.Estimate != "" {
		r, err := RunEstimate(ctx, EstimateParams{
			TaskKey:  p.TaskKey,
			Duration: p.Estimate,
			Updater:  p.Source,
			Getter:   p.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("estimate: %w", err)
		}
		result.EstimateResult = r
	}

	if p.NoEstimate {
		if err := p.Source.UpdateEstimate(ctx, p.TaskKey, 0); err != nil {
			return nil, fmt.Errorf("remove estimate: %w", err)
		}
		result.EstimateRemoved = true
	}

	// For Kendo, due dates are stored in the title — handled in the summary section below.
	if p.Due != "" && p.Provider != "kendo" {
		if isRecurrenceDueString(p.Due) {
			dsu, ok := p.Source.(DueStringUpdater)
			if !ok {
				return nil, fmt.Errorf("due date: provider %q does not support recurring due dates", p.Provider)
			}
			if err := dsu.UpdateDueDateString(ctx, p.TaskKey, p.Due); err != nil {
				return nil, fmt.Errorf("due date: %w", err)
			}
			result.DueDateResult = &DueDateResult{TaskKey: p.TaskKey, DueString: p.Due}
		} else {
			r, err := RunDueDate(ctx, DueDateParams{
				TaskKey: p.TaskKey,
				Date:    p.Due,
				Updater: p.Source,
				Getter:  p.Source,
			})
			if err != nil {
				return nil, fmt.Errorf("due date: %w", err)
			}
			result.DueDateResult = r
		}
	}

	if p.NoDue && p.Provider != "kendo" {
		if err := p.Source.RemoveDueDate(ctx, p.TaskKey); err != nil {
			return nil, fmt.Errorf("remove due date: %w", err)
		}
		result.DueDateRemoved = true
	}

	if p.Priority != "" {
		r, err := RunPriority(ctx, PriorityParams{
			TaskKey:  p.TaskKey,
			Priority: p.Priority,
			Updater:  p.Source,
			Getter:   p.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("priority: %w", err)
		}
		result.PriorityResult = r
	}

	if p.NoPriority {
		if err := p.Source.UpdatePriority(ctx, p.TaskKey, 0); err != nil {
			return nil, fmt.Errorf("remove priority: %w", err)
		}
		result.PriorityRemoved = true
	}

	if p.Project != "" {
		if pu, ok := p.Source.(ProjectUpdater); ok {
			if err := pu.UpdateProject(ctx, p.TaskKey, p.Project); err != nil {
				return nil, fmt.Errorf("update project: %w", err)
			}
			result.ProjectUpdated = true
		}
	}

	if p.Parent != "" {
		if pu, ok := p.Source.(ParentUpdater); ok {
			if err := pu.UpdateParent(ctx, p.TaskKey, p.Parent); err != nil {
				return nil, fmt.Errorf("update parent: %w", err)
			}
			result.ParentUpdated = true
		}
	}

	if p.NoParent {
		if pu, ok := p.Source.(ParentUpdater); ok {
			if err := pu.UpdateParent(ctx, p.TaskKey, ""); err != nil {
				return nil, fmt.Errorf("remove parent: %w", err)
			}
			result.ParentRemoved = true
		}
	}

	if p.Section != "" {
		if su, ok := p.Source.(SectionUpdater); ok {
			if err := su.UpdateSection(ctx, p.TaskKey, p.Section); err != nil {
				return nil, fmt.Errorf("update section: %w", err)
			}
			result.SectionUpdated = true
		}
	}

	if p.NoSection {
		if su, ok := p.Source.(SectionUpdater); ok {
			if err := su.UpdateSection(ctx, p.TaskKey, ""); err != nil {
				return nil, fmt.Errorf("remove section: %w", err)
			}
			result.SectionRemoved = true
		}
	}

	// Summary-keyword operations (upnext, nosplit, not before, due for kendo, direct summary)
	// need to work on the same summary to avoid race conditions.
	kendoDue := (p.Due != "" || p.NoDue) && p.Provider == "kendo"
	needsSummaryUpdate := p.UpNext || p.NoUpNext || p.NoSplit || p.NoNoSplit ||
		p.NotBefore != "" || p.NoNotBefore || p.Summary != "" || kendoDue

	if needsSummaryUpdate {
		summary, err := p.Source.GetSummary(ctx, p.TaskKey)
		if err != nil {
			return nil, fmt.Errorf("get summary: %w", err)
		}

		// Strip bracket estimate (e.g. [2h]) so keyword operations don't lose it
		titleEst, summary := task.ParseTitleEstimate(summary)

		summary, changed := applySummaryKeywords(p, summary, result)

		// Due date in title (Kendo)
		if kendoDue {
			hasDue := dueRe.MatchString(summary)
			if p.Due != "" {
				d, err := ParseDate(p.Due)
				if err != nil {
					return nil, fmt.Errorf("due date: %w", err)
				}
				if hasDue {
					summary = strings.TrimSpace(dueRe.ReplaceAllString(summary, ""))
					summary = strings.Join(strings.Fields(summary), " ")
				}
				summary = strings.TrimSpace(summary) + " due " + d.Format("2006-01-02")
				changed = true
				result.DueDateResult = &DueDateResult{TaskKey: p.TaskKey, DueDate: d}
			} else if p.NoDue && hasDue {
				summary = strings.TrimSpace(dueRe.ReplaceAllString(summary, ""))
				summary = strings.Join(strings.Fields(summary), " ")
				changed = true
				result.DueDateRemoved = true
			} else if p.NoDue && !hasDue {
				result.DueDateRemoved = true
			}
		}

		if changed {
			// Normalize modifiers into parenthesized format
			summary = normalizeModifierParens(summary)
			// Re-add bracket estimate if one was present
			if titleEst > 0 {
				summary = task.SetTitleEstimate(summary, titleEst)
			}
			if err := p.Source.UpdateSummary(ctx, p.TaskKey, summary); err != nil {
				return nil, fmt.Errorf("update summary: %w", err)
			}
		}
	}

	if p.SprintID != nil || p.NoSprint {
		if su, ok := p.Source.(SprintUpdater); ok {
			if err := su.UpdateSprint(ctx, p.TaskKey, p.SprintID); err != nil {
				return nil, fmt.Errorf("update sprint: %w", err)
			}
		}
	}

	return result, nil
}

// applySummaryKeywords applies the summary/keyword edits (direct summary
// replace, upnext, nosplit, not before) to a bracket-free summary, recording
// what changed on result. It returns the new summary and whether anything
// changed. The Kendo title-due edit is handled separately by the caller.
func applySummaryKeywords(p EditParams, summary string, result *EditResult) (string, bool) {
	changed := false

	// Handle direct summary update: replace the base summary
	// (strip existing keywords, set new base, then re-apply keywords below)
	if p.Summary != "" {
		// Strip constraint keywords from current summary to get current base
		baseSummary := stripKeywords(summary)
		if baseSummary != p.Summary {
			// Replace base summary while preserving keywords
			summary = p.Summary + extractKeywordSuffix(summary)
			changed = true
			result.SummaryUpdated = true
		}
	}

	// UpNext
	hasUpNext := upnextRe.MatchString(summary)
	if p.UpNext && !hasUpNext {
		summary = strings.TrimSpace(summary) + " upnext"
		changed = true
		result.UpNextSet = true
	} else if p.UpNext && hasUpNext {
		result.UpNextSet = true
	} else if p.NoUpNext && hasUpNext {
		summary = strings.TrimSpace(upnextRe.ReplaceAllString(summary, ""))
		summary = strings.Join(strings.Fields(summary), " ")
		changed = true
		result.UpNextRemoved = true
	} else if p.NoUpNext && !hasUpNext {
		result.UpNextRemoved = true
	}

	// NoSplit
	hasNoSplit := nosplitRe.MatchString(summary)
	if p.NoSplit && !hasNoSplit {
		summary = strings.TrimSpace(summary) + " nosplit"
		changed = true
		result.NoSplitSet = true
	} else if p.NoSplit && hasNoSplit {
		result.NoSplitSet = true
	} else if p.NoNoSplit && hasNoSplit {
		summary = strings.TrimSpace(nosplitRe.ReplaceAllString(summary, ""))
		summary = strings.Join(strings.Fields(summary), " ")
		changed = true
		result.NoSplitRemoved = true
	} else if p.NoNoSplit && !hasNoSplit {
		result.NoSplitRemoved = true
	}

	// NotBefore
	hasNotBefore := notBeforeRe.MatchString(summary)
	if p.NotBefore != "" {
		if hasNotBefore {
			summary = strings.TrimSpace(notBeforeRe.ReplaceAllString(summary, ""))
			summary = strings.Join(strings.Fields(summary), " ")
		}
		summary = strings.TrimSpace(summary) + " not before " + p.NotBefore
		changed = true
		result.NotBeforeSet = true
	} else if p.NoNotBefore && hasNotBefore {
		summary = strings.TrimSpace(notBeforeRe.ReplaceAllString(summary, ""))
		summary = strings.Join(strings.Fields(summary), " ")
		changed = true
		result.NotBeforeRemoved = true
	}

	return summary, changed
}

// hasSign reports whether s is a relative adjustment (+/- prefix).
func hasSign(s string) bool {
	return strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-")
}

// batchNeedsSummary reports whether the edit touches the summary/keyword fields.
func batchNeedsSummary(p EditParams) bool {
	return p.UpNext || p.NoUpNext || p.NoSplit || p.NoNoSplit ||
		p.NotBefore != "" || p.NoNotBefore || p.Summary != ""
}

// batchEditOps counts how many independent provider round-trips the slow path
// would issue for this edit (the summary/keyword group counts as one).
func batchEditOps(p EditParams) int {
	ops := 0
	if p.Estimate != "" || p.NoEstimate {
		ops++
	}
	if p.Due != "" || p.NoDue {
		ops++
	}
	if p.Priority != "" || p.NoPriority {
		ops++
	}
	if batchNeedsSummary(p) {
		ops++
	}
	return ops
}

// canBatchEdit reports whether the edit can be applied as a single combined
// update. Move/sprint fields and relative adjustments fall through to the
// per-field slow path, as do edits that collapse fewer than two round-trips.
func canBatchEdit(p EditParams) bool {
	if p.Provider == "kendo" {
		// Kendo encodes due in the title; not representable here.
		return false
	}
	if p.Project != "" || p.NoProject || p.Section != "" || p.NoSection ||
		p.Parent != "" || p.NoParent || p.SprintID != nil || p.NoSprint {
		return false
	}
	if hasSign(p.Estimate) || hasSign(p.Priority) || (p.Due != "" && hasSign(p.Due)) {
		return false
	}
	// With full state the title is composed locally (no read), so even a single
	// field is worth the batch path — it turns the edit into one request.
	if p.FullState {
		return batchEditOps(p) >= 1
	}
	return batchEditOps(p) >= 2
}

// runBatchEdit applies the edit via a single BatchUpdate call and synthesizes
// the same EditResult the per-field path would produce.
func runBatchEdit(ctx context.Context, bu BatchUpdater, p EditParams) (*EditResult, error) {
	result := &EditResult{TaskKey: p.TaskKey}
	var u task.BatchUpdate

	// Estimate (absolute only — relative falls through in canBatchEdit).
	if p.Estimate != "" {
		d, err := ParseDuration(p.Estimate)
		if err != nil {
			return nil, fmt.Errorf("estimate: %w", err)
		}
		u.Estimate = &d
		result.EstimateResult = &EstimateResult{TaskKey: p.TaskKey, Duration: d}
	} else if p.NoEstimate {
		zero := time.Duration(0)
		u.Estimate = &zero
		result.EstimateRemoved = true
	}

	// Priority (absolute only).
	if p.Priority != "" {
		level, ok := priorityNames[strings.ToLower(strings.TrimSpace(p.Priority))]
		if !ok {
			n, err := strconv.Atoi(strings.TrimSpace(p.Priority))
			if err != nil || n < 1 || n > 5 {
				return nil, fmt.Errorf("priority: invalid priority %q (use Highest, High, Medium, Low, Lowest or 1-5)", p.Priority)
			}
			level = n
		}
		u.Priority = &level
		result.PriorityResult = &PriorityResult{TaskKey: p.TaskKey, Priority: level, Name: priorityLevelNames[level]}
	} else if p.NoPriority {
		zero := 0
		u.Priority = &zero
		result.PriorityRemoved = true
	}

	// Due.
	if p.Due != "" {
		if isRecurrenceDueString(p.Due) {
			du := p.Due
			u.DueString = &du
			result.DueDateResult = &DueDateResult{TaskKey: p.TaskKey, DueString: p.Due}
		} else {
			d, err := ParseDate(p.Due)
			if err != nil {
				return nil, fmt.Errorf("due date: %w", err)
			}
			u.DueDate = &d
			result.DueDateResult = &DueDateResult{TaskKey: p.TaskKey, DueDate: d}
		}
	} else if p.NoDue {
		u.RemoveDue = true
		result.DueDateRemoved = true
	}

	// Summary + keyword toggles.
	if batchNeedsSummary(p) {
		if p.FullState && p.Summary != "" {
			// Caller supplied the complete state: compose the title from the
			// clean base summary + keyword flags, no read required. Estimate is
			// pinned below so the provider never needs to read it either.
			newSummary, _ := applySummaryKeywords(p, p.Summary, result)
			newSummary = normalizeModifierParens(newSummary)
			u.Title = &newSummary
			result.SummaryUpdated = true
			if u.Estimate == nil {
				zero := time.Duration(0)
				u.Estimate = &zero
			}
		} else {
			// Read the current title to preserve the existing keywords the
			// caller did not resend and the [estimate] bracket.
			current, err := p.Source.GetSummary(ctx, p.TaskKey)
			if err != nil {
				return nil, fmt.Errorf("get summary: %w", err)
			}
			titleEst, base := task.ParseTitleEstimate(current)
			newSummary, changed := applySummaryKeywords(p, base, result)
			if changed {
				newSummary = normalizeModifierParens(newSummary)
			}
			u.Title = &newSummary
			if u.Estimate == nil && titleEst > 0 {
				e := titleEst
				u.Estimate = &e
			}
		}
	}

	if err := bu.BatchUpdate(ctx, p.TaskKey, u); err != nil {
		return nil, err
	}
	return result, nil
}

// isRecurrenceDueString reports whether s should be sent as a Todoist due_string
// (recurrence) rather than parsed as a date.
func isRecurrenceDueString(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "every")
}

// stripKeywords removes constraint keywords (upnext, nosplit, not before <date>, due <date>)
// and parenthesized modifier blocks from a summary.
func stripKeywords(summary string) string {
	s := upnextRe.ReplaceAllString(summary, "")
	s = nosplitRe.ReplaceAllString(s, "")
	s = notBeforeRe.ReplaceAllString(s, "")
	s = dueRe.ReplaceAllString(s, "")
	s = emptyParensRe.ReplaceAllString(s, "")
	s = trailingOpenParenRe.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// extractKeywordSuffix returns the keyword portion of a summary as a parenthesized block.
func extractKeywordSuffix(summary string) string {
	var parts []string
	if upnextRe.MatchString(summary) {
		parts = append(parts, "upnext")
	}
	if nosplitRe.MatchString(summary) {
		parts = append(parts, "nosplit")
	}
	if loc := notBeforeRe.FindString(summary); loc != "" {
		loc = strings.TrimRight(loc, ")")
		parts = append(parts, loc)
	}
	if loc := dueRe.FindString(summary); loc != "" {
		loc = strings.TrimRight(loc, ")")
		parts = append(parts, loc)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " ") + ")"
}

// normalizeModifierParens collects all bare modifiers from the summary,
// strips them, and re-adds them in a single parenthesized block.
func normalizeModifierParens(summary string) string {
	var mods []string

	if loc := dueRe.FindString(summary); loc != "" {
		loc = strings.TrimRight(loc, ")")
		mods = append(mods, loc)
		summary = dueRe.ReplaceAllString(summary, "")
	}
	if loc := notBeforeRe.FindString(summary); loc != "" {
		loc = strings.TrimRight(loc, ")")
		mods = append(mods, loc)
		summary = notBeforeRe.ReplaceAllString(summary, "")
	}
	if upnextRe.MatchString(summary) {
		mods = append(mods, "upnext")
		summary = upnextRe.ReplaceAllString(summary, "")
	}
	if nosplitRe.MatchString(summary) {
		mods = append(mods, "nosplit")
		summary = nosplitRe.ReplaceAllString(summary, "")
	}

	// Clean up any orphaned parentheses
	summary = emptyParensRe.ReplaceAllString(summary, "")
	summary = trailingOpenParenRe.ReplaceAllString(summary, "")
	summary = strings.TrimSpace(strings.Join(strings.Fields(summary), " "))

	if len(mods) > 0 {
		summary += " (" + strings.Join(mods, " ") + ")"
	}
	return summary
}

// PrintEditResult writes the edit confirmation to the given writer.
func PrintEditResult(w io.Writer, result *EditResult) {
	if result.EstimateResult != nil {
		PrintEstimateResult(w, result.EstimateResult)
	}
	if result.DueDateResult != nil {
		PrintDueDateResult(w, result.DueDateResult)
	}
	if result.DueDateRemoved {
		fmt.Fprintf(w, "Due date for %s removed\n", result.TaskKey)
	}
	if result.PriorityResult != nil {
		PrintPriorityResult(w, result.PriorityResult)
	}
	if result.SummaryUpdated {
		fmt.Fprintf(w, "%s summary updated\n", result.TaskKey)
	}
	if result.UpNextSet {
		fmt.Fprintf(w, "%s marked as up next\n", result.TaskKey)
	}
	if result.UpNextRemoved {
		fmt.Fprintf(w, "%s unmarked as up next\n", result.TaskKey)
	}
	if result.NoSplitSet {
		fmt.Fprintf(w, "%s marked as no-split\n", result.TaskKey)
	}
	if result.NoSplitRemoved {
		fmt.Fprintf(w, "%s unmarked as no-split\n", result.TaskKey)
	}
	if result.NotBeforeSet {
		fmt.Fprintf(w, "%s not-before date set\n", result.TaskKey)
	}
	if result.NotBeforeRemoved {
		fmt.Fprintf(w, "%s not-before date removed\n", result.TaskKey)
	}
	if result.ProjectUpdated {
		fmt.Fprintf(w, "%s project updated\n", result.TaskKey)
	}
	if result.ProjectRemoved {
		fmt.Fprintf(w, "%s project removed\n", result.TaskKey)
	}
	if result.ParentUpdated {
		fmt.Fprintf(w, "%s parent updated\n", result.TaskKey)
	}
	if result.ParentRemoved {
		fmt.Fprintf(w, "%s parent removed\n", result.TaskKey)
	}
	if result.SectionUpdated {
		fmt.Fprintf(w, "%s section updated\n", result.TaskKey)
	}
	if result.SectionRemoved {
		fmt.Fprintf(w, "%s section removed\n", result.TaskKey)
	}
	if result.EstimateRemoved {
		fmt.Fprintf(w, "Estimate for %s removed\n", result.TaskKey)
	}
	if result.PriorityRemoved {
		fmt.Fprintf(w, "Priority for %s removed\n", result.TaskKey)
	}
}
