# Fylla-native fields are owned locally, never encoded in Kendo titles

## Status

accepted

## Context

The Go app smuggled Fylla-native scheduling metadata into the Kendo issue
**title** as inline clauses — due date (`{YYYY-MM-DD}` / `(due ...)`),
`(not before ...)`, `upnext`, `nosplit`, and recurrence (`(every ...)`). It
parsed them back out on read and rewrote the title on edit. This made the Kendo
title a smuggling channel for concepts Kendo does not model.

ADR-0003 makes the local database the source of truth for locally-owned data.
These fields are Fylla's own annotations on an issue, not Kendo concepts — so
title-encoding is both fragile (string munging on every read/write) and at odds
with local-as-truth.

## Decision

Fylla-native fields (due date, not-before, up-next, no-split, recurrence) live
**only** as local columns keyed by `kendo_id`. Fylla never parses them out of a
Kendo title and never writes them into one.

- Sync writes **only** Kendo-mirror fields (`updateOrCreate` on `kendo_id`);
  the local-owned columns are never touched by sync.
- These fields are set through Fylla's own UI, stored locally, used locally.
- **No migration** of existing title-encoded clauses. Legacy clause text in
  pre-existing Kendo titles is ignored; it is not parsed or cleaned.
- Estimate stays a native Kendo field (`estimated_minutes`), not title-encoded.

## Consequences

- Kendo titles stay clean going forward. Leftover clause text in issues created
  by the old Go app is cosmetic noise we accept rather than migrate.
- These fields are invisible in Kendo's own UI and non-portable — deliberate:
  Fylla is the surface where the user works, per `CONTEXT.md`.
- No fragile title string-parsing in the PHP client.
- `updateOrCreate` on `kendo_id` preserves local-owned columns across sync for
  free. The delete-absent reconciliation (the my-issues feed is authoritative
  for "my open work") will drop a row's local fields when an issue leaves the
  feed. Accepted while these fields are low-stakes and re-enterable; revisit
  (soft-delete / preserve-on-absent) if that changes.
- **Update:** `up_next`, `due_date`, and `not_before` are now user-editable
  (worklist edit UI; priority edits go through Kendo per ADR-0014). The
  loss-on-feed-absence stance above was revisited and **reaffirmed**: sync
  preserves the fields on every normal pass, deletion is guarded by the
  timer-history and truncation checks, and the only remaining loss is an issue
  genuinely leaving the user's open work — when a due date or pin no longer
  matters. `no_split` / `recurrence` stay reserved (no consumer yet).
