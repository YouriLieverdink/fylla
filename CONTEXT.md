# Fylla

Fylla schedules and tracks work by combining task providers (where work comes from) with a calendar and a worklog destination (where hours are recorded). It serves several distinct contexts in one person's life — paid work, personal business, and volunteer commissions.

## Language

### Provider roles

**Task provider**:
A backend listed in the `providers` array that supplies schedulable work — it can fetch tasks and (usually) set estimates, due dates, and priorities. Examples: Kendo, Todoist, GitHub, Local.
_Avoid_: source (ambiguous with calendar source)

**Worklog provider**:
The single backend named by `worklog.provider` that receives logged hours. It need only post and read back time; it does not have to supply tasks. A provider can be one, the other, or both. Kendo is both; GitHub is task-only; Jibble is worklog-only.
_Avoid_: time tracker (use only for the external product, not the role)

**Worklog**:
A record of time spent — a duration with a start time, attributed to a unit of work. Posted to the worklog provider, read back to measure progress against targets.
_Avoid_: time entry (that is Jibble's term for the same thing — see below), timesheet

### Jibble

**Jibble**:
An external time-clock product used as a worklog provider. It has no concept of tasks or issues — only the entities below.

**Jibble Client**:
The top of Jibble's hierarchy — an organization or party work is done for (e.g. "Tjas", the association Youri volunteers for). Groups Projects. In Fylla it is display/grouping context only, not the booking target.

**Jibble Project**:
The bucket hours are attributed to, optionally belonging to a Client (e.g. "ICie", "KasCie" under Tjas). **This is the booking target** — Fylla's worklog "issueKey" slot carries the Jibble Project, and `targets` are keyed by Project. Displayed as `Client / Project` to disambiguate same-named projects across clients.

**Jibble Activity**:
A category of work within a Project (e.g. "Development", "Admin"). **Not used by Fylla** — granularity stops at Project; the per-worklog note carries "what I did" instead.

**Jibble Time Entry**:
Jibble's record of clocked time — Jibble's name for a Worklog from Fylla's side. Carries a Project, a start, a duration, and a free-text note.

## Flagged ambiguities

- **"Project"** is overloaded. Fylla task providers (Kendo/Todoist) have their own projects; Jibble has Projects; worklog targets key off a "project code". When Jibble is the worklog provider, the mapping from a task's origin to a **Jibble Project** is an open design question, not an identity.
- **Chores vs. trackable work**: a Todoist task like "clean the bathroom" is schedulable work but is NOT something to log to Jibble. Not every task that produces a calendar event should produce a Worklog. **Resolved:** there is no automatic rule — when Jibble is the worklog provider, every booking is a manual target pick in the worklog flow. Events you don't pick a Jibble Project for are simply not logged. A static task→target map is explicitly deferred (YAGNI).
- **Worklog target / "issueKey"**: the `PostWorklog(issueKey, …)` parameter normally carries a task key (e.g. Kendo `PROJ-123`). Jibble has no issue keys, so for Jibble the same slot carries a **Jibble target identifier** (a Jibble Project, optionally `Project/Activity`). One worklog provider per profile — hours never split across providers in a single run.

## Example dialogue

**Dev:** If I set `worklog.provider: jibble`, does scheduling a Todoist task automatically log it to Jibble?

**Youri:** No. Todoist is my *task provider* — it tells me what to do and when. Jibble is just the *worklog provider* — it only matters when I want the hours recorded. Cleaning the bathroom is a Todoist task I might schedule, but it never becomes a Jibble Time Entry.

**Dev:** So what decides which scheduled work turns into a Worklog?

**Youri:** Only work that belongs to a Jibble Project — my personal business, or a volunteer commission. Everything else I just do; I don't clock it.
