# Fylla - Go CLI Task Scheduler

## Build & Test Commands

```bash
go build ./cmd/fylla        # Build
go vet ./...                # Lint
go test ./...               # Test all
go test ./internal/kendo/   # Test single package
go mod tidy                 # Resolve dependencies
go run ./cmd/fylla          # Run
```

## Project Structure

- Entry point: `cmd/fylla/main.go`
- Internal packages: `internal/{cli,calendar,config,github,kendo,prutil,scheduler,task,timer,todoist,web}`
- Config template: `config/default_config.yaml`
- Tests: colocated `_test.go` files (Go convention)

## Architecture

### Multi-Provider System
Fylla supports multiple task providers (Todoist, GitHub, Kendo) simultaneously via the `providers` array in config. Key concepts:

- **Config:** `providers: [kendo, todoist, github]` — configures which task providers to use. At least one is required (validated at load); the default template seeds `[local]` so fresh profiles work without credentials
- **Provider routing:** `isKendoKey()` / `isGitHubKey()` / `providerForKey()` infers provider from task key format (`PROJ-123` → Kendo, numeric → Todoist, `repo#123` or `owner/repo#123` → GitHub)
- **MultiTaskSource:** wraps multiple `TaskSource` instances, routes key-based operations to the correct provider. `routeToWithProvider()` uses the explicit provider name when available, falling back to key-based inference
- **multiFetcher:** concurrent fetch from all providers, merges results, handles partial failures
- **Progressive loading:** TUI fires per-provider fetch commands via `LoadTasksByProvider`, results trickle in as each provider responds (`TasksPartialMsg`), merged and sorted incrementally
- **Per-provider credentials:** each provider stores credentials in a separate file under the active profile dir (`profiles/<name>/<provider>_credentials.json`). Paths resolve by convention via `config.DefaultProviderCredentialsPath("<provider>")` — there are no credential path fields in `Config`.
- **Profiles:** fylla supports multiple isolated config profiles under `~/.config/fylla/profiles/<name>/`. The active profile is chosen at startup with precedence `--profile` flag > `FYLLA_PROFILE` env > `~/.config/fylla/current` pointer file > literal `default`. `config.RootDir()` returns the root; `config.ProfileDir()` returns the active profile dir. `config.MigrateLegacyLayout()` moves pre-profile state into `profiles/default/` on first run. Subcommands live in `internal/cli/commands/profile.go`
- **Calendar descriptions:** `BuildDescriptionWithProvider()` constructs event descriptions with provider-aware markers (`fylla:kendo` for Kendo, etc.) and correct URLs. `TaskKeyAndProviderFromDescription()` extracts both the task key and provider from event descriptions
- **Done marker:** `DoneMarker` (`✓ `) prefix on calendar event titles indicates completed work. `ParseTitle` strips it and sets `Done bool`. Used by `timer stop` to mark events as done.
- **Past event preservation:** `reconcile()` takes a `now` parameter and skips events whose end time is before `now`, preserving them as a calendar record
- **Worklog command:** `fylla worklog` walks calendar events (tasks + meetings), prompts for adjustments, fills remaining hours, and bulk-posts worklogs. Kendo tasks are posted directly to Kendo as time entries
- **Bulk operations:** `RunBulk()` supports bulk done/delete on multiple selected tasks. TUI multi-select mode (ctrl+v) with space to toggle, then d/D to apply

### Key Interfaces
- `TaskSource` — composite interface combining all task operations (fetch, create, complete, delete, estimate, priority, due date, summary, worklog)
- `TaskFetcher` — single-method interface for fetching tasks
- `CalendarClient` — abstracts Google Calendar operations
- `Surveyor` — abstracts interactive prompts (supports `Select`, `MultiSelect`, `Input`, `InputWithDefault`, `Password`)
- `IssueKeyResolver` — resolves non-native task keys (e.g. GitHub PR) to worklog-compatible issue keys

### Scheduling
- `defaultEstimateMinutes` in config controls the fallback estimate when a task has no remaining estimate (default: 60 minutes)
- Configurable via `scheduling.defaultEstimateMinutes` in config.yaml
- `providerTimeoutSeconds` bounds each provider's fetch call (default: 15s). On timeout the `multiFetcher` serves the stale cache entry if present and attaches a warning instead of blocking.
- `taskCacheTTLSeconds` controls the shared `TaskCache` TTL (default: 30s). Cache is populated by both `multiFetcher` and `LoadTasksByProvider`, so the schedule tab reuses tasks fetched for the tasks tab without a second round-trip. Mutations on `MultiTaskSource` (create/complete/delete/update*) invalidate the affected provider's entry.
- `TaskCache.FetchOrShare` provides singleflight semantics: concurrent `FetchTasks` calls for the same provider share one in-flight call instead of issuing duplicates. Used by both `multiFetcher` and `cachedFetcher`, so switching to the schedule tab while the tasks tab is still loading reuses the in-flight fetch rather than starting a second one.
- `previewTimeoutSeconds` bounds the whole `SyncPreview` (default: 20s) so the schedule tab cannot hang indefinitely even if a provider ignores its per-call deadline.
- `multiFetcher` returns `ErrPartialProviders` when some providers fail; `RunSync` proceeds with partial tasks and surfaces warnings via `SyncResult.Warnings`. `SyncApply` does NOT degrade — a missing provider would cause reconcile to delete its events.
- TUI caches the last `SyncResult` in `m.cachedSync`: switching to the schedule tab renders the cached result instantly and fires a background refresh (mirrors `m.cachedTasks`).

### GitHub Provider
- `github.defaultQueries` is a list of queries, each passed verbatim to the Search Issues API; results are merged and deduped by issue ID. Multiple queries are needed because GitHub search forbids `OR` between qualifiers — splitting `(assignee:@me OR review-requested:@me)` into two queries is the documented workaround. Any qualifiers work (`is:pr`, `is:issue`, `author:@me`, `-user:work-org`, `repo:x/y`, etc.). `FetchTasks` returns both issues and PRs; `IssueType` is `"Issue"` or `"Pull Request"` based on the presence of the `pull_request` field. Internally `buildProviderQueries` joins the list with `\n` to keep the single-string `FetchTasks(ctx, query)` interface; the client splits on `\n`.
- `github.repos` is optional. When populated it auto-appends `repo:owner/repo` qualifiers to the query and provides short-name resolution (`fylla#42` → `iruoy/fylla`). Leave empty to control scope purely via query — task keys then use the short repo name discovered from results. Keys in `owner/repo#N` form are also accepted everywhere.
- **Title metadata:** priority, estimate, and due date are encoded into the issue title using the same inline clauses as other providers — `[30m]`/`[1h30m]` (estimate via `task.ParseTitleEstimate`/`SetTitleEstimate`), `{YYYY-MM-DD}` (due via `task.ParseTitleDueDate`/`SetTitleDueDate`/`RemoveTitleDueDate`), and `(priority:pN)` (standalone, via `task.ParseTitlePriority`/`SetTitlePriority`/`RemoveTitlePriority`). `FetchTasks` strips all three clauses from the title for `Task.Summary` and populates `Priority`/`RemainingEstimate`/`DueDate` accordingly. `UpdateEstimate`/`UpdateDueDate`/`UpdatePriority` each GET the current title, apply the matching setter, and PATCH — other clauses are preserved.
- Write ops: `CreateTask` appends estimate/due/priority clauses to the title (accepts `Project` as short name or `owner/repo`); `CompleteTask` closes with `state_reason=completed`; `DeleteTask` closes with `state_reason=not_planned`; `UpdateSummary` rewrites the non-clause portion while re-applying any existing estimate/due/priority. `PostWorklog` returns `ErrUnsupported`.

### GitHub Rate Limiting
- Client tracks `X-RateLimit-Remaining` and `X-RateLimit-Reset` from API responses
- Auto-pauses requests when < 50 remaining until reset window passes

## Dependencies

- CLI: `github.com/spf13/cobra`
- Google Calendar: `google.golang.org/api/calendar/v3` + `golang.org/x/oauth2`
- TUI: `github.com/charmbracelet/bubbletea` + `bubbles` + `lipgloss`
- YAML: `gopkg.in/yaml.v3`
- Interactive prompts: `github.com/AlecAivazis/survey/v2`
- GitHub API: `github.com/google/go-github/v68`
- Natural date parsing: `github.com/tj/go-naturaldate`
- HTTP, JSON, filepath, time, os: stdlib

## Code Conventions

- Follow standard Go conventions: `gofmt`, `go vet`, effective Go
- Exported types/functions get doc comments; unexported don't need them
- Errors: return `error` as last value, wrap with `fmt.Errorf("context: %w", err)`
- Naming: `MixedCaps`/`mixedCaps`, no underscores; acronyms stay caps (`URL`, `HTTP`)
- Interfaces: single-method interfaces named with `-er` suffix (`Reader`, `Sorter`)
- Package names: short, lowercase, singular (`config` not `configs`)
- Cobra commands: one file per command in `internal/cli/commands/`
- Config structs: use `yaml:"fieldName"` tags matching `config/default_config.yaml` keys
- Tests: table-driven with `t.Run` subtests, use `testify` only if already a dependency
- Context: pass `context.Context` as first param where needed (HTTP calls, calendar API)
- No `init()` functions; no global mutable state
- Kendo client uses `sync.Map` for per-project caches (lanes, epics) and `sync.Once` for global state (projects, user)
