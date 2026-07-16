# Ad-hoc timing of issues that aren't on the worklist

Some work — unassigned PM tasks (a shared "anyone logs here" bucket) and reviews of
tickets assigned to others — must be timed even though it never appears in the user's
my-issues feed, and self-assigning it in Kendo would be wrong (it breaks the shared
bucket or steals a colleague's ticket). We let the user search Kendo live (reusing the
`searchIssues` path from ADR-0009), pick any issue, and start a timer on it immediately;
the worklog posts to Kendo on stop like any other (the token is assignment-agnostic).

The picked issue is stored as an ordinary `Issue` row purely so the timer has a subject
to book against — but its `synced_at` is **left unstamped**, so the worklist's
"latest sync" filter (ADR-0013 render) never shows it as a card. This makes the feature
**transient by construction**: it surfaces only as the running timer and is gone the
moment the timer stops, with nothing to add to or remove from the list. The row survives
(the delete-guard keeps anything with timer/worklog history) and is reused via
`updateOrCreate` on `kendo_id` if the same issue is picked again.

## Considered options

- **Persistent worklist item** (a `foreign` flag force-including it on the list, plus a
  manual remove) — rejected: the user wants search → time → gone, not a growing pile of
  borrowed tickets to prune.
- **A separate lightweight timeable model** holding only Kendo coordinates — rejected:
  reusing the `Issue` mirror gets the timer, worklog, and delete-guard for free; the
  unstamped `synced_at` already keeps it off the list.
- **Manual after-the-fact duration entry** — deferred: Fylla is timer-driven end to end,
  and that would be a distinct feature applying to owned issues too.
