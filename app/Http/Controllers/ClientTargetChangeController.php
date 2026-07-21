<?php

namespace App\Http\Controllers;

use App\Models\Client;
use App\Models\ClientTargetChange;
use Carbon\CarbonImmutable;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;

/**
 * Effective-dated target overrides (ADR-0017), edited from the client page's
 * history widget (#68). One row per (client, first-of-month); dates are
 * normalized here so callers may submit any day in the month.
 */
class ClientTargetChangeController extends Controller
{
    /** Add an override; a second submit for the same month corrects that row. */
    public function store(Request $request, Client $client): RedirectResponse
    {
        $data = $this->validated($request);

        $client->targetChanges()->updateOrCreate(
            ['effective_from' => $data['effective_from']],
            ['hours' => $data['hours']],
        );

        return back();
    }

    public function update(Request $request, ClientTargetChange $targetChange): RedirectResponse
    {
        $data = $this->validated($request);

        // Moving onto a month that already has a row replaces that row.
        $targetChange->client->targetChanges()
            ->where('effective_from', $data['effective_from'])
            ->whereKeyNot($targetChange->id)
            ->delete();

        $targetChange->update($data);

        return back();
    }

    /** Affected months revert to the prior override, or the client default. */
    public function destroy(ClientTargetChange $targetChange): RedirectResponse
    {
        $targetChange->delete();

        return back();
    }

    /** @return array{effective_from: string, hours: int} first-of-month normalized. */
    private function validated(Request $request): array
    {
        $data = $request->validate([
            'effective_from' => 'required|date',
            'hours' => 'required|integer|min:0',
        ]);

        $data['effective_from'] = CarbonImmutable::parse($data['effective_from'])->startOfMonth()->toDateString();

        return $data;
    }
}
