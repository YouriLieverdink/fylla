<?php

namespace App\Http\Controllers;

use App\Http\Requests\StoreCapacityAdjustmentRequest;
use App\Models\CapacityAdjustment;
use App\Models\VacationAccrual;
use App\Vacation\VacationLedger;
use Carbon\CarbonImmutable;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class CapacityController extends Controller
{
    /**
     * Time off & vacation: a year calendar grid over capacity adjustments plus a
     * vacation ledger (ADR-0010). One row per date carries type + status; the
     * ledger is derived from those rows plus a per-year accrual.
     */
    public function index(Request $request, VacationLedger $ledger): Response
    {
        $overview = $ledger->overview((int) CarbonImmutable::now()->year);
        $years = array_keys($overview);
        $year = (int) ($request->integer('year') ?: CarbonImmutable::now()->year);
        if (! in_array($year, $years, true)) {
            $year = (int) CarbonImmutable::now()->year;
        }

        return Inertia::render('Capacity', [
            'year' => $year,
            'years' => $years,
            'adjustments' => CapacityAdjustment::whereYear('date', $year)
                ->orderBy('date')
                ->get(['id', 'date', 'type', 'hours', 'status', 'reason']),
            'accrual' => VacationAccrual::where('year', $year)->value('hours'),
            'ledger' => $overview[$year],
            'overview' => array_values($overview),
            'baseCapacity' => (int) config('fylla.contracted_hours_per_week'),
            'offWeekday' => (int) config('fylla.contracted_off_weekday'),
        ]);
    }

    /**
     * Upsert per date. Off, holiday and sick expand over weekdays; an extra day
     * is a single date, any day. Type is written directly (ADR-0010); the sign
     * still follows the type (everything but extra is negative; ADR-0008).
     */
    public function store(StoreCapacityAdjustmentRequest $request): RedirectResponse
    {
        $data = $request->validated();
        $signed = $data['type'] === 'extra' ? abs($data['hours']) : -abs($data['hours']);

        foreach ($this->dates($data) as $date) {
            CapacityAdjustment::updateOrCreate(
                ['date' => $date],
                [
                    'type' => $data['type'],
                    'hours' => $signed,
                    'status' => $data['status'] ?? 'planned',
                    'reason' => $data['reason'] ?? null,
                ],
            );
        }

        return back();
    }

    /** Edit type/hours/status/reason in place; the date is immutable. */
    public function update(Request $request, CapacityAdjustment $capacityAdjustment): RedirectResponse
    {
        $data = $request->validate([
            'type' => ['required', 'in:off,holiday,sick,extra'],
            'hours' => ['required', 'numeric', 'between:0.25,24'],
            'status' => ['required', 'in:planned,confirmed'],
            'reason' => ['nullable', 'string', 'max:255'],
        ]);

        $capacityAdjustment->update([
            'type' => $data['type'],
            'hours' => $data['type'] === 'extra' ? abs($data['hours']) : -abs($data['hours']),
            'status' => $data['status'],
            'reason' => $data['reason'] ?? null,
        ]);

        return back();
    }

    public function destroy(CapacityAdjustment $capacityAdjustment): RedirectResponse
    {
        $capacityAdjustment->delete();

        return back();
    }

    /** Set the manually-entered vacation accrual for a year (Vakantieuren). */
    public function accrual(Request $request): RedirectResponse
    {
        $data = $request->validate([
            'year' => ['required', 'integer', 'between:2000,2100'],
            'hours' => ['required', 'numeric', 'between:0,2000'],
        ]);

        VacationAccrual::updateOrCreate(['year' => $data['year']], ['hours' => $data['hours']]);

        return back();
    }

    /** Off/holiday/sick → Mon–Fri days in [start, end] (skipping the contracted
     * off-day); extra day → [start]. */
    private function dates(array $data): array
    {
        $start = CarbonImmutable::parse($data['start']);

        if ($data['type'] === 'extra') {
            return [$start->toDateString()];
        }

        $end = isset($data['end']) ? CarbonImmutable::parse($data['end']) : $start;
        $offDay = (int) config('fylla.contracted_off_weekday');
        $dates = [];
        for ($d = $start; $d->lte($end); $d = $d->addDay()) {
            if ($d->dayOfWeekIso <= 5 && $d->dayOfWeekIso !== $offDay) {
                $dates[] = $d->toDateString();
            }
        }

        return $dates;
    }
}
