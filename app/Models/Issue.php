<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

class Issue extends Model
{
    // updated_at mirrors Kendo; there is no created_at. Manage timestamps by hand.
    public $timestamps = false;

    protected $guarded = [];

    protected $casts = [
        'updated_at' => 'datetime',
        'synced_at' => 'datetime',
        'due_date' => 'date',
        'not_before' => 'date',
        'up_next' => 'boolean',
        'no_split' => 'boolean',
    ];
}
