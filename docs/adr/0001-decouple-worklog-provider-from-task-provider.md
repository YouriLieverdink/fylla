# Decouple the worklog provider from the task provider

## Status

accepted

## Context

Until now every supported backend that logged hours (notably Kendo) was *both* the task provider and the worklog provider, so the code conflated the two: a focus timer remembers the provider its task came from (`sr.Provider`) and posts the worklog there, and `resolveWorklogTarget` hardcodes `worklogProv == "kendo"` when deciding whether a task key needs target resolution.

Adding Jibble breaks this. Jibble is a time-clock with no tasks — it can only ever be a **worklog provider**, never a task provider. The intended setup is a split: Todoist supplies tasks (including chores), Jibble receives hours for a chosen subset. A Todoist task key is meaningless to Jibble.

## Decision

The worklog provider is independent of the task provider. In the stop/post path:

- Hours always post to `worklog.provider`, regardless of which provider the task came from (the `sr.Provider` override is dropped for posting).
- Task operations — mark-done, remaining-estimate — stay routed to the task's own provider.
- Target resolution generalizes off the Kendo-specific check: when the worklog provider's key format can't be derived from the task key (always true for Jibble, since no task key names a Jibble Project), the flow prompts the user to pick a target from the provider's `ListProjects`.

## Consequences

- `stop.go` no longer assumes task-provider == worklog-provider; this is the assumption a future reader would otherwise expect, so it is recorded here.
- Jibble must still appear in the `providers` array (only registered providers are routable as the worklog provider) and therefore stubs `FetchTasks` to return an empty slice so it contributes no tasks to the task/schedule tabs — mirroring how GitHub is a task provider that stubs `PostWorklog`.
- A static task→Jibble-Project map and per-task target memory were considered and deferred (YAGNI); the stop flow prompts for the Jibble Project on every stop instead.
