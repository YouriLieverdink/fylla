package calendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleClient wraps the Google Calendar API for fetching and creating events.
type GoogleClient struct {
	Service         *googlecalendar.Service
	SourceCalendars []string
	FyllaCalendar   string
	JiraBaseURL     string
}

// NewGoogleClient creates a GoogleClient using the given OAuth2 token.
func NewGoogleClient(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token, sourceCalendars []string, fyllaCalendar, jiraBaseURL string) (*GoogleClient, error) {
	src := cfg.TokenSource(ctx, token)
	svc, err := googlecalendar.NewService(ctx, option.WithTokenSource(src))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}
	return &GoogleClient{
		Service:         svc,
		SourceCalendars: sourceCalendars,
		FyllaCalendar:   fyllaCalendar,
		JiraBaseURL:     jiraBaseURL,
	}, nil
}

// FetchEvents retrieves events from all source calendars within the given time range.
func (c *GoogleClient) FetchEvents(ctx context.Context, start, end time.Time) ([]Event, error) {
	var events []Event
	for _, calID := range c.SourceCalendars {
		pageToken := ""
		for {
			call := c.Service.Events.List(calID).
				Context(ctx).
				TimeMin(start.Format(time.RFC3339)).
				TimeMax(end.Format(time.RFC3339)).
				SingleEvents(true).
				OrderBy("startTime")
			if pageToken != "" {
				call = call.PageToken(pageToken)
			}
			result, err := call.Do()
			if err != nil {
				return nil, fmt.Errorf("list events from %s: %w", calID, err)
			}
			for _, item := range result.Items {
				events = append(events, parseGoogleEvent(item))
			}
			pageToken = result.NextPageToken
			if pageToken == "" {
				break
			}
		}
	}
	return events, nil
}

// parseGoogleEvent converts a Google Calendar event to our Event type.
func parseGoogleEvent(item *googlecalendar.Event) Event {
	e := Event{
		ID:           item.Id,
		Title:        item.Summary,
		Description:  item.Description,
		Location:     item.Location,
		EventType:    item.EventType,
		Transparency: item.Transparency,
	}

	if item.Start != nil {
		if item.Start.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, item.Start.DateTime); err == nil {
				e.Start = t
			}
		} else if item.Start.Date != "" {
			if t, err := time.Parse("2006-01-02", item.Start.Date); err == nil {
				e.Start = t
				e.AllDay = true
			}
		}
	}

	if item.End != nil {
		if item.End.DateTime != "" {
			if t, err := time.Parse(time.RFC3339, item.End.DateTime); err == nil {
				e.End = t
			}
		} else if item.End.Date != "" {
			if t, err := time.Parse("2006-01-02", item.End.Date); err == nil {
				e.End = t
				e.AllDay = true
			}
		}
	}

	return e
}

// latePrefix is added before the summary for at-risk tasks.
const latePrefix = "⚠️ "

// fyllaMarker is written into event descriptions to identify Fylla-managed events.
const fyllaMarker = "fylla:"

// DeleteFyllaEvents removes all Fylla-managed events from the fylla calendar
// within the given time range.
func (c *GoogleClient) DeleteFyllaEvents(ctx context.Context, start, end time.Time) error {
	pageToken := ""
	for {
		call := c.Service.Events.List(c.FyllaCalendar).
			Context(ctx).
			TimeMin(start.Format(time.RFC3339)).
			TimeMax(end.Format(time.RFC3339)).
			SingleEvents(true)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		result, err := call.Do()
		if err != nil {
			return fmt.Errorf("list fylla events: %w", err)
		}
		for _, item := range result.Items {
			if isFyllaEvent(item.Description) {
				if err := c.Service.Events.Delete(c.FyllaCalendar, item.Id).Context(ctx).Do(); err != nil {
					return fmt.Errorf("delete event %s: %w", item.Id, err)
				}
			}
		}
		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return nil
}

// isFyllaEvent checks whether an event description contains the Fylla marker.
func isFyllaEvent(description string) bool {
	return len(description) >= len(fyllaMarker) && containsStr(description, fyllaMarker)
}

// BuildTitle constructs the calendar event title for a Fylla task.
func BuildTitle(project, summary string, atRisk bool) string {
	title := summary
	if project != "" {
		title = "[" + project + "] " + title
	}
	if atRisk {
		return latePrefix + title
	}
	return title
}

// BuildTitleWithSection constructs the calendar event title including section.
func BuildTitleWithSection(project, section, summary string, atRisk bool) string {
	prefix := project
	if prefix != "" && section != "" {
		prefix = prefix + " / " + section
	}
	return BuildTitle(prefix, summary, atRisk)
}

// ParsedTitle holds the components extracted from a Fylla event title.
type ParsedTitle struct {
	Project string
	Section string
	Summary string
	AtRisk  bool
}

// ParseTitle extracts project, summary, and atRisk from a Fylla event title.
// It reverses BuildTitle, handling formats: "⚠️ [PROJ] Summary", "[PROJ] Summary",
// "⚠️ Summary", "Summary".
func ParseTitle(title string) ParsedTitle {
	var p ParsedTitle
	s := title

	if strings.HasPrefix(s, latePrefix) {
		p.AtRisk = true
		s = s[len(latePrefix):]
	}

	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "] ")
		if end != -1 {
			bracket := s[1:end]
			if parts := strings.SplitN(bracket, " / ", 2); len(parts) == 2 {
				p.Project = parts[0]
				p.Section = parts[1]
			} else {
				p.Project = bracket
			}
			s = s[end+2:]
		}
	}

	p.Summary = s
	return p
}

// BuildDescription constructs the calendar event description for a Fylla task.
// It infers the source from the task key prefix: GH# for GitHub,
// numeric for Todoist, otherwise Jira. The project field is used for
// GitHub URLs since the key only contains the repo name (not the owner).
func BuildDescription(taskKey, project, jiraBaseURL string) string {
	if strings.Contains(taskKey, "#") {
		number := parseGitHubNumber(taskKey)
		if project != "" && number != "" {
			return fmt.Sprintf("%s %s\nhttps://github.com/%s/pull/%s", fyllaMarker, taskKey, project, number)
		}
	}
	if isNumericKey(taskKey) {
		return fmt.Sprintf("%s %s\nhttps://todoist.com/app/task/%s", fyllaMarker, taskKey, taskKey)
	}
	return fmt.Sprintf("%s %s\n%s/browse/%s", fyllaMarker, taskKey, jiraBaseURL, taskKey)
}

// parseGitHubNumber extracts the PR number from a GH#repo#123 key.
func parseGitHubNumber(key string) string {
	// Strip "GH#" prefix
	rest := key[3:]
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '#' {
			return rest[i+1:]
		}
	}
	return ""
}


// isNumericKey returns true if key consists entirely of digits (Todoist task ID).
func isNumericKey(key string) bool {
	if key == "" {
		return false
	}
	for _, c := range key {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// TaskKeyFromDescription extracts the task key from a Fylla event description.
// Returns "" if the description does not contain the Fylla marker.
func TaskKeyFromDescription(description string) string {
	// Description format: "fylla: KEY\n..."
	if !containsStr(description, fyllaMarker) {
		return ""
	}
	// Find the marker and extract the key after it
	for i := 0; i <= len(description)-len(fyllaMarker); i++ {
		if description[i:i+len(fyllaMarker)] == fyllaMarker {
			rest := description[i+len(fyllaMarker):]
			// Skip whitespace
			start := 0
			for start < len(rest) && rest[start] == ' ' {
				start++
			}
			rest = rest[start:]
			// Read until newline or end
			end := 0
			for end < len(rest) && rest[end] != '\n' {
				end++
			}
			return rest[:end]
		}
	}
	return ""
}

// FetchFyllaEvents retrieves only Fylla-managed events from the fylla calendar.
func (c *GoogleClient) FetchFyllaEvents(ctx context.Context, start, end time.Time) ([]Event, error) {
	var events []Event
	pageToken := ""
	for {
		call := c.Service.Events.List(c.FyllaCalendar).
			Context(ctx).
			TimeMin(start.Format(time.RFC3339)).
			TimeMax(end.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		result, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("list fylla events: %w", err)
		}
		for _, item := range result.Items {
			if isFyllaEvent(item.Description) {
				events = append(events, parseGoogleEvent(item))
			}
		}
		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return events, nil
}

// UpdateEvent updates an existing event on the fylla calendar.
func (c *GoogleClient) UpdateEvent(ctx context.Context, eventID string, input CreateEventInput) error {
	title := BuildTitleWithSection(input.Project, input.Section, input.Summary, input.AtRisk)
	description := BuildDescription(input.TaskKey, input.Project, c.JiraBaseURL)

	event := &googlecalendar.Event{
		Summary:     title,
		Description: description,
		Start: &googlecalendar.EventDateTime{
			DateTime: input.Start.Format(time.RFC3339),
		},
		End: &googlecalendar.EventDateTime{
			DateTime: input.End.Format(time.RFC3339),
		},
	}

	if _, err := c.Service.Events.Update(c.FyllaCalendar, eventID, event).Context(ctx).Do(); err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

// DeleteEvent deletes a single event from the fylla calendar.
func (c *GoogleClient) DeleteEvent(ctx context.Context, eventID string) error {
	if err := c.Service.Events.Delete(c.FyllaCalendar, eventID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete event %s: %w", eventID, err)
	}
	return nil
}

// CreateEventInput holds the fields needed to create a calendar event.
type CreateEventInput struct {
	TaskKey  string
	Project  string
	Section  string
	Summary  string
	Start    time.Time
	End      time.Time
	AtRisk   bool
}

// CreateEvent creates a new event on the fylla calendar.
func (c *GoogleClient) CreateEvent(ctx context.Context, input CreateEventInput) error {
	title := BuildTitleWithSection(input.Project, input.Section, input.Summary, input.AtRisk)
	description := BuildDescription(input.TaskKey, input.Project, c.JiraBaseURL)

	event := &googlecalendar.Event{
		Summary:     title,
		Description: description,
		Start: &googlecalendar.EventDateTime{
			DateTime: input.Start.Format(time.RFC3339),
		},
		End: &googlecalendar.EventDateTime{
			DateTime: input.End.Format(time.RFC3339),
		},
	}

	if _, err := c.Service.Events.Insert(c.FyllaCalendar, event).Context(ctx).Do(); err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	return nil
}
