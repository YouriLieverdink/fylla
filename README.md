# Fylla

Fylla is a Go CLI that pulls tasks from Jira, Todoist, GitHub, and/or Bitbucket,
scores and sorts them by configurable priority rules, finds free time in Google
Calendar, and schedules tasks into those slots. Multiple task providers can be
used simultaneously — tasks from all sources are pooled and merged into a single
schedule.

> **Name origin:** *Fylla* (Swedish) means "to fill".

## Tech Stack

- Go `1.24+`
- CLI: `github.com/spf13/cobra`
- Jira / Todoist / Bitbucket API: standard `net/http`
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
  - Bitbucket Cloud account + API token
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

# Bitbucket
fylla auth bitbucket --username YOUR_USERNAME --api-token YOUR_API_TOKEN
# optionally add --workspace WORKSPACE to filter PRs to a specific workspace

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

#### Bitbucket

Create an [API token](https://support.atlassian.com/bitbucket-cloud/docs/using-app-passwords/)
in Personal Settings with the following permission:

| Permission | Required |
|---|---|
| **Pull requests: Read** | Yes |

Fylla fetches PRs where you are a reviewer and calls the diffstat endpoint per PR.

If you already have a Jira Cloud instance on the same Atlassian account, you can
reuse the same [Atlassian API token](https://id.atlassian.com/manage-profile/security/api-tokens)
for both Jira and Bitbucket.

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
- `~/.config/fylla/bitbucket_credentials.json`

Other data files:

- `~/.config/fylla/google_credentials.json` (Google OAuth client config + access/refresh token)
- `~/.config/fylla/timer.json` (running timer state)

### Multi-provider config example

```yaml
providers: [jira, todoist, github, bitbucket]

jira:
  credentials: ~/.config/fylla/jira_credentials.json  # set by fylla auth jira
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"

todoist:
  credentials: ~/.config/fylla/todoist_credentials.json  # set by fylla auth todoist
  defaultFilter: "today | overdue"

github:
  credentials: ~/.config/fylla/github_credentials.json  # set by fylla auth github
  defaultQuery: "is:pr state:open review-requested:@me"  # customize search query

bitbucket:
  credentials: ~/.config/fylla/bitbucket_credentials.json  # set by fylla auth bitbucket
  username: your-username
  workspace: myteam  # optional: filter PRs to a specific workspace

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
# Multi-provider: --jql and --filter are used for their respective providers
# GitHub and Bitbucket PRs are fetched using config defaults (no CLI flag needed)

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
# Multi-provider: task key format routes to correct provider
# PROJ-123 → Jira, 12345 → Todoist, GH#owner/repo#42 → GitHub, BB#ws/repo#17 → Bitbucket

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
fylla config set providers "[jira, todoist, github]"  # set providers
```

## Pull Request Reviews

When `github` or `bitbucket` is added to `providers`, PRs awaiting your review
appear alongside regular tasks in `fylla task list` and `fylla schedule sync`.

PR reviews are **read-only** — you cannot complete, delete, or create tasks
through the GitHub/Bitbucket providers. Operations like `fylla task done` on a
PR key will return an unsupported error.

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

- **GitHub:** `GH#owner/repo#number` (e.g. `GH#iruoy/fylla#42`)
- **Bitbucket:** `BB#workspace/repo#id` (e.g. `BB#myteam/api#17`)

Calendar events link directly to the PR URL on the respective platform.
