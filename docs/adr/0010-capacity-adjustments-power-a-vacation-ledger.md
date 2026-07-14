# Capacity adjustments carry an explicit type and power a vacation ledger

## Status

accepted (amends ADR-0008)

## Context

The capacity page was a sticky add-form plus a scrolling list of adjustment
rows. It doesn't match how the user actually reasons about time off: they keep a
year-at-a-glance calendar grid in a spreadsheet, plus an "Overzicht" tab that
tracks a running **vacation balance** in hours — how much is accrued, banked,
taken, and left, carried over year to year — to answer "can I afford another
trip later this year". Fylla had no notion of a vacation balance at all.

ADR-0008 modelled adjustments as **one signed row per date**, with the sign
alone distinguishing time off (negative) from an extra day (positive), hours as
a signed integer. Two new requirements break that encoding:

- **Public holidays** must shrink a week's capacity (a holiday week has fewer
  workdays, so measuring billable against the full 32h reads too low) but must
  **not** draw from the vacation balance — you don't spend vacation on Kingsday.
  A holiday is negative like time off, so sign no longer identifies the kind.
- **Planned vs confirmed.** Adjustments are planned in Fylla first, then
  transcribed into the external leave system of record. The user needs to see
  what's still un-entered.

## Decision

Reshape the page into a **year calendar grid** (12 month-rows × 31 day-columns)
that replaces the form and the list entirely. Cells are edited in place —
click, or drag a range for a vacation block, then a popover sets type / hours /
reason. The grid is one year with a year switcher; a live ledger panel sits
above it and a multi-year Overzicht table below.

Extend `capacity_adjustments`:

- Add an explicit **`type`**: `off` | `holiday` | `extra`. Type, not sign,
  identifies the kind. Backfill from existing sign (negative → `off`,
  positive → `extra`).
- **`hours` becomes decimal**, not integer — half-days and a 1.5h early finish
  are real.
- Add **`status`**: `planned` | `confirmed`, default `planned`.

Add one new stored input: a per-year **vacation accrual** (decimal hours,
manually entered inline in the ledger panel). Everything else in the ledger is
**derived** from the adjustment rows — no separate leave-balance store:

```
balance(uren)  = carryover + accrual + banked-extra + taken
carryover      = previous year's closing balance      (rolls forward indefinitely)
banked-extra   = Σ extra   adjustments that year       (positive)
taken          = Σ off      adjustments that year       (negative; holidays excluded)
balance(dagen) = balance(uren) / 8
balance(weken) = balance(uren) / 32                     (the contracted week)
```

Status semantics: the **vacation balance counts both** planned and confirmed
(so penciled-in trips show against affordability), but **only confirmed**
adjustments move the **utilization capacity denominator**. Holidays move
capacity but are **excluded from `taken`** — the one kind that is
capacity-affecting yet ledger-neutral.

Rejected: a standalone leave-balance system with its own entitlement/consumption
tables. The signed adjustment rows already encode every inflow and outflow; the
balance is a view over them plus one accrual number. A second ledger would drift
from the first.

## Consequences

- ADR-0008's "sign alone distinguishes the two kinds" is **superseded** by the
  explicit `type`. The signed-hours convention still drives the capacity sum.
- Migration: add `type` (backfilled from sign) and `status` columns; change
  `hours` from integer to decimal. New per-year accrual storage.
- `UtilizationReport` must filter adjustments to **`status = confirmed`** when
  summing capacity — planned days no longer move the metric.
- The store path no longer infers type from an `off`/`extra` toggle; it writes
  the chosen `type` directly, and `holiday` (like `off`) expands over weekdays.
- Still Fylla-native and locally-owned (ADR-0004); no Kendo mirror.
