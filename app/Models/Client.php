<?php

namespace App\Models;

use Carbon\CarbonImmutable;
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

    public function targetChanges(): HasMany
    {
        return $this->hasMany(ClientTargetChange::class);
    }

    /**
     * The monthly target effective in $month: the latest target change with
     * effective_from on or before the month's start, else the
     * monthly_target_hours default (#66).
     */
    public function targetForMonth(CarbonImmutable $month): ?int
    {
        $change = $this->targetChanges()
            ->where('effective_from', '<=', $month->startOfMonth()->toDateString())
            ->orderByDesc('effective_from')
            ->first();

        return $change?->hours ?? $this->monthly_target_hours;
    }
}
