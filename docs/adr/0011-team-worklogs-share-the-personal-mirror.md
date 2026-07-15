# Team worklogs share the personal mirror, gated by a `mine()` scope

## Status

accepted

## Context

ADR-0007 stood up `synced_worklogs` as a read mirror of Kendo time entries,
scoped to the single user by a `config('fylla.kendo_user_id')` filter in
`SyncKendoWorklogs`. Its final consequence deferred a question: the PM lens
(CONTEXT.md → Client monthly target, Developer) needs **teammates'** logged
hours too, to total a managed client's delivery against its monthly target —
"widen the filter or store all users" was left open.

Issue #13 forces the answer. A managed client's target is met by the whole team
(*"all developers' logged hours plus the manager's own project hours"*), so the
sync must keep rows it currently drops. But the mirror is also read by the
**personal** billable-utilization metric — the promotion case, and the one place
teammates' hours must **never** appear. Two lenses, one table, opposite scoping
needs.

The obvious isolation is a second table, `team_worklogs`, so the personal metric
physically cannot see a teammate's row. Rejected: the schema is byte-identical to
`synced_worklogs` (it's the same Kendo time-entry mirror), and the manager's own
hours count toward *both* lenses — so a separate table means either storing your
rows twice or unioning the two at every team read. That is a real duplication
smell traded for avoiding a one-line scope.

## Decision

**One table.** `synced_worklogs` gains a `kendo_user_id` column and holds every
developer's rows for managed-client projects. The personal-vs-team split is a
**filter, not a table**:

- **Sync widening.** `SyncKendoWorklogs` keeps a fetched entry when it is *mine*
  (`user_id === kendo_user_id`) **or** its project is assigned to a client
  (`projects.client_id` is not null — the managed gate, see below). Everything
  else is still dropped. Reconciliation deletes within the window as before, now
  over the widened kept-set.
- **Managed = assigned to a client.** A `clients` table (Fylla-owned: `name`,
  nullable `monthly_target_hours`) and a nullable `projects.client_id` FK carry
  the mapping. A project with a `client_id` is managed and pulls the whole team;
  an unassigned project falls back to its own-name pseudo-client and stays
  yours-only. Marking a client "managed" is not a separate flag — a client's
  existence *is* the mark.
- **`mine()` scope.** `SyncedWorklog::mine()` filters to `kendo_user_id`. Every
  **personal** reader applies it — today `UtilizationController` and
  `UtilizationReport`. Team readers query the table unscoped, filtered by client.

## Consequences

- The personal utilization metric now reads a table that contains other people's
  hours. Its correctness depends on the `mine()` scope being present at every
  personal read site. This is the load-bearing, non-obvious invariant — a
  regression test (insert a teammate row, assert utilization ignores it) guards
  it, and this ADR is why the scope is **not** dead code to be deleted.
- Any new reader of `synced_worklogs` must decide, explicitly, whether it is a
  personal or a team read, and scope accordingly. Unscoped means team-wide.
- Team-issue sync (per-developer estimate-vs-actual, in-progress aging) is a
  separate concern with its own reconciliation and is **not** stored here —
  deferred to the developer-progress lens, not designed in this ADR.
- Reversible only by splitting the table back out and rewiring the sync and both
  read paths — which is why the shared-table choice is recorded rather than left
  implicit.
