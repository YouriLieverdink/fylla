<?php

namespace App\Http\Controllers;

use App\Estimation\EstimationReport;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class EstimationController extends Controller
{
    /** Estimate vs actual per finished issue + rolling bias, sliceable by project. */
    public function index(Request $request): Response
    {
        // Hyphen-separated project ids (?projects=3-14) — a comma would be
        // percent-encoded by the client. Empty/absent = every project.
        $projectIds = collect(explode('-', (string) $request->query('projects', '')))
            ->map(fn ($id) => (int) trim($id))
            ->filter()
            ->values()
            ->all();

        return Inertia::render('Estimation', (new EstimationReport)->generate($projectIds));
    }
}
