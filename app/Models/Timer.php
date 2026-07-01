<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;
use Illuminate\Database\Eloquent\Relations\HasOne;

class Timer extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'stopped_at' => 'datetime',
    ];

    public function issue(): BelongsTo
    {
        return $this->belongsTo(Issue::class);
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
