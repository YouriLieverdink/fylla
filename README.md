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

### Worklist ranking

The home page (`/`) is the **Worklist**: open issues, GitHub PRs, and
Fylla-native drafts merged into one list, ranked by a weighted composite score
recomputed on every render
(`App\Services\WorklistScorer`, ADR-0013) — never hand-dragged. The score
weights priority, due date, and estimate, boosts items in a crunch window or
pinned via `up_next`, and demotes (never hides) those with a future
`not_before`. Each row shows a single "why" string (e.g. "2 days overdue",
"quick win", "pinned"). A PR carries none of these fields, so it is scored via a
synthetic due date (`opened_at + 1 day` grace, High priority): high the day it
opens, climbing to the top once it sits past the grace. This needs the PR's
GitHub `created_at` persisted as `opened_at` on `pull_requests`.

Issue rows are editable inline (`PATCH /issues/{issue}`, ADR-0014): a 📌 toggle
pins/unpins `up_next` on click, and a `⋯` popover sets `priority`, `due_date`,
`not_before`, and the estimate (hours). Scheduling fields are local-only writes;
`priority` and the estimate are Kendo-mirror fields written back synchronously
(one read-modify-write on the full issue) — on failure Fylla keeps its values and
shows an inline error, while any scheduling-field edits in the same save still
persist. PRs get no edit UI.

**Drafts (ADR-0012)** are a third, Fylla-owned work source — lightweight to-dos
("email this client") that shouldn't be a Kendo ticket yet. Capture one in a
single gesture from the input above the list (`POST /drafts`); it lands in the
worklist ranked by the same scorer, defaulting to Medium priority. Drafts live
in their own `drafts` table with no `kendo_id`, so the Kendo sync never touches
or deletes them. They share the issue edit UI (📌 pin, `⋯` popover for priority
and scheduling — all local, no Kendo write) via `PATCH /drafts/{draft}`, and are
**not timeable** (no start-timer affordance; Kendo is the sole worklog provider,
ADR-0006) — the row's action is a **Done** button that removes it
(`DELETE /drafts/{draft}`).

**Promote (ADR-0012)** turns a draft into a real, timeable Kendo issue. Pick a
target project (search-and-select, so it scales past a dropdown) in the `⋯`
popover and hit **Promote** (`POST /drafts/{draft}/promote`): the controller
creates the issue via `Kendo\Client::createIssue` — assigned to
`FYLLA_KENDO_USER_ID` (so it returns in the my-issues feed), dropped in the
project's first lane and active sprint, typed Task, with the title doubling as
the required description — then runs a sync so it mirrors in immediately as an
ordinary timeable issue, and deletes the draft. One-way: a create failure
leaves the draft intact and surfaces the error; there is no demote.

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

**Ad-hoc timing** (ADR-0015) times work that never reaches your worklist —
unassigned PM tasks, reviews of others' tickets. "Log time on another task"
(under the timer stack) opens a live Kendo search; picking a result starts a
timer immediately (`POST /timers/adhoc`). The picked issue is stored only as the
timer's subject with `synced_at` left unstamped, so it never renders as a card
and is gone once the timer stops. The worklog posts to Kendo on stop like any
other.

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
toggling a project on the `/delivery` page re-classifies every worklog with no
re-sync. Manage the list at `/delivery` (`PATCH /projects/{project}`).

**Clients group projects (ADR-0011).** The `/delivery` page assigns each project
to a Fylla-owned client via `PATCH /projects/{project}` (`client_id`). Clients
are managed inline — `POST /clients`, `PATCH /clients/{client}` (rename,
set/clear `monthly_target_hours`), `DELETE /clients/{client}` (nulls its
projects' `client_id`, no cascade). Assigning a project to a client widens its
worklog sync to the whole team; unassigned projects stay yours-only.

### Delivery

The `/delivery` page (the **Delivery** nav tab) shows one projection chart per
client (`DeliveryProjectionChart.vue`): cumulative team-aggregate hours logged
against the client's projects this calendar month, a dashed run-rate
**projection** to month-end, and the client's monthly target — the
`monthly_target_hours` default, overridden per month by effective-dated
`client_target_changes` rows (ADR-0017) — as a reference line. `App\Delivery\DeliveryReport` reads `synced_worklogs`
**unscoped** (ADR-0011) — every developer's hours plus your own, billable and
non-billable — bucketing `started_at` by month/day in
`config('fylla.display_timezone')`. Projection = delivered × working days in
month ÷ working days elapsed (Mon–Fri, not holiday-adjusted; CONTEXT.md →
_Delivery projection_). Clients without a target show the delivered burn-up
alone (no projection or target line). The controller also passes raw `projects`
rows (`id, name, code, billable, client_id`) driving the footer.

Each card has two zones: the **chart region** (top) is the `<Link>` drill-down
to `/delivery/{client}`; a **config footer strip** below never navigates. The
footer holds clickable **billable pills** for the client's assigned projects
(`PATCH /projects/{project}`, colour/dot reflects state), and `+ project`/`Delete`
buttons that open modals; targets are edited on the client page (#68, ADR-0018). A header-row **+ New client** button opens a create
modal (`POST /clients`); `+ project` opens a search modal that assigns an
unassigned project (`PATCH /projects/{project}` setting `client_id`); `Delete`
opens a confirm modal (projects fall back to unassigned, worklog history kept,
irreversible — `DELETE /clients/{client}`). All three use the `useModalGuard`
pattern (#43): keybindings beneath the scrim are suppressed, Escape is the sole
exit.

A **By client ⇆ By project** segmented toggle switches between the projection
cards and a flat list of **all** projects (assigned + unassigned), one
`ProjectRow` each (name + billable checkbox, `PATCH /projects/{project}`). The
flat list is the sole billable editor for unassigned/yours-only projects (the
cards render managed clients only). `c` selects By client, `p` By project;
`j`/`k` walk the active view's cards/rows.

### Client context

Clicking a Delivery card's chart opens `/delivery/{client}` (`delivery.show`,
`ClientContextController`) — a board over the team issue mirror + roster, scoped
to one managed client; read-only except the target editor below (ADR-0018). `App\ClientContext\ClientContextReport`
ships a **totals band** (team hours vs target this month with a run-rate **pace**
line, active issues, flagged count; the current sprint's `done/total` and days
left live in the header chip), a flat issue
list, the client's lanes, and **per-developer subtotals** (month hours). The page
(`resources/js/Pages/ClientContext.vue`) renders a **kanban** whose columns are
the client's real Kendo lanes and whose cards are its issues — colored per
developer, flagged **overrunning** (`logged > estimate`) or **stuck** (no worklog
or lane move in the last 5 working days). Filtering is client-side: current
sprint (default on), overrunning, stuck, and a multi-select developer legend.

Below the board, a **delivery-history card** (`App\Delivery\ClientDeliveryHistory`)
shows delivered vs target for the last `fylla.delivery_history_months` completed
months (default 3, `/settings`-editable) plus the current month, each with its
per-month resolved target and over/under. A cumulative surplus/deficit sums the
completed months only — the in-progress month is shown as delivered-so-far but
excluded, framed as where the gap gets spent. Clients without a target show
delivered-only rows and no gap.

The history card is also the **target editor** (#68, ADR-0018) — the page's only
write. Its Edit toggle exposes the `monthly_target_hours` default
(`PATCH /clients/{client}`) and the effective-dated override rows (ADR-0017):
add via `POST /clients/{client}/target-changes`, edit/delete via
`PATCH`/`DELETE /target-changes/{targetChange}`. Submitted dates are normalized
to first-of-month server-side; re-adding an existing month corrects that row,
and deleting one reverts affected months to the previous entry or the default.

Its data comes from three background jobs (all run by "Sync now" and scheduled
**daily**):

- `SyncKendoProjectIssues` (the renamed, generalized ex-`SyncKendoFinishedIssues`)
  mirrors **every** issue — all assignees, all lanes — for the union of
  managed-client projects and the projects you've logged time in, into
  `synced_issues`. Per row it stamps `assignee_id`, `lane_position`
  (`first`/`middle`/`done`, classified from lane order), `lane_name`, `sprint_id`,
  and a forward-only `lane_entered_at` (Kendo exposes no transition time, so
  Fylla records it on first sight and re-stamps on a lane change; aging resolves
  to days). It also mirrors each board's sprints into `sprints` for the brief's
  sprint tile. The mirror carries no local history, so reconcile is a plain
  delete within the refetched project set.
- `SyncKendoUsers` mirrors the whole Kendo roster (`GET /api/users`) into
  `developers` (`kendo_id`, name, email, `active`, `avatar_url`) — one global
  call — so an issue's `assignee_id` resolves to a name.

### Estimation

The `/estimation` page (the **Estimation** nav tab) is the personal estimation
feedback loop (issue #17): per finished issue, the Kendo estimate vs. the hours
actually logged, plus a single **rolling bias** (positive % = you underestimate)
over the last 20 estimated issues. Sliceable by project (label-slicing deferred
until labels are synced).

The data source is the `synced_issues` mirror, filled by the
`SyncKendoProjectIssues` job (scheduled **daily** — slow-changing — and run by
"Sync now"). The open my-issues feed excludes the done lane, so finished issues
are read from the **per-project issues feed** (`GET /api/projects/{id}/issues`),
which carries each issue's `estimated_minutes` and `logged_minutes` (the actual).
`App\Estimation\EstimationReport` reads that table filtered to **your own
done-lane issues** (`assignee_id` = `FYLLA_KENDO_USER_ID`, `lane_position` =
`done`): actual = the issue's `logged_minutes`, ordered most-recently-worked
first; issues without an estimate list but sit out the bias.

### Notes search

The `/notes` page (the **Notes** nav tab, Team lens) searches synced worklogs
(issue #70). Corpus: the `synced_worklogs` mirror — your own worklogs
everywhere plus teammates' on managed-client projects (ADR-0011; a team read,
deliberately unscoped). Noteless worklogs are listed too (note column shows
`—`) so developers who don't write notes still show their logged time.
Free-text matches the note text and the issue key/title with plain `LIKE`,
newest-first, filterable by clients, projects, developers (searchable
multi-selects, `MultiSelectFilter.vue`), and date range; each row shows
date · developer · issue · note · hours. Filter options are scoped to the
corpus — only clients/projects/developers with synced worklogs appear. Results cap at 200 rows (total match count still
shown). Filters live in the URL
(`?q=&clients[]=&projects[]=&developers[]=&from=&to=`), so searches are
shareable. Keys: `g n` navigates here, `j`/`k` walk the rows, `s` focuses the
search field.

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

### Settings

The `/settings` page (the **gear icon** in the header) edits the tuning knobs in
`config/fylla.php` without touching the file (ADR-0016): the utilization
target/soft-floor, contracted hours and day off, the trend window, the delivery
history window, the worklog sync window, `kendo_user_id`, the GitHub PR queries
and excluded repos, and the display timezone. Routes: `GET /settings` (edit), `PUT /settings` (save).

The file values stay the built-in defaults; a save writes a row to the
`settings` table (`key`, JSON `value`), and `SettingsProvider` reads that table
on every request and overrides the matching `config('fylla.*')` — so edits apply
with no restart. Deleting a row restores the default. **Secrets** (`KENDO_TOKEN`,
`GITHUB_TOKEN`, …) are deliberately not editable here; they stay in `.env`.

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
#
# Optional GitHub PR feed overrides:
#   GITHUB_PR_QUERIES=<comma-separated search filters>
#   GITHUB_PR_EXCLUDE_REPOS=<comma-separated owner/name repos to hide>

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

Then open `/`. Hit **Sync now** to pull issues immediately (or press `.`).

## Keyboard

Bindings are registered through the `useAction` composable into a reactive
registry; the persistent `AppLayout` rebinds one guarded tinykeys listener from
it, so shortcuts fire from any page and are suppressed while typing in an
editable context (focus guard, ADR/issue #39).

Global `g`-leader navigation (depth-2 sequences dispatched natively by
tinykeys, no timeout logic of our own) — Inertia visits to each page:

| Keys | Page | Keys | Page |
|---|---|---|---|
| `g w` | Worklist | `g e` | Estimation |
| `g u` | Utilization | `g d` | Delivery |
| `g c` | Capacity | `g s` | Settings |

`.` — Sync now.

Every page with cards/rows carries a persistent **cursor** (`useListCursor`,
wired for full-page use by `usePageCursor`) that navigates its focus sequence —
its summary cards, then its rows. On the Worklist that's the utilization and
timer cards then the worklist rows; the Estimation, Utilization, Capacity and
Delivery pages walk their own cards + visible table/grid rows the
same way (tab- and expand-gated: only currently-shown rows are targets, in DOM
order). `j`/`k` move down/up (focused target ringed and scrolled into view),
digits `1`–`9` jump to that position. `g g` / `G` jump to the top / bottom of
the page; `k` past the first target and `j` past the last both **deselect and
scroll to that page edge**, so the cursor never traps you inside the list. It's
unset and invisible until the first `j`/`k`, tracks the same target by key
across re-sort/sync, and reserves `j`/`k`/`1`–`9`/`g g`/`G` app-wide (never
bound as page-local action keys). On any page without a cursor those keys drive
the viewport instead (`j`/`k` smooth-scroll, `g g`/`G` jump to page top/bottom)
— a global fallback bound only while no cursor is live. Holding `j`/`k` repeats.
A static **Navigation** section in the `?`-overlay documents it.

`?` opens a searchable cheat-sheet overlay listing every live binding grouped
by scope (reads the registry, so a page's bindings show only while it's
mounted); `Escape` closes it.

The **Worklist** is the one page that earns a full action keyset (rule #33,
table #35), registered under the `worklist` scope. Per-item verbs act on the
cursor's current row (a no-op while the cursor is unset or on a summary card):
`t` start timer, `o` open the work item, `e` edit priority/scheduling, `u`
toggle up-next, `m` promote a draft, `r` resolve a PR (confirm its suggested key
or open the pick modal), `d` mark a draft done — **confirm-gated** so no
keystroke destroys data in one press. Page verbs: `c` capture a draft (focuses
the field), `a` log time on another task, `p` pause/resume the timer, `s` stop
it, `n` add a timer note (focuses the field).

A few pages earn a small view-switcher keyset (rule #33, table #35, issue #45),
registered under their own scope (independent of the row cursor): **Utilization**
`w` weekly breakdown, `p` by project, `t` time entries; **Delivery** `c` by
client, `p` by project; **Estimation** `c` clears the
project filter. Capacity, Settings and Playground register no action keys.

While a **blocking modal** is open (edit, promote-pick, manual-pick, ad-hoc,
add-project, or the `?` cheat-sheet) the global listener early-returns
(`useModalGuard`, issue #43): every binding beneath the scrim — page-local,
`j`/`k` cursor, and global (`g`-nav / `.` / `?`) — is suppressed regardless of
focus, so no keystroke reaches a hidden row. `Escape` is the sole exit, closing
the modal via its own native handler. One modal at a time (single-layer
invariant, asserted). `CellEditor`'s inline popover is excluded — it stays on
the focus-guard / `data-kb-ignore` path.

## Design system

Ported from a Claude Design UI kit. Tokens (colours, type, radii, shadows) live
in the `@theme` block of `resources/css/app.css`; fonts (Hanken Grotesk + Spline
Sans Mono) load via Google Fonts in `resources/views/app.blade.php`. Reusable
presentational components are in `resources/js/Components/`. The `/playground`
route renders a live catalog of every component — the reference when composing
screens.

## Test

```bash
php artisan test   # backend (PHPUnit)
npm test           # front-end (Vitest) — keybinding registry, composables
```
