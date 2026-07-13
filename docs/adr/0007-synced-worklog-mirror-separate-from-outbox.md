# Synced Worklog mirror separate from the posting outbox

## Status

accepted

## Context

Fylla already has a `worklogs` table: an **outbox**. Each row is a Worklog the
app *created* from a closed Segment (ADR-0005) and *posts* to Kendo via the
`PostWorklog` job. It carries non-nullable `issue_id` and `timer_id` foreign
keys, `posted_at`, `kendo_worklog_id` — the state of an outbound write.

Issue #11 needs the opposite direction: **read** the user's logged hours back
from Kendo to measure personal billable utilization. Kendo is an admin-token
API — `GET /api/time-entries` returns the *whole team's* entries over a date
range — so the read is a bulk, filtered pull, not a per-Segment write.

The obvious temptation is to reuse the `worklogs` table for both: sync Kendo
time entries into it, make `timer_id` nullable (entries logged directly in
Kendo have no Fylla timer), and reconcile the app's own posted rows against
their `kendo_worklog_id`. That was considered and rejected.

Two directions, two natures:

- The **outbox** is authored locally, sparse (only what you tracked in Fylla),
  and hangs FK intent off issues and timers. Its lifecycle is "created →
  posted → stamped".
- The **read mirror** is authored by Kendo, dense (every entry in the window,
  across projects you never timed in Fylla), keyed on Kendo ids, and its
  lifecycle is "upsert → reconcile against the feed" — exactly the shape of the
  `issues` mirror under ADR-0003.

Collapsing them forces a nullable FK, a two-source reconciliation (some rows
mine-and-posted, some Kendo-only), and a table that is simultaneously a write
buffer and a read cache. The billable metric is a *read*; it should read a
mirror, not an outbox threaded with posting state.

## Decision

**Keep two tables.** The existing `worklogs` outbox is untouched. A new
`synced_worklogs` table is a read-only mirror of Kendo time entries, following
the `issues`/`SyncKendoIssues` pattern (ADR-0003):

- Keyed on `kendo_worklog_id`; stores `kendo_issue_id`, `kendo_project_id`,
  `minutes`, `started_at`, `note`, denormalized `issue_key`/`issue_title`,
  `synced_at`. No local foreign keys — Kendo ids only, like the issues mirror.
- `SyncKendoWorklogs` pulls the rolling window (`config('fylla.worklog_sync_days')`),
  filters to `config('fylla.kendo_user_id')` (required — the admin token returns
  everyone), upserts, and deletes rows **inside the fetched window** absent from
  the latest feed (a Kendo-side delete). Rows outside the window are never
  touched — absence there proves nothing, the direct analogue of the issues
  sync's truncated-feed guard.
- **Billability is not stored on the worklog.** It is a property of the project
  (per CONTEXT.md): a locally-owned `billable` boolean on the synced `projects`
  table (ADR-0004 pattern — mirrored rows, Fylla-owned column, preserved across
  sync). A worklog's billability is *derived* by joining `kendo_project_id →
  projects.billable`, so editing the billable list re-classifies on the next
  read, with no per-worklog re-tag.

Your own Fylla-posted hours therefore exist in **both** tables — as an outbound
write in `worklogs` and as Kendo's authoritative record in `synced_worklogs`.
That duplication is intended: they answer different questions ("what did I
send?" vs "what does Kendo say is true?") and are never reconciled against each
other.

## Consequences

- Two tables named for the same glossary concept (Worklog). A future reader will
  see `worklogs` and `synced_worklogs` and may want to merge them — this ADR is
  the answer to why not. Both speak Fylla's language (`kendo_worklog_id`, not a
  `time_entries`/Kendo-dialect table); the split is by *direction*, not concept.
- The billable metric and any future estimation-actuals read the mirror; the
  posting path keeps owning the outbox. Neither grows a nullable FK or a
  cross-source reconcile.
- The mirror is scoped to the single user for now (`kendo_user_id` filter).
  Team-wide worklogs (the PM lens) will revisit whether to widen the filter or
  store all users — deferred, not designed here.
- Reversible only by a migration that merges the tables and rewires two data
  flows, which is why it is recorded here rather than left implicit.
