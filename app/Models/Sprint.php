<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * Minimal Kendo sprint mirror (issue #56) for the Client brief's current-sprint
 * tile. Synced per project by SyncKendoProjectIssues; status 1 = active.
 */
class Sprint extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'status' => 'integer',
        'starts_at' => 'datetime',
        'ends_at' => 'datetime',
        'synced_at' => 'datetime',
    ];
}
