<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\SyncedWorklog;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

/**
 * Pull the user's Kendo time entries into the `synced_worklogs` read mirror
 * (ADR-0007) over a rolling window.
 *
 * The admin token returns the whole team, so rows are filtered to
 * `fylla.kendo_user_id` client-side. Upserts on `kendo_worklog_id`, then
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

        $seen = [];
        foreach ($entries as $entry) {
            if ((string) $entry['user_id'] !== (string) $userId) {
                continue;
            }

            $seen[] = $entry['id'];

            SyncedWorklog::updateOrCreate(
                ['kendo_worklog_id' => $entry['id']],
                [
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
