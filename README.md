# Fylla

Fylla is a Go CLI that pulls tasks from Jira, Todoist, and/or GitHub,
scores and sorts them by configurable priority rules, finds free time in Google
Calendar, and schedules tasks into those slots. Multiple task providers can be
used simultaneously — tasks from all sources are pooled and merged into a single
schedule.

> **Name origin:** *Fylla* (Swedish) means "to fill".

## Tech Stack

- Go `1.24+`
- CLI: `github.com/spf13/cobra`
- Jira / Todoist API: standard `net/http`
- GitHub API: `github.com/google/go-github/v68`
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
  - GitHub account + personal access token
  - Any combination can be used simultaneously
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

Set up your task source(s) and Google Calendar credentials:

```bash
# Jira
fylla auth jira --url https://company.atlassian.net --email you@example.com --token YOUR_API_TOKEN

# Todoist
fylla auth todoist --token YOUR_API_TOKEN

# GitHub
fylla auth github --token YOUR_GITHUB_PAT

# Google Calendar (required for scheduling)
fylla auth google --client-credentials path/to/client_credentials.json
```

Each `auth` command stores the token in a per-provider credential file and saves
the file path to config, so subsequent commands need no credential flags.

### Required permissions

#### Jira

Create an [API token](https://id.atlassian.com/manage-profile/security/api-tokens).
The token inherits your Jira user permissions — no additional scopes to configure.
You need at least read access to the projects you want to schedule.

#### Todoist

Create an [API token](https://todoist.com/help/articles/find-your-api-token-Jpzx9IIlB)
in Settings > Integrations > Developer. The token grants full access to your account.

#### GitHub

Create a [personal access token](https://github.com/settings/tokens). Either token type works:

| Token type | Required permissions |
|---|---|
| **Fine-grained PAT** (recommended) | **Pull requests: Read** on the repositories you want to review. Public repos are readable by default; only private repos need explicit permission. |
| **Classic PAT** | `repo` scope (grants access to private repo PRs). For public repos only, no scope is needed. |

Fylla uses the Search API (`review-requested:@me`) and fetches PR detail for diff stats.

#### Google Calendar

Create OAuth 2.0 credentials in the [Google Cloud Console](https://console.cloud.google.com/apis/credentials).
Enable the **Google Calendar API** for your project. The OAuth flow requests
these scopes automatically:

- `https://www.googleapis.com/auth/calendar` — read/write access to calendars

## Configuration

Default config path:

- `~/.config/fylla/config.yaml`

Per-provider credential files (created by `fylla auth`):

- `~/.config/fylla/jira_credentials.json`
- `~/.config/fylla/todoist_credentials.json`
- `~/.config/fylla/github_credentials.json`

Other data files:

- `~/.config/fylla/google_credentials.json` (Google OAuth client config + access/refresh token)
- `~/.config/fylla/timer.json` (running timer state)

### Multi-provider config example

```yaml
providers: [jira, todoist, github]

jira:
  credentials: ~/.config/fylla/jira_credentials.json  # set by fylla auth jira
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"
  defaultProject: WEB
  doneTransitions: {}

todoist:
  credentials: ~/.config/fylla/todoist_credentials.json  # set by fylla auth todoist
  defaultFilter: "today | overdue"
  defaultProject: Inbox

github:
  credentials: ~/.config/fylla/github_credentials.json  # set by fylla auth github
  defaultQuery: "is:pr state:open review-requested:@me"  # customize search query
  repos: []                                              # optional: limit to specific repos

calendar:
  credentials: ~/.config/fylla/google_credentials.json  # set by fylla auth google
  sourceCalendars: [primary]
  fyllaCalendar: fylla

scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  bufferMinutes: 15
  travelBufferMinutes: 30
  snapMinutes: [0, 15, 30, 45]
  autoResync: false               # re-sync calendar after task changes

businessHours:
  - start: "09:00"
    end: "17:00"
    workDays: [1, 2, 3, 4, 5]

projectRules:
  ADMIN:
    - start: "09:00"
      end: "10:00"
      workDays: [1, 2, 3, 4, 5]

worklog:
  provider: ""                    # set to "jira" to route all worklogs to Jira
  fallbackIssues: []              # Jira issues for non-task time (meetings, admin)

weights:
  priority: 0.45
  dueDate: 0.30
  estimate: 0.15
  age: 0.10
  upNext: 50
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
# First-time setup
fylla init                              # interactive setup wizard

# List tasks sorted by priority score
fylla task list                          # uses default query from config
fylla task list --jql "project = WEB"    # Jira: custom JQL
fylla task list --filter "today"         # Todoist: custom filter
# Multi-provider: --jql and --filter are used for their respective providers
# GitHub PRs are fetched using config defaults (no CLI flag needed)

# Schedule tasks into Google Calendar
fylla sync                               # schedule using defaults
fylla sync --dry-run                     # preview without creating events
fylla sync --days 3                      # override scheduling window
fylla sync --from 2025-03-01 --to 2025-03-07
# Multi-provider: tasks from all providers are merged into one schedule

# View today's schedule
fylla today                              # show all Fylla tasks for today
fylla next                               # show current/next task

# Remove all Fylla events from calendar
fylla clear                              # delete all Fylla-managed events
fylla clear --dry-run                    # preview what would be removed
fylla clear --from 2025-01-01 --to 2025-06-30

# Time tracking
fylla timer start TASK-KEY               # start timer
fylla timer status                       # check running timer
fylla timer stop -d "worked on feature"  # stop + log + update calendar + show remaining
fylla timer log TASK-KEY 2h "description" # manual worklog
# Multi-provider: task key format routes to correct provider
# PROJ-123 → Jira, 12345 → Todoist, owner/repo#42 → GitHub

# Bulk worklog posting
fylla worklog                            # review & post worklogs from today's calendar
fylla worklog --date 2025-02-18          # post worklogs for a past date

# Task management
fylla task add                           # create task interactively
fylla task add --provider todoist        # create on specific provider
fylla task add 'Write docs [2h] (due Friday priority:p2 upnext)'  # inline syntax
fylla task done PROJ-123                 # complete task (routes to Jira)
fylla task done 8765432101               # complete task (routes to Todoist)
fylla task delete TASK-KEY               # permanently delete a task
fylla task edit TASK-KEY --estimate 4h   # set remaining estimate
fylla task edit TASK-KEY --due Friday --priority p1  # set due date and priority
fylla task edit TASK-KEY --up-next       # mark as up next

# Web dashboard
fylla serve                              # start dashboard on http://localhost:8002
fylla serve --port 3000                  # custom port

# Configuration
fylla config show                        # display current config
fylla config edit                        # open config in editor
fylla config set providers "[jira, todoist, github]"  # set providers
```

## Inline Task Syntax

When creating tasks with `fylla task add`, you can specify properties inline:

```bash
fylla task add 'Write the docs [30m] (due Friday priority:p1 not before Monday upnext nosplit)'
```

**Estimate** — in `[brackets]`:

- `[2h]`, `[30m]`, `[1h30m]`

**Attributes** — in `(parentheses)`:

| Attribute | Example | Description |
|---|---|---|
| `due <date>` | `due Friday`, `due 2025-04-01` | Due date (natural language or `YYYY-MM-DD`) |
| `not before <date>` | `not before Monday` | Earliest scheduling date |
| `not before -<N>d` | `not before -3d` | Relative to due date (`d`ays, `w`eeks, `m`onths) |
| `priority:<level>` | `priority:p1` | Priority — `p1` Highest, `p2` High, `p3` Medium, `p4` Low, `p5` Lowest |
| `upnext` | `upnext` | Schedule before other tasks |
| `nosplit` | `nosplit` | Keep in a single calendar slot |

## Web Dashboard

`fylla serve` starts a local web dashboard (default port 8002).

```bash
fylla serve              # http://localhost:8002
fylla serve --port 3000  # custom port
```

### Pages

| Route | Description |
|---|---|
| `/` or `/timeline` | Today's timeline |
| `/tasks` | Sorted task list |
| `/schedule` | Full schedule view |
| `/status` | Config summary |

### API

| Endpoint | Description |
|---|---|
| `GET /api/today` | Today's Fylla + calendar events as a timeline |
| `GET /api/tasks` | Sorted task list (scored) |
| `GET /api/schedule` | Full dry-run schedule (allocations, at-risk, unscheduled) |
| `GET /api/status` | Config summary: providers, business hours, window, buffer |

## Worklog Posting

`fylla worklog` turns your calendar into a timesheet. It walks through every
event for the day — both Fylla-scheduled tasks and regular meetings — lets you
adjust durations, assign Jira issues to meetings, fills remaining hours with a
fallback issue, and bulk-posts everything to Jira.

```bash
fylla worklog                    # today
fylla worklog --date 2025-02-18  # past date
```

### Interactive flow

1. **For each Fylla task** — shows the task key, summary, and calendar duration.
   You confirm or adjust the duration.
2. **For each meeting** — shows meeting name and duration. You adjust the
   duration, then pick a Jira issue from configured `worklog.fallbackIssues`
   (or type one manually).
3. **Remainder** — if logged hours are below the daily target (derived from
   `businessHours`), prompts for a fallback issue to cover the gap.
4. **Summary table** — shows all entries before posting.
5. **Confirm** — select Yes/No to post all worklogs to Jira.

### Configuration

```yaml
worklog:
  provider: jira                  # route all worklogs to Jira
  fallbackIssues:
    - ADMIN-1    # general admin
    - MEET-1     # meetings
```

The daily target is computed from `businessHours`. For example, two windows
`09:00-12:00` + `13:00-17:00` on workdays yield a 7h daily target.

### Worklog provider routing

When using multiple task providers, worklogs are normally routed to the provider
that owns the task key. This means stopping a timer on a Todoist task posts a
comment to Todoist — not a real Jira worklog. GitHub's worklog support returns an
error outright.

Set `worklog.provider: jira` to route **all** worklogs to Jira. Non-Jira task
keys (Todoist, GitHub, local) are resolved to a Jira fallback issue before
posting. GitHub PRs and local tasks already had this resolution; the `provider`
setting extends it to Todoist tasks as well.

## Sync Behavior

### Past event preservation

Re-running `fylla sync` preserves past events — only future events are
reconciled against the new schedule. This makes the calendar a reliable record
of what was planned and worked on:

- **Incremental mode** (default): past events (those whose end time is before
  now) are kept as-is. Only future events are matched, updated, created, or
  deleted.
- **Force mode** (`--force`): deletes and recreates future events only. Past
  events are preserved.

### Timer stop integration

`fylla timer stop` now does more than log work:

1. Posts worklog to Jira (as before)
2. Updates the calendar event end time to match actual work duration
3. Marks the event as done (✓ prefix visible in Google Calendar)
4. Shows remaining estimate — suggests next steps if the task has time left
   or is at zero

### Auto-resync

When `scheduling.autoResync` is enabled, commands that change the schedule
automatically trigger a re-sync:

| Command | Why |
|---------|-----|
| `fylla timer stop` | Task duration changed |
| `fylla task done` | Task completed, free up slots |
| `fylla task add` | New task needs scheduling |
| `fylla task delete` | Task removed, free up slots |
| `fylla task edit` | Estimate/due date/priority changed |

Enable in config:

```yaml
scheduling:
  autoResync: true
```

## Pull Request Reviews

When `github` is added to `providers`, PRs awaiting your review appear alongside
regular tasks in `fylla task list` and `fylla sync`.

PR reviews are **read-only** — you cannot complete, delete, or create tasks
through the GitHub provider. Operations like `fylla task done` on a PR key will
return an unsupported error.

### How PRs are scored

PRs flow through the same scoring algorithm as regular tasks:

| Factor | How it applies |
|---|---|
| **Priority** | Default **2 (High)** — reviews block someone else's work |
| **Due date** | None — scores 0 |
| **Estimate** | Derived from PR size (lines changed, see below) |
| **Age** | Uses PR `created_at` — older PRs bubble up naturally |

### PR size to time estimate

The total lines changed (additions + deletions) determine the calendar slot duration:

| Lines changed | Estimate |
|---|---|
| < 50 | 15 min |
| 50 - 199 | 30 min |
| 200 - 499 | 45 min |
| 500 - 999 | 1 hour |
| 1000+ | 1 hour 30 min |

### Key format

**GitHub:** `owner/repo#number` (e.g. `iruoy/fylla#42`)

Calendar events link directly to the PR URL on GitHub.
