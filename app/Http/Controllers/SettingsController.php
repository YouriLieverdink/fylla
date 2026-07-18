<?php

namespace App\Http\Controllers;

use App\Http\Requests\UpdateSettingsRequest;
use App\Models\Setting;
use Illuminate\Http\RedirectResponse;
use Inertia\Inertia;
use Inertia\Response;

class SettingsController extends Controller
{
    /** The ten UI-editable `fylla.*` tuning keys (ADR-0016). */
    public const KEYS = [
        'kendo_user_id',
        'worklog_sync_days',
        'display_timezone',
        'github_pr_queries',
        'github_pr_exclude_repos',
        'contracted_hours_per_week',
        'contracted_off_weekday',
        'utilization_window_weeks',
        'utilization_target',
        'utilization_soft_floor',
    ];

    /** Show current effective values — config already carries any DB override
     * (SettingsProvider ran this request), so the form reflects live values. */
    public function edit(): Response
    {
        $values = [];
        foreach (self::KEYS as $key) {
            $values[$key] = config("fylla.{$key}");
        }

        return Inertia::render('Settings', ['values' => $values]);
    }

    /** A row exists only for a key that differs from the file default (ADR-0016);
     * a value back at the default deletes its row, restoring the default. */
    public function update(UpdateSettingsRequest $request): RedirectResponse
    {
        $defaults = require config_path('fylla.php');

        foreach (self::KEYS as $key) {
            $value = $request->validated($key);

            if ($value == $defaults[$key]) {
                Setting::where('key', $key)->delete();
            } else {
                Setting::updateOrCreate(['key' => $key], ['value' => $value]);
            }
        }

        return redirect()->route('settings.edit');
    }
}
