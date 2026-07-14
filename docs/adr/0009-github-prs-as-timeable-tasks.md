# GitHub PRs as timeable tasks routed to a resolved Kendo issue

## Status

accepted

## Context

GitHub is a **task-only** provider (CONTEXT.md); Kendo is the sole **worklog
provider**. PR review is ~half the work and must be timed. But a Worklog can
only post to Kendo, against a numeric `project_id` + issue `id`
(`POST /api/projects/{pid}/issues/{iid}/time-entries`). A PR has no such
coordinates of its own — it is linked, by convention, to a Kendo issue whose
key (`{PROJECT_KEY}-NNNN`) rides in the branch name or PR body.

Two facts make this awkward against the existing model:

1. **The linked issue is usually not the user's.** They review other people's
   work, so the issue is absent from the personal my-issues feed and therefore
   from the local `issues` mirror. It cannot be found by looking locally.
2. **Timers and worklogs assume a local issue.** `timers.issue_id` and
   `worklogs.issue_id` are non-null FKs; `PostWorklog` reads
   `issue->project_id` / `issue->kendo_id`. Nothing carries the coordinates of
   an issue that was never synced.

Rejected alternative — **mint a local `issues` row for the resolved issue.**
Everything downstream would work unchanged, but it pollutes "issues = my open
work" with foreign rows the my-issues sync would never update (and only spares
from reconcile-deletion because of timer history). The mirror would stop
meaning what it says.

Rejected alternative — **resolve only to local issues.** Simplest wiring, but
fact (1) means the common case (reviewing someone else's PR) can't be timed at
all.

## Decision

**A PR is a first-class timeable task with its own mirror; the timer subject is
polymorphic; and the Worklog carries the Kendo coordinates it posts to.**

- **`pull_requests` mirror** (GitHub-owned, ADR-0003 pattern). Keyed on the
  GitHub PR id; stores `number`, `repo`, `title`, `url`, `state`, `synced_at`,
  plus a recomputed `suggested_key`. `SyncGithubPullRequests` unions the GitHub
  Search API over each query in `config('fylla.github_pr_queries')` (default
  `['review-requested:@me', 'assignee:@me']`, env-overridable) — the filter is a
  config knob, not hardcoded, so it takes the full search syntax (`org:`,
  `author:@me`, `draft:false`, …). It upserts and reconcile-deletes PRs absent
  from the feed **unless they carry local timer history** — the exact rule the
  issues sync uses.
- **Key parsed from title then body** (not the branch name). The original plan
  read `head_ref` first via a per-PR `GET /repos/{o}/{r}/pulls/{n}` call, but the
  `Back-to-code` org **forbids classic PATs on repo-scoped endpoints** (that call
  returns 403), while the Search API — which carries `title` and `body` but not
  the branch ref — is unblocked. So the sync parses `suggested_key` from the
  search feed's `title` then `body`, makes no per-PR call, and never populates
  `head_ref`. A PR whose key lives only in the branch name falls back to the
  manual pick. Reading the branch name again would require a fine-grained PAT or
  GitHub App; revisit if resolution accuracy proves insufficient.
- **Resolution is Fylla-owned** (ADR-0004 pattern), never written by sync:
  `kendo_issue_id`, `kendo_project_id`, `kendo_key`, `resolved_at`. Set by a
  resolve *action* — one-click confirm of `suggested_key`, or a manual pick.
- **Polymorphic timer subject.** `timers` drops `issue_id` for
  `timeable_type`/`timeable_id` (Issue or PullRequest). The stack state machine
  (ADR-0005) is otherwise untouched; the live-uniqueness index moves to the
  morph pair.
- **Worklog carries its own coordinates.** `worklogs` gains
  `kendo_project_id` + `kendo_issue_id`, stamped at segment roll-up from
  whichever subject (`Issue` → its mirror fields; `PullRequest` → its resolved
  fields). `PostWorklog` posts to those columns and no longer dereferences
  `->issue`. The old `issue_id` FK stays as nullable provenance.
- **Resolution is a live Kendo read** (`GET /api/issues/search?query=<key>`),
  both for the confirm and the manual pick. This is a **deliberate exception to
  ADR-0003** (UI reads the local mirror): the set of all Kendo issues cannot be
  pre-mirrored, and resolution is a write-side action, not the steady-state read
  path. A PR cannot be timed until resolved, so the exception is bounded to that
  one action.

## Consequences

- The billable metric is unaffected: it reads `synced_worklogs` (ADR-0007), not
  the outbox. Denormalizing coordinates onto the outbox `worklogs` row is cheap
  precisely because that table is only ever posted from, never queried by issue.
- `PostWorklog` and `TimerService::rollUpSegment` change once, for all subjects;
  no PR-specific branch in the posting path.
- A second live-read exception now exists alongside "Sync now". Future readers
  should not generalize it — it is scoped to key→coordinate resolution, and
  ADR-0003 still governs every steady-state read.
- Reversible only by a migration that collapses the polymorphic timer back to an
  `issue_id` FK and drops the worklog coordinate columns — two data-flow
  rewrites — which is why it is recorded here.
