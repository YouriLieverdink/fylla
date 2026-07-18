# Spec — UI-editable config

Make the `config/fylla.php` tuning values editable from a `/settings` page. Locked by
ADR-0016. This is the build sheet: schema, wiring, validation, UI, tests.

## In scope — the ten keys

| Config key (`fylla.*`)      | Default (file)                         | Type / rule                    |
| --------------------------- | -------------------------------------- | ------------------------------ |
| `kendo_user_id`             | `env(FYLLA_KENDO_USER_ID)` (no default)| `required|string`              |
| `worklog_sync_days`         | `90`                                   | `integer|min:1`                |
| `display_timezone`          | `Europe/Amsterdam`                     | `timezone`                     |
| `github_pr_queries`         | 2-entry list (see file)                | `array`, each `string` non-empty |
| `github_pr_exclude_repos`   | 2-entry list (see file)                | `array`, each `string` non-empty |
| `contracted_hours_per_week` | `32`                                   | `integer|min:1`                |
| `contracted_off_weekday`    | `5`                                    | `integer|between:1,7`          |
| `utilization_window_weeks`  | `13`                                   | `integer|min:1`                |
| `utilization_target`        | `75`                                   | `integer|between:0,100`        |
| `utilization_soft_floor`    | `73`                                   | `integer|between:0,100`; `≤ target` |

Out of scope: secrets in `config/services.php`, framework config, a reset-to-default
button.

## Storage

Migration `create_settings_table`:

- `key` — string, unique
- `value` — json
- timestamps

Model `App\Models\Setting` — `$fillable = ['key', 'value']`, `$casts = ['value' => 'array']`,
`$table` default. The table starts empty; a row exists only for an overridden key.

## Read-path — the override provider

`App\Providers\SettingsProvider` (registered in `bootstrap/providers.php`), `boot()`:

```php
public function boot(): void
{
    if (! Schema::hasTable('settings')) {
        return; // fresh install, pre-migrate
    }

    foreach (Setting::all() as $setting) {
        config()->set("fylla.{$setting->key}", $setting->value);
    }
}
```

- Runs every request → edits apply with no restart.
- `Schema::hasTable` guard keeps `migrate`/`config:cache` on a fresh DB from throwing.
- `value` is cast to array, so scalar keys come back as their JSON scalar and the two
  list keys come back as PHP arrays — set directly onto config, **bypassing** the
  CSV `explode()` in `config/fylla.php` (that parse only ever produces the file default
  now). No cache layer (ADR-0016).

## Routes & controller

`routes/web.php`:

```php
Route::get('/settings', [SettingsController::class, 'edit'])->name('settings.edit');
Route::put('/settings', [SettingsController::class, 'update'])->name('settings.update');
```

`App\Http\Controllers\SettingsController`:

- `edit()` → `Inertia::render('Settings', ['values' => /* the ten effective config('fylla.*') values */])`.
  Effective = already includes any DB override (the provider ran this request), so the
  form shows current values directly.
- `update(UpdateSettingsRequest $request)` → for each key, upsert a row **only when
  the submitted value differs from the file default** (`require config_path('fylla.php')`);
  a value back at the default **deletes** its row (the reset path). Keeps the invariant
  "a row exists only for an overridden key". Redirect back to `settings.edit`.

`App\Http\Requests\UpdateSettingsRequest` — `rules()` per the table above; add
`utilization_soft_floor` ≤ `utilization_target` via a closure/`after` validation hook.
List fields arrive already split (see UI) as arrays of non-empty strings.

## UI — `resources/js/Pages/Settings.vue`

- Reached from a nav link alongside the existing Issues page.
- One `useForm` over the ten fields, grouped into fieldsets:
  **Utilization** (target, soft-floor, contracted hours, off-weekday, window) ·
  **Sync** (worklog days, kendo_user_id) · **GitHub PRs** (queries, exclude-repos) ·
  **Display** (timezone).
- Scalars → number/text inputs. The two list fields → a `<textarea>`, one entry per
  line; on submit, split on newline + `trim` + drop empties (mirrors the file's
  CSV-parse intent). `contracted_off_weekday` → a 1–7 / weekday select. `display_timezone`
  → text (or a tz `<select>` if cheap).
- Single **Save** button → `form.put(route('settings.update'))`; inline errors from the
  FormRequest; success flash on return.

## Test (the one runnable check)

`tests/Feature/SettingsTest.php`:

1. **Override applies** — `PUT /settings` with `utilization_target = 80` persists a
   `settings` row and a follow-up request sees `config('fylla.utilization_target') === 80`.
2. **Validation** — `PUT /settings` with `utilization_soft_floor` > `utilization_target`
   (and an out-of-range field) fails validation, writes no row.

## Docs to update on build

- `README.md` — new `/settings` route, the `settings` table, and that these ten keys are
  now UI-editable (config-key change → README update is required, per repo convention).
- `CLAUDE.md` Architecture — one line on the settings-override provider.
- `CONTEXT.md` — term **Setting**: a runtime override of a `fylla.php` config default.
