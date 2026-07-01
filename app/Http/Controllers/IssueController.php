<?php

namespace App\Http\Controllers;

use App\Jobs\SyncKendoIssues;
use App\Models\Issue;
use App\Models\Timer;
use App\Services\TimerService;
use Inertia\Inertia;
use Inertia\Response;

class IssueController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    /** List issues from the local table (never live Kendo, per ADR-0003). */
    public function index(): Response
    {
        $live = Timer::live()->with(['issue:id,key,title', 'segments'])->get();

        return Inertia::render('Issues', [
            'issues' => Issue::orderByDesc('updated_at')->get([
                'id', 'key', 'title', 'priority', 'type', 'updated_at',
            ]),
            'lastSyncedAt' => Issue::max('synced_at'),
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
                'comment' => $open?->comment,
                'segments' => $top->segments->map(fn ($s) => [
                    'started_at' => $s->started_at,
                    'ended_at' => $s->ended_at,
                    'comment' => $s->comment,
                ])->values(),
            ],
            'paused' => $live->skip(1)->map($row)->values(),
        ];
    }
}
