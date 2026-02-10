# Fylla

Fylla is a Go CLI that helps you plan Jira work in your calendar.

It pulls Jira tasks, scores and sorts them by configurable priority rules, finds free time in Google Calendar, and schedules tasks into those slots.

> **Name origin:** *Fylla* (Swedish) means “to fill”.

## Short Description

Fill your calendar with what matters: a Go CLI that prioritizes Jira tasks and schedules them into Google Calendar.

## What The Project Does

- Fetches Jira issues using JQL
- Scores tasks with configurable weights (priority, due date, estimate, type, age)
- Applies a crunch-mode boost for near-term due dates
- Reads Google Calendar busy events and out-of-office blocks
- Finds free slots by business hours (global + project-specific windows)
- Allocates tasks using first-fit scheduling with minimum-duration safeguards
- Marks at-risk tasks when scheduled past their due date
- Creates `[Fylla]` (and `[LATE] [Fylla]`) events in a dedicated calendar
- Supports timer state + Jira worklog and estimate APIs in core packages

## Current Status

The repository contains both implemented logic and CLI scaffolding.

- Implemented and tested core packages: `internal/config`, `internal/jira`, `internal/calendar`, `internal/scheduler`, `internal/timer`
- Partially implemented CLI:
  - `config show`, `config edit`, `config set` are implemented
  - Command shapes/flags exist for `auth`, `sync`, `list`, `start`, `stop`, `status`, `log`, `estimate`, `add`
  - Several command handlers are currently placeholders (`RunE` returns `nil`)

If you are contributing to CLI behavior, start in `internal/cli/commands`.

## Architecture

1. Remove existing Fylla-owned events from the Fylla calendar
2. Fetch Jira tasks from configured/default JQL
3. Score and sort tasks by composite score
4. Fetch source calendar events in the scheduling window
5. Build project-aware free slots (business hours, buffers, OOO exclusions)
6. Allocate tasks into slots (first-fit, split rules, min-duration checks)
7. Create fresh calendar events and surface at-risk warnings

## Tech Stack

- Go `1.24+`
- CLI: `github.com/spf13/cobra`
- Jira API: standard `net/http`
- Google Calendar API: `google.golang.org/api/calendar/v3`
- OAuth2: `golang.org/x/oauth2`
- Config parsing: `gopkg.in/yaml.v3`
- Interactive prompts (planned/partial command integration): `github.com/AlecAivazis/survey/v2`

## Installation

### Prerequisites

- Go `1.24` or newer
- A Jira Cloud instance + API token
- A Google Cloud OAuth client for Calendar API access

### Build locally

```bash
git clone https://github.com/iruoy/fylla.git
cd fylla
go build ./cmd/fylla
```

This creates the executable at `./fylla` (or your platform equivalent).

### Install with `go install`

```bash
go install github.com/iruoy/fylla/cmd/fylla@latest
```

## Configuration

Default config path:

- `~/.config/fylla/config.yaml`

Credentials/token files:

- `~/.config/fylla/credentials.json` (Jira + optional stored OAuth token)
- `~/.config/fylla/google_token.json` (Google OAuth access/refresh token)
- `~/.config/fylla/timer.json` (running timer state)

`config.yaml` example:

```yaml
jira:
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"

calendar:
  sourceCalendar: primary
  fyllaCalendar: fylla

scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  bufferMinutes: 15

businessHours:
  start: "09:00"
  end: "17:00"
  workDays: [1, 2, 3, 4, 5]

projectRules:
  ADMIN:
    start: "09:00"
    end: "10:00"
    workDays: [1, 2, 3, 4, 5]

weights:
  priority: 0.40
  dueDate: 0.30
  estimate: 0.15
  issueType: 0.10
  age: 0.05

typeScores:
  Bug: 100
  Task: 70
  Story: 50
```

## CLI Commands

```bash
fylla auth jira --url https://company.atlassian.net --email you@example.com --token TOKEN
fylla auth google

fylla sync --dry-run
fylla sync --jql "project = MYPROJ"
fylla sync --days 10
fylla sync --from 2025-01-20 --to 2025-01-24

fylla list

fylla start PROJ-123
fylla stop --description "Worked on auth refresh"
fylla status
fylla log PROJ-123 2h "Worked on feature"

fylla estimate PROJ-123 4h
fylla estimate PROJ-123 +2h
fylla estimate PROJ-123 -30m

fylla add --quick
fylla add --project PROJ

fylla config show
fylla config edit
fylla config set scheduling.windowDays 7
```

## Development

### Run tests

```bash
go test ./...
```

### Suggested focused checks

```bash
go test ./internal/config ./internal/scheduler ./internal/timer
go test ./internal/jira ./internal/calendar
go test ./internal/cli/commands
```

## Contributing

See `CONTRIBUTING.md` for contribution workflow, validation steps, and current priority areas.

## License

No license file is currently present in this repository. Add one before publishing broadly.
