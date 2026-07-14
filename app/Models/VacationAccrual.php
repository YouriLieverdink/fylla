<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * The vacation hours granted for a year (Dutch: Vakantieuren) — the one new
 * stored input the vacation ledger needs (ADR-0010). Manually entered, one
 * decimal per year; everything else in the ledger is derived from adjustment
 * rows.
 */
class VacationAccrual extends Model
{
    protected $table = 'vacation_accruals';

    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'year' => 'integer',
        'hours' => 'decimal:2',
    ];
}
