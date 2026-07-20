<?php

namespace App\Http\Controllers;

use App\Models\Project;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;

class ProjectController extends Controller
{
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
