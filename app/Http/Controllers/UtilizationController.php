<?php

namespace App\Http\Controllers;

use App\Models\SyncedWorklog;
use App\Utilization\UtilizationReport;
use Carbon\CarbonImmutable;
use Inertia\Inertia;
use Inertia\Response;

class UtilizationController extends Controller
{
    /** The data behind the 75%: window totals, weekly breakdown, raw entries. */
    public function index(): Response
    {
        $windowWeeks = (int) config('fylla.utilization_window_weeks');
        $windowStart = CarbonImmutable::now()
            ->startOfWeek(CarbonImmutable::MONDAY)
            ->subWeeks($windowWeeks - 1);

        $entries = SyncedWorklog::mine()
            ->where('started_at', '>=', $windowStart)
            ->with('project:kendo_id,name,billable')
            ->orderByDesc('started_at')
            ->get()
            ->map(fn (SyncedWorklog $w) => [
                'id' => $w->id,
                'date' => $w->started_at->toDateString(),
                'time' => $w->started_at->format('H:i'),
                'issueKey' => $w->issue_key,
                'issueTitle' => $w->issue_title,
                'project' => $w->project?->name,
                'billable' => $w->billable,
                'minutes' => $w->minutes,
                'note' => $w->note,
            ]);

        return Inertia::render('Utilization', [
            'report' => (new UtilizationReport)->breakdown(),
            'windowWeeks' => $windowWeeks,
            'entries' => $entries,
        ]);
    }
}
