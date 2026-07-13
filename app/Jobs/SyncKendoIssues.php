<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\Issue;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;
use Illuminate\Support\Facades\Cache;

/**
 * Pull the current user's Kendo issues into the local `issues` table.
 *
 * Upserts Kendo-mirror fields on `kendo_id` (ADR-0004: never touches the
 * Fylla-owned columns). The my-issues feed is authoritative for "my open
 * work", so rows absent from the latest feed are deleted — UNLESS the feed
 * came back truncated, in which case absence is not conclusive.
 */
class SyncKendoIssues implements ShouldQueue
{
    use Queueable;

    public function handle(KendoClient $kendo): void
    {
        $feed = $kendo->getMyIssues();
        $now = now();
        $seen = [];

        // Estimates live on the per-project feed, not my-issues — fetch each
        // distinct project once and index the estimate fields by issue id.
        $projectIds = array_unique(array_filter(array_column($feed['issues'], 'project_id')));
        $estimates = [];
        foreach ($projectIds as $projectId) {
            $estimates += $kendo->getProjectEstimates($projectId);
        }

        foreach ($feed['issues'] as $issue) {
            $seen[] = $issue['id'];
            $estimate = $estimates[$issue['id']] ?? [];

            Issue::updateOrCreate(
                ['kendo_id' => $issue['id']],
                [
                    'key' => $issue['key'],
                    'title' => $issue['title'],
                    'priority' => $issue['priority'] ?? null,
                    'type' => $issue['type'] ?? null,
                    'estimated_minutes' => $estimate['estimated_minutes'] ?? null,
                    'remaining_minutes' => $estimate['remaining_minutes'] ?? null,
                    'lane_id' => $issue['lane_id'] ?? null,
                    'project_id' => $issue['project_id'] ?? null,
                    'epic_id' => $issue['epic_id'] ?? null,
                    'updated_at' => $issue['updated_at'] ?? null,
                    'synced_at' => $now,
                ],
            );
        }

        // Reconcile: drop issues that left the feed. Skip when truncated —
        // absence from a capped response does not mean the issue is gone.
        // Keep any issue with local timer/worklog history: you tracked time on
        // it, so it must survive even after it leaves your open-issues feed.
        if (! $feed['truncated']) {
            Issue::whereNotIn('kendo_id', $seen)
                ->whereDoesntHave('timers')
                ->whereDoesntHave('worklogs')
                ->delete();
        }

        // "Last synced" = when this job last ran, not the newest issue's
        // synced_at. An empty (or all-retained) feed stamps no rows, so
        // max(synced_at) would freeze; this advances regardless.
        Cache::forever('kendo.synced_at', $now->toJSON());
    }
}
