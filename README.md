# Fylla

Single-user **command center** for tracking work at Back to code — personal
billable-utilization plus a project-manager lens over clients and developers.
Laravel + Inertia/Vue web app, backed by a local database that background jobs
sync from Kendo (and, later, GitHub). See `CONTEXT.md` for the domain language
and `docs/adr/` for the decisions behind the design.

> Rewritten from the original Go CLI/TUI (ADR-0002). Runs locally, single user,
> **no auth** — the app is open on localhost.

## Stack

- **Laravel 13** (PHP 8.4) — HTTP, queues, scheduler, Eloquent
- **Inertia + Vue 3** — single-page UI, no separate API
- **SQLite** — local database, the source of truth the UI reads from (ADR-0003)
- **Vite + Tailwind** — front-end build

## How it works

The UI never calls Kendo live for reads. A queued `SyncKendoIssues` job pulls
the current user's issues from Kendo's my-issues endpoint into a local `issues`
table (Kendo-mirror fields only). The lean my-issues feed omits estimates, so
the job also fetches each distinct project's `/api/projects/{id}/issues` feed to
mirror `estimated_minutes` / `remaining_minutes` (shown as the Estimate/Remaining
columns; `—` when unset in Kendo). It reconciles deletes for issues that left the
feed — skipped when the feed comes back truncated, and issues with local
timer/worklog history are kept regardless. The scheduler runs it every
15 minutes (queued); the **Sync now** button runs the same job synchronously so
the page returns fresh rows immediately — the button spins for the real
duration, and a failed sync shows a red "Sync failed" in place of the status
label. The page also polls every 60s so scheduled syncs surface without a
manual refresh. Fylla-native scheduling fields (due, not-before, up-next,
no-split, recurrence) are owned locally and never written back to Kendo (ADR-0004).

### Timer stack

Each issue row has a **Start** button. Starting a timer while one runs pushes a
new timer on top (LIFO); pausing closes the current segment and resuming opens a
new one. **Each segment posts its own `worklogs` row the moment it closes**
(seconds rounded to the nearest minute, discarded if 0) — so one issue worked in
three sittings yields three worklogs at three real start times (ADR-0005), not
one summed entry. Stack order is derived from timer id — no position column. Only
the top timer is interactive; buried ones are display-only. One live timer per
issue. `TimerService` owns the state machine; the running segment's elapsed time
ticks client-side from timestamps, so a reload recomputes it. Worklog posting to
Kendo is not wired yet — the `posted_at` / `kendo_worklog_id` / `post_error`
columns are reserved (ADR-0001/0003).

**Notes** attach to the open segment: add one (Enter or the Add button) while the
timer runs and it's stamped with the wall-clock time. A segment's notes ride into
that segment's worklog comment, one `HH:MM — text` line each. The notes panel
shows only the open segment's notes and is disabled while paused (ADR-0005).

Routes: `POST /timers` (start), `POST /timers/pause`, `POST /timers/resume`,
`POST /timers/stop`, `POST /timers/notes`.

## Setup

```bash
composer install
npm install
cp .env.example .env
php artisan key:generate

# Kendo credentials — add to .env:
#   KENDO_BASE_URL=https://<tenant>.kendo.dev
#   KENDO_TOKEN=<bearer token>

php artisan migrate
npm run build
```

## Run (dev)

Three processes:

```bash
php artisan serve          # web server → http://127.0.0.1:8000
php artisan schedule:work  # runs the 15-minute sync
php artisan queue:work     # processes the database queue
```

Then open `/`. Hit **Sync now** to pull issues immediately.

## Design system

Ported from a Claude Design UI kit. Tokens (colours, type, radii, shadows) live
in the `@theme` block of `resources/css/app.css`; fonts (Hanken Grotesk + Spline
Sans Mono) load via Google Fonts in `resources/views/app.blade.php`. Reusable
presentational components are in `resources/js/Components/`. The `/playground`
route renders a live catalog of every component — the reference when composing
screens.

## Test

```bash
php artisan test
```
