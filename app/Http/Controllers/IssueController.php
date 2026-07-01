<?php

namespace App\Http\Controllers;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use App\Models\Timer;
use App\Services\TimerService;
use Illuminate\Support\Carbon;
use Inertia\Inertia;
use Inertia\Response;

class IssueController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    /** List issues from the local table (never live Kendo, per ADR-0003). */
    public function index(): Response
    {
        $live = Timer::live()->with(['issue:id,key,title', 'segments.notes'])->get();

        // max() is a raw aggregate — reparse as UTC so it serializes with a tz
        // marker (the frontend reads a naive string as local, showing it off).
        $syncedAt = Issue::max('synced_at');

        return Inertia::render('Issues', [
            'issues' => Issue::orderByDesc('updated_at')->get([
                'id', 'key', 'title', 'priority', 'type',
                'estimated_minutes', 'remaining_minutes', 'updated_at',
            ]),
            'lastSyncedAt' => $syncedAt ? Carbon::parse($syncedAt, 'UTC')->toJSON() : null,
            'liveIssueIds' => $live->pluck('issue_id'),
            'timer' => $this->stack($live),
        ]);
    }

    /** Manual "sync now" — dispatches the same job the scheduler runs. */
    public function sync(): \Illuminate\Http\RedirectResponse
    {
        SyncKendoIssues::dispatch();

        return back();
    }

    /** Shape the live-timer stack for the page (top-first). */
    private function stack($live): ?array
    {
        if ($live->isEmpty()) {
            return null;
        }

        $top = $live->first();
        $open = $top->segments->firstWhere('ended_at', null);

        $row = fn (Timer $t) => [
            'issue_id' => $t->issue_id,
            'key' => $t->issue->key,
            'title' => $t->issue->title,
            'accumulated_seconds' => $this->timers->accumulatedSeconds($t),
        ];

        return [
            'active' => $row($top) + [
                'running' => (bool) $open,
                'started_at' => $open?->started_at,
                // Notes of the open segment only (ADR-0005); empty when paused.
                'notes' => $open
                    ? $open->notes->sortBy('created_at')->values()->map(fn ($n) => [
                        'at' => $n->created_at->format('H:i'),
                        'text' => $n->text,
                    ])
                    : [],
            ],
            'paused' => $live->skip(1)->map($row)->values(),
        ];
    }
}
