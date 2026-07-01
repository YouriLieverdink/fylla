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
_Avoid_: timesheet

### Billable tracking

**Billable project**:
A project on a user-configured list whose logged hours count as billable. Not everything worked on is billable — internal, admin, and some client work is not. Billability is a property of the **project**, not of the individual worklog. The list keys off **Kendo projects** (Kendo is the sole worklog provider).
_Avoid_: client project (a project can be client-facing but non-billable, and vice versa)

**Contracted capacity**:
The hours the user is contracted to work per week — currently **32h** — minus any time off that week. Time off (PTO, holiday, sick) is **excluded from the denominator**: the target is 75% of the hours the user *should have been working*, so a week with a day off caps at `32 − 8 = 24h` of capacity. This is the denominator of the billable target.

**Billable target / utilization**:
`billable-hours ÷ contracted-capacity`, cumulative over a **rolling window of ~1–3 months**, target **75%**. The user's **personal** metric only (the promotion case, Jan 2027) — never computed for other developers, and distinct from the client monthly targets in Project management below. The target is **soft, not a cliff** — 73% is acceptable; 75% is the aim. Reported as a trend, not pass/fail. Distinct from hours-actually-worked: a light productive week must not inflate it, a heavy week must not be required to hit it.
_Avoid_: productivity, efficiency (those are different, and calm mode already uses "efficiency")

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
One continuous run of a single timer, bounded by start and pause (or stop). A timer accumulates multiple segments across pause/resume cycles, each with its own optional comment. Stopping sums the segments (rounded) into one Worklog.

## Flagged ambiguities

- **"Project"** is overloaded. Kendo and GitHub each have their own projects/repos; the billable-projects list and every worklog key off **Kendo projects**. A reviewed PR books its hours to its linked Kendo issue (below), whose project may differ from anything implied by the repo.

**PR review**:
Reviewing a GitHub pull request — about half of all work. A first-class timed work item. Every PR is linked to a **Kendo issue** (by convention: the branch name or PR body carries the issue key), so review hours book to *that issue*, and the issue's Kendo project decides billability. Resolution follows the existing rule — parse the issue key from the branch/body, confirm, and fall back to a manual pick only when none is found.
