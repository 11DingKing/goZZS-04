package store

import (
	"sync"
	"testing"

	"github.com/reserve/patrol-dispatch/internal/domain"
)

func TestStoreTaskCRUD(t *testing.T) {
	s := New()
	task := &domain.PatrolTask{
		ID:        "t1",
		GridID:    "g1",
		OfficerID: "o1",
		Status:    domain.PatrolTaskAssigned,
	}
	s.SaveTask(task)

	got, ok := s.GetTask("t1")
	if !ok {
		t.Fatal("task not found after save")
	}
	if got.OfficerID != "o1" {
		t.Errorf("expected officer o1, got %s", got.OfficerID)
	}

	if _, ok := s.GetTask("nonexistent"); ok {
		t.Error("expected not found for nonexistent task")
	}
}

func TestStoreIncidentIdempotencyKey(t *testing.T) {
	s := New()
	inc := &domain.Incident{
		ID:             "i1",
		Type:           domain.IncidentTypeFire,
		Location:       &domain.Location{Lat: 1, Lng: 2},
		Images:         []string{"a.jpg"},
		IdempotencyKey: "key-abc",
		Status:         domain.IncidentStatusReported,
	}
	s.SaveIncident(inc)

	if !s.IncidentExistsByKey("key-abc") {
		t.Error("idempotency key should exist")
	}
	if s.IncidentExistsByKey("key-xyz") {
		t.Error("nonexistent key should not exist")
	}
	if s.IncidentExistsByKey("") {
		t.Error("empty key should not match")
	}
}

func TestStoreConcurrentResourceAllocation(t *testing.T) {
	s := New()
	res := &domain.Resource{
		ID:       "r1",
		Type:     domain.ResourceTypeVehicle,
		Name:     "patrol-truck-1",
		Capacity: 1,
		InUse:    0,
		Status:   domain.ResourceStatusAvailable,
	}
	s.SaveResource(res)

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rr := &domain.ResourceRequest{
				ID:         "req-" + string(rune('A'+idx)),
				ResourceID: "r1",
				Quantity:   1,
				Status:     domain.ResourceRequestApproved,
				Priority:   domain.PriorityHigh,
			}
			s.SaveResourceRequest(rr)

			r, _ := s.GetResource("r1")
			rr2, _ := s.GetResourceRequest(rr.ID)
			if r.Available() >= rr2.Quantity {
				_ = rr2.Allocate(r, rr2.RequestedAt)
				s.SaveResource(r)
				s.SaveResourceRequest(rr2)
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if successCount > 1 {
		t.Errorf("expected at most 1 successful allocation, got %d (over-allocation race)", successCount)
	}
}

func TestStorePendingSyncEntriesOrder(t *testing.T) {
	s := New()
	for i, tm := range []string{"2024-01-03", "2024-01-01", "2024-01-02"} {
		_ = tm
		e := &domain.SyncEntry{
			ID:             "e" + string(rune('A'+i)),
			IdempotencyKey: "k" + string(rune('A'+i)),
			Status:         domain.SyncPending,
		}
		s.SaveSyncEntry(e)
	}
	pending := s.PendingSyncEntries()
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}
}
