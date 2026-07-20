<?php

namespace App\Http\Controllers;

use App\Delivery\DeliveryReport;
use App\Models\Project;
use Inertia\Inertia;
use Inertia\Response;

class DeliveryController extends Controller
{
    /**
     * One delivery card per client (this month's team hours vs target) plus the
     * raw project rows the inline footer edits — billable pills + assignment.
     * The card's `name`/`target` subsume the old `clients` prop (#62).
     */
    public function index(): Response
    {
        return Inertia::render('Delivery', [
            'clients' => (new DeliveryReport)->cards(),
            'projects' => Project::orderBy('name')
                ->get(['id', 'name', 'code', 'billable', 'client_id']),
        ]);
    }
}
