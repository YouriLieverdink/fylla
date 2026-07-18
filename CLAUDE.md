# Fylla — Laravel/Vue command center

Single-user web app tracking the user's work at Back to code (personal billable
utilization + a PM lens over clients/developers). Rewritten from a Go CLI to
Laravel + Inertia/Vue (ADR-0002). Local-only, single user, **no auth**.

Read `CONTEXT.md` for domain language and `docs/adr/` for locked decisions
before making design changes.

## Commands

```bash
composer install && npm install   # deps
php artisan test                  # run tests (PHPUnit)
php artisan migrate               # apply migrations
npm run build                     # build front-end assets
# dev: run these three together
php artisan serve                 # web
php artisan schedule:work         # scheduler (15-min sync)
php artisan queue:work            # database queue
```

## Architecture

- **Local DB is the source of truth the UI reads (ADR-0003).** The UI never
  calls providers live for reads; background jobs sync into local tables and the
  UI queries those. A manual "Sync now" is the freshness escape hatch.
- **`App\Kendo\Client`** — thin Laravel HTTP wrapper, Bearer auth from
  `config/services.php` (`KENDO_BASE_URL`/`KENDO_TOKEN`). `getMyIssues()` hits
  `GET /api/issues/my`, which returns `{data:[...], meta:{truncated,count,limit}}`;
  `priority`/`type` arrive as ints and are mapped to labels in the client.
  The my-issues feed omits estimates; `getProjectEstimates($pid)` reads
  `GET /api/projects/{id}/issues` for `estimated_minutes`/`remaining_minutes`.
- **`SyncKendoIssues` job** (queued, `database` driver) — `updateOrCreate` on
  `kendo_id` writing **only Kendo-mirror fields** (incl. estimates fetched
  per-project), then deletes local rows absent from the feed **unless
  `truncated`**, and **never** deletes issues with local timer/worklog history
  (FK + intent). Scheduled every 15 min in
  `routes/console.php`; "Sync now" (`POST /sync`) dispatches the same job.
- **Fylla-native fields owned locally (ADR-0004):** `due_date`, `not_before`,
  `up_next`, `no_split`, `recurrence` are local `issues` columns, never parsed
  from or written to Kendo. Reserved (nullable) and unpopulated so far.
  `updateOrCreate` preserves them across sync.
- **Page:** one Inertia page (`resources/js/Pages/Issues.vue`) via
  `IssueController@index`, reading the local `Issue` model.
- **UI-editable config (ADR-0016):** the `config/fylla.php` tuning values are
  overridable from `/settings`. The file stays the default; a `settings` row
  (`key`, JSON `value`) overrides it, and `SettingsProvider@boot` applies every
  row onto `config('fylla.*')` per request. Secrets stay in `.env`.

## Conventions

- PSR-12, typed properties/returns. Eloquent for data access.
- Provider clients are thin REST wrappers on Laravel's HTTP client.
- Tests: PHPUnit under `tests/Feature`; fake HTTP with `Http::fake()` /
  `Http::sequence()`. The sync reconciliation branch is the piece worth testing.
- Update `README.md` whenever config keys, commands, routes, or UI change.

## Agent skills

### Issue tracker

Issues and PRDs live in the YouriLieverdink/fylla GitHub Issues, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical roles, default label strings (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at repo root. See `docs/agents/domain.md`.
