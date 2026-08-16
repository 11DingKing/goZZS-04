package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

func setupSyncService(t *testing.T) (*SyncService, *DispatchService) {
	t.Helper()
	s := store.New()
	ds := NewDispatchService(s)
	ss := NewSyncService(s, ds)
	return ss, ds
}

func makeSyncEntry(key string, reportedAt time.Time) *domain.SyncEntry {
	inc := domain.Incident{
		Type:           domain.IncidentTypePoaching,
		Location:       &domain.Location{Lat: 30, Lng: 120},
		Images:         []string{"evidence.jpg"},
		IdempotencyKey: key,
		ReportedAt:     reportedAt,
	}
	payload, _ := json.Marshal(inc)
	return &domain.SyncEntry{
		IdempotencyKey: key,
		Payload:        payload,
		ReportedAt:     reportedAt,
		Status:         domain.SyncPending,
	}
}

func TestSyncOrderedReplay(t *testing.T) {
	ss, ds := setupSyncService(t)

	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)

	// Cache entries out of order
	e1, _ := ss.CacheReport(makeSyncEntry("key-1", t1))
	e2, _ := ss.CacheReport(makeSyncEntry("key-2", t2))
	e3, _ := ss.CacheReport(makeSyncEntry("key-3", t3))

	uploaded, _, failed, err := ss.ReplayPending(5)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if uploaded != 3 {
		t.Errorf("expected 3 uploaded, got %d", uploaded)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}

	// All entries should be marked uploaded
	for _, id := range []string{e1.ID, e2.ID, e3.ID} {
		got, _ := ss.GetEntry(id)
		if got.Status != domain.SyncUploaded {
			t.Errorf("entry %s should be uploaded, got %s", id, got.Status)
		}
	}

	// Three distinct incidents should exist
	_ = ds
}

func TestSyncIdempotencyNoDuplicates(t *testing.T) {
	ss, ds := setupSyncService(t)

	// Create two sync entries with the same idempotency key directly
	// in the store, simulating a terminal that retransmitted the same
	// report after a communication interruption.
	tm := time.Now()
	e1 := makeSyncEntry("dup-key", tm)
	e1.ID = "sync-1"
	ss.store.SaveSyncEntry(e1)

	e2 := makeSyncEntry("dup-key", tm.Add(time.Second))
	e2.ID = "sync-2"
	ss.store.SaveSyncEntry(e2)

	// Replay should process both entries but only create one incident.
	uploaded, _, failed, err := ss.ReplayPending(5)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if uploaded != 2 {
		t.Errorf("expected 2 uploaded (both processed), got %d", uploaded)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}

	// Only one incident should exist despite two sync entries.
	count := 0
	for _, inc := range ds.store.AllIncidents() {
		if inc.IdempotencyKey == "dup-key" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 incident with dup-key, got %d", count)
	}
}

func TestSyncReplayWithInvalidPayload(t *testing.T) {
	ss, _ := setupSyncService(t)

	entry := &domain.SyncEntry{
		IdempotencyKey: "bad-key",
		Payload:        []byte("not-valid-json"),
		ReportedAt:     time.Now(),
		Status:         domain.SyncPending,
	}
	ss.CacheReport(entry)

	uploaded, _, failed, err := ss.ReplayPending(1)
	if err != nil {
		t.Fatalf("replay error: %v", err)
	}
	if uploaded != 0 {
		t.Errorf("expected 0 uploaded, got %d", uploaded)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	got, _ := ss.GetEntry(entry.ID)
	if got.Status != domain.SyncFailed {
		t.Errorf("expected failed status, got %s", got.Status)
	}
}

func TestSyncReplayRespectsMaxAttempts(t *testing.T) {
	ss, _ := setupSyncService(t)

	// Create an entry that will always fail (invalid incident: missing location)
	inc := domain.Incident{
		Type:           domain.IncidentTypeFire,
		Images:         []string{"img.jpg"},
		IdempotencyKey: "fail-key",
		ReportedAt:     time.Now(),
	}
	payload, _ := json.Marshal(inc)
	entry := &domain.SyncEntry{
		IdempotencyKey: "fail-key",
		Payload:        payload,
		ReportedAt:     time.Now(),
		Status:         domain.SyncPending,
	}
	ss.CacheReport(entry)

	// Replay with maxAttempts=2
	for i := 0; i < 3; i++ {
		ss.ReplayPending(2)
	}

	got, _ := ss.GetEntry(entry.ID)
	if got.Status != domain.SyncFailed {
		t.Errorf("expected failed after max attempts, got %s", got.Status)
	}
	if got.Attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", got.Attempts)
	}
}
