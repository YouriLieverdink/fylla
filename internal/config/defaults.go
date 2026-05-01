package config

var defaultConfigYAML = []byte(`providers: [local]

todoist:
  defaultFilter: "today | overdue"
  defaultProject: ""

github:
  defaultQueries:
    - "is:pr state:open review-requested:@me"
  repos: []

local:
  storePath: ""
  defaultFilter: ""
  defaultProject: ""

kendo:
  url: ""
  defaultFilter: ""
  defaultProject: ""
  doneLane: ""

calendar:
  sourceCalendars: [primary]
  fyllaCalendar: fylla

scheduling:
  windowDays: 5
  minTaskDurationMinutes: 25
  maxTaskDurationMinutes: 0
  bufferMinutes: 15
  travelBufferMinutes: 30
  snapMinutes: [0, 15, 30, 45]
  defaultEstimateMinutes: 60

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
  provider: ""
  fallbackIssues: []
  roundMinutes: 1

efficiency:
  weeklyHours: 40
  dailyHours: 8
  target: 0.7

targets: []

weights:
  priority: 0.45
  dueDate: 0.30
  estimate: 0.15
  age: 0.10
  upNext: 50
`)
