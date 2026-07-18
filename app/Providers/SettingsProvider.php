<?php

namespace App\Providers;

use App\Models\Setting;
use Illuminate\Support\Facades\Schema;
use Illuminate\Support\ServiceProvider;

/**
 * Applies UI-edited config overrides (ADR-0016). Runs on every request (PHP is
 * shared-nothing), reading the `settings` table and overriding the matching
 * `config('fylla.*')` default. The `hasTable` guard keeps a fresh install
 * booting before its first migration.
 */
class SettingsProvider extends ServiceProvider
{
    public function boot(): void
    {
        if (! Schema::hasTable('settings')) {
            return;
        }

        foreach (Setting::all() as $setting) {
            config()->set("fylla.{$setting->key}", $setting->value);
        }
    }
}
