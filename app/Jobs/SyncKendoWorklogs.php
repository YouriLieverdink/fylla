<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\Project;
use App\Models\SyncedWorklog;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

/**
 * Pull the user's Kendo time entries into the `synced_worklogs` read mirror
 * (ADR-0007) over a rolling window.
 *
 * The admin token returns the whole team; a row is kept when it is the user's
 * OR its project is assigned to a client (managed, ADR-0011). Upserts on
 * `kendo_worklog_id`, then
 * reconciles: rows whose `started_at` is inside the fetched window but absent
 * from the latest feed were deleted in Kendo and are dropped here. Rows OUTSIDE
 * the window are never touched — absence there proves nothing (the analogue of
 * the issues sync's truncated-feed guard).
 */
class SyncKendoWorklogs implements ShouldQueue
{
    use Queueable;

    public function handle(KendoClient $kendo): void
    {
        $now = now();
        $from = $now->copy()->subDays((int) config('fylla.worklog_sync_days'));
        $userId = config('fylla.kendo_user_id');

        $entries = $kendo->getTimeEntries($from->toDateString(), $now->toDateString());

        // Managed projects (assigned to a client, ADR-0011) pull the whole team;
        // keyed by Kendo project id for lookup against the feed.
        $managed = Project::whereNotNull('client_id')->pluck('kendo_id')->flip();

        $seen = [];
        foreach ($entries as $entry) {
            $mine = (string) $entry['user_id'] === (string) $userId;
            if (! $mine && ! $managed->has($entry['project_id'])) {
                continue;
            }

            $seen[] = $entry['id'];

            SyncedWorklog::updateOrCreate(
                ['kendo_worklog_id' => $entry['id']],
                [
                    'kendo_user_id' => $entry['user_id'] ?? null,
                    'kendo_issue_id' => $entry['issue_id'] ?? null,
                    'kendo_project_id' => $entry['project_id'] ?? null,
                    'minutes' => $entry['minutes'] ?? 0,
                    'started_at' => $entry['started_at'],
                    'note' => $entry['note'] ?? null,
                    'issue_key' => $entry['issue_key'] ?? null,
                    'issue_title' => $entry['issue_title'] ?? null,
                    'synced_at' => $now,
                ],
            );
        }

        // Reconcile only within the fetched window; leave older rows alone.
        SyncedWorklog::where('started_at', '>=', $from)
            ->whereNotIn('kendo_worklog_id', $seen)
            ->delete();
    }
}
