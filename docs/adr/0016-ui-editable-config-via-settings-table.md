# UI-editable config via a settings table overriding file defaults

The tuning knobs in `config/fylla.php` — utilization target/soft-floor, contracted
hours and off-weekday, sync windows, the GitHub PR queries and exclude list,
`kendo_user_id`, and the display timezone — get retuned often enough that editing a
PHP file and restarting is friction. We make them editable from a `/settings` page in
the app.

Values are stored in a new `settings` table (`key` string, `value` JSON). The config
files stay as the **built-in defaults**; a row only exists for a key the user has
overridden. A service provider reads the table on **every request** and applies each
override with `config()->set("fylla.{$key}", $value)`, so all existing
`config('fylla.x')` call sites keep working untouched and pick up edits with no restart
(PHP is shared-nothing; `boot()` re-runs per request — consistent with ADR-0003's
"local DB is the source of truth the UI reads"). The provider guards on
`Schema::hasTable('settings')` so a fresh install boots before its first migration.

Scope is the `fylla.php` tuning values only. **Secrets stay in `.env`** (`KENDO_TOKEN`,
`GITHUB_TOKEN`, Slack/mail keys): they change rarely and editing credentials through a
no-auth local UI adds risk for no real gain. Framework config is untouched.

A FormRequest validates at the boundary — per-field type/range rules plus the one
cross-field invariant `utilization_soft_floor ≤ utilization_target` — because a bad
value here silently breaks the utilization math or the worklog sync.

## Considered options

- **`config()->set()` at every read site replaced by a `Settings::get()` accessor** —
  rejected: touches every call site across the codebase for no benefit; the boot-time
  override leaves them all working as-is.
- **`spatie/laravel-settings`** — rejected: a new dependency for ten values a
  key/JSON table and one provider already cover.
- **Rewrite `.env` / config files from the UI** — rejected: fragile and fights
  `config:cache`. The runtime override sidesteps the cache entirely.
- **Secrets editable too** — rejected (see above); revisit only if a real re-key
  workflow appears, and then behind auth.
- **Per-key cache of the settings read** — rejected: ten rows, one indexed query per
  request; a cache only adds an invalidation bug to get wrong.
- **Reset-a-key-to-default button** — deferred: deleting the row already restores the
  default; a UI affordance can come later if wanted.
