# Fylla

Fylla is a Go CLI that pulls tasks from Jira, Todoist, GitHub, and/or Kendo,
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
  - Kendo instance + API token
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

All `auth` commands **require an explicit `--profile <name>` flag** — environment
variables and the stored pointer are not honored here, so credentials always
land in the profile you named on the command line.

```bash
# Todoist
fylla --profile work auth todoist --token YOUR_API_TOKEN

# GitHub
fylla --profile work auth github --token YOUR_GITHUB_PAT

# Kendo (also writes kendo.url into the profile's config.yaml)
fylla --profile work auth kendo --url https://yourapp.kendo.dev --token YOUR_API_TOKEN

# Jibble (worklog-only provider; API key + secret, exchanged for a JWT at runtime)
fylla --profile personal auth jibble --key YOUR_API_KEY --secret YOUR_API_SECRET

# Google Calendar (optional — enables sync / timeline / worklog calendar features)
fylla --profile work auth google --client-credentials path/to/client_credentials.json
```

Credentials are written to
`~/.config/fylla/profiles/<name>/<provider>_credentials.json`.

Calendar is optional: fylla runs without Google credentials. Calendar-
dependent features (sync, today timeline, clear, worklog posting from
calendar events) return a clear error if triggered without auth.

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

#### Kendo

Create an API token in your Kendo instance settings. The token grants access
to your projects and issues. Kendo hosts apps on subdomains of `kendo.dev`
(e.g. `https://yourapp.kendo.dev`) — use your app's URL as the `--url` value.

#### Jibble

Create an API key/secret in Jibble under **Organization Settings → API**. Jibble
is a **worklog-only** provider: it has no tasks, so it supplies nothing to the
tasks/schedule tabs. List it in `providers` (alongside your task provider) and
set `worklog.provider: jibble` to route logged hours to Jibble. The key/secret
are exchanged for a short-lived bearer token at runtime. See
[Jibble Worklog](#jibble-worklog) below.

#### Google Calendar

Create OAuth 2.0 credentials in the [Google Cloud Console](https://console.cloud.google.com/apis/credentials).
Enable the **Google Calendar API** for your project. The OAuth flow requests
these scopes automatically:

- `https://www.googleapis.com/auth/calendar` — read/write access to calendars

## Configuration

Fylla stores all state under per-profile directories. The root layout is:

```
~/.config/fylla/
  current                     # plain text, holds the active profile name
  profiles/
    default/
      config.yaml
      timer.json              # running timer state
      kendo_credentials.json
      todoist_credentials.json
      github_credentials.json
      jira_credentials.json
      google_credentials.json # Google OAuth client config + tokens
    work/
      ...
```

On first run after upgrading from a pre-profile install, fylla migrates the
legacy flat layout (`~/.config/fylla/config.yaml`, credential files, and
`timer.json`) into `profiles/default/` automatically.

Credential paths are resolved by convention — there are no `credentials:`
fields in `config.yaml`. Each provider uses
`profiles/<active>/<provider>_credentials.json`.

### Multi-provider config example

```yaml
providers: [jira, todoist, github, kendo]

jira:
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"
  defaultProject: WEB
  doneTransitions: {}

todoist:
  defaultFilter: "today | overdue"
  defaultProject: Inbox

github:
  defaultQueries:                                        # one or more GitHub search queries; results merged + deduped
    - "is:pr state:open review-requested:@me"
  repos: []                                              # optional: limit to specific repos

kendo:
  url: https://yourapp.kendo.dev
  defaultFilter: ""                                    # project name/prefix to filter by
  defaultProject: ""                                   # default project for task creation
  doneLane: done                                       # lane name for completing tasks

calendar:
  sourceCalendars: [primary]
  fyllaCalendar: fylla

scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  maxTaskDurationMinutes: 0       # 0 = unlimited; set e.g. 240 to cap chunks at 4h
  bufferMinutes: 15
  travelBufferMinutes: 30
  snapMinutes: [0, 15, 30, 45]
  providerTimeoutSeconds: 15      # per-provider fetch timeout; 0 = disabled
  taskCacheTTLSeconds: 30         # shared task cache TTL (tasks + schedule tabs)
  previewTimeoutSeconds: 20       # hard deadline for schedule-tab refresh

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

efficiency:
  weeklyHours: 40                 # weekly hour target
  dailyHours: 8                   # daily hour target
  target: 0.7                     # target efficiency (0.0–1.0, 70% = 0.7)

holidays:                         # days/windows blocked from work (subtracted from targets and scheduler)
  - date: 2026-04-27              # full day off
  - date: 2026-05-05
    start: "13:00"                # afternoon off
    end:   "17:00"
  - date: 2026-05-06              # multiple non-overlapping ranges per date allowed
    start: "09:00"
    end:   "10:00"
  - date: 2026-05-06
    start: "16:00"
    end:   "17:00"

sickDays:                         # same format/effect as holidays, tracked separately
  - date: 2026-05-12              # full sick day

weights:
  priority: 0.45
  dueDate: 0.30
  estimate: 0.15
  age: 0.10
  upNext: 50
  priorityLevels: [100, 80, 60, 40, 20]   # raw score per priority P1..P5
  # typeBonus:                            # flat score bonus per issue type
  #   "Pull Request": 15

tui:
  disabledTabs: []                # hide TUI tabs by label; valid: Dashboard, Focus, Tasks, Schedule, Tuning, Worklog, Targets, Config
```

When `tui.disabledTabs` is non-empty, the listed tabs are removed from the TUI
tab bar. Number keys map positionally to the visible tabs (e.g. with `Targets`
and `Worklog` disabled, `1`–`6` cover `Dashboard`/`Focus`/`Tasks`/`Schedule`/`Tuning`/`Config`),
and `tab`/`shift+tab` cycle only the visible set. At least one tab must remain
enabled.

### Tuning tab

The **Tuning** tab is a live editor for the priority algorithm. Use `j`/`k` to
navigate between parameters, `h`/`l` (or `-`/`+`) to nudge the selected value
(`Shift+h`/`Shift+l` for a 5× jump). A right-hand pane re-ranks the top tasks
in real time as you adjust:

- `weights.priority`, `dueDate`, `estimate`, `age` — must still sum to 1.0
- `weights.upNext` — flat boost applied to tasks marked Up Next
- `weights.priorityLevels[1..5]` — per-priority raw score (defaults to
  `100/80/60/40/20`); raise P1 to make highest-priority tasks dominate, or
  flatten the curve if priorities feel too punishing
- `weights.typeBonus` — flat additive bonus per issue type (e.g. `"Pull Request": 15`)

Press `s` to persist all changes to `config.yaml` at once, or `r` to revert
back to the saved values. Unsaved edits are flagged in the footer.

### Single-provider config example (Jira only)

```yaml
providers: [jira]

jira:
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"

calendar:
  sourceCalendars: [primary]
  fyllaCalendar: fylla
```

## Profiles

Profiles let you keep multiple isolated configurations — for example one for
work (pointing at a work Kendo instance, work Google account) and one for
personal projects. Each profile has its own `config.yaml`, credentials, and
timer state under `profiles/<name>/`.

### Commands

```bash
fylla profile list                    # list profiles; current marked with *
fylla profile current                 # print the active profile name
fylla profile use <name>              # set the stored default profile
fylla profile create <name>           # create a new profile from the template
fylla profile create <name> --from X  # copy profile X (config + credentials)
fylla profile delete <name>           # delete a profile (with confirmation)
fylla profile delete <name> --force   # skip prompt and allow deleting current
```

### Selecting the active profile

At launch, fylla picks the active profile using this precedence (highest first):

1. `--profile <name>` flag
2. `FYLLA_PROFILE=<name>` environment variable
3. `~/.config/fylla/current` pointer file
4. Literal `default`

`--profile` and `FYLLA_PROFILE` are ephemeral overrides — they do not change
the stored pointer. Use `fylla profile use <name>` to change the default.

A flag or env var pointing at a non-existent profile is a hard error. A
stale pointer file falls back to `default` if it exists.

Profile names must match `^\w+$` (letters, digits, underscores). The names
`config`, `credentials`, `current`, and `profiles`, and any name starting
with `.`, are reserved.

Switching profiles requires restarting fylla — the TUI does not swap
configurations at runtime.

### Example

```bash
fylla profile create work               # seed from template
fylla --profile work                    # launch TUI with the work profile
FYLLA_PROFILE=work fylla                # same, via env
fylla profile use work                  # make work the default from now on
fylla profile create client --from work # fork work into a client profile
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
# Kendo keys also use PROJ-123 format — provider is tracked explicitly

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
fylla config set providers "[jira, todoist, github, kendo]"  # set providers
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

### Efficiency tracking

The worklog TUI view shows an efficiency percentage — how much of your
target hours you've logged. Configure `dailyHours` and `weeklyHours`
separately from `businessHours` so you can account for lunch breaks.

```yaml
efficiency:
  weeklyHours: 40   # used in week view header
  dailyHours: 7     # used in day view header (e.g. 8h minus 1h lunch)
  target: 0.7       # 70% target
```

Efficiency is calculated as `posted worklogs / target hours`. The percentage
is color-coded: green when at or above target, yellow when within 10% of
target, red when below. In the week view, per-day efficiency is shown in each
day header.

Set all three values (`weeklyHours`, `dailyHours`, `target`) to `0` to disable
efficiency tracking — handy for personal projects. The dashboard then shows
logged hours without target coloring or the heatmap legend, and the worklog
view hides the efficiency line.

### Holidays & sick days

Use `holidays:` to record national holidays, vacations, or appointments that
reduce expected hours for that date. Each entry needs a `date: YYYY-MM-DD`.
Omit `start`/`end` for a full day off; set both (HH:MM) to block only part of
the day. Multiple non-overlapping entries may share a date — useful for
splitting a couple hours off morning and afternoon.

```yaml
holidays:
  - date: 2026-04-27              # full day off
  - date: 2026-05-05
    start: "13:00"                # half-day, afternoon off
    end:   "17:00"
  - date: 2026-05-06
    start: "09:00"                # 1h off morning + 1h off afternoon
    end:   "10:00"
  - date: 2026-05-06
    start: "16:00"
    end:   "17:00"
```

`sickDays:` uses the exact same format and has the same effect, but is tracked
and labelled separately so you can tell holidays and sick leave apart:

```yaml
sickDays:
  - date: 2026-05-12              # full sick day
  - date: 2026-05-13
    start: "13:00"                # left sick at noon
    end:   "17:00"
```

The worklog tab subtracts holiday and sick-day hours from the daily/weekly
target. The sync scheduler treats both as out-of-office ranges — no tasks are
auto-scheduled into them. In the dashboard heatmap, holidays show `⛱` and sick
days show `✚`; the "Missed" tile breaks the month down into work / holiday /
sick days.

### Worklog provider routing

When using multiple task providers, worklogs are normally routed to the provider
that owns the task key. This means stopping a timer on a Todoist task posts a
comment to Todoist — not a real Jira worklog. GitHub's worklog support returns an
error outright.

Set `worklog.provider: jira` to route **all** worklogs to Jira. Non-Jira task
keys (Todoist, GitHub, local) are resolved to a Jira fallback issue before
posting. GitHub PRs and local tasks already had this resolution; the `provider`
setting extends it to Todoist tasks as well. Kendo tasks have native worklog
support via time entries, so Kendo worklogs are posted directly to Kendo
regardless of the `worklog.provider` setting.

The worklog provider is **independent of the task provider**: hours always post
to `worklog.provider`, while task operations (mark-done, remaining estimate)
stay with the task's own provider. This is what lets Todoist supply your tasks
while a different backend (Jibble, Kendo) records the hours.

### Jibble Worklog

[Jibble](https://jibble.io) is a time-clock with no tasks — only Clients,
Projects, and Time Entries. Fylla uses it as a **worklog-only** provider, ideal
for tracking hours on personal-business or volunteer work whose tasks live in
Todoist.

Setup:

```yaml
providers: [todoist, jibble]   # Todoist supplies tasks; Jibble is worklog-only
worklog:
  provider: jibble
targets:
  - project: ICie             # a Jibble Project (bare name)
    hours: 16
    period: monthly
```

- **Logging hours**: start a focus timer on a task, then stop it. Because no
  task key maps to a Jibble Project, fylla prompts you to pick a
  `Client / Project` (e.g. `Tjas / ICie`) from your live Jibble project list. In
  the TUI this prompt appears **when you start the timer**, so the destination is
  chosen upfront; the choice is stored on the timer and reused at stop without
  asking again. (Via `fylla timer stop` on the CLI the prompt appears at stop.)
  The block is posted as a Jibble **time entry** (an `HourEntry` — date +
  duration, the same as Jibble's "add time entry" feature) with the
  task/comment as its note. The task itself stays on its own provider, so
  marking it done still updates Todoist. Chores you never pick a project for are
  simply not logged. Note: an `HourEntry` records the day and duration, not a
  clock-in time, so the worklog tab groups by day rather than showing a start
  time.
- **Reading hours**: the Worklog dashboard and `targets` read back logged hours
  from Jibble. Targets are keyed by the **bare Jibble Project name** (`ICie`),
  not the `Client / Project` label shown in the picker.
- Jibble cannot create or complete tasks, and its tasks never appear in the
  tasks/schedule tabs.

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

## Kendo Integration

When `kendo` is added to `providers`, issues from your Kendo instance appear
alongside tasks from other providers in `fylla task list` and `fylla sync`.

Kendo is a full-featured provider — you can create, complete, delete, and edit
tasks, post worklogs (time entries), and manage estimates, due dates, and
priorities.

### Key format

**Kendo:** `PREFIX-number` (e.g. `WEB-42`)

Kendo uses the same `PROJ-123` key format as Jira. To disambiguate, Fylla
tracks the provider explicitly on each task and calendar event (via the
`fylla:kendo` marker in event descriptions).

### Configuration

```yaml
providers: [kendo]

kendo:
  url: https://yourapp.kendo.dev
  defaultFilter: ""       # project name/prefix to filter issues
  defaultProject: WEB     # default project for fylla task add
  doneLane: done          # lane to move issues to on fylla task done
```

### Task completion

`fylla task done` moves the issue to the configured `doneLane` (defaults to
`"done"`). Configure `kendo.doneLane` to match your board's done column name.

### Remaining estimate

Fylla uses Kendo's server-computed `remaining_minutes` (total estimate minus
logged time) everywhere it represents "time to schedule": the tasks/schedule
tabs, sorter scoring, allocator slot sizing, `fylla task view`, and the
"remaining" line after `fylla timer stop`. A 2h task with 1h already logged
shows and schedules as 1h.

`fylla task estimate KEY 90m` sets the remaining to 90m; logged time is
preserved (Fylla writes `estimated_minutes = spent + new_remaining` so the
server-computed remaining matches the value you set).

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

## Troubleshooting

### TUI crashed with a panic

Panic stack traces are appended to `~/.config/fylla/profiles/<active>/crash.log`
on every run. If fylla exits with `program experienced a panic`, inspect that
file — the stack captured from stderr is preserved there even if the terminal
scrolls the output away.
