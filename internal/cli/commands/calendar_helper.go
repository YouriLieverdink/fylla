package commands

import (
	"context"
	"fmt"

	"github.com/iruoy/fylla/internal/calendar"
	"github.com/iruoy/fylla/internal/config"
)

func loadCalendarClient(ctx context.Context, cfg *config.Config) (CalendarClient, error) {
	credPath := cfg.Calendar.Credentials
	if credPath == "" {
		return nil, fmt.Errorf("calendar not configured: run 'fylla auth google'")
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
	baseURL := cfg.Jira.URL
	return calendar.NewGoogleClient(ctx, oauthCfg, creds.Token,
		cfg.Calendar.SourceCalendars, cfg.Calendar.FyllaCalendar, baseURL)
}
