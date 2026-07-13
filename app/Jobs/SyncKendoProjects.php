<?php

namespace App\Jobs;

use App\Kendo\Client as KendoClient;
use App\Models\Project;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Queue\Queueable;

/**
 * Mirror Kendo projects into the local `projects` table.
 *
 * Upserts on `kendo_id`, writing only Kendo-mirror fields — the Fylla-owned
 * `billable` flag (ADR-0004) is preserved across sync. Never deletes: a project
 * dropping off the feed must not silently un-classify its worklog history.
 */
class SyncKendoProjects implements ShouldQueue
{
    use Queueable;

    public function handle(KendoClient $kendo): void
    {
        $now = now();

        foreach ($kendo->getProjects() as $project) {
            Project::updateOrCreate(
                ['kendo_id' => $project['id']],
                [
                    'name' => $project['name'],
                    'code' => $project['code'] ?? null,
                    'synced_at' => $now,
                ],
            );
        }
    }
}
