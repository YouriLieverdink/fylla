<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;
use Illuminate\Database\Eloquent\Relations\MorphMany;

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

    public function timers(): MorphMany
    {
        return $this->morphMany(Timer::class, 'timeable');
    }

    public function worklogs(): HasMany
    {
        return $this->hasMany(Worklog::class);
    }

    /**
     * Kendo coordinates a Worklog books to (ADR-0009) — an issue books to its
     * own mirror fields.
     *
     * @return array{project_id: ?int, issue_id: ?int}
     */
    public function kendoCoords(): array
    {
        return ['project_id' => $this->project_id, 'issue_id' => $this->kendo_id];
    }

    /** Deep link to the issue in the Kendo web UI. */
    public function getKendoUrlAttribute(): string
    {
        return rtrim((string) config('services.kendo.base_url'), '/')
            ."/projects/{$this->project_id}/issues/{$this->key}";
    }
}
