# Kendo is the only worklog provider — post directly, no provider abstraction

## Status

accepted (supersedes the multi-provider direction of ADR-0001 in the Laravel rewrite)

## Context

ADR-0001 (Go era) decoupled the worklog provider from the task provider so
Jibble could receive hours for Todoist tasks. That world is gone: the Laravel
rewrite ships **Kendo and GitHub only**, and GitHub is task-only. CONTEXT.md
states Kendo is the **sole worklog provider** — GitHub PR hours still post to
Kendo via the PR's linked Kendo issue.

Issue #10 (post worklogs to Kendo) forces the choice: reintroduce a
`WorklogProvider` interface as ADR-0001 implies, or post straight to
`App\Kendo\Client`.

## Decision

Post worklogs **directly on `App\Kendo\Client`**. No `WorklogProvider`
interface, no `worklog.provider` config, no registry — there is exactly one
implementation and no second destination on the roadmap.

- `Client::postWorklog(int $projectId, int $issueId, int $minutes, string $startedAt, ?string $note): int`
  wraps `POST /api/projects/{projectId}/issues/{issueId}/time-entries` and
  returns the created entry `id`.
- If a second worklog destination ever appears, extract the interface then,
  against its real shape.

## Consequences

- A future reader who finds ADR-0001 saying "decouple the worklog provider"
  will expect an interface. There isn't one, on purpose — this ADR records why.
- The stale global instruction "worklog features must route through a
  `worklog.provider` interface" is retired; it described the Go multi-provider
  world, not this rewrite.
- Reversible at moderate cost (introduce interface + adapter) if a second
  worklog provider lands — but YAGNI until then.
