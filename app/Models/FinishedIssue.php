<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * A finished (Done-lane) Kendo issue assigned to the user — the data points for
 * the personal estimation feedback loop (issue #17). Rebuilt each sync by
 * SyncKendoFinishedIssues; carries no Fylla-owned fields.
 */
class FinishedIssue extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'estimated_minutes' => 'integer',
        'logged_minutes' => 'integer',
        'last_worked_at' => 'datetime',
        'synced_at' => 'datetime',
    ];

    /** Deep link to the issue in the Kendo web UI (mirrors Issue::kendo_url). */
    public function getKendoUrlAttribute(): string
    {
        return rtrim((string) config('services.kendo.base_url'), '/')
            ."/projects/{$this->project_id}/issues/{$this->key}";
    }
}
