<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Model;

/**
 * A runtime override of a `config/fylla.php` tuning default (ADR-0016).
 * `value` holds the JSON-typed override (scalar or list); `SettingsProvider`
 * reads every row and applies it onto `config('fylla.*')` per request.
 */
class Setting extends Model
{
    protected $fillable = ['key', 'value'];

    protected $casts = ['value' => 'array'];
}
