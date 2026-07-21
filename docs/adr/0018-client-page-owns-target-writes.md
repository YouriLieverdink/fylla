# The client page owns client-target writes, relaxing its read-only rule

The Client context page (#56) was read-only by design, and target editing lived in the
Delivery card's config footer. With effective-dated overrides (ADR-0017) the target is
no longer one number — it's a default plus a history of changes, and the only place
that history is *visible* is the delivery-history widget on the client page (#67).
Editing a value away from where its effect shows made the footer field misleading:
it silently wrote the default while overrides were what months actually resolved to.

All target editing moves into the history widget on the client page (#68): the default
(`PATCH /clients/{client}`, the existing route) plus add/edit/delete of override rows
(`POST /clients/{client}/target-changes`, `PATCH`/`DELETE /target-changes/{targetChange}`).
Submitted dates are normalized to first-of-month server-side; a second submit for an
existing month corrects that row rather than erroring. The Delivery config footer keeps
billable pills and project assignment only.

This is the client page's first write. The read-only rule is relaxed, not dropped:
board data (issues, lanes, developers) stays read-only Kendo mirror; only Fylla-owned
client-target config is writable here.

## Considered options

- **Keep target editing on Delivery, add override editing there too** — rejected: the
  footer has no month axis; overrides without the history they affect is exactly the
  edit-blind-to-effect problem again.
- **Edit both places** — rejected: two editors for one value invites drift and doubles
  the UI to maintain, for a single-user app.
- **A separate client-settings page** — rejected: a whole page for one default and a
  short list of overrides; the widget already renders the timeline the overrides act on.
