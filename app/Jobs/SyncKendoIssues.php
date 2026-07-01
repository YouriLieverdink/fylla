<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\Issue;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

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

        foreach ($feed['issues'] as $issue) {
            $seen[] = $issue['id'];

            Issue::updateOrCreate(
                ['kendo_id' => $issue['id']],
                [
                    'key' => $issue['key'],
                    'title' => $issue['title'],
                    'priority' => $issue['priority'] ?? null,
                    'type' => $issue['type'] ?? null,
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
        if (! $feed['truncated']) {
            Issue::whereNotIn('kendo_id', $seen)->delete();
        }
    }
}
