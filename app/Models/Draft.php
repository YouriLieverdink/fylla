<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * A Fylla-native draft (ADR-0012): a lightweight to-do owned entirely by Fylla,
 * with no provider coordinates. Un-timeable while a draft (Kendo is the sole
 * worklog provider, ADR-0006); promotion to a Kendo issue is a separate slice.
 */
class Draft extends Model
{
    protected $guarded = [];

    protected $casts = [
        'due_date' => 'date',
        'not_before' => 'date',
        'up_next' => 'boolean',
    ];
}
