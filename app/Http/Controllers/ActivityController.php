<?php

namespace App\Http\Controllers;

use App\Models\JobRun;
use Inertia\Inertia;
use Inertia\Response;

class ActivityController extends Controller
{
    /** Rows returned per request — the daily prune (#85, later slice) keeps this small. */
    private const LIMIT = 200;

    /**
     * Job & Sync Activity Log (#87): a flat list of every recorded job run,
     * newest first. Grouping by "sync moment" is a later slice.
     */
    public function index(): Response
    {
        $runs = JobRun::orderByDesc('started_at')->limit(self::LIMIT)->get()
            ->map(fn (JobRun $r) => [
                'id' => $r->id,
                'jobClass' => class_basename($r->job_class),
                'trigger' => $r->trigger,
                'status' => $r->status,
                'startedAt' => $r->started_at,
                'finishedAt' => $r->finished_at,
                'error' => $r->error,
            ]);

        return Inertia::render('Activity', ['runs' => $runs]);
    }
}
