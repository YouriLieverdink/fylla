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
timer/worklog history are kept regardless. Those retained rows are hidden from
the work-items list — it shows only issues from the latest sync (current open
work), so an issue moved to Kendo's done lane drops off on the next sync. The
scheduler runs it every
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
ticks client-side from timestamps, so a reload recomputes it.

When a segment closes, its worklog is posted to Kendo as a time entry by a queued
`PostWorklog` job (`queue:work` must be running), stamping `posted_at` /
`kendo_worklog_id`. It's idempotent on `posted_at`, so a retry never double-posts.
After 3 failed tries the error is recorded in `post_error` and the worklog stays
unposted (no auto-retry). Kendo-only, direct on `Kendo\Client` (ADR-0006).

**Notes** attach to the open segment: add one (Enter or the Add button) while the
timer runs and it's stamped with the wall-clock time. A segment's notes ride into
that segment's worklog comment, one `HH:MM — text` line each. Timestamps store
UTC; stamps render in `fylla.display_timezone` (default `Europe/Amsterdam`). The
notes panel shows only the open segment's notes and is disabled while paused
(ADR-0005).

The active timer's start is editable inline ("started HH:MM · edit"): sets the
open segment's `started_at` to any wall-clock time at or before now (display tz),
correcting a forgotten/late start. Pulling it before the previous stretch may
overlap an already-posted worklog — accepted, user-reconciled in Kendo. Only
available while running; a future time is rejected.

Routes: `POST /timers` (start), `POST /timers/pause`, `POST /timers/resume`,
`POST /timers/stop`, `POST /timers/notes`, `POST /timers/start-time`.

### Billable projects & synced worklogs

A separate read path measures personal utilization (ADR-0007). Two
queued jobs run alongside the issues sync (every 15 min, and on **Sync now**):

- `SyncKendoProjects` mirrors `GET /api/projects` into a local `projects` table.
  Each project carries a locally-owned `billable` flag (ADR-0004, preserved
  across sync); projects are never deleted.
- `SyncKendoWorklogs` pulls the user's time entries (`GET /api/time-entries`)
  over a rolling window (`fylla.worklog_sync_days`, default 90) into the
  `synced_worklogs` read mirror — separate from the `worklogs` outbox. The admin
  token returns the whole team, so rows are filtered to `FYLLA_KENDO_USER_ID`.
  Reconcile deletes rows inside the window absent from the feed; rows outside it
  are never touched.

**Billability is a property of the project, not the worklog.** A worklog is
billable iff its project's `billable` flag is set, derived at read time — so
toggling a project on the `/clients` page re-classifies every worklog with no
re-sync. Manage the list at `/clients` (`PATCH /projects/{project}`).

**Clients group projects (ADR-0011).** The `/clients` page assigns each project
to a Fylla-owned client via `PATCH /projects/{project}` (`client_id`). Clients
are managed inline — `POST /clients`, `PATCH /clients/{client}` (rename,
set/clear `monthly_target_hours`), `DELETE /clients/{client}` (nulls its
projects' `client_id`, no cascade). Assigning a project to a client widens its
worklog sync to the whole team; unassigned projects stay yours-only.

### Delivery

The `/delivery` page (the **Delivery** nav tab) shows one card per client:
team-aggregate hours logged against the client's projects this calendar month
vs its `monthly_target_hours`. `App\Delivery\DeliveryReport` reads
`synced_worklogs` **unscoped** (ADR-0011) — every developer's hours plus your
own, billable and non-billable — bucketing `started_at` by month in
`config('fylla.display_timezone')`. Clients without a target show delivered
hours alone (no progress bar). Distinct from the `/clients` management tab.

### Utilization dashboard

The home page headlines personal utilization (`App\Utilization\UtilizationReport`,
issue #12). Utilization = billable hours ÷ **capacity**, where weekly capacity is
`fylla.contracted_hours_per_week` (default 32) **± the signed capacity
adjustments** that week (ADR-0008/0010). Only **confirmed** adjustments move
capacity — planned ones are penciled-in and do not shift the metric. The current
(partial) week prorates over elapsed Mon–Fri workdays; completed weeks use full
capacity.

- **Headline** = one cumulative `Σbillable ÷ Σcapacity` over the last
  `fylla.utilization_window_weeks` weeks (default 13), with a delta vs. the
  preceding equal-length window.
- **Trend chart** = each week's own utilization %, with a second line for that
  week's billable share (billable ÷ worked).
- **This-week gauge** = the prorated current-week number.
- `fylla.utilization_target` (75) is a soft target; at or above
  `fylla.utilization_soft_floor` (73) reads as "on track" — trend, not pass/fail.
- A week with no capacity (fully time off) drops out of both sums; an all-off
  window shows "—".

The `/utilization` page (the **Utilization** nav tab) exposes the data behind
the headline via `UtilizationReport::breakdown()`: window totals (Σ capacity /
worked / billable, billable share, + the cumulative %), and — behind a
segmented **Weekly breakdown** ⇆ **Time entries** ⇆ **By project** toggle below
the totals card — a per-week table (`Week | Capacity | Worked | Billable |
Billable share | Utilization | Adjustments`, current week prorated, utilization
target-coloured — the same % as the dashboard, with that week's signed
adjustments shown as chips), the window's synced time entries grouped into
collapsible week sections (newest first, current week open), and a **By
project** breakdown (collapsible project → issue rows with hours subtotals,
billable/internal badge, each project's share of total worked hours, and a
per-week hours sparkline across the window; hours-desc, all collapsed by
default). The active view is reflected in `?view=` (`weekly`/`entries`/`projects`). Worked = Σ **all** worklog minutes
that week (billable + non-billable) as effort context; it is never a
denominator. Billable share = billable ÷ worked (share of worked hours that
billed; "—" when nothing worked) — distinct from Utilization (billable ÷
capacity).

### Time off & vacation

Capacity adjustments live in the Fylla-native `capacity_adjustments` table
(`date` unique, `type`, signed decimal `hours`, `status`, `reason`;
ADR-0004/0008/0010 — Kendo has no leave concept). One row per date carries an
explicit **`type`** — `off` (vacation), `holiday` (public holiday), `sick`, or
`extra` (an agreed extra day, banked toward vacation) — and a **`status`** of
`planned` (exists only in Fylla) or `confirmed` (entered in the external leave
system). Off/holiday/sick store negative hours, extra positive. `holiday` and
`sick` shrink capacity but are **excluded from the vacation ledger** (you don't
spend vacation on Kingsday or a sick day); only `off` draws the balance.

The `/capacity` page (the **Capacity** nav tab) is a **year calendar grid**
(12 month-rows × 31 day-columns) with a running **vacation ledger** above it:

- `GET /capacity?year=Y` · `POST /capacity` · `PATCH /capacity/{capacityAdjustment}` ·
  `DELETE /capacity/{capacityAdjustment}` · `POST /capacity/accrual` (per-year
  vacation accrual upsert).
- Click a day or drag a range → a popover sets type / decimal hours / trip name /
  planned↔confirmed. Off and holiday expand over **worked weekdays** — weekends
  and the contracted non-working day (`fylla.contracted_off_weekday`, default
  Friday) are skipped, so a full week off is 4 × 8h = −32h against the 32h
  contract; an extra day is a single date, **any day** allowed. Each date upserts
  (`updateOrCreate` on `date`); hours are decimal, 0.25–24h.
- The **vacation ledger** (`App\Vacation\VacationLedger`) is derived per year:
  `balance = carryover + accrual + banked-extra + taken`, where `taken` = Σ off
  hours (holidays and sick days **excluded**), `banked` = Σ extra hours, `carryover` = the
  previous year's closing balance (rolls forward). Accrual is one manually-entered
  decimal per year (`vacation_accruals` table), edited inline in the ledger panel.
  The ledger counts planned + confirmed alike; balance is shown in hours, days
  (÷8), and weeks (÷32). A multi-year **Overzicht** table and a trips-this-year
  list sit below the grid. The per-week capacity view lives on `/utilization`.

## Setup

```bash
composer install
npm install
cp .env.example .env
php artisan key:generate

# Kendo credentials — add to .env:
#   KENDO_BASE_URL=https://<tenant>.kendo.dev
#   KENDO_TOKEN=<bearer token>
#   FYLLA_KENDO_USER_ID=<your Kendo user id>   # required: filters worklogs to you

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
