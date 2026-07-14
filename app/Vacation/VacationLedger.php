<?php

namespace App\Vacation;

use App\Models\CapacityAdjustment;
use App\Models\VacationAccrual;

/**
 * The vacation ledger (ADR-0010, CONTEXT.md). A per-year running account of
 * vacation entitlement in vacation hours, derived entirely from adjustment rows
 * plus one accrual number per year — no separate balance store.
 *
 *   taken     = Σ off   hours that year   (negative; holidays EXCLUDED)
 *   banked    = Σ extra hours that year   (positive)
 *   carryover = previous year's closing balance (rolls forward indefinitely)
 *   balance   = carryover + accrual + banked + taken
 *
 * Counts planned + confirmed alike — a penciled-in trip shows against
 * affordability. (Only confirmed rows move the *utilization* denominator; that
 * split lives in UtilizationReport, not here.)
 */
class VacationLedger
{
    private const HOURS_PER_DAY = 8;
    private const HOURS_PER_WEEK = 32;

    /**
     * One row per year from the earliest year with data through
     * max(last data year, $currentYear + 1) so the switcher can look ahead.
     * Keyed by year, ascending.
     *
     * @return array<int,array{year:int,carryover:float,accrual:float,banked:float,taken:float,balance:float,days:float,weeks:float}>
     */
    public function overview(int $currentYear): array
    {
        $banked = [];
        $taken = [];
        // Planned sub-sums track the still-to-confirm portion of each figure so
        // the panel can show "of which X planned" — the balance already nets
        // planned + confirmed alike (ADR-0010).
        $bankedPlanned = [];
        $takenPlanned = [];
        foreach (CapacityAdjustment::get(['date', 'type', 'hours', 'status']) as $a) {
            $year = $a->date->year;
            if ($a->type === 'extra') {
                $banked[$year] = ($banked[$year] ?? 0.0) + (float) $a->hours;
                if ($a->status === 'planned') {
                    $bankedPlanned[$year] = ($bankedPlanned[$year] ?? 0.0) + (float) $a->hours;
                }
            } elseif ($a->type === 'off') {
                $taken[$year] = ($taken[$year] ?? 0.0) + (float) $a->hours;
                if ($a->status === 'planned') {
                    $takenPlanned[$year] = ($takenPlanned[$year] ?? 0.0) + (float) $a->hours;
                }
            }
            // holiday and sick: capacity-affecting but ledger-neutral (you don't
            // spend vacation on Kingsday or a sick day), so both are skipped.
        }

        $accruals = VacationAccrual::pluck('hours', 'year')
            ->map(fn ($h) => (float) $h)
            ->all();

        // Range spans the earliest year with data through a planning year ahead,
        // and always includes $currentYear so the page's selected year has a row.
        $known = array_merge(array_keys($banked), array_keys($taken), array_keys($accruals));
        $min = min($known ? min($known) : $currentYear, $currentYear);
        $max = max($known ? max($known) : $currentYear, $currentYear + 1);

        $rows = [];
        $carryover = 0.0;
        for ($year = $min; $year <= $max; $year++) {
            $accrual = $accruals[$year] ?? 0.0;
            $bank = $banked[$year] ?? 0.0;
            $take = $taken[$year] ?? 0.0;
            $balance = $carryover + $accrual + $bank + $take;

            $rows[$year] = [
                'year' => $year,
                'carryover' => round($carryover, 2),
                'accrual' => round($accrual, 2),
                'banked' => round($bank, 2),
                'bankedPlanned' => round($bankedPlanned[$year] ?? 0.0, 2),
                'taken' => round($take, 2),
                'takenPlanned' => round($takenPlanned[$year] ?? 0.0, 2),
                'balance' => round($balance, 2),
                'days' => round($balance / self::HOURS_PER_DAY, 2),
                'weeks' => round($balance / self::HOURS_PER_WEEK, 2),
            ];
            $carryover = $balance;
        }

        return $rows;
    }

    /** The ledger row for a single year (within the overview range). */
    public function forYear(int $year): array
    {
        return $this->overview($year)[$year];
    }
}
