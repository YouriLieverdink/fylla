<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Note extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'created_at' => 'datetime',
    ];

    public function segment(): BelongsTo
    {
        return $this->belongsTo(Segment::class);
    }
}
