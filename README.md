# Fylla

Fylla is a Go CLI that helps you plan Jira work in your calendar.

It pulls tasks, scores and sorts them by configurable priority rules, finds
free time in Google Calendar, and schedules tasks into those slots.

> **Name origin:** *Fylla* (Swedish) means “to fill”.

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
