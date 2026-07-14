<?php

namespace App\Utilization;

use App\Models\CapacityAdjustment;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

/**
 * Personal billable utilization (issue #12). Reads the local worklog mirror +
 * capacity-adjustment table; no HTTP, so it is directly testable.
 *
 * - Numerator: billable minutes only, bucketed by ISO week of started_at.
 * - Denominator (capacity) per week: contracted hours + Σ signed adjustments
 *   that week (time off is stored negative, an extra day positive; ADR-0008).
 *   The current (partial) week prorates over Mon–Fri elapsed workdays.
 * - Headline = one cumulative Σbillable ÷ Σcapacity over the window.
 * - Trend points = each week's own billable ÷ capacity (not running-cumulative).
 * - A zero-capacity week (all time off) drops out of both sums and the trend.
 */
class UtilizationReport
{
    private int $contracted;
    private int $offWeekday;
    private int $windowWeeks;
    private int $target;
    private int $softFloor;
    private CarbonImmutable $now;
    private CarbonImmutable $currentMonday;

    /** @var Collection<string,int> billable minutes keyed by week-start date */
    private Collection $billableByWeek;
    /** @var Collection<string,int> all worked minutes keyed by week-start date */
    private Collection $workedByWeek;
    private Collection $adjustments;

    public function __construct(?CarbonImmutable $now = null)
    {
        $this->contracted = (int) config('fylla.contracted_hours_per_week');
        $this->offWeekday = (int) config('fylla.contracted_off_weekday');
        $this->windowWeeks = (int) config('fylla.utilization_window_weeks');
        $this->target = (int) config('fylla.utilization_target');
        $this->softFloor = (int) config('fylla.utilization_soft_floor');
        $this->now = $now ?? CarbonImmutable::now();
        $this->currentMonday = $this->now->startOfWeek(CarbonImmutable::MONDAY);
    }

    public function generate(): array
    {
        $this->load();

        // Current window: build trend points, headline sums, and the gauge.
        $points = [];
        $bill = 0.0;
        $cap = 0.0;
        $week = null;
        for ($i = $this->windowWeeks - 1; $i >= 0; $i--) {
            $weekStart = $this->currentMonday->subWeeks($i);
            [$b, $c] = $this->weekData($weekStart);

            if ($i === 0) {
                $week = [
                    'value' => $c > 0 ? round($b / $c * 100, 1) : null,
                    'billableHours' => round($b, 1),
                    'capacityHours' => round($c, 1),
                ];
            }

            if ($c <= 0) {
                continue; // zero-capacity week: not a data point, not in the sums
            }
            $bill += $b;
            $cap += $c;
            $w = ($this->workedByWeek->get($weekStart->toDateString(), 0)) / 60;
            $points[] = [
                'label' => $weekStart->format('M j'),
                'value' => round($b / $c * 100, 1),
                'billableShare' => $w > 0 ? round($b / $w * 100, 1) : null,
            ];
        }

        $value = $cap > 0 ? round($bill / $cap * 100, 1) : null;

        // Preceding equal-length window (all complete past weeks) for the delta.
        $prevStarts = [];
        for ($i = 2 * $this->windowWeeks - 1; $i >= $this->windowWeeks; $i--) {
            $prevStarts[] = $this->currentMonday->subWeeks($i);
        }
        $prev = $this->cumulative($prevStarts);
        $delta = ($value !== null && $prev !== null) ? round($value - $prev, 1) : null;

        $onTrack = $value !== null && $value >= $this->softFloor;

        return [
            'value' => $value,
            'target' => $this->target,
            'status' => $value === null ? 'no data' : ($onTrack ? 'on track' : 'below band'),
            'onTrack' => $onTrack,
            'note' => $this->note($value),
            'delta' => $delta === null ? null : sprintf('%+.1f pts', $delta),
            'deltaCaption' => "vs. previous {$this->windowWeeks} weeks",
            'week' => $week,
            'points' => $points,
        ];
    }

    /**
     * Per-week breakdown for the window (newest first) plus window totals.
     * Utilization + capacity reuse weekData() so they reconcile with generate()
     * exactly; worked is Σ all minutes that week (effort context, not a
     * denominator — see the handoff).
     */
    public function breakdown(): array
    {
        $this->load();

        $weeks = [];
        $bill = 0.0;
        $cap = 0.0;
        $worked = 0.0;
        for ($i = 0; $i < $this->windowWeeks; $i++) {
            $weekStart = $this->currentMonday->subWeeks($i);
            [$b, $c] = $this->weekData($weekStart);
            $w = ($this->workedByWeek->get($weekStart->toDateString(), 0)) / 60;

            $weeks[] = [
                'label' => $weekStart->format('M j'),
                'capacity' => round($c, 1),
                'worked' => round($w, 1),
                'billable' => round($b, 1),
                'billableShare' => $w > 0 ? round($b / $w * 100, 1) : null,
                'utilization' => $c > 0 ? round($b / $c * 100, 1) : null,
                'adjustments' => $this->weekAdjustments($weekStart),
            ];

            $worked += $w;
            if ($c > 0) { // zero-capacity weeks stay out of the totals, as in generate()
                $bill += $b;
                $cap += $c;
            }
        }

        return [
            'weeks' => $weeks,
            'totals' => [
                'capacity' => round($cap, 1),
                'worked' => round($worked, 1),
                'billable' => round($bill, 1),
                // ponytail: $bill sums only capacity>0 weeks, $worked sums all —
                // off only in an all-time-off week; use displayed totals as-is.
                'billableShare' => $worked > 0 ? round($bill / $worked * 100, 1) : null,
                'utilization' => $cap > 0 ? round($bill / $cap * 100, 1) : null,
            ],
            'target' => $this->target,
            'softFloor' => $this->softFloor,
        ];
    }

    /**
     * The week's signed adjustments folded into chips (identical hours values
     * collapse into one chip with a count), newest-magnitude first. Whole week
     * Mon–Sun, no proration — this is the entry-verification view the capacity
     * page used to carry.
     *
     * @return array<int,array{hours:int,count:int}>
     */
    private function weekAdjustments(CarbonImmutable $weekStart): array
    {
        return $this->adjustments
            ->filter(fn ($a) => $a->date->gte($weekStart) && $a->date->lt($weekStart->addWeek()))
            ->countBy('hours')
            ->map(fn ($count, $hours) => ['hours' => (int) $hours, 'count' => $count])
            ->sortByDesc('hours')
            ->values()
            ->all();
    }

    /**
     * Load the current + preceding window in one pass (the delta needs both).
     * Worklogs are fetched once with their project so billable vs. worked are
     * split in PHP off the derived billable attribute.
     */
    private function load(): void
    {
        $rangeStart = $this->currentMonday->subWeeks(2 * $this->windowWeeks - 1);
        $rangeEnd = $this->currentMonday->addWeek();

        $worklogs = SyncedWorklog::whereBetween('started_at', [$rangeStart, $rangeEnd])
            ->with('project:kendo_id,billable')
            ->get(['minutes', 'started_at', 'kendo_project_id']);

        $byWeek = fn (Collection $logs) => $logs
            ->groupBy(fn ($w) => $w->started_at->startOfWeek(CarbonImmutable::MONDAY)->toDateString())
            ->map(fn ($group) => $group->sum('minutes'));

        $this->workedByWeek = $byWeek($worklogs);
        $this->billableByWeek = $byWeek($worklogs->filter(fn ($w) => $w->billable));
        $this->adjustments = CapacityAdjustment::whereBetween('date', [$rangeStart, $rangeEnd])->get();
    }

    /** @return array{0: float, 1: float} [billableHours, capacityHours] */
    private function weekData(CarbonImmutable $weekStart): array
    {
        $billableHours = ($this->billableByWeek->get($weekStart->toDateString(), 0)) / 60;

        if ($weekStart->equalTo($this->currentMonday)) {
            // Partial week: prorate over worked weekdays elapsed (incl. today),
            // then fold in only adjustments whose date has already passed.
            // Signed, so + covers both time off (−) and an extra day (+).
            $isWorkday = fn (int $dow) => $dow <= 5 && $dow !== $this->offWeekday;
            $workdaysPerWeek = count(array_filter([1, 2, 3, 4, 5], $isWorkday));
            $workdaysElapsed = count(array_filter(range(1, min($this->now->dayOfWeekIso, 5)), $isWorkday));
            $adjPassed = $this->adjustments
                ->filter(fn ($a) => $a->date->gte($weekStart) && $a->date->lte($this->now))
                ->sum('hours');
            $capacity = $this->contracted * $workdaysElapsed / $workdaysPerWeek + $adjPassed;
        } else {
            $adj = $this->adjustments
                ->filter(fn ($a) => $a->date->gte($weekStart) && $a->date->lt($weekStart->addWeek()))
                ->sum('hours');
            $capacity = $this->contracted + $adj;
        }

        return [$billableHours, (float) $capacity];
    }

    /** Cumulative % over the given week-starts, or null if zero total capacity. */
    private function cumulative(array $weekStarts): ?float
    {
        $bill = 0.0;
        $cap = 0.0;
        foreach ($weekStarts as $weekStart) {
            [$b, $c] = $this->weekData($weekStart);
            if ($c <= 0) {
                continue;
            }
            $bill += $b;
            $cap += $c;
        }

        return $cap > 0 ? round($bill / $cap * 100, 1) : null;
    }

    private function note(?float $value): string
    {
        if ($value === null) {
            return 'No capacity in this window — all time off logged.';
        }
        $gap = round($value - $this->target, 1);
        if ($value >= $this->target) {
            return sprintf('%.1f pts over your %d%% target.', $gap, $this->target);
        }
        if ($value >= $this->softFloor) {
            return sprintf('%.1f pts under target — within your soft band, no action needed.', abs($gap));
        }

        return sprintf('%.1f pts under target — below your %d%% soft floor.', abs($gap), $this->softFloor);
    }
}
