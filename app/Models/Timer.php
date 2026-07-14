<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;
use Illuminate\Database\Eloquent\Relations\HasOne;
use Illuminate\Database\Eloquent\Relations\MorphTo;

class Timer extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'stopped_at' => 'datetime',
    ];

    /** The timed subject — an Issue or a PullRequest (ADR-0009). */
    public function timeable(): MorphTo
    {
        return $this->morphTo();
    }

    public function segments(): HasMany
    {
        return $this->hasMany(Segment::class);
    }

    /** The open (running) segment, if any. */
    public function openSegment(): HasOne
    {
        return $this->hasOne(Segment::class)->whereNull('ended_at');
    }

    /** Live = not stopped. Ordered top-of-stack first (max id). */
    public function scopeLive($query)
    {
        return $query->whereNull('stopped_at')->orderByDesc('id');
    }
}
