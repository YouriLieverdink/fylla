<?php

namespace App\Utilization;

use App\Models\SyncedWorklog;
use App\Models\TimeOff;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

/**
 * Personal billable utilization (issue #12). Reads the local worklog mirror +
 * time-off table; no HTTP, so it is directly testable.
 *
 * - Numerator: billable minutes only, bucketed by ISO week of started_at.
 * - Denominator (capacity) per week: contracted hours − time off that week.
 *   The current (partial) week prorates over Mon–Fri elapsed workdays.
 * - Headline = one cumulative Σbillable ÷ Σcapacity over the window.
 * - Trend points = each week's own billable ÷ capacity (not running-cumulative).
 * - A zero-capacity week (all time off) drops out of both sums and the trend.
 */
class UtilizationReport
{
    private int $contracted;
    private int $windowWeeks;
    private int $target;
    private int $softFloor;
    private CarbonImmutable $now;
    private CarbonImmutable $currentMonday;

    /** @var Collection<string,int> billable minutes keyed by week-start date */
    private Collection $billableByWeek;
    private Collection $timeOff;

    public function __construct(?CarbonImmutable $now = null)
    {
        $this->contracted = (int) config('fylla.contracted_hours_per_week');
        $this->windowWeeks = (int) config('fylla.utilization_window_weeks');
        $this->target = (int) config('fylla.utilization_target');
        $this->softFloor = (int) config('fylla.utilization_soft_floor');
        $this->now = $now ?? CarbonImmutable::now();
        $this->currentMonday = $this->now->startOfWeek(CarbonImmutable::MONDAY);
    }

    public function generate(): array
    {
        // Load current + preceding window in one pass (delta needs both).
        $rangeStart = $this->currentMonday->subWeeks(2 * $this->windowWeeks - 1);
        $rangeEnd = $this->currentMonday->addWeek();

        $this->billableByWeek = SyncedWorklog::billable()
            ->whereBetween('started_at', [$rangeStart, $rangeEnd])
            ->get(['minutes', 'started_at'])
            ->groupBy(fn ($w) => $w->started_at->startOfWeek(CarbonImmutable::MONDAY)->toDateString())
            ->map(fn ($group) => $group->sum('minutes'));

        $this->timeOff = TimeOff::whereBetween('date', [$rangeStart, $rangeEnd])->get();

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
            $points[] = [
                'label' => $weekStart->format('M j'),
                'value' => round($b / $c * 100, 1),
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

    /** @return array{0: float, 1: float} [billableHours, capacityHours] */
    private function weekData(CarbonImmutable $weekStart): array
    {
        $billableHours = ($this->billableByWeek->get($weekStart->toDateString(), 0)) / 60;

        if ($weekStart->equalTo($this->currentMonday)) {
            // Partial week: prorate over Mon–Fri elapsed (incl. today), then
            // subtract only time off that has already passed.
            $workdaysElapsed = min($this->now->dayOfWeekIso, 5);
            $offPassed = $this->timeOff
                ->filter(fn ($t) => $t->date->gte($weekStart) && $t->date->lte($this->now))
                ->sum('hours');
            $capacity = $this->contracted * $workdaysElapsed / 5 - $offPassed;
        } else {
            $off = $this->timeOff
                ->filter(fn ($t) => $t->date->gte($weekStart) && $t->date->lt($weekStart->addWeek()))
                ->sum('hours');
            $capacity = $this->contracted - $off;
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
