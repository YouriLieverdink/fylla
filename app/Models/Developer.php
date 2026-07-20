<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * Kendo roster row (issue #55 / R2): id→name for the Client context page.
 * Mirrored whole from GET /api/users by SyncKendoUsers; `kendo_id` joins to
 * synced_issues.assignee_id and synced_worklogs.kendo_user_id.
 */
class Developer extends Model
{
    protected $guarded = [];

    protected $casts = [
        'active' => 'boolean',
    ];
}
