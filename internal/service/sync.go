package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

// SyncService handles the offline-cache replay pipeline.  When field
// communication is interrupted, reports are cached on the terminal.  After
// connectivity is restored the entries are replayed in ReportedAt order.
// The IdempotencyKey on each entry prevents duplicate processing so that a
// retransmitted report never creates a second incident or overwrites an
// existing one.
type SyncService struct {
	store    *store.Store
	dispatch *DispatchService
}

// NewSyncService creates a sync service backed by the given store and
// dispatch service.  The dispatch service is used to materialise cached
// incident payloads during replay.
func NewSyncService(s *store.Store, ds *DispatchService) *SyncService {
	return &SyncService{store: s, dispatch: ds}
}

// CacheReport stores a locally-cached report for later replay.  If a sync
// entry with the same idempotency key already exists it is returned as-is,
// ensuring that repeated caching of the same report does not create
// duplicates.
func (ss *SyncService) CacheReport(entry *domain.SyncEntry) (*domain.SyncEntry, error) {
	if entry.IdempotencyKey == "" {
		return nil, fmt.Errorf("sync entry requires an idempotency key")
	}
	if existing, ok := ss.store.SyncEntryByKey(entry.IdempotencyKey); ok {
		return existing, nil
	}
	if entry.ID == "" {
		entry.ID = newID("sync")
	}
	if entry.Status == "" {
		entry.Status = domain.SyncPending
	}
	if entry.CachedAt.IsZero() {
		entry.CachedAt = time.Now()
	}
	ss.store.SaveSyncEntry(entry)
	return entry, nil
}

// ReplayPending processes all pending sync entries in ReportedAt order.
// For each entry the incident payload is decoded and reported through the
// dispatch service.  Entries whose idempotency key already corresponds to
// a processed incident are marked uploaded without re-processing.
func (ss *SyncService) ReplayPending(maxAttempts int) (uploaded, skipped, failed int, err error) {
	entries := ss.store.PendingSyncEntries()
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].ReportedAt.Before(entries[j].ReportedAt)
	})

	now := time.Now()
	for _, e := range entries {
		if maxAttempts > 0 && e.Attempts >= maxAttempts {
			skipped++
			continue
		}
		if uploadErr := ss.replayEntry(e, now); uploadErr != nil {
			e.MarkFailed(uploadErr.Error(), maxAttempts)
			ss.store.SaveSyncEntry(e)
			failed++
			continue
		}
		e.MarkUploaded(now)
		ss.store.SaveSyncEntry(e)
		uploaded++
	}
	return uploaded, skipped, failed, nil
}

// replayEntry decodes the cached incident payload and submits it through
// the dispatch service.  The idempotency key on the incident ensures that
// a previously-processed report is not duplicated.
func (ss *SyncService) replayEntry(e *domain.SyncEntry, now time.Time) error {
	var inc domain.Incident
	if err := json.Unmarshal(e.Payload, &inc); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if inc.IdempotencyKey == "" {
		inc.IdempotencyKey = e.IdempotencyKey
	}
	if inc.ReportedAt.IsZero() {
		inc.ReportedAt = e.ReportedAt
	}
	if _, err := ss.dispatch.ReportIncident(&inc); err != nil {
		return err
	}
	return nil
}

// GetEntry retrieves a sync entry by ID.
func (ss *SyncService) GetEntry(entryID string) (*domain.SyncEntry, bool) {
	return ss.store.GetSyncEntry(entryID)
}
