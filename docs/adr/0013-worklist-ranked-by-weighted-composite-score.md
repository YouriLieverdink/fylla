# The worklist is ranked by a weighted composite score

## Status

accepted

## Context

Fylla's original purpose is a single worklist the user attacks top-to-bottom
without hand-ranking dozens of items every day. A naive sort (by priority, or
by due date) buries the nuance: a low-priority task due tomorrow should beat a
high-priority task with no deadline; a quick win should surface over a
day-long slog of equal priority; a deferred task should sink without vanishing.
The retired Go TUI (ADR-0002) already solved this with a weighted composite
scorer (`internal/scheduler/sorter.go`), and the user wants that behavior back.

## Decision

Rank the worklist by a **weighted composite score**, ported from the Go app,
recomputed on every render — never hand-drag ordering.

```
score =  w_priority · PriorityScore   (1–5 → levels 100/80/60/40/20)
       + w_due      · DueDateScore     (due today=100, ≥30d out=0, linear)
       + w_estimate · EstimateScore    (quick wins: <8h inverse, ≥8h=0)
       + CrunchBoost                   (+20 for due ≤3d, overdue=full 20)
       + TypeBonus                     (flat per issue-type)

then:  up_next  → score += w_upnext         (pin near top)
       else     → score *= NotBefore(0.2–1.0)   (defer, don't hide)
```

Default weights (from the Go app): `priority 0.45, due 0.30, estimate 0.15`,
`upNext +50`, priority levels `100/80/60/40/20`.

- **Age is dropped.** The Go app had an age component (`w_age 0.10`), but Fylla
  stores no issue `created_at` (the `Issue` mirror has `updated_at` only), and
  `updated_at` is a bad proxy — an edited task would look brand new. Not worth a
  new sync column for the weakest signal.
- **`not_before` demotes, it does not hide** (×0.2–1.0). A future-dated task
  stays visible, just pushed down.
- **`up_next` is a large additive boost, not an absolute pin** — a strong nudge
  to the top that the user flips on the few things they commit to next.
- **Weights are hardcoded to the proven defaults for now.** A settings page to
  tune them live (as the Go TUI's Tuning tab did) is a deferred fast-follow, not
  a launch requirement.

## Consequences

- The score is a pure function of fields already synced or locally owned
  (priority, `due_date`, `not_before`, `up_next`, `remaining_minutes`, `type`),
  so ranking needs no new provider data.
- Because it recomputes on render, the list re-ranks itself as due dates
  approach and as the user edits fields — no stored rank to keep in sync.
- The scorer should expose a per-item **breakdown with reasons** ("due in 2
  days", "3 days overdue") as the Go app did, so an auto-ranked list is not a
  black box.
- Reversible in principle (it is an algorithm), but recorded because a future
  reader will otherwise "simplify" it to a plain sort and lose the deliberate
  behavior above.
