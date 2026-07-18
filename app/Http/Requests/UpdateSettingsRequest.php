<?php

namespace App\Http\Requests;

use Illuminate\Foundation\Http\FormRequest;

/**
 * Validates the ten UI-editable `fylla.*` tuning keys (ADR-0016). A bad value
 * here silently breaks the utilization math or the worklog sync, so this is the
 * boundary guard — including the one cross-field invariant (soft_floor ≤ target).
 */
class UpdateSettingsRequest extends FormRequest
{
    public function rules(): array
    {
        return [
            'kendo_user_id' => ['required', 'string'],
            'worklog_sync_days' => ['required', 'integer', 'min:1'],
            'display_timezone' => ['required', 'timezone'],
            'github_pr_queries' => ['present', 'array'],
            'github_pr_queries.*' => ['required', 'string'],
            'github_pr_exclude_repos' => ['present', 'array'],
            'github_pr_exclude_repos.*' => ['required', 'string'],
            'contracted_hours_per_week' => ['required', 'integer', 'min:1'],
            'contracted_off_weekday' => ['required', 'integer', 'between:1,7'],
            'utilization_window_weeks' => ['required', 'integer', 'min:1'],
            'utilization_target' => ['required', 'integer', 'between:0,100'],
            'utilization_soft_floor' => ['required', 'integer', 'between:0,100'],
        ];
    }

    public function after(): array
    {
        return [function ($validator) {
            if ((int) $this->input('utilization_soft_floor') > (int) $this->input('utilization_target')) {
                $validator->errors()->add(
                    'utilization_soft_floor',
                    'The soft floor cannot be above the target.',
                );
            }
        }];
    }
}
