<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Issue extends Model
{
    // updated_at mirrors Kendo; there is no created_at. Manage timestamps by hand.
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'updated_at' => 'datetime',
        'synced_at' => 'datetime',
        'estimated_minutes' => 'integer',
        'remaining_minutes' => 'integer',
        'due_date' => 'date',
        'not_before' => 'date',
        'up_next' => 'boolean',
        'no_split' => 'boolean',
    ];

    public function timers(): HasMany
    {
        return $this->hasMany(Timer::class);
    }

    public function worklogs(): HasMany
    {
        return $this->hasMany(Worklog::class);
    }
}
