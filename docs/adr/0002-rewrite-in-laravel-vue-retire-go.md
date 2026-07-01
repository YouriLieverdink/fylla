# Rewrite Fylla in Laravel/Vue and retire the Go app

## Status

accepted

## Context

Fylla today is a Go CLI/TUI. Its purpose is expanding from a personal task
scheduler into a single-user **command center** for the user's work at Back to
code: personal billable-utilization tracking (the promotion metric), plus a
project-manager lens over ≥2 clients and 4 developers (per-client monthly hour
targets, sprint pacing, team progress). See `CONTEXT.md`.

That target is a data-heavy, dashboard-shaped, multi-view web application with
scheduled background syncing, trend history, and rich UI — not a terminal tool.
The scope also shrank on the provider side: Jira, Jibble, Todoist, and Local
are dropped, leaving only Kendo (task + worklog) and GitHub (task-only, source
of PR reviews). The elaborate multi-provider abstraction that dominates the Go
codebase (`MultiTaskSource`, `multiFetcher`, provider routing, key-format
inference) was built for five providers and is now largely dead weight.

## Decision

Rewrite Fylla as a **Laravel (PHP) + Vue** web application and retire the Go
codebase entirely. Do not wrap the Go binary as a sidecar; do not run two
runtimes.

- Provider clients (Kendo, GitHub, Google Calendar) are reimplemented in PHP
  with Laravel's HTTP client — they are thin REST wrappers.
- Laravel's queues, scheduler, events, and Eloquent carry the background sync,
  billable rollups, and storage.
- The one genuinely non-trivial piece of Go logic — the calendar
  slot-finding/scoring algorithm — is *ported* carefully rather than
  rewritten, if and when scheduling stays in scope.

## Consequences

- A working Go application is abandoned. This is the surprising, expensive part
  a future reader would question — it is deliberate: the new target is a web
  dashboard, and a two-runtime deploy (Go sidecar + Laravel) is not justified
  for a single-user tool.
- The domain language in `CONTEXT.md` and the worklog/task-provider decoupling
  in ADR-0001 are stack-independent and carry over verbatim.
- Single-user (one person, acting as themselves) makes auth trivial in Laravel
  — no multi-tenancy, one login.
- Reimplementing provider clients in PHP is accepted cost; they are documented
  REST APIs and short with Laravel's HTTP client.
