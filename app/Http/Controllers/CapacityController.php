<?php

namespace App\Http\Controllers;

use App\Http\Requests\StoreCapacityAdjustmentRequest;
use App\Models\CapacityAdjustment;
use Carbon\CarbonImmutable;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class CapacityController extends Controller
{
    /** Time off & extra days: one signed row per date (ADR-0008). */
    public function index(): Response
    {
        return Inertia::render('Capacity', [
            'adjustments' => CapacityAdjustment::orderByDesc('date')
                ->get(['id', 'date', 'hours', 'reason']),
            'baseCapacity' => (int) config('fylla.contracted_hours_per_week'),
        ]);
    }

    /** Upsert on date. Time off expands to weekdays; extra day is one date. */
    public function store(StoreCapacityAdjustmentRequest $request): RedirectResponse
    {
        $data = $request->validated();
        $mag = (int) $data['hours'];
        $signed = $data['type'] === 'off' ? -$mag : $mag;
        $reason = $data['reason'] ?? null;

        foreach ($this->dates($data) as $date) {
            CapacityAdjustment::updateOrCreate(
                ['date' => $date],
                ['hours' => $signed, 'reason' => $reason],
            );
        }

        return back();
    }

    /** Edit hours/reason in place; the date is immutable. */
    public function update(Request $request, CapacityAdjustment $capacityAdjustment): RedirectResponse
    {
        $data = $request->validate([
            'type' => ['required', 'in:off,extra'],
            'hours' => ['required', 'integer', 'between:1,24'],
            'reason' => ['nullable', 'string', 'max:255'],
        ]);
        $mag = (int) $data['hours'];

        $capacityAdjustment->update([
            'hours' => $data['type'] === 'off' ? -$mag : $mag,
            'reason' => $data['reason'] ?? null,
        ]);

        return back();
    }

    public function destroy(CapacityAdjustment $capacityAdjustment): RedirectResponse
    {
        $capacityAdjustment->delete();

        return back();
    }

    /** Time off → Mon–Fri days in [start, end]; extra day → [start]. */
    private function dates(array $data): array
    {
        $start = CarbonImmutable::parse($data['start']);

        if ($data['type'] !== 'off') {
            return [$start->toDateString()];
        }

        $end = isset($data['end']) ? CarbonImmutable::parse($data['end']) : $start;
        $dates = [];
        for ($d = $start; $d->lte($end); $d = $d->addDay()) {
            if ($d->dayOfWeekIso <= 5) {
                $dates[] = $d->toDateString();
            }
        }

        return $dates;
    }
}
