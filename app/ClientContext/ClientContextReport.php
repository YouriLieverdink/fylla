<?php

namespace App\ClientContext;

use App\Estimation\EstimationReport;
use App\Models\Client;
use App\Models\Developer;
use App\Models\Sprint;
use App\Models\SyncedIssue;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

/**
 * Read-only Client context page (issue #56): a single-column report over the
 * `synced_issues` team mirror + the `developers` roster, scoped to one managed
 * client. Never writes; never calls Kendo (ADR-0003) — the background jobs
 * (SyncKendoProjectIssues + SyncKendoUsers) fill the tables this reads.
 *
 * Four sections (Variant A, #57): a brief stat-band, a per-developer
 * estimate-vs-actual table (rolling-20 bias, D1), and two attention panels —
 * overrunning-now and in-progress-aging.
 */
class ClientContextReport
{
    /** Rolling window per developer, matching the personal loop (D1). */
    private const WINDOW = 20;

    /** ±this % counts as an on-target estimate. */
    private const WITHIN = 15;

    private CarbonImmutable $now;
    private CarbonImmutable $monthStart;
    private CarbonImmutable $monthEnd;

    public function __construct(?CarbonImmutable $now = null)
    {
        $tz = config('fylla.display_timezone');
        $this->now = ($now ?? CarbonImmutable::now())->setTimezone($tz);
        $this->monthStart = $this->now->startOfMonth()->utc();
        $this->monthEnd = $this->now->endOfMonth()->utc();
    }

    /** @return array<string,mixed> the whole page payload for one client. */
    public function generate(Client $client): array
    {
        $projectIds = $client->projects->pluck('kendo_id')->all();

        $overrunning = $this->overrunning($projectIds);
        $aging = $this->aging($projectIds);
        $developers = $this->developers($projectIds);

        return [
            'client' => $this->brief($client, $projectIds, $overrunning, $aging, $developers),
            'developers' => $developers,
            'overrunning' => $overrunning,
            'aging' => $aging,
            'devById' => $this->devById($developers, $overrunning, $aging),
        ];
    }

    /**
     * Brief stat-band: hours-vs-target this month, active issues, current sprint,
     * needs-attention count.
     *
     * @param  array<int,int>  $projectIds
     * @param  array<int,array<string,mixed>>  $overrunning
     * @param  array<int,array<string,mixed>>  $aging
     * @param  array<int,array<string,mixed>>  $developers
     */
    private function brief(Client $client, array $projectIds, array $overrunning, array $aging, array $developers): array
    {
        // Team hours this month (unscoped, like Delivery — every developer's rows).
        $minutes = (int) SyncedWorklog::whereIn('kendo_project_id', $projectIds)
            ->whereBetween('started_at', [$this->monthStart, $this->monthEnd])
            ->sum('minutes');
        $hours = (int) round($minutes / 60);
        $target = $client->monthly_target_hours;

        $activeIssues = SyncedIssue::whereIn('project_id', $projectIds)
            ->where('lane_position', '!=', 'done')
            ->count();

        return [
            'name' => $client->name,
            'meta' => count($projectIds).' projects · '.count($developers).' developers',
            'hours' => $hours,
            'target' => $target,
            'pct' => $target ? (int) round($hours / $target * 100) : 0,
            'activeIssues' => $activeIssues,
            'sprint' => $this->sprint($projectIds),
            'overrunningCount' => count($overrunning),
            'agingCount' => count($aging),
        ];
    }

    /**
     * The client's current sprint: the active one (status 1) ending soonest, if
     * any board has one running. done/total from synced_issues membership.
     *
     * @param  array<int,int>  $projectIds
     */
    private function sprint(array $projectIds): ?array
    {
        $sprint = Sprint::whereIn('project_id', $projectIds)
            ->where('status', 1)
            ->orderByRaw('ends_at is null, ends_at asc')
            ->first();

        if (! $sprint) {
            return null;
        }

        $inSprint = SyncedIssue::where('sprint_id', $sprint->kendo_id);
        $total = (clone $inSprint)->count();
        $done = (clone $inSprint)->where('lane_position', 'done')->count();

        $tz = config('fylla.display_timezone');
        $ends = $sprint->ends_at?->setTimezone($tz);
        $starts = $sprint->starts_at?->setTimezone($tz);

        return [
            'name' => $sprint->name ?? 'Current sprint',
            'dates' => $starts && $ends ? $starts->format('M j').' – '.$ends->format('M j') : null,
            'done' => $done,
            'total' => $total,
            'daysLeft' => $ends ? max(0, (int) $this->now->startOfDay()->diffInDays($ends->startOfDay(), false)) : null,
        ];
    }

    /**
     * One row per developer with any work (any lane) on the client — the client's
     * dev team, not just the ones with completed estimates. Where a developer has
     * done+estimated issues, the row carries estimate-vs-actual over their
     * rolling-20 window (D1: ordered uniformly by lane_entered_at desc, nulls
     * last, median est/act, bias, within-±15%); otherwise it's a data-less row
     * (hasData=false). Rows with data sort first, then alphabetical.
     *
     * @param  array<int,int>  $projectIds
     * @return array<int,array<string,mixed>>
     */
    private function developers(array $projectIds): array
    {
        $names = Developer::pluck('name', 'kendo_id');

        // Rolling-20 estimate-vs-actual sample per developer, keyed by assignee.
        $samples = SyncedIssue::whereIn('project_id', $projectIds)
            ->where('lane_position', 'done')
            ->where('estimated_minutes', '>', 0)
            ->whereNotNull('assignee_id')
            ->orderByRaw('lane_entered_at is null, lane_entered_at desc')
            ->get()
            ->groupBy('assignee_id');

        // Every developer assigned any issue on the client (excludes unassigned).
        $assigneeIds = SyncedIssue::whereIn('project_id', $projectIds)
            ->whereNotNull('assignee_id')
            ->distinct()
            ->pluck('assignee_id');

        return $assigneeIds
            ->map(fn (int $id) => $this->developerRow($id, $names[$id] ?? "User {$id}", ($samples->get($id) ?? collect())->take(self::WINDOW)))
            ->sortBy(fn (array $d) => ($d['hasData'] ? '0' : '1').'_'.mb_strtolower($d['name']))
            ->values()
            ->all();
    }

    /** @param  Collection<int,SyncedIssue>  $sample */
    private function developerRow(int $id, string $name, Collection $sample): array
    {
        if ($sample->isEmpty()) {
            return [
                'id' => $id, 'name' => $name, 'hasData' => false,
                'medianEst' => null, 'medianActual' => null, 'biasPct' => null, 'withinPct' => null, 'sample' => 0,
            ];
        }

        $estimate = (int) $sample->sum('estimated_minutes');
        $actual = (int) $sample->sum('logged_minutes');
        $within = $sample->filter(function (SyncedIssue $i) {
            $pct = EstimationReport::biasPct((int) $i->estimated_minutes, (int) $i->logged_minutes);

            return $pct !== null && abs($pct) <= self::WITHIN;
        })->count();

        return [
            'id' => $id,
            'name' => $name,
            'hasData' => true,
            'medianEst' => $this->median($sample->map(fn (SyncedIssue $i) => (int) $i->estimated_minutes)),
            'medianActual' => $this->median($sample->map(fn (SyncedIssue $i) => (int) $i->logged_minutes)),
            'biasPct' => EstimationReport::biasPct($estimate, $actual) ?? 0,
            'withinPct' => (int) round($within / $sample->count() * 100),
            'sample' => $sample->count(),
        ];
    }

    /**
     * Overrunning now: in-flight issues (not done) where logged > estimate,
     * worst overrun first.
     *
     * @param  array<int,int>  $projectIds
     * @return array<int,array<string,mixed>>
     */
    private function overrunning(array $projectIds): array
    {
        return SyncedIssue::whereIn('project_id', $projectIds)
            ->where('lane_position', '!=', 'done')
            ->where('estimated_minutes', '>', 0)
            ->whereColumn('logged_minutes', '>', 'estimated_minutes')
            ->get()
            ->map(fn (SyncedIssue $i) => [
                'key' => $i->key,
                'title' => $i->title,
                'assignee' => $i->assignee_id,
                'est' => $this->hours((int) $i->estimated_minutes),
                'logged' => $this->hours((int) $i->logged_minutes),
                'overPct' => EstimationReport::biasPct((int) $i->estimated_minutes, (int) $i->logged_minutes),
            ])
            ->sortByDesc('overPct')
            ->values()
            ->all();
    }

    /**
     * In-progress aging: middle-lane issues by time in lane, longest first.
     *
     * @param  array<int,int>  $projectIds
     * @return array<int,array<string,mixed>>
     */
    private function aging(array $projectIds): array
    {
        return SyncedIssue::whereIn('project_id', $projectIds)
            ->where('lane_position', 'middle')
            ->orderByRaw('lane_entered_at is null, lane_entered_at asc')
            ->get()
            ->map(fn (SyncedIssue $i) => [
                'key' => $i->key,
                'title' => $i->title,
                'assignee' => $i->assignee_id,
                'lane' => $i->lane_name,
                'days' => $i->lane_entered_at
                    ? (int) $i->lane_entered_at->diffInDays($this->now)
                    : null,
            ])
            ->all();
    }

    /**
     * id → {name} for every developer referenced anywhere on the page (the table
     * skips those without estimates, but they can still own flagged issues).
     *
     * @param  array<int,array<string,mixed>>  $developers
     * @param  array<int,array<string,mixed>>  $overrunning
     * @param  array<int,array<string,mixed>>  $aging
     * @return array<int,array{name:string}>
     */
    private function devById(array $developers, array $overrunning, array $aging): array
    {
        $names = Developer::pluck('name', 'kendo_id');

        $ids = collect($developers)->pluck('id')
            ->merge(collect($overrunning)->pluck('assignee'))
            ->merge(collect($aging)->pluck('assignee'))
            ->filter()
            ->unique();

        return $ids->mapWithKeys(fn (int $id) => [$id => ['name' => $names[$id] ?? "User {$id}"]])->all();
    }

    /** Median of a collection of minutes → hours, one decimal (0 if empty). */
    private function median(Collection $minutes): float
    {
        $sorted = $minutes->sort()->values();
        $count = $sorted->count();
        if ($count === 0) {
            return 0.0;
        }

        $mid = intdiv($count, 2);
        $median = $count % 2
            ? $sorted[$mid]
            : ($sorted[$mid - 1] + $sorted[$mid]) / 2;

        return $this->hours((int) round($median));
    }

    /** Minutes → hours, one decimal. */
    private function hours(int $minutes): float
    {
        return round($minutes / 60, 1);
    }
}
