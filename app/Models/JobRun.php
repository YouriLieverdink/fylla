<?php

namespace App\Models;

use App\Jobs\PostWorklog;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * One background job run (#87). Written by JobRunRecorder off the queue events
 * (plus `worklog_id`, stamped by PostWorklog itself, #89); the /activity page
 * reads it. `started_at` is the creation marker, so this model carries no
 * Eloquent timestamps.
 */
class JobRun extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'started_at' => 'datetime',
        'finished_at' => 'datetime',
        'attempts' => 'integer',
    ];

    /** The retry subject — set on PostWorklog runs only (#89). */
    public function worklog(): BelongsTo
    {
        return $this->belongsTo(Worklog::class);
    }

    /**
     * Only failed worklog posts can be retried: a failed sync self-heals on the
     * next scheduled run (#86). Runs predating #89 carry no worklog_id.
     */
    public function isRetryable(): bool
    {
        return $this->status === 'failed'
            && $this->job_class === PostWorklog::class
            && $this->worklog_id !== null;
    }
}
