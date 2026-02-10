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
