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
     * Notes search page (#70): free-text over synced worklog notes, matching
     * note + issue key/title with plain LIKE, newest-first. Team read —
     * deliberately unscoped (ADR-0011): the corpus is teammates' notes on
     * managed-client projects plus the user's own everywhere.
     */
    public function index(Request $request): Response
    {
        $filters = [
            'q' => trim((string) $request->query('q', '')),
            'client' => (int) $request->query('client') ?: null,
            'project' => (int) $request->query('project') ?: null,
            'developer' => (int) $request->query('developer') ?: null,
            'from' => $request->query('from') ?: null,
            'to' => $request->query('to') ?: null,
        ];

        $query = SyncedWorklog::query()
            ->whereNotNull('note')
            ->where('note', '!=', '')
            ->orderByDesc('started_at');

        if ($filters['q'] !== '') {
            $like = '%'.$filters['q'].'%';
            $query->where(fn ($w) => $w
                ->where('note', 'like', $like)
                ->orWhere('issue_key', 'like', $like)
                ->orWhere('issue_title', 'like', $like));
        }
        if ($filters['client']) {
            $query->whereHas('project', fn ($p) => $p->where('client_id', $filters['client']));
        }
        if ($filters['project']) {
            $query->where('kendo_project_id', $filters['project']);
        }
        if ($filters['developer']) {
            $query->where('kendo_user_id', $filters['developer']);
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

        return Inertia::render('Notes', [
            'rows' => $rows,
            'total' => $total,
            'filters' => $filters,
            'clients' => Client::orderBy('name')->get(['id', 'name']),
            'projects' => Project::orderBy('name')->get(['kendo_id', 'name']),
            'developers' => Developer::orderBy('name')->get(['kendo_id', 'name']),
        ]);
    }
}
