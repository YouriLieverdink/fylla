<?php

namespace App\Http\Controllers;

use App\Models\SyncedWorklog;
use App\Utilization\UtilizationProjection;
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
                // Stored UTC; render in the display zone (Amsterdam) so the wall
                // clock matches when the work happened.
                'date' => $w->started_at->timezone(config('fylla.display_timezone'))->toDateString(),
                'time' => $w->started_at->timezone(config('fylla.display_timezone'))->format('H:i'),
                'issueKey' => $w->issue_key,
                'issueTitle' => $w->issue_title,
                'project' => $w->project?->name,
                'billable' => $w->billable,
                'minutes' => $w->minutes,
                'note' => $w->note,
            ]);

        $report = new UtilizationReport;

        return Inertia::render('Utilization', [
            'report' => $report->breakdown(),
            // Sibling key, not nested: breakdown()'s contract stays as it is.
            'projection' => (new UtilizationProjection($report))->payload(),
            'windowWeeks' => $windowWeeks,
            'entries' => $entries,
        ]);
    }
}
