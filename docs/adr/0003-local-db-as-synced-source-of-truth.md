# Local database as the synced source of truth

## Status

accepted

## Context

The command center's core views — personal billable utilization over a rolling
1–3 month window, per-client monthly delivery vs. target, sprint pacing,
estimate-vs-actual trends across four developers — are all **aggregations over
time**. The upstream providers (Kendo, GitHub) do not expose these rollups, and
computing them by fetching live on every page load would be slow, would hammer
provider rate limits (GitHub's Search API is 30 req/min), and could not show
historical trend at all.

The Go app already leaned this way: a `TaskCache` with TTL, singleflight
fetches, and stale-cache-on-timeout fallback. The rewrite makes the pattern
first-class.

## Decision

A **local relational database (Eloquent) is the source of truth the UI reads
from.** Background sync jobs, on Laravel's scheduler, pull from Kendo and GitHub
and reconcile into local tables. The UI never calls providers live for reads;
it queries local data. Writes (log hours, create issue, set estimate) go to the
provider *and* update local state.

- Logged time is owned locally (Fylla is the logging surface) and posted out to
  Kendo; the local worklog record is authoritative for the billable metric.
- Trend history that providers cannot supply is retained locally.

## Consequences

- **Sync lag** is accepted: the UI can be up to one sync interval stale. This is
  the trade a future reader would question — it is deliberate; a command center
  dashboard values speed, history, and rate-limit safety over real-time
  freshness. A manual "sync now" escape hatch covers urgency.
- **Reconciliation** logic is now owned by Fylla (upserts, deletes for issues
  that vanished upstream, conflict handling when a local write races a sync).
  This is real complexity that live-fetch would not have.
- Partial provider failure degrades gracefully — the last good local snapshot is
  served (mirrors the Go app's stale-cache-on-timeout behavior), rather than a
  broken page.
- Enables features impossible against live APIs: rolling utilization windows,
  month-over-month client delivery, developer estimate-vs-actual over time.
