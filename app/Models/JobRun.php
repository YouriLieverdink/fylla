<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * One background job run (#87). Written only by JobRunRecorder off the queue
 * events; the /activity page reads it. `started_at` is the creation marker, so
 * this model carries no Eloquent timestamps.
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
}
