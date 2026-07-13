<?php

namespace App\Http\Controllers;

use App\Models\Project;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class ProjectController extends Controller
{
    /** Billable-projects settings: the full project list, billable-first. */
    public function index(): Response
    {
        return Inertia::render('Projects', [
            'projects' => Project::orderBy('name')
                ->get(['id', 'name', 'code', 'billable']),
        ]);
    }

    /** Flip a project's locally-owned billable flag. */
    public function update(Request $request, Project $project): RedirectResponse
    {
        $project->update($request->validate(['billable' => 'required|boolean']));

        return back();
    }
}
