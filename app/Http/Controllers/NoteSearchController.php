<?php

namespace App\Http\Controllers;

use App\Models\Client;
use App\Models\Developer;
use App\Models\Project;
use App\Models\SyncedWorklog;
use Illuminate\Http\Request;
use Inertia\Inertia;
use Inertia\Response;

class NoteSearchController extends Controller
{
    /** Rows returned per request; the corpus is ~1k rows, this keeps the page light. */
    private const LIMIT = 200;

    /**
     * Notes search page (#70): free-text over synced worklogs, matching
     * note + issue key/title with plain LIKE, newest-first. Noteless worklogs
     * are listed too — hiding them made non-note-writers look like they logged
     * nothing. Team read — deliberately unscoped (ADR-0011): the corpus is
     * teammates' worklogs on managed-client projects plus the user's own
     * everywhere.
     */
    public function index(Request $request): Response
    {
        $ids = fn (string $key) => array_values(array_filter(array_map('intval', (array) $request->query($key, []))));

        $filters = [
            'q' => trim((string) $request->query('q', '')),
            'clients' => $ids('clients'),
            'projects' => $ids('projects'),
            'developers' => $ids('developers'),
            'from' => $request->query('from') ?: null,
            'to' => $request->query('to') ?: null,
        ];

        // One definition of the corpus (all synced worklogs, noteless included)
        // for both the results and the filter options below — never drift apart.
        $corpus = fn () => SyncedWorklog::query();

        $query = $corpus()->orderByDesc('started_at');

        if ($filters['q'] !== '') {
            $like = '%'.$filters['q'].'%';
            $query->where(fn ($w) => $w
                ->where('note', 'like', $like)
                ->orWhere('issue_key', 'like', $like)
                ->orWhere('issue_title', 'like', $like));
        }
        if ($filters['clients']) {
            $query->whereHas('project', fn ($p) => $p->whereIn('client_id', $filters['clients']));
        }
        if ($filters['projects']) {
            $query->whereIn('kendo_project_id', $filters['projects']);
        }
        if ($filters['developers']) {
            $query->whereIn('kendo_user_id', $filters['developers']);
        }
        if ($filters['from']) {
            $query->whereDate('started_at', '>=', $filters['from']);
        }
        if ($filters['to']) {
            $query->whereDate('started_at', '<=', $filters['to']);
        }

        $total = (clone $query)->count();
        $names = Developer::pluck('name', 'kendo_id');

        $rows = $query->limit(self::LIMIT)->get()->map(fn (SyncedWorklog $w) => [
            'id' => $w->id,
            'date' => $w->started_at->format('Y-m-d'),
            'developer' => $names[$w->kendo_user_id] ?? '—',
            'issueKey' => $w->issue_key,
            'issueTitle' => $w->issue_title,
            'note' => $w->note,
            'minutes' => $w->minutes,
        ]);

        // Filter options come from the corpus itself — a client/project/developer
        // with no synced worklog can never match, so it never shows as an option.
        $projectIds = $corpus()->distinct()->pluck('kendo_project_id');

        return Inertia::render('Notes', [
            'rows' => $rows,
            'total' => $total,
            'filters' => $filters,
            'clients' => Client::whereIn('id', Project::whereIn('kendo_id', $projectIds)->whereNotNull('client_id')->pluck('client_id'))
                ->orderBy('name')->get(['id', 'name']),
            'projects' => Project::whereIn('kendo_id', $projectIds)->orderBy('name')->get(['kendo_id', 'name']),
            'developers' => Developer::whereIn('kendo_id', $corpus()->distinct()->pluck('kendo_user_id'))
                ->orderBy('name')->get(['kendo_id', 'name']),
        ]);
    }
}
