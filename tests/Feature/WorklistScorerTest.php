<?php

namespace Tests\Feature;

use App\Models\Issue;
use App\Models\PullRequest;
use App\Services\WorklistScorer;
use Carbon\Carbon;
use Tests\TestCase;

/**
 * Ported from the retired Go sorter (internal/scheduler/sorter_test.go), minus
 * the dropped Age component (ADR-0013). Component math is exact; orderings prove
 * the composite, the up_next band, not_before penalty, and PR synthetic-due.
 */
class WorklistScorerTest extends TestCase
{
    private WorklistScorer $scorer;

    private Carbon $now;

    protected function setUp(): void
    {
        parent::setUp();
        $this->scorer = new WorklistScorer;
        $this->now = Carbon::parse('2025-06-15 09:00:00', 'UTC');
    }

    private function issue(array $attrs): Issue
    {
        return new Issue($attrs);
    }

    private function scoreOf(array $attrs): float
    {
        return $this->scorer->scoreIssue($this->issue($attrs), $this->now)['score'];
    }

    // --- Priority (SORT006) ---

    public function test_priority_levels(): void
    {
        $this->assertSame(100.0, WorklistScorer::priorityScore(1));
        $this->assertSame(80.0, WorklistScorer::priorityScore(2));
        $this->assertSame(60.0, WorklistScorer::priorityScore(3));
        $this->assertSame(40.0, WorklistScorer::priorityScore(4));
        $this->assertSame(20.0, WorklistScorer::priorityScore(5));
    }

    public function test_unset_priority_defaults_to_medium(): void
    {
        $this->assertSame(60.0, WorklistScorer::priorityScore(null));
        $this->assertSame(60.0, WorklistScorer::priorityScore(0));
        $this->assertSame(60.0, WorklistScorer::priorityScore(9));
    }

    public function test_priority_label_maps_and_orders(): void
    {
        $high = $this->scoreOf(['priority' => 'Highest']);
        $low = $this->scoreOf(['priority' => 'Lowest']);
        $this->assertGreaterThan($low, $high);
        // Priority 1 = 100 * 0.45 = 45 (no other components).
        $this->assertEqualsWithDelta(45.0, $high, 0.01);
    }

    // --- Due date (SORT007) ---

    public function test_due_date_scoring(): void
    {
        $this->assertSame(100.0, WorklistScorer::dueDateScore($this->now->copy(), $this->now));
        $this->assertSame(0.0, WorklistScorer::dueDateScore(null, $this->now));
        $this->assertSame(0.0, WorklistScorer::dueDateScore($this->now->copy()->addDays(31), $this->now));
        $this->assertEqualsWithDelta(50.0, WorklistScorer::dueDateScore($this->now->copy()->addDays(15), $this->now), 0.1);
        // Overdue keeps climbing on the same slope, no ceiling.
        $this->assertEqualsWithDelta(106.67, WorklistScorer::dueDateScore($this->now->copy()->subDays(2), $this->now), 0.01);
    }

    public function test_older_overdue_outranks_newer_overdue(): void
    {
        $recent = WorklistScorer::dueDateScore($this->now->copy()->subDays(3), $this->now);
        $stale = WorklistScorer::dueDateScore($this->now->copy()->subDays(73), $this->now);

        $this->assertGreaterThan($recent, $stale);
        // Same slope as the pre-due ramp: 70 extra days overdue = 70 * 100/30 points.
        $this->assertEqualsWithDelta(70 * 100 / 30, $stale - $recent, 0.01);
    }

    // --- Estimate (SORT008) ---

    public function test_estimate_scoring(): void
    {
        // 30 min = 0.5h → 100*(1-0.5/8) = 93.75
        $this->assertEqualsWithDelta(93.75, WorklistScorer::estimateScore(30), 0.01);
        $this->assertSame(0.0, WorklistScorer::estimateScore(8 * 60));
        $this->assertSame(0.0, WorklistScorer::estimateScore(0));
        $this->assertSame(0.0, WorklistScorer::estimateScore(null));
        $this->assertGreaterThan(WorklistScorer::estimateScore(4 * 60), WorklistScorer::estimateScore(60));
    }

    // --- Crunch (SORT010) ---

    public function test_crunch_boost(): void
    {
        // 2 days → 20*(1-2/3) ≈ 6.67
        $this->assertEqualsWithDelta(20.0 * (1 - 2 / 3), WorklistScorer::crunchBoost($this->now->copy()->addDays(2), $this->now), 0.01);
        $this->assertSame(0.0, WorklistScorer::crunchBoost($this->now->copy()->addDays(5), $this->now));
        $this->assertSame(20.0, WorklistScorer::crunchBoost($this->now->copy()->subDays(2), $this->now));
        $this->assertSame(0.0, WorklistScorer::crunchBoost(null, $this->now));
    }

    // --- Not-before penalty (SORT012) ---

    public function test_not_before_penalty(): void
    {
        $this->assertSame(1.0, WorklistScorer::notBeforePenalty(null, $this->now));
        $this->assertSame(1.0, WorklistScorer::notBeforePenalty($this->now->copy()->subDay(), $this->now));
        $this->assertSame(0.2, WorklistScorer::notBeforePenalty($this->now->copy()->addDays(10), $this->now));
        // 3.5 days → 1 - 0.8*(3.5/7) = 0.6
        $this->assertEqualsWithDelta(0.6, WorklistScorer::notBeforePenalty($this->now->copy()->addHours(84), $this->now), 0.01);
    }

    public function test_actionable_ranks_above_deferred(): void
    {
        $actionable = $this->scoreOf(['priority' => 'Medium']);
        $deferred = $this->scoreOf(['priority' => 'Medium', 'not_before' => $this->now->copy()->addDays(5)]);
        $this->assertGreaterThan($deferred, $actionable);
    }

    // --- up_next (SORT011) ---

    public function test_upnext_is_a_band_not_a_score_boost(): void
    {
        $pinned = $this->scorer->scoreIssue($this->issue(['priority' => 'Lowest', 'up_next' => true]), $this->now);
        $regular = $this->scorer->scoreIssue($this->issue(['priority' => 'Highest']), $this->now);

        $this->assertTrue($pinned['pinned']);
        $this->assertFalse($regular['pinned']);
        // The pin adds nothing to the score; the controller's band does the lifting.
        $this->assertEqualsWithDelta($this->scoreOf(['priority' => 'Lowest']), $pinned['score'], 0.01);
        $this->assertGreaterThan($pinned['score'], $regular['score']);
    }

    public function test_upnext_exempt_from_not_before_penalty(): void
    {
        $future = $this->now->copy()->addDays(10);
        $upnext = $this->scoreOf(['priority' => 'Medium', 'not_before' => $future, 'up_next' => true]);
        $regular = $this->scoreOf(['priority' => 'Medium']);
        // Penalty skipped, so a pinned deferred item scores as if actionable now.
        $this->assertEqualsWithDelta($regular, $upnext, 0.01);
    }

    // --- Reasons ---

    public function test_reason_precedence(): void
    {
        $reason = fn (array $a) => $this->scorer->scoreIssue($this->issue($a), $this->now)['reason'];

        $this->assertSame('pinned', $reason(['priority' => 'Low', 'up_next' => true]));
        $this->assertSame('2 days overdue', $reason(['priority' => 'Low', 'due_date' => $this->now->copy()->subDays(2)]));
        $this->assertSame('due tomorrow', $reason(['priority' => 'Low', 'due_date' => $this->now->copy()->addDay()]));
        $this->assertSame('starts in 5 days', $reason(['priority' => 'Low', 'not_before' => $this->now->copy()->addDays(5)]));
        $this->assertSame('quick win', $reason(['priority' => 'Low', 'remaining_minutes' => 30]));
        $this->assertSame('Low', $reason(['priority' => 'Low']));
    }

    // --- PR synthetic due (ADR-0013) ---

    public function test_pr_climbs_past_grace(): void
    {
        $pr = fn (Carbon $opened) => (new PullRequest(['opened_at' => $opened]));

        $fresh = $this->scorer->scorePr($pr($this->now->copy()), $this->now)['score'];
        $aged = $this->scorer->scorePr($pr($this->now->copy()->subDays(4)), $this->now)['score'];

        $this->assertGreaterThan($fresh, $aged);
        // Fresh PR still ranks above a plain low-priority issue.
        $this->assertGreaterThan($this->scoreOf(['priority' => 'Lowest']), $fresh);
        $this->assertSame('4 days old', $this->scorer->scorePr($pr($this->now->copy()->subDays(4)), $this->now)['reason']);
    }

    public function test_older_pr_outranks_newer_pr(): void
    {
        $pr = fn (int $days) => $this->scorer->scorePr(new PullRequest(['opened_at' => $this->now->copy()->subDays($days)]), $this->now)['score'];

        $this->assertGreaterThan($pr(3), $pr(73));
        // +1 point of final score per day overdue (0.30 weight x 100/30 slope).
        $this->assertEqualsWithDelta(70.0, $pr(73) - $pr(3), 0.01);
    }

    public function test_pr_age_shows_hours_under_a_day(): void
    {
        $pr = fn (Carbon $opened) => new PullRequest(['opened_at' => $opened]);

        $this->assertSame('5 hours old', $this->scorer->scorePr($pr($this->now->copy()->subHours(5)), $this->now)['reason']);
        $this->assertSame('1 hour old', $this->scorer->scorePr($pr($this->now->copy()->subHour()), $this->now)['reason']);
        $this->assertSame('1 day old', $this->scorer->scorePr($pr($this->now->copy()->subDay()), $this->now)['reason']);
    }

    public function test_pr_age_shows_minutes_under_an_hour(): void
    {
        $pr = fn (Carbon $opened) => new PullRequest(['opened_at' => $opened]);

        $this->assertSame('30 minutes old', $this->scorer->scorePr($pr($this->now->copy()->subMinutes(30)), $this->now)['reason']);
        $this->assertSame('1 minute old', $this->scorer->scorePr($pr($this->now->copy()->subMinute()), $this->now)['reason']);
    }

    public function test_pr_null_opened_at_treated_as_now(): void
    {
        $result = $this->scorer->scorePr(new PullRequest(['opened_at' => null]), $this->now);
        $this->assertGreaterThan(0, $result['score']);
        $this->assertSame('now', $result['reason']);
    }

    // --- Breakdown (ADR-0013 hover reveal) ---

    public function test_breakdown_components_sum_to_score(): void
    {
        // High, due in 4 days, 2h remaining, slightly deferred: every additive
        // component present, then the not_before multiplier.
        $bd = $this->scorer->scoreIssue($this->issue([
            'priority' => 'High',
            'due_date' => $this->now->copy()->addDays(4),
            'remaining_minutes' => 120,
            'not_before' => $this->now->copy()->addDays(2),
        ]), $this->now)['breakdown'];

        $this->assertSame('Priority (High)', $bd['components'][0]['label']);
        $this->assertEqualsWithDelta(array_sum(array_column($bd['components'], 'points')), $bd['subtotal'], 0.01);
        // total = subtotal × not_before factor, and matches the transform shown.
        $this->assertSame('×', $bd['transform']['op']);
        $this->assertEqualsWithDelta($bd['subtotal'] * $bd['transform']['amount'], $bd['total'], 0.01);
    }

    public function test_breakdown_upnext_has_no_transform(): void
    {
        $bd = $this->scorer->scoreIssue($this->issue(['priority' => 'Low', 'up_next' => true, 'not_before' => $this->now->copy()->addDays(10)]), $this->now)['breakdown'];

        $this->assertNull($bd['transform']);
        $this->assertEqualsWithDelta($bd['subtotal'], $bd['total'], 0.01);
    }

    public function test_breakdown_omits_zero_components_but_keeps_priority(): void
    {
        // Plain Medium: no due, no estimate → only the priority row survives.
        $bd = $this->scorer->scoreIssue($this->issue(['priority' => 'Medium']), $this->now)['breakdown'];

        $this->assertCount(1, $bd['components']);
        $this->assertNull($bd['transform']);
    }
}
