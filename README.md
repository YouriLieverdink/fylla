# Fylla

Fylla is a Go CLI that pulls tasks from Jira or Todoist, scores and sorts them
by configurable priority rules, finds free time in Google Calendar, and
schedules tasks into those slots.

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
- A task source — **one** of:
  - Jira Cloud instance + API token
  - Todoist account + API token
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

# Google Calendar (both sources need this)
fylla auth google --client-credentials path/to/client_credentials.json
```

## Configuration

Default config path:

- `~/.config/fylla/config.yaml`

Credentials/token files:

- `~/.config/fylla/credentials.json` (Jira/Todoist tokens)
- `~/.config/fylla/google_token.json` (Google OAuth access/refresh token)
- `~/.config/fylla/timer.json` (running timer state)

### Jira config example

```yaml
source: jira

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

### Todoist config example

```yaml
source: todoist

todoist:
  defaultFilter: "today | overdue"

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
fylla list                          # uses default query from config
fylla list --jql "project = WEB"    # Jira: custom JQL
fylla list --filter "today"         # Todoist: custom filter

# Schedule tasks into Google Calendar
fylla sync                          # schedule using defaults
fylla sync --dry-run                # preview without creating events
fylla sync --days 3                 # override scheduling window
fylla sync --from 2025-03-01 --to 2025-03-07

# Time tracking
fylla start TASK-KEY                # start timer
fylla status                        # check running timer
fylla stop -d "worked on feature"   # stop timer and log work
fylla log TASK-KEY 2h "description" # manual worklog

# Task management
fylla add                           # create task interactively
fylla add --quick --project WEB     # quick mode with defaults
fylla estimate TASK-KEY 4h          # set remaining estimate
fylla estimate TASK-KEY +1h         # adjust estimate relatively

# Configuration
fylla config show                   # display current config
fylla config edit                   # open config in editor
fylla config set source todoist     # set a config value
```
