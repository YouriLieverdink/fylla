<?php

namespace App\Http\Requests;

use Carbon\CarbonImmutable;
use Illuminate\Foundation\Http\FormRequest;

class StoreCapacityAdjustmentRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'type' => ['required', 'in:off,extra'],
            // Magnitude only; the sign comes from the type (ADR-0008).
            'hours' => ['required', 'integer', 'between:1,24'],
            'reason' => ['nullable', 'string', 'max:255'],
            // Time off is a weekday range (end optional); an extra day is a
            // single date, any day. A weekend time-off row would wrongly shrink
            // a Mon–Fri week, so reject weekend starts for time off.
            'start' => ['required', 'date', function ($attr, $value, $fail) {
                if ($this->input('type') === 'off'
                    && in_array(CarbonImmutable::parse($value)->dayOfWeekIso, [6, 7], true)) {
                    $fail('Time off must fall on a weekday.');
                }
            }],
            'end' => ['nullable', 'date', 'after_or_equal:start'],
        ];
    }
}
