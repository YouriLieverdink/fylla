package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
)

// loadCalendarClient loads the Google Calendar client for the active profile.
// If the calendar credentials file does not exist, it returns (nil, nil) to
// indicate that calendar integration is disabled — callers must handle a nil
// client. Other errors (corrupt file, refresh failure) are returned.
func loadCalendarClient(ctx context.Context, cfg *config.Config) (CalendarClient, error) {
	credPath, err := calendar.TokenPath()
	if err != nil {
		return nil, fmt.Errorf("calendar credentials path: %w", err)
	}

	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat calendar credentials: %w", err)
	}

	creds, err := calendar.LoadGoogleCredentials(credPath)
	if err != nil {
		return nil, fmt.Errorf("load google credentials: %w", err)
	}

	if err := calendar.EnsureValidToken(ctx, creds); err != nil {
		return nil, fmt.Errorf("google auth: %w", err)
	}

	// Save back in case the token was refreshed
	if saveErr := calendar.SaveGoogleCredentials(creds, credPath); saveErr != nil {
		return nil, fmt.Errorf("save refreshed credentials: %w", saveErr)
	}

	oauthCfg := creds.OAuthConfig()
	client, err := calendar.NewGoogleClient(ctx, oauthCfg, creds.Token,
		cfg.Calendar.SourceCalendars, cfg.Calendar.FyllaCalendar)
	if err != nil {
		return nil, err
	}
	client.KendoBaseURL = cfg.Kendo.URL
	return client, nil
}
