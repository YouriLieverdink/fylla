# One Worklog per segment, not one per timed task

## Status

accepted

## Context

The stack-based timer (commit 068ecf1) modelled a timer as a task with many
segments across pause/resume cycles, and `rollUpWorklog` **summed** all of a
timer's segments into a **single** Worklog on stop, joining the per-segment
comments as `[i/n] …`.

That collapses the shape of the day. The user works one issue, pauses it to pick
up another, comes back — a normal interleaved day across ≥2 clients. A single
summed Worklog per task loses when each stretch actually happened and forces the
comment into a `[i/n]` blob. The user wants Kendo to reflect the real linear
history: each continuous stretch of work as its own entry.

Because the timer stack keeps **at most one segment open at a time** (starting a
timer pauses whatever ran), segments already tile the day with no overlap — they
are exactly the non-overlapping intervals a faithful day-history needs.

## Decision

**Each segment posts its own Worklog**, written the moment the segment closes
(on pause, on starting another timer, or on stop) — the point at which its
duration is final.

- A Worklog's `minutes` = that one segment's duration, rounded to the nearest
  minute. A zero-minute segment is discarded (no Worklog row).
- A **note** attaches to the currently open segment and rides into that
  segment's Worklog comment. Notes carry a wall-clock stamp; the comment is one
  `HH:MM — text` line per note. No cross-segment summing, no `[i/n]` join.
- The notes panel shows only the open segment's notes and starts empty when a
  new segment opens on resume.
- Segments (hence Worklogs) are the time-accounting unit; the former single
  `segments.comment` field is replaced by a per-segment `notes` relation.

## Consequences

- Kendo shows a truthful linear day: one issue worked in three sittings yields
  three Worklogs at three real start times, not one merged entry. This is the
  point, but it is more Kendo entries per task than the old model — accepted.
- Worklog rows are created incrementally (on every pause) rather than only on
  stop. `rollUpWorklog` becomes a per-segment roll-up called from
  `closeOpenSegment`, not a stop-only sum.
- A note can only be added while a segment is open. When the active timer is
  paused and nothing else runs, there is no open segment, so the note input is
  disabled. (Reverses an earlier "allow notes while paused" intent, which is
  incompatible with per-segment notes.)
- Reversible only via a migration + service rewrite, which is why it is recorded
  here rather than left implicit.
