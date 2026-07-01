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
        'started_at' => 'datetime',
        'posted_at' => 'datetime',
    ];

    public function issue(): BelongsTo
    {
        return $this->belongsTo(Issue::class);
    }

    public function timer(): BelongsTo
    {
        return $this->belongsTo(Timer::class);
    }
}
