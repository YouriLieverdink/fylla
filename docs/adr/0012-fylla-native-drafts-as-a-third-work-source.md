# Fylla-native drafts are a third work source

## Status

accepted

## Context

Until now every unit on the worklist came from a provider: a Kendo issue or a
GitHub PR (see "task provider" in `CONTEXT.md`). But the user constantly needs
to jot lightweight to-dos — "email this client", "talk to this person" — that
should not become a Kendo ticket, at least not yet. Forcing every such thought
through Kendo's create flow is exactly the friction Fylla exists to remove, so
those to-dos currently live nowhere and the "control center" has a hole.

## Decision

Introduce the **Work item** umbrella and a Fylla-native **draft** as a third
work source alongside Kendo issues and GitHub PRs.

- A **draft** is owned entirely by Fylla — never synced to or from any provider.
- A draft is **un-timeable** while it remains a draft: it has no Kendo
  coordinates to book against, and Kendo is the sole worklog provider
  (ADR-0006). To log time you **promote** it (below).
- **Promote** converts a draft into a real Kendo issue. One-way: after
  promotion it is an ordinary Kendo-mirrored issue and the draft is gone.
- Drafts live in the worklist alongside provider items and are ranked by the
  same scorer (ADR-0013). A draft carries the schedulable fields the scorer
  reads (`priority` defaulting to Medium, plus optional `due_date` /
  `not_before` / `up_next`); the components it cannot have (remaining-estimate
  quick-win, type bonus) simply contribute 0.

## Consequences

- The worklist spans three sources but the timer/worklog path is unchanged:
  only provider-backed items are timeable, so ADR-0006 (Kendo-only worklog)
  and the whole posting pipeline stay intact.
- Drafts need their own local table (Fylla-owned, no `kendo_id`), untouched by
  the sync reconciliation that deletes absent Kendo issues.
- Promotion is deliberately one-way — no "demote to draft". A promoted item is
  a Kendo ticket like any other; reversing would mean deleting a Kendo issue,
  which is out of scope.
- This does **not** re-open the "no email/Slack send" boundary in `CONTEXT.md`:
  a draft is a private note-to-self, never an outbound message.
