<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * Team-wide mirror of Kendo issues on managed-client + user-worked projects
 * (issue #55): every assignee, every lane. Rebuilt each sync by
 * SyncKendoProjectIssues; carries no Fylla-owned fields. Feeds both the personal
 * estimation loop (#17, filtered to the user's done issues) and the Client
 * context page (#56, whole team).
 */
class SyncedIssue extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'estimated_minutes' => 'integer',
        'logged_minutes' => 'integer',
        // Cast so the lane-change comparison in SyncKendoProjectIssues stays
        // int-vs-int on MySQL/Postgres (else a string round-trip re-stamps
        // lane_entered_at every sync, resetting aging).
        'lane_id' => 'integer',
        'last_worked_at' => 'datetime',
        'lane_entered_at' => 'datetime',
        'synced_at' => 'datetime',
    ];

    /** Deep link to the issue in the Kendo web UI (mirrors Issue::kendo_url). */
    public function getKendoUrlAttribute(): string
    {
        return rtrim((string) config('services.kendo.base_url'), '/')
            ."/projects/{$this->project_id}/issues/{$this->key}";
    }
}
