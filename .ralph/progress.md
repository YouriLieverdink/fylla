## Feature: project-setup
- SETUP-001: PASS - go.mod with all required dependencies (cobra, google api, oauth2, yaml, survey)
- SETUP-002: PASS - Project structure with cmd/fylla/main.go, internal/{cli,jira,calendar,scheduler,config,timer}, config/default_config.yaml
- SETUP-003: PASS - CLI entry point runs, shows help with all commands listed
Build: SUCCESS | Lint: SUCCESS | Test: 0 passed (no test target)

## Feature: config-system
- CFG-001: PASS - Default path (~/.config/fylla/config.yaml), auto-create dir and defaults on first load
- CFG-002: PASS - Jira config fields (URL, email, defaultJQL) parsed from YAML
- CFG-003: PASS - Calendar config fields (sourceCalendar, fyllaCalendar) parsed
- CFG-004: PASS - Scheduling config fields (windowDays, minTaskDurationMinutes, bufferMinutes) parsed
- CFG-005: PASS - Business hours (start, end, workDays) parsed
- CFG-006: PASS - BusinessHoursFor returns project-specific rule or default fallback
- CFG-007: PASS - Weights config (priority, dueDate, estimate, issueType, age) parsed
- CFG-008: PASS - TypeScores map values (Bug, Task, Story) parsed
- CFG-009: PASS - Credentials stored separately as JSON with 0600 permissions, round-trip save/load
Build: SUCCESS | Lint: SUCCESS | Test: 22 passed

## Feature: jira-client
- JIRA-001: PASS - Fetch tasks via REST API with basic auth, HTTP error handling
- JIRA-002: PASS - Custom JQL sent in search request body, invalid JQL returns clear error
- JIRA-003: PASS - Post worklog to /issue/{key}/worklog with timeSpentSeconds and ADF comment
- JIRA-004: PASS - Update remaining estimate via PUT /issue/{key} with formatted duration
- JIRA-005: PASS - Create issues via POST /issue with all fields (project, type, summary, description, estimate, priority)
- JIRA-006: PASS - Priority parsing: Highest=1, High=2, Medium=3, Low=4, Lowest=5, nil defaults to 3
- JIRA-007: PASS - Due date parsed from "2006-01-02" format, nil for missing dates
- JIRA-008: PASS - Original and remaining estimate parsed from seconds, nil timetracking handled
- JIRA-009: PASS - Issue type (Bug, Task, Story) parsed from issuetype.name field
Build: SUCCESS | Lint: SUCCESS | Test: 24 passed

## Feature: task-sorter
- SORT-001: PASS - Priority 40% weight, higher priority tasks sorted first
- SORT-002: PASS - Due date 30% weight, earlier due dates prioritized
- SORT-003: PASS - Estimate 15% weight, smaller tasks score higher (quick wins)
- SORT-004: PASS - Issue type 10% weight, Bug prioritized over Task
- SORT-005: PASS - Age 5% weight, older tasks get slight boost
- SORT-006: PASS - Priority scoring: Highest(1)=100, High(2)=80, Medium(3)=60, Low(4)=40, Lowest(5)=20
- SORT-007: PASS - Due date scoring: 0 days=100, 30+ days=0, linear decay
- SORT-008: PASS - Estimate scoring: inverse relationship, 30min=93.75, 8h=0
- SORT-009: PASS - Issue type scoring: Bug=100, Task=70, Story=50
- SORT-010: PASS - Crunch mode: tasks due within 3 days get extra priority boost
Build: SUCCESS | Lint: SUCCESS | Test: 28 passed

## Feature: google-oauth
- GCAL-001: PASS - OAuth flow opens browser with auth URL, callback server exchanges code for token, success message displayed
- GCAL-002: PASS - Token saved to disk with 0600 permissions, round-trip save/load, CachedToken reuses valid cached token without re-auth
Build: SUCCESS | Lint: SUCCESS | Test: 11 passed

## Feature: calendar-client
- GCAL-003: PASS - FetchEvents retrieves events from source calendar within time range, parsed as busy times
- GCAL-004: PASS - CreateEvent inserts events on fylla calendar with correct start/end times
- GCAL-005: PASS - DeleteFyllaEvents removes [Fylla] and [LATE] [Fylla] prefixed events, skips non-Fylla events
- GCAL-006: PASS - Event title format "[Fylla] PROJ-123: Summary", at-risk tasks get "[LATE] [Fylla]" prefix
- GCAL-007: PASS - Event description contains Jira issue URL using configured base URL
- GCAL-008: PASS - Events with eventType "outOfOffice" detected as OOO via IsOOO()
- GCAL-009: PASS - Title patterns (OOO, Out of Office, PTO, Vacation) detected as OOO, case-insensitive
Build: SUCCESS | Lint: SUCCESS | Test: 28 passed

## Feature: free-slot-finder
- SLOT-001: PASS - Slots filtered to configured business hours (09:00-17:00 default, custom supported)
- SLOT-002: PASS - Weekends skipped by default, configurable via workDays including weekend support
- SLOT-003: PASS - Buffer applied after events (15 min default, 30 min configurable, zero supported)
- SLOT-004: PASS - Project-aware time windows via BusinessHoursFor(), ADMIN gets 09:00-10:00
- SLOT-005: PASS - OOO events block scheduling (full day, partial day, title pattern detection)
- SLOT-006: PASS - Today's slots start from current time (not start of day), buffer applied
- SLOT-007: PASS - Multi-day OOO handled correctly (week-long vacation, partial week, multiple OOO)
Build: SUCCESS | Lint: SUCCESS | Test: 21 passed
