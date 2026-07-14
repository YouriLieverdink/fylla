# Fylla

Fylla is a single-user command center for one person's work at Back to code. It serves that person in **two roles**, and the same logged hours roll up into both:

- **Individual contributor** — measured on **personal utilization**: is 75% of contracted hours billable (the promotion metric). Purely personal; no other developer's data involved.
- **Project manager** for ≥2 clients — directing 4 developers, tracking **team-aggregate delivery** against per-client monthly hour targets, and planning each sprint.

It combines task providers (where work comes from) with a calendar and a worklog destination (where hours are recorded). Every write is the user acting as themselves; team data is **read-only** and scoped to the clients the user manages. Fylla is the surface where the user logs their own time — logging in Kendo directly is painful, which is why Fylla exists.

## Language

### Provider roles

**Task provider**:
A backend that supplies schedulable work — it can fetch tasks and (usually) set estimates, due dates, and priorities. Two remain: **Kendo** (task + worklog) and **GitHub** (task-only, and the source of PR reviews — roughly half of all work).
_Avoid_: source (ambiguous with calendar source)

**Worklog provider**:
The backend that receives logged hours — **Kendo**, the sole worklog provider. GitHub is task-only, so hours on a GitHub PR still post to Kendo (via the PR's linked Kendo issue).
_Avoid_: time tracker (use only for the external product, not the role)

**Worklog**:
A record of time spent — a duration with a start time, attributed to a unit of work. Posted to Kendo, read back to measure progress against targets.
_Avoid_: timesheet; time entry (Kendo's wire word — it's a Worklog on our side)

### Billable tracking

**Billable project**:
A project on a user-configured list whose logged hours count as billable. Not everything worked on is billable — internal, admin, and some client work is not. Billability is a property of the **project**, not of the individual worklog. The list keys off **Kendo projects** (Kendo is the sole worklog provider).
_Avoid_: client project (a project can be client-facing but non-billable, and vice versa)

**Contracted capacity**:
The hours the user is contracted to work per week — a **32h** base, moved per week by capacity adjustments (below): time off shrinks it, an extra day grows it. The target is 75% of the hours the user *should have been working*, so a week with a day off caps at `32 − 8 = 24h` and a week with an agreed extra day rises to `32 + 8 = 40h`. This is the denominator of the billable target. The workweek is **Mon–Fri**; the **current (partial) week prorates** the base to workdays elapsed so far (`32 × elapsed÷5`), folding in only adjustments whose date has already passed, so the metric reflects pace-to-date rather than dipping every Monday. Completed weeks use the full base ± adjustments.

**Capacity adjustment**:
A dated delta to a single week's contracted capacity, **owned by Fylla** — Kendo has no such concept. One adjustment per date, carrying an explicit **type** — **time off**, **holiday**, or **extra day** — and a **planned/confirmed** status (both below). Type, not sign alone, distinguishes the kinds (time off and holiday are both negative). Moves the **denominator** (capacity) only once **confirmed**, and never the billable numerator.
_Avoid_: overtime (an extra day is agreed, contracted extra work — not uncounted grind, which the metric captures in the numerator on its own)

**Time off**:
A negative capacity adjustment for **vacation** — hours not worked on a given date (PTO, sick). Dated hours (e.g. `−8` = a full day off). Falls on a **weekday** (weekend leave is meaningless against a Mon–Fri week). A week fully off contributes zero billable and zero capacity, dropping out of every average. **Draws the vacation balance** (unlike a holiday).

**Holiday**:
A negative capacity adjustment for a **public holiday** — shrinks that week's capacity exactly like time off, but is **excluded from the vacation balance** (you don't spend vacation on Kingsday). Entered manually per date. The one adjustment kind that moves capacity yet is ledger-neutral.

**Extra day**:
A positive capacity adjustment — an agreed extra workday (e.g. `+8`), arranged with the user's manager and **banked toward extra vacation** taken later. Raises that week's capacity (a 40h week instead of 32) so the 75% target scales to the hours actually contracted; without it, a normal-effort extra-work week reads far above target and the metric is wrong. May fall on **any day**, including a weekend.

**Worked hours**:
The total hours logged in a week — Σ of **all** Worklogs that week, billable and non-billable alike (internal, admin, and non-billable client work all count). Distinct from **Actual** (which is per-issue) and from **capacity** (contracted, not logged). Reflects only Worklogs already posted to Kendo; a timer still running is not yet worked hours. Shown alongside utilization so the effort behind the metric is visible, but is not itself a denominator — `worked − billable` is simply the non-billable remainder.

**Billable target / utilization**:
`billable-hours ÷ contracted-capacity`, cumulative over a **rolling window of ~1–3 months** (a configurable number of weeks, default 13), target **75%**. The **headline** is that single cumulative figure; the **trend** plots each week's own utilization (per-week, not running-cumulative) to show volatility. The user's **personal** metric only (the promotion case, Jan 2027) — never computed for other developers, and distinct from the client monthly targets in Project management below. The target is **soft, not a cliff** — 73% is acceptable; 75% is the aim. Reported as a trend, not pass/fail. Distinct from hours-actually-worked: a light productive week must not inflate it, a heavy week must not be required to hit it.
_Avoid_: productivity, efficiency (those are different, and calm mode already uses "efficiency")

### Vacation ledger

**Vacation ledger**:
A per-year running account of vacation entitlement, in **vacation hours**. The same capacity-adjustment rows that move the utilization denominator double as its inflows and outflows: an **extra day** (positive) banks hours, **time off** (negative) draws them down. Purely a Fylla-owned view over dated adjustments plus a yearly accrual input — no new provider data. Carries over year to year.

**Vacation hours**:
The unit of the ledger — hours, **decimal** (e.g. `41,19`). Converted for display to **days** (`÷8`) and **weeks** (`÷32`, the contracted week). Individual day-cell adjustments are decimal too (e.g. a `1,5h` early finish), not restricted to whole days.

**Vacation accrual** (Dutch: _Vakantieuren_):
The vacation hours granted for a given year — a **manually entered decimal, one per year** (statutory accrual varies; a partial first year is smaller). The only new stored input the ledger needs.

**Banked extra** (Dutch: _Meeruren_):
Σ of the year's **positive** capacity adjustments (extra days). Adds to the balance. Derived, not stored separately.

**Taken** (Dutch: _Opgenomen_):
Σ of the year's **time-off** adjustments only (the vacation kind). A negative number that draws the balance down. **Holidays are excluded.** Derived, not stored separately.

**Carryover** (Dutch: _Restant_):
The previous year's closing balance, opening the current year. Balances roll forward indefinitely.

**Vacation balance** (Dutch: _Saldo_):
`carryover + accrual + banked-extra + taken` for a year, in vacation hours; shown also in days and weeks. Answers "how much vacation is left" and "can I afford another trip later this year". Counts **planned and confirmed** adjustments alike (see below).

**Planned / confirmed**:
A status on a capacity adjustment. **Planned** = it exists only in Fylla, still to be entered into the external system of record (the official leave tracker). **Confirmed** = already entered there. New adjustments default to **planned**. The two consumers treat status differently: the **vacation balance counts both** (so penciled-in trips show against affordability), but only **confirmed** adjustments move the **utilization capacity denominator** — until a day off is confirmed, contracted hours are unchanged for the metric. Planned cells render distinctly (hollow) so the still-to-enter work is visible at a glance.

### Estimation

**Estimate**:
A Kendo issue's expected effort, always in **hours** (never story points). Set at creation and editable.

**Actual**:
The hours Fylla has logged against an issue. Because Fylla is the logging surface, actual is known directly and compares to the estimate without conversion.

**Estimation bias**:
The systemic gap between estimates and actuals over finished issues (e.g. "underestimates by ~40%"), optionally sliced by project/label. The estimation **feedback loop** (per-issue estimate-vs-actual + rolling bias) ships first; an **estimation aid** that surfaces similar past actuals when estimating a new issue comes later, and "similar" means same project + label ranked by recency — no ML.

### Client & project management

**Client**:
A party work is done for. The work hierarchy: **Client → one or more Kendo projects → one or more repositories → PRs/issues.** A client maps to a configured set of Kendo projects; an unmapped project defaults to a client of its own name. For clients the user **manages** (PM role), the client is the unit a monthly hour target and delivery pacing attach to; for others it is just a grouping lens over synced Kendo data (a context view — active issues, logged hours, sprint status — to brief before a call). Fylla stores no client communications; reminders and send-email/Slack are **out of scope**.

**Client monthly target**:
A per-client goal of **hours to deliver each month**, met by the whole team assigned to that client — all developers' logged hours **plus the manager's own project hours** count toward it. This is **team-aggregate**, and orthogonal to personal utilization (the same manager hours count toward both). Configured as a simple list of `client → target hours/month`, fixed and manually maintained; not auto-scaled for holidays.
_Avoid_: capacity (that is the personal, per-week denominator — a different thing)

**Sprint pacing**:
Because a month holds one or more sprints, a client's monthly target must be spread across them rather than back-loaded. The pacing check: cumulative committed/estimated hours by sprint N should reach that sprint's proportional share of the monthly target (with two sprints, ≥50% by the first). A sprint is attributed to the month containing its **end date** (no proportional splitting across the boundary). Sprint dates come from Kendo. The sprint view shows, per managed client, two series against the pace line: **committed estimate** (the plan, read at planning time) and **accumulated actual** (the delivered reality, watched mid-sprint).

**Developer**:
One of the (currently four) teammates working a managed client's projects. They log hours to Kendo like the manager does. Fylla **reads** their issues, estimates, and logged hours — never writes to their work. Progress is shown two ways: **estimate-vs-actual per issue** (the estimation feedback loop pointed at the team — are estimates holding, who is blowing through them) and **in-progress aging** (days since an issue moved to "in progress"). Nothing fancier (no cycle-time, WIP, or throughput analytics). Team visibility is scoped to developers on managed clients, not the whole company.

### Timer

**Timer stack**:
The set of running/paused timers. Starting a timer while one runs pushes a new timer on top (a nested interruption); stopping the top timer auto-resumes the one beneath. Index 0 is active, the rest are paused.

**Segment**:
One continuous run of a single timer, bounded by start/resume and pause/stop. Because only one segment is open across the whole timer stack, segments tile the day with no overlap **automatically**. The open segment's **start is hand-editable** (correcting a forgotten/late start): the user may set it to any wall-clock time at or before now, which can pull it *before* the previous stretch's already-posted Worklog. So the no-overlap tiling is the automatic default, not a hard guarantee — a manual start correction may create overlap (double-counted minutes) or a gap (unaccounted time), which the user owns and reconciles in Kendo. **Each segment posts its own Worklog** when it closes (duration rounded to the minute; a zero-minute segment is discarded). A single issue worked in three sittings yields three Worklogs, not one.

**Note**:
A free-text entry the user adds while a segment runs, stamped with the **wall-clock time** it was added (e.g. `14:32`). A note belongs to the **open segment** and rides into that segment's Worklog comment. The notes panel shows only the current segment's notes and starts empty when a new segment opens (on resume). Because a segment has no internal pauses, the gap between consecutive note stamps is worked time. Notes replace the former per-segment comment.
_Avoid_: comment (the old single-per-segment field this supersedes)

## Flagged ambiguities

- **"Project"** is overloaded. Kendo and GitHub each have their own projects/repos; the billable-projects list and every worklog key off **Kendo projects**. A reviewed PR books its hours to its linked Kendo issue (below), whose project may differ from anything implied by the repo.

**PR review**:
Reviewing a GitHub pull request — about half of all work. A first-class timed work item, timed through the same **timer stack** as an issue, its hours posted as a **Worklog**. Every PR is linked to a **Kendo issue** (by convention: the branch name, else the PR body, carries the issue key `{PROJECT_KEY}-NNNN`), so review hours book to *that issue*, and the issue's Kendo project decides billability. The linked issue is usually **not the user's own** — they are reviewing someone else's work — so it is not in the personal task feed; resolution looks it up across **all** Kendo issues (a live global search by key), not the local mirror. Resolution — parse the key, confirm the match, or fall back to a manual pick when none is found — is **required before the PR can be timed** (a Worklog cannot post without a resolved issue).
