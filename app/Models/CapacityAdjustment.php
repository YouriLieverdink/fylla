<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * Fylla-native capacity adjustment (ADR-0004, ADR-0008, ADR-0010). One row per
 * date carrying an explicit `type` (off | holiday | extra) and `status`
 * (planned | confirmed). Signed decimal hours (off/holiday negative, extra
 * positive) shift a week's utilization capacity (denominator, confirmed only),
 * never the billable numerator. No Kendo mirror.
 */
class CapacityAdjustment extends Model
{
    protected $table = 'capacity_adjustments';

    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        // date-only (no 00:00:00) so updateOrCreate on `date` matches the
        // stored value and upserts instead of duplicating.
        'date' => 'date:Y-m-d',
        'hours' => 'decimal:2',
    ];
}
