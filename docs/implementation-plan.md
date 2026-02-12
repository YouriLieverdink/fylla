# Fylla Implementation Plan

Nine improvements organized by effort. Each section includes rationale, affected
files, implementation steps, and testing strategy.

---

## 1. Report Unscheduled Tasks After Sync

**Effort:** Small
**Why:** When tasks don't fit into available calendar slots the user gets no
feedback. They see "Scheduled 3 event(s)" but don't know 5 others were dropped.

### Design

Add an `Unscheduled` field to `SyncResult`. After allocation, compare the input
task list against allocated task keys to find the difference.

### Files

| File | Change |
|---|---|
| `internal/cli/commands/sync.go` | Add `Unscheduled []task.Task` to `SyncResult`. After `Allocate()`, compute the set of scheduled keys and collect unscheduled tasks. Update `PrintSyncResult` to print them. |

### Steps

1. Add `Unscheduled []task.Task` field to `SyncResult`.
2. In `RunSync`, after step 6 (allocate), build a `map[string]bool` of allocated
   task keys. Iterate `sorted` and collect any task whose key is absent.
3. In `PrintSyncResult`, if `len(result.Unscheduled) > 0`, print a
   "Could not schedule:" section with task key, summary, and estimate.
4. Include unscheduled count in the dry-run output as well.

### Tests

- Add a `sync_test.go` subtest where the slot capacity is smaller than the total
  task estimates. Assert that `SyncResult.Unscheduled` contains the expected
  tasks.
- Add a subtest where all tasks fit. Assert `Unscheduled` is empty.

---

## 2. Replace `interface{}` in `loadTaskSource` With a Combined Interface

**Effort:** Small
**Why:** `jira_helper.go` returns bare `interface{}`, requiring unsafe type
assertions (`source.(TaskFetcher)`) at every call site. A single combined
interface makes the contract explicit and lets the compiler catch mistakes.

### Design

Define a `TaskSource` interface in `jira_helper.go` that composes all the
per-command interfaces. Change `loadTaskSource` to return `TaskSource` instead
of `interface{}`. Remove type assertions from every command.

### Files

| File | Change |
|---|---|
| `internal/cli/commands/jira_helper.go` | Define `TaskSource` interface, change return type. |
| `internal/cli/commands/sync.go` | Remove `source.(TaskFetcher)` cast. |
| `internal/cli/commands/stop.go` | Remove `source.(WorklogPoster)` cast. |
| `internal/cli/commands/add.go` | Remove `source.(TaskCreator)` cast (and `ProjectLister` if applicable). |
| `internal/cli/commands/estimate.go` | Remove `source.(EstimateGetter)` / `source.(EstimateUpdater)` casts. |
| `internal/cli/commands/log.go` | Remove `source.(WorklogPoster)` cast. |

### Steps

1. In `jira_helper.go`, define:
   ```go
   type TaskSource interface {
       TaskFetcher
       TaskCreator
       WorklogPoster
       EstimateGetter
       EstimateUpdater
   }
   ```
   Note: `ProjectLister` may also be needed. Check whether `add.go` calls
   `ListProjects` — if so, either embed `ProjectLister` or handle it separately
   since `jira.Client` may not implement it.
2. Change `loadTaskSource` signature to `func loadTaskSource() (TaskSource, *config.Config, error)`.
3. Verify that both `jira.Client` and `todoist.Client` satisfy `TaskSource` by
   adding compile-time checks:
   ```go
   var _ TaskSource = (*jira.Client)(nil)
   var _ TaskSource = (*todoist.Client)(nil)
   ```
   If `jira.Client` is missing `ListProjects`, either add a stub or keep
   `ProjectLister` separate.
4. Update each command to use the typed `TaskSource` directly instead of
   casting.
5. Run `go vet ./...` to confirm no interface satisfaction issues.

### Tests

- Existing tests should continue to pass since the behavior is unchanged.
- The compile-time `var _` checks are the primary safety net.

---

## 3. Add Shell Completions

**Effort:** Small
**Why:** Cobra has built-in completion generation. One registration call enables
bash/zsh/fish/powershell completions, which significantly improves usability.

### Files

| File | Change |
|---|---|
| `internal/cli/root.go` | Enable completion command on the root command. |

### Steps

1. Cobra automatically adds a `completion` subcommand when using
   `cobra.Command`. Verify this is already present (Cobra ≥1.1 adds it by
   default). If not, explicitly add it:
   ```go
   rootCmd.AddCommand(completionCmd())
   ```
2. Optionally add custom completions for arguments that accept known values:
   - `start TASK-KEY`: Could register `ValidArgsFunction` that calls `list`
     logic to suggest task keys (stretch goal, skip for now).
   - `config set KEY VALUE`: Could complete config key paths from the YAML
     structure.
3. Update README with installation instructions:
   ```
   # Bash
   fylla completion bash > /etc/bash_completion.d/fylla
   # Zsh
   fylla completion zsh > "${fpath[1]}/_fylla"
   # Fish
   fylla completion fish > ~/.config/fish/completions/fylla.fish
   ```

### Tests

- Run `fylla completion bash` and verify it outputs a valid bash script.
- Manual smoke test in a shell.

---

## 4. Update README for Todoist Support

**Effort:** Small
**Why:** The README only mentions Jira. Todoist is fully supported but
undiscoverable.

### Files

| File | Change |
|---|---|
| `README.md` | Update description, prerequisites, config examples, and command docs. |

### Steps

1. Update the opening description to mention both Jira and Todoist.
2. In prerequisites, add Todoist API token as an alternative to Jira.
3. Add a Todoist config example alongside the existing Jira one.
4. Document `source: todoist` in the config section.
5. Show `fylla auth todoist --token TOKEN` in the authentication section.
6. Mention `--filter` flag alongside `--jql` in the sync/list docs.

### Tests

- Review only — no automated tests for README content.

---

## 5. `done` Command — Mark Tasks as Complete

**Effort:** Medium
**Why:** The current workflow is `list → sync → start → stop` but there's no
way to mark a task as done from the CLI. Users have to open Jira/Todoist
separately to close the issue.

### Design

A new `done` command that transitions a task to its completed state:
- **Jira:** POST to the transitions API to move the issue to "Done".
- **Todoist:** POST to close the task.

### Files

| File | Change |
|---|---|
| `internal/cli/commands/done.go` | New file. `DoneParams`, `RunDone`, `PrintDoneResult`, `newDoneCmd()`. |
| `internal/cli/commands/register.go` | Register `newDoneCmd()`. |
| `internal/jira/client.go` | Add `TransitionToDone(ctx, issueKey)` method. |
| `internal/todoist/client.go` | Add `CloseTask(ctx, taskID)` method. |
| `internal/cli/commands/jira_helper.go` | Add `TaskCompleter` to `TaskSource` (or keep separate). |

### Interface

```go
// TaskCompleter marks a task as done/closed.
type TaskCompleter interface {
    CompleteTask(ctx context.Context, taskKey string) error
}
```

### Jira Implementation (`jira/client.go`)

Jira requires a two-step process:
1. `GET /rest/api/3/issue/{key}/transitions` — fetch available transitions.
2. Find the transition whose `name` matches "Done" (case-insensitive).
3. `POST /rest/api/3/issue/{key}/transitions` with `{"transition":{"id":"<id>"}}`.

If no "Done" transition is found (different workflow), return an error listing
available transition names so the user knows what's possible.

### Todoist Implementation (`todoist/client.go`)

Simple: `POST /tasks/{id}/close` with empty body. Returns 204 on success.

### Command Design

```
fylla done TASK-KEY
```

Flags: none needed for v1. Could later add `--transition NAME` for Jira to
allow custom transition names.

### Steps

1. Add `CompleteTask` method to both `jira.Client` and `todoist.Client`.
2. Define `TaskCompleter` interface in `done.go`.
3. Add `TaskCompleter` to the `TaskSource` combined interface (from feature #2).
4. Implement `RunDone(ctx, DoneParams) (*DoneResult, error)`.
5. Implement `PrintDoneResult(w, result)`.
6. Wire up `newDoneCmd()` cobra command.
7. Register in `register.go`.

### Tests

- `done_test.go`: Mock `TaskCompleter`. Test success path, test error when
  transition not found (Jira), test Todoist close.
- `jira/client_test.go`: Add test for `TransitionToDone` with httptest server
  returning transition list → verify correct transition ID is POSTed.
- `todoist/client_test.go`: Add test for `CloseTask` verifying POST to
  `/tasks/{id}/close`.

---

## 6. Config Validation

**Effort:** Medium
**Why:** Invalid config silently produces wrong behavior. Weights that don't sum
to 1.0 produce misleading scores. Invalid business hours cause runtime panics or
empty slot results with no explanation.

### Design

Add a `Validate() error` method on `Config` that checks all invariants. Call it
at the end of `Load()` and `LoadFrom()`.

### Files

| File | Change |
|---|---|
| `internal/config/config.go` | Add `Validate() error` method. |
| `internal/config/store.go` | Call `cfg.Validate()` at the end of `Load()` and `LoadFrom()`. |
| `internal/config/config_test.go` | Add validation test cases. |

### Validations

| Rule | Error message |
|---|---|
| `source` must be `"jira"` or `"todoist"` or empty (defaults to jira) | `"source must be 'jira' or 'todoist', got %q"` |
| `weights` sum must be in range `[0.99, 1.01]` (float tolerance) | `"weights must sum to 1.0, got %.2f"` |
| `businessHours.start` must parse as `HH:MM` | `"businessHours.start: invalid time format %q"` |
| `businessHours.end` must parse as `HH:MM` | `"businessHours.end: invalid time format %q"` |
| `businessHours.start` must be before `end` | `"businessHours.start must be before end"` |
| `businessHours.workDays` values must be 1-7 | `"businessHours.workDays: invalid day %d (must be 1-7)"` |
| `scheduling.windowDays` must be > 0 | `"scheduling.windowDays must be positive"` |
| `scheduling.minTaskDurationMinutes` must be > 0 | `"scheduling.minTaskDurationMinutes must be positive"` |
| Each `projectRule` start/end must be valid `HH:MM` and start < end | `"projectRules.%s: ..."` |
| Each `projectRule.workDays` values must be 1-7 | same pattern |

### Steps

1. Add a helper `parseHHMM(s string) (int, int, error)` in `config.go`.
2. Implement `Validate()` checking each rule. Collect all errors into a slice
   and return them joined (or return on first error — simpler).
3. Call `Validate()` at the end of `LoadFrom()` (after unmarshal).
4. `Load()` already calls `LoadFrom()`, so it inherits validation.
5. `SetIn()` should also validate after writing — call `LoadFrom()` on the
   result path (it already parses the written YAML, just add the validate call
   to `LoadFrom`).

### Tests

Table-driven tests in `config_test.go`:
- Valid config → no error.
- Weights sum to 0.5 → error.
- Invalid source → error.
- Business hours "25:00" → error.
- Start after end → error.
- WorkDays contains 0 → error.
- Project rule with invalid times → error.

---

## 7. `next` Command — What Should I Work On Now?

**Effort:** Medium
**Why:** After syncing, users have to open Google Calendar to see what they
should be working on. A `next` command answers "what now?" directly in the
terminal.

### Design

Read Fylla calendar events for today, find the event covering `now` or the next
upcoming one, and display it.

### Files

| File | Change |
|---|---|
| `internal/cli/commands/next.go` | New file. `NextParams`, `RunNext`, `PrintNextResult`, `newNextCmd()`. |
| `internal/cli/commands/register.go` | Register `newNextCmd()`. |

### Interface

Reuses the existing `CalendarClient.FetchEvents`. No new interface needed.

### Logic

1. Fetch events from the Fylla calendar for today (start of day → end of day).
2. Filter events with `[Fylla]` prefix.
3. Find the event where `now` falls between `Start` and `End` → "Current task".
4. If no current task, find the next event with `Start > now` → "Next up".
5. If no upcoming events today, print "No more Fylla tasks today."

### Output Format

```
Current: PROJ-123: Fix login bug (until 11:30)
Next:    PROJ-456: Update docs (12:00 – 13:00)
```

Or:
```
Next up: PROJ-456: Update docs (starts in 25m)
```

### Steps

1. Create `next.go` with `NextParams` containing `Cal CalendarClient`, `Now`,
   `FyllaCalendar string`.
2. `RunNext` fetches today's events, filters for Fylla prefix, finds
   current/next.
3. Parse task key from event title (strip `[Fylla] ` or `[LATE] [Fylla] `
   prefix, split on `:`).
4. `PrintNextResult` formats the output.
5. Wire up `newNextCmd()` — needs Google OAuth (same pattern as `sync`).
6. Register in `register.go`.

### Tests

- Mock `CalendarClient` returning events at known times.
- Test case: current time inside an event → shows "Current".
- Test case: current time between events → shows "Next up" with duration.
- Test case: no events today → shows "No more tasks" message.
- Test case: events with `[LATE]` prefix handled correctly.

---

## 8. Progress Output During Sync

**Effort:** Medium
**Why:** Sync makes multiple API calls (delete events, fetch tasks, fetch
calendar, create events). On slow connections this can take 10+ seconds with
zero feedback.

### Design

Add a `ProgressWriter` (just `io.Writer`) to `SyncParams`. Print step
indicators as sync progresses. Keep it simple — no spinners or progress bars,
just line-by-line status messages.

### Files

| File | Change |
|---|---|
| `internal/cli/commands/sync.go` | Add `Progress io.Writer` to `SyncParams`. Add `fmt.Fprintf` calls at each step. |

### Output

```
Clearing previous schedule...
Fetching tasks...
Sorting 12 tasks...
Reading calendar...
Finding free slots...
Scheduling 12 tasks into 8 slots...
Creating 8 calendar events...
Done.
```

In dry-run mode:
```
Fetching tasks...
Sorting 12 tasks...
Reading calendar...
Finding free slots...
Scheduling 12 tasks into 8 slots...
Done (dry run).
```

### Steps

1. Add `Progress io.Writer` field to `SyncParams`.
2. Define a helper `progress(w io.Writer, format string, args ...interface{})`
   that no-ops when `w` is nil (so tests can opt out).
3. Add progress calls before each step in `RunSync`.
4. In `newSyncCmd()`, pass `cmd.ErrOrStderr()` as the progress writer (progress
   goes to stderr, results to stdout — allows piping).

### Tests

- Existing `sync_test.go` tests should keep passing (pass `nil` for Progress
  to maintain silence in tests).
- Add one test that passes a `bytes.Buffer` as Progress and asserts the
  expected step messages appear.

---

## 9. Incremental Sync

**Effort:** Large
**Why:** The current approach deletes all `[Fylla]` events and recreates them
on every sync. This is wasteful (unnecessary API calls), discards any manual
adjustments the user made to event times, and creates visual churn in the
calendar.

### Design

Compare the desired schedule against existing Fylla events. Only create, update,
or delete events where the schedule has changed.

### Files

| File | Change |
|---|---|
| `internal/calendar/google.go` | Add `FetchFyllaEvents(ctx, start, end)` that returns only Fylla-prefixed events. Add `UpdateEvent(ctx, eventID, input)`. |
| `internal/cli/commands/sync.go` | Add reconciliation logic between desired allocations and existing events. |

### Algorithm

1. Fetch existing Fylla events → build a map keyed by task key.
2. Compute desired allocations (same as today).
3. Reconcile:
   - **New tasks** (key not in existing): create event.
   - **Removed tasks** (key in existing but not in desired): delete event.
   - **Changed tasks** (key in both but times differ): update event.
   - **Unchanged tasks** (key in both, same times): skip.
4. Report what changed: "Created 2, updated 1, removed 3, unchanged 6."

### Complications

- Split tasks: a single task can produce multiple events. Need to handle
  one-to-many matching (match by key + time ordering).
- Manual adjustments: if a user moved a Fylla event by 15 minutes, an update
  would overwrite it. Consider a `--force` flag for full recreation vs.
  default incremental behavior.
- Event IDs: store the Google Calendar event ID somewhere (perhaps in event
  description or extended properties) to enable precise updates.

### Steps

1. Add `FetchFyllaEvents` to `GoogleClient` — filter `FetchEvents` results by
   Fylla prefix and parse task keys from titles.
2. Add `UpdateEvent(ctx, eventID string, input CreateEventInput)` to
   `GoogleClient`.
3. In `RunSync`, add a `--force` flag that uses the current delete-all behavior.
4. Default mode: fetch existing, compute desired, reconcile, apply diff.
5. Update `SyncResult` with counts: `Created`, `Updated`, `Deleted`,
   `Unchanged`.
6. Update `PrintSyncResult` to show the diff summary.

### Tests

- Mock calendar returning existing events. Provide desired allocations that
  partially overlap. Assert correct create/update/delete calls.
- Test full recreation with `--force`.
- Test split task reconciliation (task with 2 events, one changed).
- Test no-op when schedule hasn't changed.

---

## Implementation Order

Recommended sequence based on dependencies and incremental value:

```
Phase 1 — Foundation (no dependencies between items)
  #2  Combined TaskSource interface     (simplifies all subsequent command work)
  #4  Update README                     (independent, quick)
  #6  Config validation                 (independent, prevents bugs)

Phase 2 — New commands (depend on #2 for clean interface)
  #5  done command                      (needs TaskSource from #2)
  #7  next command                      (independent of #5)
  #1  Report unscheduled tasks          (small sync.go change)
  #3  Shell completions                 (independent)

Phase 3 — UX polish
  #8  Progress output during sync       (small change, big UX win)

Phase 4 — Advanced
  #9  Incremental sync                  (largest change, builds on all prior work)
```

Within each phase, items can be worked on in parallel.
