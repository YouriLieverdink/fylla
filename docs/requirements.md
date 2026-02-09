# Fylla - Dart CLI Jira Scheduler

A CLI tool that pulls Jira tasks, sorts them by priority rules, and schedules them into free slots on Google Calendar.

*Fylla (Swedish): "to fill" - filling your calendar with what matters.*

## Project Structure

```
fylla/
├── bin/
│   └── fylla.dart                   # CLI entry point
├── lib/
│   └── src/
│       ├── cli/
│       │   ├── commands/
│       │   │   ├── auth_command.dart
│       │   │   ├── sync_command.dart
│       │   │   ├── list_command.dart
│       │   │   ├── config_command.dart
│       │   │   ├── start_command.dart
│       │   │   ├── stop_command.dart
│       │   │   ├── status_command.dart
│       │   │   ├── log_command.dart
│       │   │   ├── estimate_command.dart
│       │   │   └── add_command.dart
│       │   └── cli_runner.dart
│       ├── jira/
│       │   ├── jira_client.dart
│       │   └── jira_task.dart
│       ├── calendar/
│       │   ├── google_calendar_client.dart
│       │   ├── calendar_event.dart
│       │   ├── free_slot_finder.dart
│       │   └── oauth_handler.dart
│       ├── scheduler/
│       │   ├── task_sorter.dart
│       │   ├── sort_config.dart
│       │   └── slot_allocator.dart
│       ├── config/
│       │   ├── app_config.dart
│       │   └── config_store.dart
│       └── timer/
│           └── timer_state.dart         # Active timer persistence
├── config/
│   └── default_config.yaml
├── pubspec.yaml
└── README.md
```

## Dependencies

```yaml
dependencies:
  args: ^2.4.0              # CLI parsing
  googleapis: ^13.0.0       # Google Calendar API
  googleapis_auth: ^1.6.0   # OAuth2
  http: ^1.2.0              # HTTP client
  yaml: ^3.1.0              # Config parsing
  path: ^1.9.0              # Path handling
  interact_cli: ^2.1.1      # Interactive prompts (Input, Select, Confirm)
```

## Sorting Algorithm

Default weights (configurable via YAML):

| Field | Weight | Logic |
|-------|--------|-------|
| Priority | 40% | Jira priority 1-5 → score 100-20 |
| Due Date | 30% | Days until due: 0=100, 30+=0 |
| Estimate | 15% | Smaller tasks score higher (quick wins) |
| Issue Type | 10% | Bug=100, Task=70, Story=50 |
| Age | 5% | Older tasks get slight boost |

## CLI Commands

```bash
# Authentication
fylla auth jira --url https://company.atlassian.net --email you@example.com --token TOKEN
fylla auth google    # Interactive OAuth flow

# Scheduling
fylla sync                          # Schedule tasks → create calendar events
fylla sync --dry-run                # Preview without creating events
fylla sync --jql "project = MYPROJ"
fylla sync --days 10                # Override scheduling window (default: 5)
fylla sync --from 2025-01-20 --to 2025-01-24  # Explicit date range

fylla list                          # Show sorted tasks without scheduling

# Time tracking
fylla start PROJ-123                # Start timer for a task
fylla stop                          # Stop timer, prompts for description, logs to Jira
fylla stop --description "Fixed the auth bug"  # Stop with inline description
fylla status                        # Show currently running task + elapsed time
fylla log PROJ-123 2h "Description" # Manual worklog without timer

# Adjust estimates
fylla estimate PROJ-123 4h          # Set remaining estimate to 4 hours
fylla estimate PROJ-123 +2h         # Add 2 hours to current estimate
fylla estimate PROJ-123 -1h         # Reduce estimate by 1 hour

# Quick add task
fylla add                           # Interactive walkthrough (prompts for each field)
fylla add --quick                   # Just summary + estimate (minimal prompts)
fylla add --project PROJ            # Pre-select project

# Config
fylla config show                   # View current config
fylla config edit                   # Edit config in $EDITOR
fylla config set scheduling.windowDays 7
fylla config set projectRules.ADMIN.end "11:00"
```

## Data Flow

```
1. Delete all existing [Fylla] events from Google Calendar
         ↓
2. Fetch Jira tasks (JQL query)
         ↓
3. Sort by composite score
         ↓
4. Fetch Google Calendar events (meetings, OOO - within scheduling window)
         ↓
5. Find free slots per project
   - Default business hours for most tasks
   - Project-specific windows for configured projects
   - Exclude OOO periods
         ↓
6. Allocate tasks to slots (first-fit, respecting min duration)
         ↓
7. Create fresh [Fylla] calendar events (or dry-run output)
```

## Key Implementation Details

### Scheduling Window
- Only schedules within `windowDays` from today (default: 5 days)
- Prevents over-scheduling far into the future
- CLI override: `fylla sync --days 10`
- Events start from *now*, not from start of day

### Free Slot Finding
- Filter to business hours (configurable, default 09:00-17:00)
- Skip weekends (configurable)
- Apply buffer between tasks (default 15 min)
- **Project-aware**: Tasks from specific projects only match their configured time windows
- **Out of Office**: Detects Google Calendar OOO events and blocks those entire time ranges (no scheduling during PTO/sick days)

### Task Splitting & Minimum Duration
- `minTaskDurationMinutes` (default: 25) prevents tiny fragments
- If a free slot is smaller than the minimum, it's skipped
- If a task would be split and the remainder is < minimum, the whole task moves to the next suitable slot
- Example: 45-min slot, 60-min task, 25-min minimum → task doesn't start here, moves to next slot

### Slot Allocation
- First-fit: highest priority task gets first available slot
- **Project filtering**: Tasks check if their project has custom time rules
- Tasks without estimates default to 1 hour

### Deadline Risk Handling
- **Crunch mode boost**: Tasks with due date < 3 days away get extra priority weight
- **At-risk detection**: After allocation, check if any task is scheduled after its due date
- **Warnings**: Show "PROJ-123 due Jan 25 but scheduled for Jan 27" after sync
- **Visual flag**: At-risk tasks get "[LATE]" prefix and red color in calendar

### Google Calendar Events
- Created with prefix "[Fylla]" in title (e.g., "[Fylla] PROJ-123: Fix login bug")
- Description includes link back to Jira
- Can be viewed/edited in Google Calendar normally

### Sync = Clean Sweep
- Each `fylla sync` **wipes the entire fylla calendar** and recreates events fresh
- Calendar is purely forward-looking - not a historical record
- Jira worklogs are the source of truth for what you actually worked on

### Dual Calendar Setup
- **Source calendar** (`sourceCalendar`): Your main calendar with meetings and OOO - read-only
- **Fylla calendar** (`fyllaCalendar`): Dedicated calendar for scheduled tasks - gets wiped on sync
- Create the fylla calendar in Google Calendar first, then set its ID in config
- This way sync never touches your meetings

### Out of Office Handling
- Detects events with `eventType: "outOfOffice"` (Google Calendar's OOO feature)
- Also detects all-day events with "OOO", "Out of Office", "PTO", "Vacation" in title
- Entire OOO time range is blocked - no tasks scheduled during these periods
- Works with multi-day OOO events (e.g., week-long vacation)

## Configuration File

`~/.config/fylla/config.yaml`:

```yaml
jira:
  url: https://company.atlassian.net
  email: you@example.com
  defaultJql: "assignee = currentUser() AND status = 'To Do'"

calendar:
  sourceCalendar: primary           # Read meetings & OOO from here
  fyllaCalendar: fylla              # Write scheduled tasks here (dedicated calendar)

# Global scheduling settings
scheduling:
  windowDays: 5                    # How far ahead to schedule (default: 5 days)
  minTaskDurationMinutes: 25       # Minimum slot for a task (don't schedule 5-min fragments)
  bufferMinutes: 15                # Buffer between tasks

# Default business hours (can be overridden per project)
businessHours:
  start: "09:00"
  end: "17:00"
  workDays: [1, 2, 3, 4, 5]

# Project-specific scheduling rules (optional)
# Tasks from these projects only get scheduled in their defined windows
projectRules:
  ADMIN:                           # Project key
    start: "09:00"
    end: "10:00"                   # Admin tasks only in first hour
    workDays: [1, 2, 3, 4, 5]

  DEEPWORK:
    start: "13:00"
    end: "17:00"                   # Deep work projects in afternoons
    workDays: [1, 2, 3, 4, 5]

  # Projects not listed here use default businessHours

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

## Time Tracking & Jira Worklogs

Track time spent on tasks and automatically log to Jira:

### Workflow
```
$ fylla start PROJ-123
Started timer for PROJ-123: Fix authentication bug
Timer running...

# ... work on the task ...

$ fylla status
PROJ-123: Fix authentication bug
Running for: 1h 23m

$ fylla stop
Timer stopped: 1h 25m
What did you work on? > Fixed the OAuth token refresh logic
Worklog added to PROJ-123
```

### Implementation
- **Timer state**: Stored in `~/.config/fylla/timer.json` (task key, start time)
- **Jira worklog API**: `POST /rest/api/3/issue/{issueKey}/worklog`
- **Rounds to nearest 5 minutes** (configurable)

### Commands
| Command | Action |
|---------|--------|
| `fylla start PROJ-123` | Start timer, save state to disk |
| `fylla stop` | Stop timer, prompt for description, submit worklog |
| `fylla stop -d "desc"` | Stop with inline description (no prompt) |
| `fylla status` | Show active timer and elapsed time |
| `fylla log PROJ-123 2h "desc"` | Manual worklog without using timer |

### Adjusting Estimates
Update Jira's "Remaining Estimate" field:
```
$ fylla estimate PROJ-123 4h      # Set to exactly 4 hours
$ fylla estimate PROJ-123 +2h     # Add 2 hours
$ fylla estimate PROJ-123 -30m    # Subtract 30 minutes
```
This updates `timetracking.remainingEstimate` in Jira, which the scheduler uses for slot allocation.

## Quick Add Jira Task

Create tasks directly from the CLI with interactive prompts (using `interact_cli` package):

```
$ fylla add
? Project: [Select] PROJ, ADMIN, OTHER
? Issue type: [Select] Task, Bug, Story
? Summary: Fix the login timeout issue
? Description (optional): Users are being logged out after 5 minutes
? Estimate: 2h
? Priority: [Select] Highest, High, Medium, Low, Lowest

Created PROJ-456: Fix the login timeout issue
```

### Quick mode
Skip optional fields:
```
$ fylla add --quick
? Project: PROJ
? Summary: Quick bugfix
? Estimate: 30m

Created PROJ-457: Quick bugfix
```

### With flags
Pre-fill values:
```
$ fylla add --project PROJ --type Bug --priority High
```

## Implementation Order

1. **Project setup** - pubspec.yaml, folder structure
2. **Config system** - Load/save YAML config, credential storage
3. **Jira client** - Fetch tasks via REST API, worklog API
4. **Task sorter** - Implement scoring algorithm
5. **Google OAuth** - CLI auth flow with credential caching
6. **Calendar client** - Fetch events, create events
7. **Free slot finder** - Business hours logic
8. **Slot allocator** - First-fit assignment
9. **Timer system** - Start/stop/status with disk persistence
10. **CLI commands** - Wire everything together

## Verification

1. Run `fylla auth jira ...` and `fylla auth google` to set up credentials
2. Run `fylla list` to verify Jira tasks are fetched and sorted
3. Run `fylla sync --dry-run` to see proposed schedule
4. Run `fylla sync` to create actual calendar events
5. Check Google Calendar to verify events appear correctly
