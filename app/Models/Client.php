<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

/**
 * Fylla-owned client (ADR-0011): groups Kendo projects. A client's existence is
 * the "managed" mark — its projects' worklogs sync team-wide.
 */
class Client extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'monthly_target_hours' => 'integer',
    ];

    public function projects(): HasMany
    {
        return $this->hasMany(Project::class);
    }
}
