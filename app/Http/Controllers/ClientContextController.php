<?php

namespace App\Http\Controllers;

use App\ClientContext\ClientContextReport;
use App\Delivery\ClientDeliveryHistory;
use App\Models\Client;
use Inertia\Inertia;
use Inertia\Response;

class ClientContextController extends Controller
{
    /**
     * Client context page (#56), reached from a Delivery card. Read-only except
     * the target editor in the history widget (#68, ADR-0018).
     */
    public function show(Client $client): Response
    {
        $client->load('projects:id,client_id,kendo_id');

        return Inertia::render('ClientContext', [
            'data' => (new ClientContextReport)->generate($client),
            'history' => (new ClientDeliveryHistory)->generate($client),
            'target' => [
                'clientId' => $client->id,
                'default' => $client->monthly_target_hours,
                'changes' => $client->targetChanges()
                    ->orderBy('effective_from')
                    ->get(['id', 'effective_from', 'hours'])
                    ->map(fn ($c) => ['id' => $c->id, 'month' => $c->effective_from->format('Y-m'), 'hours' => $c->hours]),
            ],
        ]);
    }
}
