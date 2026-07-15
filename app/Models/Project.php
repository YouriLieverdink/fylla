<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class Project extends Model
{
    // synced_at is managed by the sync job; no created_at/updated_at.
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'synced_at' => 'datetime',
        'billable' => 'boolean',
    ];

    /** The client this project is assigned to; null = unmanaged, yours-only. */
    public function client(): BelongsTo
    {
        return $this->belongsTo(Client::class);
    }
}
