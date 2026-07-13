# Capacity adjustments are one signed table, not two mirror concepts

## Status

accepted

## Context

Personal billable utilization is `billable ÷ capacity`, where weekly capacity
starts from a 32h contracted base (ADR-0004 established time off as a
Fylla-native, locally-owned concept; see `CONTEXT.md`). Two things move that
base per week in opposite directions:

- **Time off** (PTO / holiday / sick) — the user was excused from working, so
  the week's capacity shrinks. Already modelled as the `time_off` table.
- **Extra day** — by arrangement with the user's manager, the user works one
  extra ~8h day some weeks (a 40h week instead of 32), banking the hours toward
  extra vacation taken later. Without raising that week's capacity to 40, a
  normal-effort extra-work week reads ~94% instead of the true 75%, and the
  metric is wrong.

Both are dated deltas to a single week's contracted capacity, differing only in
sign. The time-off entry UI had not been built yet, so the cost of reshaping the
model was low.

## Decision

Model both as **one signed concept**: a `capacity_adjustments` table (renamed
from `time_off`) with `date` (unique), `hours` (signed integer), `reason`
(nullable). Negative hours = time off; positive = an extra day. Capacity for a
week is `contracted_base + Σ(adjustment hours in that week)`.

- One entry per date (unique index); adding a date upserts it.
- Time off falls on **weekdays only** (a weekend-off row would wrongly shrink a
  Mon–Fri week). An extra day may fall on **any day**, including a weekend.
- The partial current week prorates the base over elapsed Mon–Fri workdays and
  folds in only adjustments whose date has already passed — same rule for both
  signs.
- One entry page (`/time-off`) adds/edits/removes both directions.

Rejected: a separate `extra_hours` table parallel to `time_off`. It duplicates
schema, model, and reconciliation for what is one concept with a sign, and
leaves two ledgers to reason about forever.

## Consequences

- `UtilizationReport::weekData()` sums signed adjustments and **adds** them to
  the base, replacing the subtract-only time-off path. Existing tests that
  seeded a time-off week with `hours => 8` must now seed `hours => -8`.
- "Time off" and "extra day" survive as domain terms (`CONTEXT.md`) — they are
  the negative and positive faces of the same stored row, not separate tables.
- The sign convention is load-bearing: a positive `time_off`-style value would
  silently invert the capacity math. The column name (`capacity_adjustments`,
  not `time_off`) and the model name signal that hours are signed.
- Still Fylla-native and locally-owned (ADR-0004); no Kendo mirror.
