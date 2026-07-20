<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\Project;
use App\Models\Sprint;
use App\Models\SyncedIssue;
use App\Models\SyncedWorklog;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

/**
 * Mirror Kendo issues into `synced_issues` — every assignee, every lane — for the
 * projects Fylla cares about (issue #55). Two consumers read this one table: the
 * personal estimation loop (#17, filtered to the user's done issues) and the
 * Client context page (#56, the whole team).
 *
 * Project set = union of managed-client projects (their worklogs sync team-wide,
 * ADR-0011) and the projects the user has logged time in (preserves #17 for
 * unmanaged projects). Per project: one lanes call classifies every issue into
 * first/middle/done (Kendo has no done flag), one sprints call mirrors the
 * board's sprints (Client brief), one issues call carries the mirror fields.
 *
 * Slow-changing team data — scheduled daily. `lane_entered_at` is stamped
 * forward-only (Kendo exposes no transition time, R1), so aging resolves to
 * days, which is the display unit.
 */
class SyncKendoProjectIssues implements ShouldQueue
{
    use Queueable;

    public function handle(KendoClient $kendo): void
    {
        $now = now();

        $managed = Project::whereNotNull('client_id')->pluck('kendo_id');
        $worked = SyncedWorklog::mine()
            ->whereNotNull('kendo_project_id')
            ->distinct()
            ->pluck('kendo_project_id');
        $projectIds = $managed->merge($worked)->unique()->values()->all();

        // When the user last logged time on each issue — personal recency order
        // for the estimation report (#17). Null for issues the user never timed;
        // other developers' recency has no team-wide key (R1), so it stays null.
        $lastWorked = SyncedWorklog::mine()
            ->selectRaw('kendo_issue_id, max(started_at) as last_worked_at')
            ->groupBy('kendo_issue_id')
            ->pluck('last_worked_at', 'kendo_issue_id');

        $seenIssues = [];
        $seenSprints = [];
        foreach ($projectIds as $projectId) {
            $lanes = $kendo->getProjectLanes($projectId);
            $doneLaneId = $this->doneLaneId($lanes);
            $firstLaneId = $this->firstLaneId($lanes);
            $laneNames = array_column($lanes, 'title', 'id');

            foreach ($kendo->getSprints($projectId) as $sprint) {
                $seenSprints[] = $sprint['id'];
                Sprint::updateOrCreate(
                    ['kendo_id' => $sprint['id']],
                    [
                        'project_id' => $projectId,
                        'name' => $sprint['name'],
                        'status' => $sprint['status'],
                        'starts_at' => $sprint['starts_at'],
                        'ends_at' => $sprint['ends_at'],
                        'synced_at' => $now,
                    ],
                );
            }

            foreach ($kendo->getProjectIssues($projectId) as $issue) {
                $seenIssues[] = $issue['id'];

                $existing = SyncedIssue::firstWhere('kendo_id', $issue['id']);
                $attrs = [
                    'key' => $issue['key'],
                    'title' => $issue['title'],
                    'project_id' => $projectId,
                    'assignee_id' => $issue['assignee_id'],
                    'estimated_minutes' => $issue['estimated_minutes'],
                    'logged_minutes' => $issue['logged_minutes'],
                    'lane_id' => $issue['lane_id'],
                    'lane_name' => $laneNames[$issue['lane_id']] ?? null,
                    'lane_position' => $this->lanePosition($issue['lane_id'], $firstLaneId, $doneLaneId),
                    'sprint_id' => $issue['sprint_id'],
                    'last_worked_at' => $lastWorked[$issue['id']] ?? null,
                    'synced_at' => $now,
                ];

                // Forward-only lane-entry stamp (R1): first sight, or a lane change,
                // records now(); an unchanged lane keeps the earlier stamp.
                if (! $existing || $existing->lane_id !== $issue['lane_id']) {
                    $attrs['lane_entered_at'] = $now;
                }

                SyncedIssue::updateOrCreate(['kendo_id' => $issue['id']], $attrs);
            }
        }

        // Reconcile within the projects we refetched. This mirror carries no local
        // FK/history, so a plain delete is safe (unlike the `issues` table).
        SyncedIssue::whereIn('project_id', $projectIds)
            ->whereNotIn('kendo_id', $seenIssues)
            ->delete();
        Sprint::whereIn('project_id', $projectIds)
            ->whereNotIn('kendo_id', $seenSprints)
            ->delete();
    }

    /** first | middle | done, from the lane's position on the board. */
    private function lanePosition(?int $laneId, ?int $firstLaneId, ?int $doneLaneId): string
    {
        return match ($laneId) {
            $doneLaneId => 'done',
            $firstLaneId => 'first',
            default => 'middle',
        };
    }

    /**
     * The done lane of a board. Kendo exposes no done flag, so: prefer a lane
     * literally titled "Done", else the rightmost (max order) column — the
     * convention Kendo's own my-issues feed uses to exclude finished work.
     *
     * ponytail: title/order heuristic; revisit if Kendo adds a terminal-lane flag.
     *
     * @param  array<int, array{id:int, title:?string, order:int}>  $lanes
     */
    private function doneLaneId(array $lanes): ?int
    {
        foreach ($lanes as $lane) {
            if (strcasecmp((string) $lane['title'], 'Done') === 0) {
                return $lane['id'];
            }
        }

        if ($lanes === []) {
            return null;
        }

        usort($lanes, fn (array $a, array $b) => $b['order'] <=> $a['order']);

        return $lanes[0]['id'];
    }

    /**
     * The first lane (lowest order) — the "not started yet" column, excluded from
     * in-progress aging (#56 / F1 §4).
     *
     * @param  array<int, array{id:int, title:?string, order:int}>  $lanes
     */
    private function firstLaneId(array $lanes): ?int
    {
        if ($lanes === []) {
            return null;
        }

        usort($lanes, fn (array $a, array $b) => $a['order'] <=> $b['order']);

        return $lanes[0]['id'];
    }
}
