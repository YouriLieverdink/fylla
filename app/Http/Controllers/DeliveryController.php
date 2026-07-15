<?php

namespace App\Http\Controllers;

use App\Delivery\DeliveryReport;
use Inertia\Inertia;
use Inertia\Response;

class DeliveryController extends Controller
{
    /** One delivery card per client: this month's team hours vs target. */
    public function index(): Response
    {
        return Inertia::render('Delivery', [
            'clients' => (new DeliveryReport)->cards(),
        ]);
    }
}
