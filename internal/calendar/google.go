package calendar

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	googlecalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleClient wraps the Google Calendar API for fetching and creating events.
type GoogleClient struct {
	Service        *googlecalendar.Service
	SourceCalendar string
	FyllaCalendar  string
	JiraBaseURL    string
}

// NewGoogleClient creates a GoogleClient using the given OAuth2 token.
func NewGoogleClient(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token, sourceCalendar, fyllaCalendar, jiraBaseURL string) (*GoogleClient, error) {
	src := cfg.TokenSource(ctx, token)
	svc, err := googlecalendar.NewService(ctx, option.WithTokenSource(src))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}
	return &GoogleClient{
		Service:        svc,
		SourceCalendar: sourceCalendar,
		FyllaCalendar:  fyllaCalendar,
		JiraBaseURL:    jiraBaseURL,
	}, nil
}

// FetchEvents retrieves events from the source calendar within the given time range.
func (c *GoogleClient) FetchEvents(ctx context.Context, start, end time.Time) ([]Event, error) {
	var events []Event
	pageToken := ""
	for {
		call := c.Service.Events.List(c.SourceCalendar).
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
			return nil, fmt.Errorf("list events: %w", err)
		}
		for _, item := range result.Items {
			events = append(events, parseGoogleEvent(item))
		}
		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return events, nil
}

// parseGoogleEvent converts a Google Calendar event to our Event type.
func parseGoogleEvent(item *googlecalendar.Event) Event {
	e := Event{
		ID:        item.Id,
		Title:     item.Summary,
		EventType: item.EventType,
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

// fyllaPrefix is the prefix used for all Fylla-created calendar events.
const fyllaPrefix = "[Fylla] "

// latePrefix is added before fyllaPrefix for at-risk tasks.
const latePrefix = "[LATE] "

// DeleteFyllaEvents removes all events with the [Fylla] prefix from the fylla calendar
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
			if hasFyllaPrefix(item.Summary) {
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

func hasFyllaPrefix(title string) bool {
	return len(title) >= len(fyllaPrefix) && title[:len(fyllaPrefix)] == fyllaPrefix ||
		len(title) >= len(latePrefix+fyllaPrefix) && title[:len(latePrefix+fyllaPrefix)] == latePrefix+fyllaPrefix
}

// CreateEventInput holds the fields needed to create a calendar event.
type CreateEventInput struct {
	TaskKey  string
	Summary  string
	Start    time.Time
	End      time.Time
	AtRisk   bool
}

// CreateEvent creates a new event on the fylla calendar.
func (c *GoogleClient) CreateEvent(ctx context.Context, input CreateEventInput) error {
	title := fmt.Sprintf("%s%s: %s", fyllaPrefix, input.TaskKey, input.Summary)
	if input.AtRisk {
		title = fmt.Sprintf("%s%s%s: %s", latePrefix, fyllaPrefix, input.TaskKey, input.Summary)
	}

	description := fmt.Sprintf("Jira: %s/browse/%s", c.JiraBaseURL, input.TaskKey)

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
