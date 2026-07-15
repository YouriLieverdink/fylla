<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Builder;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * Read mirror of a Kendo time entry (ADR-0007). Billability is NOT stored here:
 * it is derived from the entry's project (projects.billable), so editing the
 * billable-projects list re-classifies every worklog with no per-row re-tag.
 */
class SyncedWorklog extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'minutes' => 'integer',
        'started_at' => 'datetime',
        'synced_at' => 'datetime',
    ];

    /** The project this worklog was logged against (matched on Kendo ids). */
    public function project(): BelongsTo
    {
        return $this->belongsTo(Project::class, 'kendo_project_id', 'kendo_id');
    }

    /** Derived: billable iff its project is on the billable list. */
    public function getBillableAttribute(): bool
    {
        return (bool) ($this->project?->billable);
    }

    /** Only worklogs whose project is billable. */
    public function scopeBillable(Builder $query): Builder
    {
        return $query->whereHas('project', fn (Builder $q) => $q->where('billable', true));
    }

    /**
     * The user's own worklogs only. The mirror holds teammates' rows for
     * managed-client projects (ADR-0011); EVERY personal reader must apply this
     * scope or teammate hours inflate the utilization metric. Guarded by a
     * regression test — this is not dead code.
     */
    public function scopeMine(Builder $query): Builder
    {
        return $query->where('kendo_user_id', config('fylla.kendo_user_id'));
    }
}
