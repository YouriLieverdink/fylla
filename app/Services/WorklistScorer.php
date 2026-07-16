<?php

namespace App\Services;

use App\Models\Draft;
use App\Models\Issue;
use App\Models\PullRequest;
use Carbon\Carbon;

/**
 * Ranks worklist items by a weighted composite score (ADR-0013), ported from
 * the retired Go sorter (internal/scheduler/sorter.go) minus the dropped Age
 * component. Pure: score is a function of already-synced/local fields and `now`,
 * recomputed every render.
 *
 *   score = 0.45·Priority + 0.30·Due + 0.15·Estimate + Crunch + TypeBonus
 *   then:  up_next → +50   else → ×NotBefore(0.2–1.0)
 *
 * A PR carries none of these fields, so it is fed to the same math as a High
 * issue due `opened_at + grace` — high on open, climbing to the top once past
 * the grace. Do NOT collapse this to a flat constant: the escalation is the point.
 */
class WorklistScorer
{
    private const W_PRIORITY = 0.45;

    private const W_DUE = 0.30;

    private const W_ESTIMATE = 0.15;

    private const UP_NEXT_BOOST = 50;

    /** Priority 1–5 → 0-100. Unset/out-of-range → medium (index 2). */
    private const PRIORITY_LEVELS = [100.0, 80.0, 60.0, 40.0, 20.0];

    /** Kendo priority label → 1–5 (mirrors App\Kendo\Client::PRIORITIES order). */
    public const PRIORITY_RANK = ['Highest' => 1, 'High' => 2, 'Medium' => 3, 'Low' => 4, 'Lowest' => 5];

    /** Reserved per-type flat bonus hook (ADR-0013). Empty until a need appears. */
    private const TYPE_BONUS = [];

    /** PR synthetic-due tuning (ADR-0013): review grace, base priority. */
    private const PR_GRACE_DAYS = 1;

    private const PR_PRIORITY = 2; // High

    /**
     * Score an issue. Returns ['score' => float, 'reason' => string].
     */
    public function scoreIssue(Issue $issue, Carbon $now): array
    {
        $priority = self::PRIORITY_RANK[$issue->priority] ?? null;
        $due = $issue->due_date;
        $mins = $issue->remaining_minutes;
        $notBefore = $issue->not_before;
        $upNext = (bool) $issue->up_next;

        $score = $this->composite($priority, $due, $mins, $issue->type, $upNext, $notBefore, $now);
        $reason = $this->issueReason($issue->priority, $due, $mins, $notBefore, $upNext, $now);

        return ['score' => $score, 'reason' => $reason];
    }

    /**
     * Score a draft (ADR-0012): same math as an issue, but it carries no
     * estimate or type, so the quick-win and type-bonus components contribute 0.
     * Priority defaults to Medium at the column level.
     */
    public function scoreDraft(Draft $draft, Carbon $now): array
    {
        $priority = self::PRIORITY_RANK[$draft->priority] ?? null;

        $score = $this->composite($priority, $draft->due_date, null, null, (bool) $draft->up_next, $draft->not_before, $now);
        $reason = $this->issueReason($draft->priority, $draft->due_date, null, $draft->not_before, (bool) $draft->up_next, $now);

        return ['score' => $score, 'reason' => $reason];
    }

    /**
     * Score a PR via a synthetic due date (ADR-0013): High priority, due
     * `opened_at + grace`, no estimate, never up_next/not_before.
     */
    public function scorePr(PullRequest $pr, Carbon $now): array
    {
        $opened = $pr->opened_at ?? $now;
        $due = $opened->copy()->addDays(self::PR_GRACE_DAYS);

        $score = $this->composite(self::PR_PRIORITY, $due, null, null, false, null, $now);

        return ['score' => $score, 'reason' => self::ageLabel($opened, $now)];
    }

    private function composite(?int $priority, ?Carbon $due, ?int $mins, ?string $type, bool $upNext, ?Carbon $notBefore, Carbon $now): float
    {
        $score = self::W_PRIORITY * self::priorityScore($priority)
            + self::W_DUE * self::dueDateScore($due, $now)
            + self::W_ESTIMATE * self::estimateScore($mins)
            + self::crunchBoost($due, $now)
            + ($type !== null ? (self::TYPE_BONUS[$type] ?? 0) : 0);

        if ($upNext) {
            $score += self::UP_NEXT_BOOST;
        } else {
            $score *= self::notBeforePenalty($notBefore, $now);
        }

        return $score;
    }

    public static function priorityScore(?int $priority): float
    {
        if ($priority === null || $priority < 1 || $priority > 5) {
            return self::PRIORITY_LEVELS[2];
        }

        return self::PRIORITY_LEVELS[$priority - 1];
    }

    /** Days until due: due today=100, 30+ days out=0, linear; null=0, overdue=100. */
    public static function dueDateScore(?Carbon $due, Carbon $now): float
    {
        if ($due === null) {
            return 0.0;
        }
        $days = self::days($due, $now);
        if ($days <= 0) {
            return 100.0;
        }
        if ($days >= 30) {
            return 0.0;
        }

        return 100 * (1 - $days / 30);
    }

    /** Quick wins: <8h inverse, ≥8h or unset=0. */
    public static function estimateScore(?int $minutes): float
    {
        if ($minutes === null || $minutes <= 0) {
            return 0.0;
        }
        $hours = $minutes / 60;
        if ($hours >= 8) {
            return 0.0;
        }

        return 100 * (1 - $hours / 8);
    }

    /** +20 for due ≤3 days (linear), overdue=full 20; null=0. */
    public static function crunchBoost(?Carbon $due, Carbon $now): float
    {
        if ($due === null) {
            return 0.0;
        }
        $days = self::days($due, $now);
        if ($days > 3) {
            return 0.0;
        }
        if ($days <= 0) {
            return 20.0;
        }

        return 20 * (1 - $days / 3);
    }

    /** Multiplier 0.2–1.0: actionable now=1.0, 7+ days out=0.2, linear between. */
    public static function notBeforePenalty(?Carbon $notBefore, Carbon $now): float
    {
        if ($notBefore === null || ! $notBefore->isAfter($now)) {
            return 1.0;
        }
        $days = self::days($notBefore, $now);
        if ($days >= 7) {
            return 0.2;
        }

        return 1.0 - 0.8 * $days / 7;
    }

    /** Signed fractional days from now to a date. */
    private static function days(Carbon $date, Carbon $now): float
    {
        return ($date->getTimestamp() - $now->getTimestamp()) / 86400;
    }

    /** How long ago a PR opened, in the coarsest unit ≥ 1: days, then hours, then minutes. */
    private static function ageLabel(Carbon $opened, Carbon $now): string
    {
        $secs = max(0, $now->getTimestamp() - $opened->getTimestamp());
        if (($days = intdiv($secs, 86400)) >= 1) {
            return $days === 1 ? '1 day old' : "{$days} days old";
        }
        if (($hours = intdiv($secs, 3600)) >= 1) {
            return $hours === 1 ? '1 hour old' : "{$hours} hours old";
        }
        $mins = intdiv($secs, 60);
        if ($mins === 0) {
            return 'now';
        }

        return $mins === 1 ? '1 minute old' : "{$mins} minutes old";
    }

    /** Single dominant reason: pinned > overdue > due-soon > deferred > quick-win > priority. */
    private function issueReason(?string $label, ?Carbon $due, ?int $mins, ?Carbon $notBefore, bool $upNext, Carbon $now): string
    {
        if ($upNext) {
            return 'pinned';
        }
        if ($due !== null && self::days($due, $now) <= 3) {
            return self::dueReason($due, $now);
        }
        if ($notBefore !== null && $notBefore->isAfter($now)) {
            return self::notBeforeReason($notBefore, $now);
        }
        if ($mins !== null && $mins > 0 && $mins < 8 * 60) {
            return 'quick win';
        }

        return $label ?? 'Medium';
    }

    private static function dueReason(Carbon $due, Carbon $now): string
    {
        $days = (int) ceil(self::days($due, $now));
        if ($days < 0) {
            return abs($days).' days overdue';
        }
        if ($days === 0) {
            return 'due today';
        }
        if ($days === 1) {
            return 'due tomorrow';
        }

        return "due in {$days} days";
    }

    private static function notBeforeReason(Carbon $notBefore, Carbon $now): string
    {
        $days = (int) ceil(self::days($notBefore, $now));
        if ($days === 1) {
            return 'starts tomorrow';
        }

        return "starts in {$days} days";
    }
}
