# Fylla - Go CLI Task Scheduler

## Build & Test Commands

```bash
go build ./cmd/fylla        # Build
go vet ./...                # Lint
go test ./...               # Test all
go test ./internal/jira/    # Test single package
go mod tidy                 # Resolve dependencies
go run ./cmd/fylla          # Run
```

## Project Structure

- Entry point: `cmd/fylla/main.go`
- Internal packages: `internal/{cli,jira,calendar,scheduler,config,timer,todoist}`
- Config template: `config/default_config.yaml`
- Tests: colocated `_test.go` files (Go convention)

## Architecture

### Multi-Provider System
Fylla supports multiple task providers (Jira, Todoist) simultaneously via the `providers` array in config. Key concepts:

- **Config:** `providers: [jira, todoist]` — replaces legacy `source` field (backward compatible via `ActiveProviders()` fallback)
- **Provider routing:** `isJiraKey()` / `providerForKey()` infers provider from task key format (`PROJ-123` → Jira, numeric → Todoist)
- **MultiTaskSource:** wraps multiple `TaskSource` instances, routes key-based operations to the correct provider
- **multiFetcher:** concurrent fetch from all providers, merges results, handles partial failures
- **Per-provider credentials:** each provider stores credentials in a separate file (`jira_credentials.json`, `todoist_credentials.json`), path saved in config (`jira.credentials`, `todoist.credentials`)
- **Calendar descriptions:** `BuildDescription()` infers source from task key to generate correct URLs (Jira browse link vs Todoist app link)

### Key Interfaces
- `TaskSource` — composite interface combining all task operations (fetch, create, complete, delete, estimate, priority, due date, worklog)
- `TaskFetcher` — single-method interface for fetching tasks
- `CalendarClient` — abstracts Google Calendar operations
- `Surveyor` — abstracts interactive prompts (supports `Select`, `MultiSelect`, `Input`, `Password`)

## Dependencies

- CLI: `github.com/spf13/cobra`
- Google Calendar: `google.golang.org/api/calendar/v3` + `golang.org/x/oauth2`
- YAML: `gopkg.in/yaml.v3`
- Interactive prompts: `github.com/AlecAivazis/survey/v2`
- HTTP, JSON, filepath, time, os: stdlib

## Code Conventions

- Follow standard Go conventions: `gofmt`, `go vet`, effective Go
- Exported types/functions get doc comments; unexported don't need them
- Errors: return `error` as last value, wrap with `fmt.Errorf("context: %w", err)`
- Naming: `MixedCaps`/`mixedCaps`, no underscores; acronyms stay caps (`URL`, `HTTP`, `JQL`)
- Interfaces: single-method interfaces named with `-er` suffix (`Reader`, `Sorter`)
- Package names: short, lowercase, singular (`config` not `configs`)
- Cobra commands: one file per command in `internal/cli/commands/`, register in `root.go`
- Config structs: use `yaml:"fieldName"` tags matching `config/default_config.yaml` keys
- Tests: table-driven with `t.Run` subtests, use `testify` only if already a dependency
- Context: pass `context.Context` as first param where needed (HTTP calls, calendar API)
- No `init()` functions; no global mutable state
