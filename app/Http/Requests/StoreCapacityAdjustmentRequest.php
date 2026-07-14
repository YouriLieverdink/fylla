<?php

namespace App\Http\Requests;

use Carbon\CarbonImmutable;
use Illuminate\Foundation\Http\FormRequest;

class StoreCapacityAdjustmentRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'type' => ['required', 'in:off,holiday,sick,extra'],
            // Magnitude only; the sign comes from the type (ADR-0008/0010).
            // Decimal — half-days and a 1.5h early finish are real.
            'hours' => ['required', 'numeric', 'between:0.25,24'],
            'status' => ['nullable', 'in:planned,confirmed'],
            'reason' => ['nullable', 'string', 'max:255'],
            // Off/holiday/sick are weekday ranges (end optional); an extra day is
            // a single date, any day. A weekend negative row would wrongly shrink
            // a Mon–Fri week, so reject weekend starts for those.
            'start' => ['required', 'date', function ($attr, $value, $fail) {
                if ($this->input('type') !== 'extra'
                    && in_array(CarbonImmutable::parse($value)->dayOfWeekIso, [6, 7], true)) {
                    $fail('Time off, holidays and sick days must fall on a weekday.');
                }
            }],
            'end' => ['nullable', 'date', 'after_or_equal:start'],
        ];
    }
}
