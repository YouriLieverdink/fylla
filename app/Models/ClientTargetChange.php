<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

/**
 * Effective-dated override of a client's monthly target (#66): from
 * `effective_from` (first-of-month) onward the target is `hours`, until the
 * next change. Resolved via Client::targetForMonth().
 */
class ClientTargetChange extends Model
{
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'effective_from' => 'date:Y-m-d',
        'hours' => 'integer',
    ];

    public function client(): BelongsTo
    {
        return $this->belongsTo(Client::class);
    }
}
