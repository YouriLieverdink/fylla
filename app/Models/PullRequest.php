<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\MorphMany;

class PullRequest extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'synced_at' => 'datetime',
        'resolved_at' => 'datetime',
        'opened_at' => 'datetime',
    ];

    public function timers(): MorphMany
    {
        return $this->morphMany(Timer::class, 'timeable');
    }

    /**
     * Kendo coordinates a Worklog books to (ADR-0009), or null while unresolved.
     * A PR cannot be timed until resolved, so a timed PR always yields non-null.
     *
     * @return array{project_id: ?int, issue_id: ?int}|null
     */
    public function kendoCoords(): ?array
    {
        if ($this->resolved_at === null) {
            return null;
        }

        return ['project_id' => $this->kendo_project_id, 'issue_id' => $this->kendo_issue_id];
    }

    /** Deep link to the resolved Kendo issue in the web UI, or null if unresolved. */
    public function getKendoUrlAttribute(): ?string
    {
        if ($this->resolved_at === null) {
            return null;
        }

        return rtrim((string) config('services.kendo.base_url'), '/')
            ."/projects/{$this->kendo_project_id}/issues/{$this->kendo_key}";
    }
}
