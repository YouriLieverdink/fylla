# Fylla

Fylla is a Go CLI that pulls tasks from Jira and/or Todoist, scores and sorts them
by configurable priority rules, finds free time in Google Calendar, and
schedules tasks into those slots. Multiple task providers can be used
simultaneously — tasks from all sources are pooled and merged into a single
schedule.

> **Name origin:** *Fylla* (Swedish) means "to fill".

## Tech Stack

- Go `1.24+`
- CLI: `github.com/spf13/cobra`
- Jira / Todoist API: standard `net/http`
- Google Calendar API: `google.golang.org/api/calendar/v3`
- OAuth2: `golang.org/x/oauth2`
- Config parsing: `gopkg.in/yaml.v3`
- Interactive prompts: `github.com/AlecAivazis/survey/v2`

## Installation

### Prerequisites

- Go `1.24` or newer
- One or more task sources:
  - Jira Cloud instance + API token
  - Todoist account + API token
  - Both can be used simultaneously
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

## Authentication

Set up your task source and Google Calendar credentials:

```bash
# Jira
fylla auth jira --url https://company.atlassian.net --email you@example.com --token YOUR_API_TOKEN

# — or —

# Todoist
fylla auth todoist --token YOUR_API_TOKEN

# - and -

# Google Calendar (both sources need this)
fylla auth google --client-credentials path/to/client_credentials.json
```

Each `auth` command stores the token in a per-provider credential file and saves
the file path to config, so subsequent commands need no credential flags.

## Configuration

Default config path:

- `~/.config/fylla/config.yaml`

Per-provider credential files (created by `fylla auth`):

- `~/.config/fylla/jira_credentials.json`
- `~/.config/fylla/todoist_credentials.json`

Other data files:

- `~/.config/fylla/google_credentials.json` (Google OAuth client config + access/refresh token)
- `~/.config/fylla/timer.json` (running timer state)

### Multi-provider config example (Jira + Todoist)

```yaml
providers: [jira, todoist]

jira:
  credentials: ~/.config/fylla/jira_credentials.json  # set by fylla auth jira
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"

todoist:
  credentials: ~/.config/fylla/todoist_credentials.json  # set by fylla auth todoist
  defaultFilter: "today | overdue"

calendar:
  credentials: ~/.config/fylla/google_credentials.json  # set by fylla auth google
  sourceCalendars: [primary]
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
  priority: 0.45
  dueDate: 0.30
  estimate: 0.15
  age: 0.10
```

### Single-provider config example (Jira only)

```yaml
providers: [jira]

jira:
  credentials: ~/.config/fylla/jira_credentials.json
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"

calendar:
  credentials: path/to/client_credentials.json
  sourceCalendars: [primary]
  fyllaCalendar: fylla
```

## Shell Completion

Fylla supports shell completions for bash, zsh, fish, and powershell:

```bash
# Bash
fylla completion bash > /etc/bash_completion.d/fylla

# Zsh
fylla completion zsh > "${fpath[1]}/_fylla"

# Fish
fylla completion fish > ~/.config/fish/completions/fylla.fish

# PowerShell
fylla completion powershell > fylla.ps1
```

## Usage

```bash
# List tasks sorted by priority score
fylla task list                          # uses default query from config
fylla task list --jql "project = WEB"    # Jira: custom JQL
fylla task list --filter "today"         # Todoist: custom filter
# Multi-provider: both --jql and --filter are used for their respective providers

# Schedule tasks into Google Calendar
fylla schedule sync                      # schedule using defaults
fylla schedule sync --dry-run            # preview without creating events
fylla schedule sync --days 3             # override scheduling window
fylla schedule sync --from 2025-03-01 --to 2025-03-07
# Multi-provider: tasks from all providers are merged into one schedule

# View today's schedule
fylla schedule today                     # show all Fylla tasks for today
fylla schedule next                      # show current/next task

# Time tracking
fylla timer start TASK-KEY               # start timer
fylla timer status                       # check running timer
fylla timer stop -d "worked on feature"  # stop timer and log work
fylla timer log TASK-KEY 2h "description" # manual worklog
# Multi-provider: task key format routes to correct provider (PROJ-123 → Jira, 12345 → Todoist)

# Task management
fylla task add                           # create task interactively
fylla task add --provider todoist        # create on specific provider
fylla task done PROJ-123                 # complete task (routes to Jira)
fylla task done 8765432101               # complete task (routes to Todoist)
fylla task estimate TASK-KEY 4h          # set remaining estimate
fylla task estimate TASK-KEY +1h         # adjust estimate relatively

# Configuration
fylla config show                        # display current config
fylla config edit                        # open config in editor
fylla config set providers "[jira, todoist]"  # set providers
```
