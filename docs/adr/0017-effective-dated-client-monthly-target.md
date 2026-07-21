# Client monthly target is effective-dated: default plus forward-persisting overrides

A client's retainer changes over time, and `clients.monthly_target_hours` was a single
mutable value — the app had no memory of what the target *was* in a past month, so any
per-month read (Delivery, later reporting) silently used today's number for history.

`monthly_target_hours` stays as the **baseline default**. A new `client_target_changes`
table records overrides: one row per (client, `effective_from`), where `effective_from`
is always a first-of-month DATE and `hours` the new target. A change means "from month M
onward, the target is H" and **persists forward** until the next change.

Resolution lives in one place, `Client::targetForMonth($month)`: the latest change with
`effective_from` on or before the month's start, else the default. `DeliveryReport`
resolves its current-month target through it instead of reading the column; with no
override rows, behavior is identical to before.

## Considered options

- **Keep mutating the single column** — rejected: loses history; last month's Delivery
  view would retroactively repace against the new retainer.
- **Full validity ranges (`effective_from`/`effective_to`)** — rejected: forward
  persistence makes the end date derivable from the next row; storing it adds an
  overlap invariant to police for no query it enables.
- **Per-month snapshot rows (one row per client-month)** — rejected: writes on a clock
  instead of on intent; a retainer change is the event worth recording, not each month.
- **Replace the default column with a required initial change row** — rejected:
  migration churn across existing clients and UI for no resolver simplification; the
  nullable default already expresses "no target agreed".
