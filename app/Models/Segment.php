<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Segment extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'started_at' => 'datetime',
        'ended_at' => 'datetime',
    ];

    public function timer(): BelongsTo
    {
        return $this->belongsTo(Timer::class);
    }

    /** Elapsed whole seconds; for an open segment, up to now. */
    public function seconds(): int
    {
        return $this->started_at->diffInSeconds($this->ended_at ?? now());
    }
}
