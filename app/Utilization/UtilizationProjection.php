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
 * Issue #106 adds time to band at the recent pace; issue #107 adds the flat
 * sustained rate over the current week and next three; issue #108 exposes the
 * first four pace-held steps for the projection chart; issue #109 adds the
 * band-rate counterfactual when that pace never reaches the floor.
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

    /** Flat-rate horizon for the sustained prescription (#101, not config). */
    private const SUSTAINED_WEEKS = 4;

    /** How far the time-to-band search steps before giving up (#101, not config). */
    private const CAP_WEEKS = 26;

    private int $windowWeeks;

    private int $paceWeeks;

    /** Standard week the pace *rate* is reported against, in hours. */
    private int $contracted;

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
        $this->contracted = (int) config('fylla.contracted_hours_per_week');
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
        $paceHeld = $this->paceHeldProjection($pace === null ? null : $pace['rate']);

        return [
            'history' => $this->report->rollingHistory(),
            'paceHours' => $pace === null ? null : round($pace['hours'], 1),
            'paceWeeks' => $pace === null ? 0 : $pace['weeks'],
            'thisWeek' => $this->thisWeek(),
            'sustained' => $this->sustained(),
            'timeToBand' => $paceHeld['timeToBand'],
            'forward' => $paceHeld['forward'],
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

    private function thisWeek(): array
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

        // This week's solve cannot borrow a larger capacity from an historic
        // extra-day week. Its truthful upper bound is this week's own capacity.
        $floor = $this->solve($this->softFloor, $this->currentCapacity);
        $target = $this->solve($this->target, $this->currentCapacity);

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
     * @param  (callable(float): float)|null  $ratio
     * @return array{hours: float, reachable: bool}
     */
    private function solve(int $threshold, float $maxCapacity, ?callable $ratio = null): array
    {
        $ratio ??= fn (float $hours): float => $this->ratio($hours);

        if ($ratio(0.0) >= $threshold) {
            return ['hours' => 0.0, 'reachable' => true];
        }
        if ($ratio($maxCapacity) < $threshold) {
            return ['hours' => $maxCapacity, 'reachable' => false];
        }

        $lo = 0.0;
        $hi = $maxCapacity;
        while ($hi - $lo >= self::EPSILON) {
            $mid = ($lo + $hi) / 2;
            if ($ratio($mid) >= $threshold) {
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
     * Flat billable hours per capacity-bearing week that put the rolling
     * window in the band at the end of cur+3. A fully off week still advances
     * the calendar and evicts history, but contributes to neither sum.
     */
    private function sustained(): array
    {
        $effectiveWeeks = 0;
        $maxCapacity = 0.0;
        for ($i = 0; $i < self::SUSTAINED_WEEKS; $i++) {
            $capacity = $this->report->weekCapacity($this->currentMonday->addWeeks($i));
            if ($capacity > 0) {
                $effectiveWeeks++;
                $maxCapacity = max($maxCapacity, $capacity);
            }
        }

        $ratio = fn (float $hours): float => $this->sustainedRatio($hours);
        $floor = $this->solve($this->softFloor, $maxCapacity, $ratio);
        $target = $this->solve($this->target, $maxCapacity, $ratio);

        return [
            'horizonWeeks' => self::SUSTAINED_WEEKS,
            'effectiveWeeks' => $effectiveWeeks,
            'floor' => ['hoursPerWeek' => $this->ceilQuarter($floor['hours']), 'feasible' => $floor['reachable']],
            'target' => ['hoursPerWeek' => $this->ceilQuarter($target['hours']), 'feasible' => $target['reachable']],
        ];
    }

    /** Window ending at cur+3 with one flat rate across its simulated weeks. */
    private function sustainedRatio(float $hours): float
    {
        $bill = 0.0;
        $cap = 0.0;
        for ($i = self::SUSTAINED_WEEKS - $this->windowWeeks; $i < self::SUSTAINED_WEEKS; $i++) {
            $weekStart = $this->currentMonday->addWeeks($i);
            $weekCapacity = $this->report->weekCapacity($weekStart);
            if ($weekCapacity <= 0) {
                continue;
            }

            $simulated = min($hours, $weekCapacity);
            $bill += match (true) {
                $i < 0 => $this->report->weekBillable($weekStart),
                $i === 0 => max($this->report->weekBillable($weekStart), $simulated),
                default => $simulated,
            };
            $cap += $weekCapacity;
        }

        return $cap > 0 ? $bill / $cap * 100 : 0.0;
    }

    /**
     * Recent pace as a **capacity-weighted rate**: Σ billable ÷ Σ capacity over
     * the P **complete** weeks before this one. The current partial week is
     * excluded — including it collapses the reading every Monday. Zero-capacity
     * weeks leave both sums, so a holiday does not depress the pace (#101
     * dec. 7); with none left there is no pace at all, which is not the same as
     * a rate of 0.
     *
     * A rate, not hours per week: averaging *hours* silently treats a 24h week
     * (one day off) as a 32h one, so a short-but-fully-utilized week dragged the
     * pace — and with it the whole projection — below the floor. Each week is
     * now weighed by the capacity it actually had. `hours` is the same rate
     * rendered against a standard contracted week, purely for display.
     *
     * @return array{rate: float, hours: float, weeks: int}|null
     */
    private function pace(): ?array
    {
        $billable = 0.0;
        $capacity = 0.0;
        $weeks = 0;
        for ($i = $this->paceWeeks; $i >= 1; $i--) {
            $weekStart = $this->currentMonday->subWeeks($i);
            $weekCapacity = $this->report->weekCapacity($weekStart);
            if ($weekCapacity <= 0) {
                continue;
            }
            $billable += $this->report->weekBillable($weekStart);
            $capacity += $weekCapacity;
            $weeks++;
        }
        if ($weeks === 0) {
            return null;
        }
        $rate = $billable / $capacity;

        return ['rate' => $rate, 'hours' => $rate * $this->contracted, 'weeks' => $weeks];
    }

    /**
     * Weeks until the window reaches the band, plus one full window of values
     * for the chart, from one pace-held loop. An off week stays in the chart as a
     * null calendar step but contributes to neither side of the ratio.
     *
     * @return array{timeToBand:array{weeksToFloor:int|null,weeksToTarget:int|null,capWeeks:int,counterfactual:array{floor:array{hoursPerWeek:float,weeks:int}|null,target:array{hoursPerWeek:float,weeks:int}|null}|null},forward:array<int,array{label:string,value:float|null}>}
     */
    private function paceHeldProjection(?float $rate): array
    {
        $floor = null;
        $target = null;
        $forward = [];
        $steps = $rate === null ? $this->windowWeeks : self::CAP_WEEKS;

        for ($k = 1; $k <= $steps; $k++) {
            $weekStart = $this->currentMonday->addWeeks($k - 1);
            $hasCapacity = $this->report->weekCapacity($weekStart) > 0;
            $ratio = $rate === null ? null : $this->paceRatio($rate, $k);

            if ($k <= $this->windowWeeks) {
                $point = [
                    'label' => $weekStart->format('M j'),
                    'value' => $ratio === null ? null : round($ratio, 1),
                ];
                if (! $hasCapacity) {
                    $point['off'] = true;
                }
                $forward[] = $point;
            }
            if ($ratio !== null && $floor === null && $ratio >= $this->softFloor) {
                $floor = $k;
            }
            if ($ratio !== null && $target === null && $ratio >= $this->target) {
                $target = $k;
            }
            if ($target !== null && $k >= $this->windowWeeks) {
                break;
            }
        }

        return [
            'timeToBand' => [
                'weeksToFloor' => $floor,
                'weeksToTarget' => $target,
                'capWeeks' => self::CAP_WEEKS,
                'counterfactual' => $rate !== null && $floor === null ? $this->counterfactual() : null,
            ],
            'forward' => $forward,
        ];
    }

    /**
     * Hold each simulated week at the band percentage of its own capacity and
     * solve only for the crossing week. With fewer than W capacity-bearing
     * weeks available inside the calendar cap, the recovery promise is not
     * useful during that long-leave corner and falls back to the plain state.
     *
     * @return array{floor:array{hoursPerWeek:float,weeks:int}|null,target:array{hoursPerWeek:float,weeks:int}|null}|null
     */
    private function counterfactual(): ?array
    {
        $capacityWeeks = 0;
        for ($k = 1; $k <= self::CAP_WEEKS; $k++) {
            if ($this->report->weekCapacity($this->currentMonday->addWeeks($k - 1)) > 0) {
                $capacityWeeks++;
            }
        }
        if ($capacityWeeks < $this->windowWeeks) {
            return null;
        }

        $floor = $this->counterfactualForThreshold($this->softFloor);
        $target = $this->counterfactualForThreshold($this->target);

        return $floor === null && $target === null ? null : ['floor' => $floor, 'target' => $target];
    }

    /** @return array{hoursPerWeek:float,weeks:int}|null */
    private function counterfactualForThreshold(int $threshold): ?array
    {
        $displayRate = null;
        for ($k = 1; $k <= self::CAP_WEEKS; $k++) {
            $capacity = $this->report->weekCapacity($this->currentMonday->addWeeks($k - 1));
            if ($displayRate === null && $capacity > 0) {
                $displayRate = $this->bandRate($threshold, $capacity);
            }
            $ratio = $this->rollForwardRatio(
                $k,
                fn (float $weekCapacity): float => $this->bandRate($threshold, $weekCapacity),
            );
            if ($ratio >= $threshold) {
                return ['hoursPerWeek' => $displayRate ?? 0.0, 'weeks' => $k];
            }
        }

        return null;
    }

    /** Threshold-derived rates round up, so the counterfactual never under-asks. */
    private function bandRate(int $threshold, float $capacity): float
    {
        return ceil(($threshold / 100 * $capacity) / 0.25) * 0.25;
    }

    /**
     * The window ratio at the end of step $k, with the pace held as a **rate**:
     * every simulated week contributes rate × its own capacity, so a short week
     * is asked for proportionally less rather than being scored against a full
     * one. The window is always the last W weeks ending at the evaluated one, so
     * stepping forward evicts the oldest week for free; a cap ≤ 0 week adds
     * nothing to either sum but still consumes its calendar step. The current
     * week floors at the hours already logged (#101's clamp): they cannot be
     * undone by a slower pace. There is no bisection here, so the floor costs
     * the curve nothing.
     */
    private function paceRatio(float $rate, int $k): float
    {
        return $this->rollForwardRatio($k, fn (float $capacity): float => $rate * $capacity);
    }

    /**
     * Shared roll-forward step. Off weeks still consume their calendar slot and
     * evict history, while the supplied rate is ceilinged by each week's own
     * capacity before it enters the rolling ratio.
     *
     * @param  callable(float): float  $hoursForCapacity
     */
    private function rollForwardRatio(int $k, callable $hoursForCapacity): float
    {
        $bill = 0.0;
        $cap = 0.0;
        for ($i = $k - $this->windowWeeks; $i < $k; $i++) {
            $weekStart = $this->currentMonday->addWeeks($i);
            $weekCapacity = $this->report->weekCapacity($weekStart);
            if ($weekCapacity <= 0) {
                continue;
            }
            $simulated = min($hoursForCapacity($weekCapacity), $weekCapacity);
            $bill += match (true) {
                $i < 0 => $this->report->weekBillable($weekStart),
                $i === 0 => max($this->report->weekBillable($weekStart), $simulated),
                default => $simulated,
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
