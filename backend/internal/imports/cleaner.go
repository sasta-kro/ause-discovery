package imports

import (
	"context"
	"log/slog"
	"time"
)

type Cleaner struct {
	Service  Service
	Interval time.Duration
	Logger   *slog.Logger
}

func (cleaner Cleaner) Run(ctx context.Context) {
	interval := cleaner.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	cleaner.cleanup(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleaner.cleanup(ctx)
		}
	}
}

func (cleaner Cleaner) cleanup(ctx context.Context) {
	cleaned, err := cleaner.Service.CleanupExpired(ctx, 100)
	if err != nil {
		if cleaner.Logger != nil {
			cleaner.Logger.Error("Import expiry cleanup failed", "error", err)
		}
		return
	}
	if cleaned > 0 && cleaner.Logger != nil {
		cleaner.Logger.Info("Import previews expired", "batch_count", cleaned)
	}
}
