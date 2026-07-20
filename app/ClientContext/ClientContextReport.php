<?php

namespace App\ClientContext;

use App\Models\Client;
use App\Models\Developer;
use App\Models\Sprint;
use App\Models\SyncedIssue;
use App\Models\SyncedWorklog;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

/**
 * Read-only Client board (issue #56): the whole client's work on one kanban —
 * columns are the client's real Kendo lanes, cards are its issues, each tagged
 * with its developer and flagged when overrunning (logged > estimate) or stuck
 * (no activity, work or lane move, in the last 5 working days). The page filters
 * client-side (by developer, overrunning, stuck, done); this just ships the flat
 * data + a totals band. Never writes; never calls Kendo (ADR-0003).
 */
class ClientContextReport
{
    /** No activity (worklog or lane move) in this many working days = stuck. */
    private const STUCK_WORKING_DAYS = 5;

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
        $names = Developer::pluck('name', 'kendo_id');
        $lastWorked = $this->lastWorkedByIssue($projectIds);
        $monthByUser = SyncedWorklog::whereIn('kendo_project_id', $projectIds)
            ->whereBetween('started_at', [$this->monthStart, $this->monthEnd])
            ->selectRaw('kendo_user_id, sum(minutes) as m')
            ->groupBy('kendo_user_id')
            ->pluck('m', 'kendo_user_id');
        $sprint = $this->activeSprint($projectIds);

        $rows = SyncedIssue::whereIn('project_id', $projectIds)
            ->get()
            ->map(fn (SyncedIssue $i) => $this->card($i, $names, $lastWorked));

        return [
            'client' => $this->brief($client, $projectIds, $rows, $sprint),
            'currentSprintId' => $sprint?->kendo_id,
            'lanes' => $this->laneColumns($rows),
            'developers' => $this->developerOptions($rows, $names, $monthByUser),
            'issues' => $rows->values()->all(),
        ];
    }

    /**
     * @param  Collection<int|string,string>  $names
     * @param  Collection<int|string,string>  $lastWorked
     */
    private function card(SyncedIssue $i, Collection $names, Collection $lastWorked): array
    {
        $estimate = (int) $i->estimated_minutes;
        $logged = (int) $i->logged_minutes;
        $done = $i->lane_position === 'done';
        $over = $estimate > 0 && $logged > $estimate;

        // Last sign of life: a worklog on the issue, or its last lane move. Only
        // in-flight issues can be "stuck" — a done issue isn't waiting on anyone.
        $worklogAt = ($ts = $lastWorked[$i->kendo_id] ?? null) ? CarbonImmutable::parse($ts) : null;
        $lastActivity = collect([$worklogAt, $i->lane_entered_at?->toImmutable()])->filter()->max();
        $idleDays = $lastActivity ? $this->workingDaysBetween($lastActivity, $this->now->utc()) : null;
        $stuck = ! $done && ($idleDays === null || $idleDays > self::STUCK_WORKING_DAYS);

        return [
            'key' => $i->key,
            'title' => $i->title,
            'kendo_url' => $i->kendo_url,
            'lane' => $i->lane_name ?: 'No lane',
            'position' => $i->lane_position ?? 'middle',
            'done' => $done,
            'sprint' => $i->sprint_id,
            'assignee' => $i->assignee_id,
            'assigneeName' => $i->assignee_id ? ($names[$i->assignee_id] ?? "User {$i->assignee_id}") : 'Unassigned',
            'estimateHours' => $estimate > 0 ? $this->hours($estimate) : null,
            'loggedHours' => $this->hours($logged),
            'over' => $over,
            'overPct' => $over ? (int) round(($logged - $estimate) / $estimate * 100) : null,
            'stuck' => $stuck,
            'idleDays' => $idleDays,
        ];
    }

    /**
     * @param  array<int,int>  $projectIds
     * @param  Collection<int,array<string,mixed>>  $rows
     */
    private function brief(Client $client, array $projectIds, Collection $rows, ?Sprint $sprint): array
    {
        $minutes = (int) SyncedWorklog::whereIn('kendo_project_id', $projectIds)
            ->whereBetween('started_at', [$this->monthStart, $this->monthEnd])
            ->sum('minutes');
        $hours = (int) round($minutes / 60);
        $target = $client->monthly_target_hours;

        $active = $rows->where('done', false);
        $overrunning = $active->where('over', true)->count();
        $stuck = $active->where('stuck', true)->count();

        // Run-rate pace: hours scaled by working days in month / elapsed (Mon–Fri),
        // the same projection the Delivery card uses.
        $totalWd = $this->weekdaysInclusive($this->now->startOfMonth(), $this->now->endOfMonth());
        $elapsedWd = $this->weekdaysInclusive($this->now->startOfMonth(), $this->now);
        $projected = $elapsedWd > 0 ? (int) round($hours * $totalWd / $elapsedWd) : null;

        return [
            'name' => $client->name,
            'meta' => count($projectIds).' projects · '.$rows->pluck('assignee')->filter()->unique()->count().' developers',
            'hours' => $hours,
            'target' => $target,
            'pct' => $target ? (int) round($hours / $target * 100) : 0,
            'projected' => $projected,
            'paceDelta' => $target && $projected !== null ? $projected - $target : null,
            'activeIssues' => $active->count(),
            'overrunningCount' => $overrunning,
            'stuckCount' => $stuck,
            'flaggedCount' => $overrunning + $stuck,
            'sprint' => $sprint ? $this->sprintCard($sprint) : null,
        ];
    }

    /**
     * The client's lanes as ordered board columns: first lane, then middle lanes
     * alphabetically, then done (Kendo exposes no reliable cross-project lane
     * order, so this is the honest approximation).
     *
     * @param  Collection<int,array<string,mixed>>  $rows
     * @return array<int,array{name:string,done:bool}>
     */
    private function laneColumns(Collection $rows): array
    {
        $rank = ['first' => 0, 'middle' => 1, 'done' => 2];

        return $rows
            ->map(fn (array $i) => ['lane' => $i['lane'], 'rank' => $rank[$i['position']] ?? 1])
            ->unique('lane')
            ->sortBy([['rank', 'asc'], ['lane', 'asc']])
            ->map(fn (array $l) => ['name' => $l['lane'], 'done' => $l['rank'] === 2])
            ->values()
            ->all();
    }

    /**
     * Developers to offer as filter options + per-developer subtotals: everyone
     * assigned an issue on the client, alphabetical, with their hours logged this
     * month.
     *
     * @param  Collection<int,array<string,mixed>>  $rows
     * @param  Collection<int|string,string>  $names
     * @param  Collection<int,int>  $monthByUser  kendo_user_id → minutes this month
     * @return array<int,array{id:int,name:string,hoursMonth:float}>
     */
    private function developerOptions(Collection $rows, Collection $names, Collection $monthByUser): array
    {
        return $rows
            ->pluck('assignee')
            ->filter()
            ->unique()
            ->map(fn (int $id) => [
                'id' => $id,
                'name' => $names[$id] ?? "User {$id}",
                'hoursMonth' => $this->hours((int) ($monthByUser[$id] ?? 0)),
            ])
            ->sortBy('name')
            ->values()
            ->all();
    }

    /**
     * Last worklog time per issue (any developer) — the recency signal for stuck.
     *
     * @param  array<int,int>  $projectIds
     * @return Collection<int,string>  keyed by kendo_issue_id
     */
    private function lastWorkedByIssue(array $projectIds): Collection
    {
        return SyncedWorklog::whereIn('kendo_project_id', $projectIds)
            ->whereNotNull('kendo_issue_id')
            ->selectRaw('kendo_issue_id, max(started_at) as t')
            ->groupBy('kendo_issue_id')
            ->pluck('t', 'kendo_issue_id');
    }

    /** @param  array<int,int>  $projectIds */
    private function activeSprint(array $projectIds): ?Sprint
    {
        return Sprint::whereIn('project_id', $projectIds)
            ->where('status', 1)
            ->orderByRaw('ends_at is null, ends_at asc')
            ->first();
    }

    private function sprintCard(Sprint $sprint): array
    {
        $inSprint = SyncedIssue::where('sprint_id', $sprint->kendo_id);
        $tz = config('fylla.display_timezone');
        $ends = $sprint->ends_at?->setTimezone($tz);
        $starts = $sprint->starts_at?->setTimezone($tz);

        return [
            'name' => $sprint->name ?? 'Current sprint',
            'dates' => $starts && $ends ? $starts->format('M j').' – '.$ends->format('M j') : null,
            'done' => (clone $inSprint)->where('lane_position', 'done')->count(),
            'total' => (clone $inSprint)->count(),
            'daysLeft' => $ends ? max(0, (int) $this->now->startOfDay()->diffInDays($ends->startOfDay(), false)) : null,
        ];
    }

    /** Mon–Fri days from $from through $to, inclusive of both ends (for pace). */
    private function weekdaysInclusive(CarbonImmutable $from, CarbonImmutable $to): int
    {
        $count = 0;
        for ($d = $from->startOfDay(); $d->lte($to); $d = $d->addDay()) {
            if ($d->isWeekday()) {
                $count++;
            }
        }

        return $count;
    }

    /** Mon–Fri days from $from through $to (both instants), for the stuck cutoff. */
    private function workingDaysBetween(CarbonImmutable $from, CarbonImmutable $to): int
    {
        if ($from->gte($to)) {
            return 0;
        }

        $count = 0;
        for ($d = $from->startOfDay()->addDay(); $d->lte($to); $d = $d->addDay()) {
            if ($d->isWeekday()) {
                $count++;
            }
        }

        return $count;
    }

    /** Minutes → hours, one decimal. */
    private function hours(int $minutes): float
    {
        return round($minutes / 60, 1);
    }
}
