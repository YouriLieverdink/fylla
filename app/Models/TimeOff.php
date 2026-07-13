<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * Fylla-native leave (ADR-0004). Reduces a week's utilization capacity; there
 * is no Kendo mirror. Entry UI is deferred — seed rows via factory or DB.
 */
class TimeOff extends Model
{
    protected $table = 'time_off';

    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'date' => 'date',
        'hours' => 'integer',
    ];
}
