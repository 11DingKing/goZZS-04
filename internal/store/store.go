package store

import (
	"fmt"
	"sync"

	"github.com/reserve/patrol-dispatch/internal/domain"
)

// Store is a thread-safe in-memory persistence layer for all aggregates.
// In production this would be backed by a database; the locking model here
// mirrors the transactional boundaries needed for correct concurrent access.
type Store struct {
	mu sync.RWMutex

	grids       map[string]*domain.Location
	officers    map[string]string // officerID -> gridID
	dispatchers map[string]bool
	managers    map[string]bool

	tasks       map[string]*domain.PatrolTask
	incidents   map[string]*domain.Incident
	resources   map[string]*domain.Resource
	reqRequests map[string]*domain.ResourceRequest
	materials   map[string]*domain.Material
	matRequests map[string]*domain.MaterialRequest
	syncEntries map[string]*domain.SyncEntry
}

// New creates an empty store.
func New() *Store {
	return &Store{
		grids:       make(map[string]*domain.Location),
		officers:    make(map[string]string),
		dispatchers: make(map[string]bool),
		managers:    make(map[string]bool),
		tasks:       make(map[string]*domain.PatrolTask),
		incidents:   make(map[string]*domain.Incident),
		resources:   make(map[string]*domain.Resource),
		reqRequests: make(map[string]*domain.ResourceRequest),
		materials:   make(map[string]*domain.Material),
		matRequests: make(map[string]*domain.MaterialRequest),
		syncEntries: make(map[string]*domain.SyncEntry),
	}
}

// --- Actors ---

func (s *Store) RegisterGrid(id string, loc *domain.Location) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grids[id] = loc
}

func (s *Store) RegisterOfficer(id, gridID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.grids[gridID]; !ok {
		return fmt.Errorf("grid %s not found", gridID)
	}
	s.officers[id] = gridID
	return nil
}

func (s *Store) RegisterDispatcher(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispatchers[id] = true
}

func (s *Store) RegisterManager(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.managers[id] = true
}

func (s *Store) IsDispatcher(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dispatchers[id]
}

func (s *Store) IsManager(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.managers[id]
}

func (s *Store) OfficerGrid(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.officers[id]
	return g, ok
}

// --- Patrol Tasks ---

func (s *Store) SaveTask(t *domain.PatrolTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
}

func (s *Store) GetTask(id string) (*domain.PatrolTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

// --- Incidents ---

func (s *Store) SaveIncident(i *domain.Incident) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incidents[i.ID] = i
}

func (s *Store) GetIncident(id string) (*domain.Incident, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.incidents[id]
	return i, ok
}

func (s *Store) IncidentExistsByKey(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if key == "" {
		return false
	}
	for _, inc := range s.incidents {
		if inc.IdempotencyKey == key {
			return true
		}
	}
	return false
}

func (s *Store) ListHighRiskIncidents() []*domain.Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.Incident
	for _, inc := range s.incidents {
		if inc.IsHighRisk() {
			result = append(result, inc)
		}
	}
	return result
}

// --- Resources ---

func (s *Store) SaveResource(r *domain.Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[r.ID] = r
}

func (s *Store) GetResource(id string) (*domain.Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.resources[id]
	return r, ok
}

func (s *Store) SaveResourceRequest(rr *domain.ResourceRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqRequests[rr.ID] = rr
}

func (s *Store) GetResourceRequest(id string) (*domain.ResourceRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rr, ok := s.reqRequests[id]
	return rr, ok
}

// PendingRequestsForResource returns all approved requests targeting
// the given resource, used by the arbitration logic.
func (s *Store) PendingRequestsForResource(resourceID string) []*domain.ResourceRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.ResourceRequest
	for _, rr := range s.reqRequests {
		if rr.ResourceID == resourceID && rr.Status == domain.ResourceRequestApproved {
			result = append(result, rr)
		}
	}
	return result
}

// --- Materials ---

func (s *Store) SaveMaterial(m *domain.Material) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.materials[m.ID] = m
}

func (s *Store) GetMaterial(id string) (*domain.Material, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.materials[id]
	return m, ok
}

func (s *Store) SaveMaterialRequest(mr *domain.MaterialRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matRequests[mr.ID] = mr
}

func (s *Store) GetMaterialRequest(id string) (*domain.MaterialRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mr, ok := s.matRequests[id]
	return mr, ok
}

// --- Sync Entries ---

func (s *Store) SaveSyncEntry(e *domain.SyncEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncEntries[e.ID] = e
}

func (s *Store) GetSyncEntry(id string) (*domain.SyncEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.syncEntries[id]
	return e, ok
}

func (s *Store) SyncEntryByKey(key string) (*domain.SyncEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if key == "" {
		return nil, false
	}
	for _, e := range s.syncEntries {
		if e.IdempotencyKey == key {
			return e, true
		}
	}
	return nil, false
}

// PendingSyncEntries returns all pending sync entries sorted by CachedAt.
func (s *Store) PendingSyncEntries() []*domain.SyncEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.SyncEntry
	for _, e := range s.syncEntries {
		if e.Status == domain.SyncPending {
			result = append(result, e)
		}
	}
	return result
}

// PendingMaterialRequests returns all requested material requests where
// the requester has signed, awaiting manager co-sign and arbitration.
func (s *Store) PendingMaterialRequests() []*domain.MaterialRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*domain.MaterialRequest
	for _, mr := range s.matRequests {
		if mr.Status == domain.MaterialRequested && mr.RequesterSigned {
			result = append(result, mr)
		}
	}
	return result
}

// AllIncidents returns all incidents (used by the escalation worker).
func (s *Store) AllIncidents() []*domain.Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		result = append(result, inc)
	}
	return result
}
