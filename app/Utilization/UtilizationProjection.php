<?php

namespace App\Utilization;

use Carbon\CarbonImmutable;

/**
 * Forward-looking prescription off the utilization window (issue #105, math
 * contract in #101). This slice answers one question: how many billable hours
 * does *this* week need to clear the soft floor and the target?
 *
 * The shape is a simulation, never `target × capacity`: the window is always
 * the last W weeks ending at the week being evaluated, so the historic weeks
 * carry their own surplus/deficit and the answer nets out automatically.
 *
 *   ratio(h) = Σ billable where cap > 0 ÷ Σ cap where cap > 0 × 100
 *
 * with the current week's contribution capped at its own full-week capacity —
 * the feasibility ceiling lives inside the step function, so no output can
 * assume an impossible week. Capacity is full-week, unprorated (#101 dec. 2);
 * proration only enters through `remaining`, which bounds what is still
 * reachable today.
 *
 * Issue #106 adds the second question: at the pace of the last P complete
 * weeks, how far away is the band?
 */
class UtilizationProjection
{
    /**
     * Bisection halts once the bracket is narrower than this (hours). The
     * contract asks for < 0.01 h; going tighter costs a few iterations and
     * keeps the solver's own error well inside GRID_TOLERANCE.
     */
    private const EPSILON = 0.0001;

    /**
     * Quarter-hour rounding absorbs this much solver error first, so a solved
     * 8.0001 h does not report as 8.25 h.
     */
    private const GRID_TOLERANCE = 0.001;

    /** How far the time-to-band search steps before giving up (#101, not config). */
    private const CAP_WEEKS = 26;

    private int $windowWeeks;

    private int $paceWeeks;

    private int $target;

    private int $softFloor;

    private CarbonImmutable $currentMonday;

    /** Σ billable / Σ capacity over the window's capacity-bearing history. */
    private float $historicBillable = 0.0;

    private float $historicCapacity = 0.0;

    private float $currentCapacity = 0.0;

    public function __construct(private UtilizationReport $report)
    {
        $this->windowWeeks = (int) config('fylla.utilization_window_weeks');
        $this->target = (int) config('fylla.utilization_target');
        $this->softFloor = (int) config('fylla.utilization_soft_floor');
        // Kept ≤ the report's trailing worklog range (2×W−1 weeks); at the real
        // 13-week window and a 4-week pace there is no contest.
        $this->paceWeeks = (int) config('fylla.utilization_pace_weeks');
        // One clock: the report's. A second injectable clock is two sources
        // that can disagree (#103).
        $this->currentMonday = $this->report->now()->startOfWeek(CarbonImmutable::MONDAY);
    }

    /** Null wholesale when there is nothing to project against (#101). */
    public function payload(): ?array
    {
        $capacities = $this->collectWindow();

        if (max($capacities) <= 0) {
            return null; // whole window booked off — no ratio to move
        }

        $pace = $this->pace();

        return [
            'paceHours' => $pace === null ? null : round($pace['hours'], 1),
            'paceWeeks' => $pace === null ? 0 : $pace['weeks'],
            'thisWeek' => $this->thisWeek(max($capacities)),
            'timeToBand' => $this->timeToBand($pace === null ? null : $pace['hours']),
        ];
    }

    /**
     * Read the window off the report into the sums ratio() closes over, and
     * return every week's full capacity (oldest → newest, current week last).
     *
     * @return array<int,float>
     */
    private function collectWindow(): array
    {
        $capacities = [];
        $this->historicBillable = 0.0;
        $this->historicCapacity = 0.0;
        for ($i = $this->windowWeeks - 1; $i >= 0; $i--) {
            $weekStart = $this->currentMonday->subWeeks($i);
            $cap = $this->report->weekCapacity($weekStart);
            $capacities[] = $cap;

            if ($i > 0 && $cap > 0) {
                $this->historicBillable += $this->report->weekBillable($weekStart);
                $this->historicCapacity += $cap;
            }
        }
        $this->currentCapacity = end($capacities);

        return $capacities;
    }

    /**
     * @param  float  $maxCapacity  bisection's upper bound: the largest weekly
     *                              capacity in scope
     */
    private function thisWeek(float $maxCapacity): array
    {
        if ($this->currentCapacity <= 0) {
            // The week is off; there is nothing to act on.
            return [
                'capacityHours' => null,
                'remainingHours' => null,
                'loggedBillableHours' => null,
                'floor' => null,
                'target' => null,
                'headroomHours' => null,
            ];
        }

        $logged = $this->report->weekBillable($this->currentMonday);
        // Prorated current-week capacity is the breakdown's own current-week
        // figure, so `remaining` reconciles with the table rather than
        // re-deriving the proration rule here. load() is idempotent, so this
        // costs no extra query.
        $prorated = (float) $this->report->breakdown()['weeks'][0]['capacity'];
        $remaining = $this->currentCapacity - $prorated;

        $floor = $this->solve($this->softFloor, $maxCapacity);
        $target = $this->solve($this->target, $maxCapacity);

        return [
            'capacityHours' => round($this->currentCapacity, 1),
            'remainingHours' => round($remaining, 1),
            'loggedBillableHours' => round($logged, 1),
            'floor' => $this->threshold($floor, $logged, $remaining),
            'target' => $this->threshold($target, $logged, $remaining),
            'headroomHours' => max(0.0, $this->floorQuarter($logged - $floor['hours'])),
        ];
    }

    /**
     * @param  array{hours: float, reachable: bool}  $solution
     */
    private function threshold(array $solution, float $logged, float $remaining): array
    {
        $more = $solution['hours'] - $logged;

        return [
            'neededTotal' => $this->ceilQuarter($solution['hours']),
            'neededMore' => max(0.0, $this->ceilQuarter($more)),
            // Bounded by what is *left* of the week, not its full capacity: a
            // Thursday ask of 12h against 8h left is infeasible. Flagged,
            // never clamped.
            'feasible' => $solution['reachable'] && $more <= $remaining,
        ];
    }

    /**
     * Smallest weekly billable total B ≥ 0 with ratio(B) ≥ $threshold, by
     * bisection. `reachable` is false when even a fully billable week does not
     * get there — then `hours` is the best the week can do.
     *
     * @return array{hours: float, reachable: bool}
     */
    private function solve(int $threshold, float $maxCapacity): array
    {
        if ($this->ratio(0.0) >= $threshold) {
            return ['hours' => 0.0, 'reachable' => true];
        }
        if ($this->ratio($maxCapacity) < $threshold) {
            return ['hours' => $maxCapacity, 'reachable' => false];
        }

        $lo = 0.0;
        $hi = $maxCapacity;
        while ($hi - $lo >= self::EPSILON) {
            $mid = ($lo + $hi) / 2;
            if ($this->ratio($mid) >= $threshold) {
                $hi = $mid;
            } else {
                $lo = $mid;
            }
        }

        return ['hours' => $hi, 'reachable' => true];
    }

    /**
     * Window ratio with the current week contributing $hours, ceilinged at its
     * own capacity. Monotonic non-decreasing in $hours — the bisection
     * precondition. Hours already logged do not floor the contribution here:
     * with a single variable week that would only flatten the curve below b0
     * and rob `headroomHours` of its meaning; b0 enters via `neededMore` and
     * the headroom instead.
     */
    private function ratio(float $hours): float
    {
        $bill = $this->historicBillable;
        $cap = $this->historicCapacity;
        if ($this->currentCapacity > 0) {
            $bill += min($hours, $this->currentCapacity);
            $cap += $this->currentCapacity;
        }

        return $cap > 0 ? $bill / $cap * 100 : 0.0;
    }

    /**
     * Billable hours per capacity-bearing week over the P **complete** weeks
     * before this one. The current partial week is excluded — including it
     * collapses the reading every Monday. Zero-capacity weeks leave both the
     * sum and the divisor, so a holiday does not depress the pace (#101 dec. 7);
     * with none left there is no pace at all, which is not the same as 0 h/wk.
     *
     * @return array{hours: float, weeks: int}|null
     */
    private function pace(): ?array
    {
        $hours = 0.0;
        $weeks = 0;
        for ($i = $this->paceWeeks; $i >= 1; $i--) {
            $weekStart = $this->currentMonday->subWeeks($i);
            if ($this->report->weekCapacity($weekStart) <= 0) {
                continue;
            }
            $hours += $this->report->weekBillable($weekStart);
            $weeks++;
        }

        return $weeks === 0 ? null : ['hours' => $hours / $weeks, 'weeks' => $weeks];
    }

    /**
     * Weeks until the window reaches the floor and the target if the pace
     * holds. One loop over k = 1 … CAP_WEEKS, k = 1 being the end of the
     * current week; either crossing is null when it never happens in range,
     * which the card reads as "not at this pace".
     */
    private function timeToBand(?float $pace): array
    {
        $floor = null;
        $target = null;

        for ($k = 1; $pace !== null && $k <= self::CAP_WEEKS; $k++) {
            $ratio = $this->paceRatio($pace, $k);
            if ($floor === null && $ratio >= $this->softFloor) {
                $floor = $k;
            }
            if ($target === null && $ratio >= $this->target) {
                $target = $k;
                break; // target ≥ floor, so both are settled by now
            }
        }

        return ['weeksToFloor' => $floor, 'weeksToTarget' => $target, 'capWeeks' => self::CAP_WEEKS];
    }

    /**
     * The window ratio at the end of step $k, with the pace held as **hours per
     * week** rather than as a ratio: every simulated week contributes
     * min(pace, cap) against its own capacity, so booked leave delays the
     * crossing instead of bending the rate. The window is always the last W
     * weeks ending at the evaluated one, so stepping forward evicts the oldest
     * week for free; a cap ≤ 0 week adds nothing to either sum but still
     * consumes its calendar step. The current week floors at the hours already
     * logged (#101's clamp): they cannot be undone by a slower pace. There is
     * no bisection here, so the floor costs the curve nothing.
     */
    private function paceRatio(float $pace, int $k): float
    {
        $bill = 0.0;
        $cap = 0.0;
        for ($i = $k - $this->windowWeeks; $i < $k; $i++) {
            $weekStart = $this->currentMonday->addWeeks($i);
            $weekCapacity = $this->report->weekCapacity($weekStart);
            if ($weekCapacity <= 0) {
                continue;
            }
            $atPace = min($pace, $weekCapacity);
            $bill += match (true) {
                $i < 0 => $this->report->weekBillable($weekStart),
                $i === 0 => max($this->report->weekBillable($weekStart), $atPace),
                default => $atPace,
            };
            $cap += $weekCapacity;
        }

        return $cap > 0 ? $bill / $cap * 100 : 0.0;
    }

    /** Needed hours round up to the next quarter — never under-ask. */
    private function ceilQuarter(float $hours): float
    {
        return ceil(($hours - self::GRID_TOLERANCE) / 0.25) * 0.25;
    }

    /** Headroom rounds down to the quarter — never over-promise slack. */
    private function floorQuarter(float $hours): float
    {
        return floor(($hours + self::GRID_TOLERANCE) / 0.25) * 0.25;
    }
}
