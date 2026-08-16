package worker

import (
	"context"
	"log"
	"time"

	"github.com/reserve/patrol-dispatch/internal/config"
	"github.com/reserve/patrol-dispatch/internal/service"
)

// SyncReplayWorker periodically replays offline-cached reports.  When a
// patrol terminal loses connectivity, reports are stored locally.  This
// worker drains the pending queue in report-time order after connectivity
// is restored, relying on idempotency keys to avoid duplicate processing.
type SyncReplayWorker struct {
	sync        *service.SyncService
	interval    time.Duration
	maxAttempts int
}

// NewSyncReplayWorker creates a sync replay worker from the sync service
// and configuration.
func NewSyncReplayWorker(ss *service.SyncService, cfg config.Config) *SyncReplayWorker {
	return &SyncReplayWorker{
		sync:        ss,
		interval:    cfg.SyncReplayInterval,
		maxAttempts: cfg.SyncMaxAttempts,
	}
}

// Run starts the replay loop, blocking until the context is cancelled.
func (w *SyncReplayWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.ReplayOnce()
		}
	}
}

// ReplayOnce performs a single replay pass.  Exported for tests and
// manual triggering.
func (w *SyncReplayWorker) ReplayOnce() (uploaded, skipped, failed int) {
	uploaded, skipped, failed, err := w.sync.ReplayPending(w.maxAttempts)
	if err != nil {
		log.Printf("sync replay worker: error: %v", err)
	}
	if uploaded+failed > 0 {
		log.Printf("sync replay worker: uploaded=%d skipped=%d failed=%d", uploaded, skipped, failed)
	}
	return uploaded, skipped, failed
}
