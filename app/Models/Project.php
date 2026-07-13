<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Project extends Model
{
    // synced_at is managed by the sync job; no created_at/updated_at.
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'synced_at' => 'datetime',
        'billable' => 'boolean',
    ];
}
