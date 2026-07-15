<?php

namespace App\Http\Controllers;

use App\Models\Client;
use App\Models\Project;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class ProjectController extends Controller
{
    /** Clients page: projects (with their client assignment) plus the client list. */
    public function index(): Response
    {
        return Inertia::render('Clients', [
            'projects' => Project::orderBy('name')
                ->get(['id', 'name', 'code', 'billable', 'client_id']),
            'clients' => Client::orderBy('name')
                ->get(['id', 'name', 'monthly_target_hours']),
        ]);
    }

    /** Flip billable and/or (re)assign the project to a client (ADR-0011). */
    public function update(Request $request, Project $project): RedirectResponse
    {
        $project->update($request->validate([
            'billable' => 'sometimes|boolean',
            'client_id' => 'sometimes|nullable|exists:clients,id',
        ]));

        return back();
    }
}
