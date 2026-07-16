<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\FinishedIssue;
use App\Models\SyncedWorklog;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

/**
 * Mirror the user's finished (Done-lane) Kendo issues into `finished_issues`,
 * the data source for the personal estimation feedback loop (issue #17).
 *
 * The open my-issues feed excludes the done lane, so finished issues are read
 * from the per-project issues feed instead — one call per project returns every
 * issue with its estimate AND its logged (actual) minutes. The project set is
 * just the projects the user has logged time in (distinct on their own
 * synced_worklogs), so the call count scales with projects worked, not with the
 * total project count. Slow-changing data — scheduled daily, not every 15 min.
 */
class SyncKendoFinishedIssues implements ShouldQueue
{
    use Queueable;

    public function handle(KendoClient $kendo): void
    {
        $me = (int) config('fylla.kendo_user_id');
        $now = now();

        $projectIds = SyncedWorklog::mine()
            ->whereNotNull('kendo_project_id')
            ->distinct()
            ->pluck('kendo_project_id')
            ->all();

        // When the user last logged time on each issue — the recency order for
        // the report's "recent finished issues" window.
        $lastWorked = SyncedWorklog::mine()
            ->selectRaw('kendo_issue_id, max(started_at) as last_worked_at')
            ->groupBy('kendo_issue_id')
            ->pluck('last_worked_at', 'kendo_issue_id');

        $seen = [];
        foreach ($projectIds as $projectId) {
            $doneLaneId = $this->doneLaneId($kendo->getProjectLanes($projectId));
            if ($doneLaneId === null) {
                continue;
            }

            foreach ($kendo->getProjectIssues($projectId) as $issue) {
                // Personal loop: only the user's own issues, and only once done.
                if ($issue['assignee_id'] !== $me || $issue['lane_id'] !== $doneLaneId) {
                    continue;
                }

                $seen[] = $issue['id'];
                FinishedIssue::updateOrCreate(
                    ['kendo_id' => $issue['id']],
                    [
                        'key' => $issue['key'],
                        'title' => $issue['title'],
                        'project_id' => $projectId,
                        'estimated_minutes' => $issue['estimated_minutes'],
                        'logged_minutes' => $issue['logged_minutes'],
                        'lane_id' => $issue['lane_id'],
                        'last_worked_at' => $lastWorked[$issue['id']] ?? null,
                        'synced_at' => $now,
                    ],
                );
            }
        }

        // Reconcile within the projects we just refetched: an issue that left the
        // done lane, or was reassigned away, drops out. Projects the user no
        // longer logs time in aren't refetched, so their rows are left untouched.
        FinishedIssue::whereIn('project_id', $projectIds)
            ->whereNotIn('kendo_id', $seen)
            ->delete();
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
}
