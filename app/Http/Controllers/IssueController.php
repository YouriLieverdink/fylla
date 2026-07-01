<?php

namespace App\Http\Controllers;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use Inertia\Inertia;
use Inertia\Response;

class IssueController extends Controller
{
    /** List issues from the local table (never live Kendo, per ADR-0003). */
    public function index(): Response
    {
        return Inertia::render('Issues', [
            'issues' => Issue::orderByDesc('updated_at')->get([
                'key', 'title', 'priority', 'type', 'updated_at',
            ]),
            'lastSyncedAt' => Issue::max('synced_at'),
        ]);
    }

    /** Manual "sync now" — dispatches the same job the scheduler runs. */
    public function sync(): \Illuminate\Http\RedirectResponse
    {
        SyncKendoIssues::dispatch();

        return back();
    }
}
