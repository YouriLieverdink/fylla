<?php

namespace App\Http\Controllers;

use App\Models\Client;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;

class ClientController extends Controller
{
    /** Create a Fylla-owned client (ADR-0011). */
    public function store(Request $request): RedirectResponse
    {
        Client::create($request->validate([
            'name' => 'required|string|max:255',
            'monthly_target_hours' => 'nullable|integer|min:0',
        ]));

        return back();
    }

    /** Rename and/or set-or-clear the monthly target. */
    public function update(Request $request, Client $client): RedirectResponse
    {
        $client->update($request->validate([
            'name' => 'sometimes|required|string|max:255',
            'monthly_target_hours' => 'sometimes|nullable|integer|min:0',
        ]));

        return back();
    }

    /** Delete a client, nulling its projects' client_id — no cascade (ADR-0011). */
    public function destroy(Client $client): RedirectResponse
    {
        $client->projects()->update(['client_id' => null]);
        $client->delete();

        return back();
    }
}
