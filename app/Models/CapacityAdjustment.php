<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * Fylla-native capacity adjustment (ADR-0004, ADR-0008). One signed row per
 * date: negative hours = time off, positive = an extra day. Shifts a week's
 * utilization capacity (denominator), never the billable numerator. No Kendo
 * mirror.
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
        'hours' => 'integer',
    ];
}
