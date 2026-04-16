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

- **Config:** `providers: [kendo, todoist, github]` — configures which task providers to use, defaults to `["kendo"]` when unset
- **Provider routing:** `isKendoKey()` / `isGitHubKey()` / `providerForKey()` infers provider from task key format (`PROJ-123` → Kendo, numeric → Todoist, `repo#123` → GitHub)
- **MultiTaskSource:** wraps multiple `TaskSource` instances, routes key-based operations to the correct provider. `routeToWithProvider()` uses the explicit provider name when available, falling back to key-based inference
- **multiFetcher:** concurrent fetch from all providers, merges results, handles partial failures
- **Progressive loading:** TUI fires per-provider fetch commands via `LoadTasksByProvider`, results trickle in as each provider responds (`TasksPartialMsg`), merged and sorted incrementally
- **Per-provider credentials:** each provider stores credentials in a separate file (`todoist_credentials.json`, `github_credentials.json`, `kendo_credentials.json`), path saved in config (`todoist.credentials`, `github.credentials`, `kendo.credentials`)
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
