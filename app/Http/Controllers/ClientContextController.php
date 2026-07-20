<?php

namespace App\Http\Controllers;

use App\ClientContext\ClientContextReport;
use App\Models\Client;
use Inertia\Inertia;
use Inertia\Response;

class ClientContextController extends Controller
{
    /** Read-only Client context page (#56), reached from a Delivery card. */
    public function show(Client $client): Response
    {
        $client->load('projects:id,client_id,kendo_id');

        return Inertia::render('ClientContext', [
            'data' => (new ClientContextReport)->generate($client),
        ]);
    }
}
