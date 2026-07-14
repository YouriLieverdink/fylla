<?php

namespace App\Http\Controllers;

use App\Jobs\SyncGithubPullRequests;
use App\Jobs\SyncKendoIssues;
use App\Jobs\SyncKendoProjects;
use App\Jobs\SyncKendoWorklogs;
use App\Models\Issue;
use App\Models\PullRequest;
use App\Models\Timer;
use App\Services\TimerService;
use App\Utilization\UtilizationReport;
use Inertia\Inertia;
use Inertia\Response;

class IssueController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    /** List issues from the local table (never live Kendo, per ADR-0003). */
    public function index(): Response
    {
        $live = Timer::live()->with(['timeable', 'segments.notes'])->get();

        // max() is a raw aggregate — reparse as UTC so it serializes with a tz
        // marker (the frontend reads a naive string as local, showing it off).
        $syncedAt = Issue::max('synced_at');

        return Inertia::render('Issues', [
            // Only issues from the latest sync = current open work. Done issues
            // leave the my-issues feed; ones with local history are retained in
            // the DB (for their worklogs) but keep an older synced_at, so this
            // filters them out of the list without deleting them.
            'issues' => Issue::where('synced_at', $syncedAt)
                ->orderByDesc('updated_at')->get([
                    'id', 'key', 'title', 'priority', 'type', 'project_id',
                    'estimated_minutes', 'remaining_minutes', 'updated_at',
                ])->each->append('kendo_url'),
            'pullRequests' => PullRequest::orderByDesc('synced_at')->get([
                'id', 'number', 'repo', 'title', 'url', 'head_ref',
                'suggested_key', 'kendo_key', 'kendo_project_id', 'resolved_at',
            ])->each->append('kendo_url'),
            'liveIssueIds' => $this->liveIds($live, Issue::class),
            'livePrIds' => $this->liveIds($live, PullRequest::class),
            'timer' => $this->stack($live),
            'utilization' => (new UtilizationReport)->generate(),
        ]);
    }

    /** Manual "sync now" — runs the job inline so back() returns fresh data. */
    public function sync(): \Illuminate\Http\RedirectResponse
    {
        try {
            SyncKendoIssues::dispatchSync();
            // Projects before worklogs: the billability join needs project rows
            // present on the first run.
            SyncKendoProjects::dispatchSync();
            SyncKendoWorklogs::dispatchSync();
            SyncGithubPullRequests::dispatchSync();
        } catch (\Throwable $e) {
            return back()->with('syncError', true);
        }

        return back();
    }

    /** Subject ids of live timers for one morph type — drives the "live" badge. */
    private function liveIds($live, string $type)
    {
        return $live->where('timeable_type', $type)->pluck('timeable_id')->values();
    }

    /** Display key for a timed subject: an issue's key, or a PR's resolved key. */
    private function subjectKey(mixed $subject): string
    {
        if ($subject instanceof PullRequest) {
            return $subject->kendo_key ?? '#'.$subject->number;
        }

        return $subject->key;
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
            'id' => $t->id,
            'key' => $this->subjectKey($t->timeable),
            'title' => $t->timeable->title,
            'accumulated_seconds' => $this->timers->accumulatedSeconds($t),
        ];

        return [
            'active' => $row($top) + [
                'running' => (bool) $open,
                'started_at' => $open?->started_at,
                'started_at_hm' => $open?->started_at->setTimezone(config('fylla.display_timezone'))->format('H:i'),
                // Notes of the open segment only (ADR-0005); empty when paused.
                'notes' => $open
                    ? $open->notes->sortBy('created_at')->values()->map(fn ($n) => [
                        'at' => $n->created_at->setTimezone(config('fylla.display_timezone'))->format('H:i'),
                        'text' => $n->text,
                    ])
                    : [],
            ],
            'paused' => $live->skip(1)->map($row)->values(),
        ];
    }
}
