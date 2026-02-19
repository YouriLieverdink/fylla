package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/iruoy/fylla/internal/config"
)

// autoResync runs a sync operation to adjust the future schedule.
// It loads config, calendar, and task source fresh, then calls RunSync.
func autoResync(ctx context.Context, w io.Writer) error {
	source, cfg, err := loadTaskSource()
	if err != nil {
		return fmt.Errorf("load task source: %w", err)
	}

	cal, err := loadCalendarClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("load calendar: %w", err)
	}

	now := time.Now()
	query, start, end, _, err := BuildSyncParams(SyncFlags{}, cfg, now)
	if err != nil {
		return fmt.Errorf("build sync params: %w", err)
	}

	var fetcher TaskFetcher
	if ms, ok := source.(*MultiTaskSource); ok {
		fetcher = &multiFetcher{
			queries: buildProviderQueries(cfg, "", ""),
			sources: ms.sources,
		}
	} else {
		fetcher = source
	}

	result, err := RunSync(ctx, SyncParams{
		Cal:      cal,
		Tasks:    fetcher,
		Cfg:      cfg,
		Query:    query,
		Now:      now,
		Start:    start,
		End:      end,
		Progress: nil,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Auto-resync: %d created, %d updated, %d deleted, %d unchanged.\n",
		result.Created, result.Updated, result.Deleted, result.Unchanged)
	return nil
}

// maybeAutoResync calls autoResync if scheduling.autoResync is enabled in config.
func maybeAutoResync(ctx context.Context, w io.Writer) {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	if !cfg.Scheduling.AutoResync {
		return
	}
	if err := autoResync(ctx, w); err != nil {
		fmt.Fprintf(w, "Warning: auto-resync failed: %v\n", err)
	}
}
