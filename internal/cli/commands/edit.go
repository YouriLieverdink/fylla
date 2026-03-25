package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

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
	Parent      string
	NoParent    bool
	Section     string
	NoSection   bool
	NoEstimate  bool
	NoPriority  bool
	Source      TaskSource
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
	source := p.Source
	if p.Provider != "" {
		if ms, ok := source.(*MultiTaskSource); ok {
			if src, ok := ms.RouteToProvider(p.Provider); ok {
				source = src
			}
		}
	}
	p.Source = source

	result := &EditResult{TaskKey: p.TaskKey}

	if p.Estimate != "" {
		r, err := RunEstimate(ctx, EstimateParams{
			TaskKey:  p.TaskKey,
			Duration: p.Estimate,
			Jira:     p.Source,
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
		r, err := RunDueDate(ctx, DueDateParams{
			TaskKey: p.TaskKey,
			Date:    p.Due,
			Jira:    p.Source,
			Getter:  p.Source,
		})
		if err != nil {
			return nil, fmt.Errorf("due date: %w", err)
		}
		result.DueDateResult = r
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

	if p.Parent != "" {
		var pu ParentUpdater
		if u, ok := p.Source.(ParentUpdater); ok {
			pu = u
		} else if ms, ok := p.Source.(*MultiTaskSource); ok {
			routed := ms.routeTo(p.TaskKey)
			if u, ok := routed.(ParentUpdater); ok {
				pu = u
			}
		}
		if pu != nil {
			if err := pu.UpdateParent(ctx, p.TaskKey, p.Parent); err != nil {
				return nil, fmt.Errorf("update parent: %w", err)
			}
			result.ParentUpdated = true
		}
	}

	if p.NoParent {
		var pu ParentUpdater
		if u, ok := p.Source.(ParentUpdater); ok {
			pu = u
		} else if ms, ok := p.Source.(*MultiTaskSource); ok {
			routed := ms.routeTo(p.TaskKey)
			if u, ok := routed.(ParentUpdater); ok {
				pu = u
			}
		}
		if pu != nil {
			if err := pu.UpdateParent(ctx, p.TaskKey, ""); err != nil {
				return nil, fmt.Errorf("remove parent: %w", err)
			}
			result.ParentRemoved = true
		}
	}

	if p.Section != "" {
		var su SectionUpdater
		if u, ok := p.Source.(SectionUpdater); ok {
			su = u
		} else if ms, ok := p.Source.(*MultiTaskSource); ok {
			routed := ms.routeTo(p.TaskKey)
			if u, ok := routed.(SectionUpdater); ok {
				su = u
			}
		}
		if su != nil {
			if err := su.UpdateSection(ctx, p.TaskKey, p.Section); err != nil {
				return nil, fmt.Errorf("update section: %w", err)
			}
			result.SectionUpdated = true
		}
	}

	if p.NoSection {
		var su SectionUpdater
		if u, ok := p.Source.(SectionUpdater); ok {
			su = u
		} else if ms, ok := p.Source.(*MultiTaskSource); ok {
			routed := ms.routeTo(p.TaskKey)
			if u, ok := routed.(SectionUpdater); ok {
				su = u
			}
		}
		if su != nil {
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

	return result, nil
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
