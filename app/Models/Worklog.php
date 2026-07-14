<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Worklog extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'minutes' => 'integer',
        'kendo_project_id' => 'integer',
        'kendo_issue_id' => 'integer',
        'started_at' => 'datetime',
        'posted_at' => 'datetime',
    ];

    /** Nullable provenance — PR worklogs have no local issue (ADR-0009). */
    public function issue(): BelongsTo
    {
        return $this->belongsTo(Issue::class);
    }

    public function timer(): BelongsTo
    {
        return $this->belongsTo(Timer::class);
    }
}
