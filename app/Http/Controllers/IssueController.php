<?php

namespace App\Http\Controllers;

use App\Jobs\SyncGithubPullRequests;
use App\Jobs\SyncKendoIssues;
use App\Jobs\SyncKendoProjects;
use App\Jobs\SyncKendoWorklogs;
use App\Kendo\Client as KendoClient;
use App\Models\Draft;
use App\Models\Issue;
use App\Models\PullRequest;
use App\Models\Timer;
use App\Services\TimerService;
use App\Services\WorklistScorer;
use App\Utilization\UtilizationReport;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Arr;
use Illuminate\Validation\Rule;
use Inertia\Inertia;
use Inertia\Response;

class IssueController extends Controller
{
    public function __construct(private readonly TimerService $timers) {}

    /**
     * The Worklist: issues + PRs merged into one list, ranked by the weighted
     * composite score (ADR-0013), recomputed here every render. Reads the local
     * table only (never live Kendo, per ADR-0003).
     */
    public function index(WorklistScorer $scorer): Response
    {
        $live = Timer::live()->with(['timeable', 'segments.notes'])->get();

        // max() is a raw aggregate — reparse as UTC so it serializes with a tz
        // marker (the frontend reads a naive string as local, showing it off).
        $syncedAt = Issue::max('synced_at');
        $now = now();

        // Only issues from the latest sync = current open work. Done issues leave
        // the my-issues feed; ones with local history are retained (for their
        // worklogs) with an older synced_at, so this filters them out here.
        $issues = Issue::where('synced_at', $syncedAt)->get()
            ->map(fn (Issue $i) => [
                'kind' => 'issue',
                'id' => $i->id,
                'key' => $i->key,
                'title' => $i->title,
                'priority' => $i->priority,
                'type' => $i->type,
                'estimated_minutes' => $i->estimated_minutes,
                'remaining_minutes' => $i->remaining_minutes,
                'up_next' => (bool) $i->up_next,
                'due_date' => $i->due_date?->toDateString(),
                'not_before' => $i->not_before?->toDateString(),
                'kendo_url' => $i->kendo_url,
            ] + $scorer->scoreIssue($i, $now));

        // Only PRs from the latest GitHub sync = still open. Merged PRs leave the
        // feed; ones with local timer history are retained (for their worklogs)
        // with a stale synced_at, so this filters them out without deleting them —
        // the same freshness rule the issues list uses above.
        $prs = PullRequest::where('synced_at', PullRequest::max('synced_at'))
            ->orderByDesc('synced_at')->get()
            ->map(fn (PullRequest $p) => [
                'kind' => 'pr',
                'id' => $p->id,
                'number' => $p->number,
                'repo' => $p->repo,
                'title' => $p->title,
                'url' => $p->url,
                'suggested_key' => $p->suggested_key,
                'kendo_key' => $p->kendo_key,
                'resolved_at' => $p->resolved_at,
                'kendo_url' => $p->kendo_url,
            ] + $scorer->scorePr($p, $now));

        // Fylla-native drafts (ADR-0012): a third source, ranked by the same
        // scorer. No provider, so no timer affordance and no Kendo write-through.
        $drafts = Draft::get()
            ->map(fn (Draft $d) => [
                'kind' => 'draft',
                'id' => $d->id,
                'title' => $d->title,
                'priority' => $d->priority,
                'up_next' => (bool) $d->up_next,
                'due_date' => $d->due_date?->toDateString(),
                'not_before' => $d->not_before?->toDateString(),
            ] + $scorer->scoreDraft($d, $now));

        // Score desc; a stable key breaks ties (sort isn't guaranteed stable).
        $tie = fn (array $x) => match ($x['kind']) {
            'issue' => (string) $x['key'],
            'pr' => $x['repo'].'#'.$x['number'],
            'draft' => 'draft#'.$x['id'],
        };
        $items = $issues->concat($prs)->concat($drafts)
            ->sort(fn ($a, $b) => $b['score'] <=> $a['score'] ?: strcmp($tie($a), $tie($b)))
            ->values();

        return Inertia::render('Worklist', [
            'items' => $items,
            'liveIssueIds' => $this->liveIds($live, Issue::class),
            'livePrIds' => $this->liveIds($live, PullRequest::class),
            'timer' => $this->stack($live),
            'utilization' => (new UtilizationReport)->generate(),
        ]);
    }

    /**
     * Edit an issue's Kendo-owned fields (priority, estimate) and/or its
     * Fylla-native scheduling fields from the Worklist (ADR-0014). Two isolated
     * write paths: local fields go straight to the column and always succeed;
     * the Kendo fields are a single synchronous read-modify-write — if that
     * fails, the local-field edits in the same save are kept and the Kendo
     * fields are left unchanged.
     */
    public function update(Request $request, Issue $issue, KendoClient $kendo): RedirectResponse
    {
        $data = $request->validate([
            'up_next' => ['sometimes', 'boolean'],
            'due_date' => ['sometimes', 'nullable', 'date'],
            'not_before' => ['sometimes', 'nullable', 'date'],
            'priority' => ['sometimes', Rule::in(array_keys(WorklistScorer::PRIORITY_RANK))],
            'estimated_minutes' => ['sometimes', 'nullable', 'integer', 'min:0'],
        ]);

        $local = Arr::only($data, ['up_next', 'due_date', 'not_before']);
        if ($local !== []) {
            $issue->update($local);
        }

        // Kendo-mirror fields, only those actually changed (skips a needless PUT).
        $kendoEdits = [];
        foreach (['priority', 'estimated_minutes'] as $field) {
            if (array_key_exists($field, $data) && $data[$field] !== $issue->{$field}) {
                $kendoEdits[$field] = $data[$field];
            }
        }

        if ($kendoEdits !== []) {
            try {
                // Round-trip the full object and mutate only the changed keys —
                // reconstructing from the partial mirror would clobber the rest.
                $remote = $kendo->getIssue($issue->project_id, $issue->kendo_id);
                if (array_key_exists('priority', $kendoEdits)) {
                    $remote['priority'] = KendoClient::priorityToInt($kendoEdits['priority']);
                }
                if (array_key_exists('estimated_minutes', $kendoEdits)) {
                    // ponytail: assumes Kendo's write key matches the read feed; verify against live API.
                    $remote['estimated_minutes'] = $kendoEdits['estimated_minutes'];
                }
                $kendo->updateIssue($issue->project_id, $issue->kendo_id, $remote);
                $issue->update($kendoEdits); // optimistic; next sync reconfirms
            } catch (\Throwable $e) {
                return back()->withErrors(['priority' => 'Could not save changes to Kendo.']);
            }
        }

        return back();
    }

    /** Manual "sync now" — runs the job inline so back() returns fresh data. */
    public function sync(): RedirectResponse
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
