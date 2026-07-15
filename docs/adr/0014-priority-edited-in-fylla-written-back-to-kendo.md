# Priority is edited in Fylla but written back to Kendo

## Status

accepted

## Context

Priority drives worklist ranking (ADR-0013) but is a Kendo-mirror field
(ADR-0003/0004) — every sync overwrites it, and until now Fylla wrote *nothing*
back to Kendo except Worklogs (ADR-0006). The user wants to change priority from
Fylla without switching to Kendo. Kendo stays the store of record for priority.

## Decision

Priority is editable in Fylla and **written back to Kendo synchronously**, via
read-modify-write: `GET /api/projects/{pid}/issues/{id}`, set the `priority` int
(0 Highest … 4 Lowest), `PUT` the whole object back. On success the local mirror
is updated optimistically so the row re-ranks immediately; on any failure Fylla
is left unchanged and the user sees an inline error. The local-owned scheduling
fields (`up_next`, `due_date`, `not_before`) are written to the local column only
— no provider round-trip.

## Consequences

- The Kendo update is a **full-replace PUT** (no PATCH). Reconstructing the body
  from Fylla's partial mirror would clobber unmirrored fields (`description`), so
  we round-trip Kendo's own object and mutate one field. Costs one extra GET per
  edit.
- The GET/PUT is a live provider call inside a *write* path — an accepted
  ADR-0003 exception, like the live Kendo search in PR resolution (ADR-0009).
- The next sync re-pulls priority and wins over the optimistic local value —
  deliberate, Kendo is store of record.
- Two edit paths now exist by design: local-only writes for scheduling fields,
  write-through for priority. Their failure semantics differ (local can't fail;
  priority can, and is isolated so a failed priority write still lets the
  scheduling-field edits in the same save succeed).
