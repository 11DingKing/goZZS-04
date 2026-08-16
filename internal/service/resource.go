package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

// ResourceService manages emergency resource allocation with priority-based
// arbitration and concurrency-safe over-allocation rejection.
type ResourceService struct {
	store   *store.Store
	allocMu sync.Mutex // serialises all resource allocations
}

// NewResourceService creates a resource service backed by the given store.
func NewResourceService(s *store.Store) *ResourceService {
	return &ResourceService{store: s}
}

// CreateResource registers a new emergency resource.
func (rs *ResourceService) CreateResource(r *domain.Resource) error {
	if r.Capacity <= 0 {
		return fmt.Errorf("resource capacity must be positive")
	}
	if r.ID == "" {
		r.ID = newID("res")
	}
	r.Status = domain.ResourceStatusAvailable
	rs.store.SaveResource(r)
	return nil
}

// RequestResource creates a pending resource request tied to an incident.
// The request must be approved by a dispatcher before allocation.
func (rs *ResourceService) RequestResource(incidentID, resourceID, requestedBy string, quantity int, priority domain.Priority) (*domain.ResourceRequest, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if _, ok := rs.store.GetResource(resourceID); !ok {
		return nil, fmt.Errorf("resource %s not found", resourceID)
	}
	rr := &domain.ResourceRequest{
		ID:          newID("req"),
		IncidentID:  incidentID,
		ResourceID:  resourceID,
		Quantity:    quantity,
		Priority:    priority,
		RequestedAt: time.Now(),
		RequestedBy: requestedBy,
		Status:      domain.ResourceRequestPending,
	}
	rs.store.SaveResourceRequest(rr)
	return rr, nil
}

// ApproveRequest transitions a pending request to approved after dispatcher
// review.  Emergency resource dispatch may only proceed after approval.
func (rs *ResourceService) ApproveRequest(requestID, dispatcherID string) error {
	rr, ok := rs.store.GetResourceRequest(requestID)
	if !ok {
		return fmt.Errorf("resource request %s not found", requestID)
	}
	if !rs.store.IsDispatcher(dispatcherID) {
		return fmt.Errorf("dispatcher %s is not registered", dispatcherID)
	}
	return rr.Approve(dispatcherID)
}

// AllocateResource attempts to allocate a single approved request.
// Returns an error if capacity is insufficient (over-allocation rejected).
func (rs *ResourceService) AllocateResource(requestID string) error {
	rs.allocMu.Lock()
	defer rs.allocMu.Unlock()

	rr, ok := rs.store.GetResourceRequest(requestID)
	if !ok {
		return fmt.Errorf("resource request %s not found", requestID)
	}
	if rr.Status != domain.ResourceRequestApproved {
		return fmt.Errorf("request %s is %s, must be approved", requestID, rr.Status)
	}
	r, ok := rs.store.GetResource(rr.ResourceID)
	if !ok {
		return fmt.Errorf("resource %s not found", rr.ResourceID)
	}
	now := time.Now()
	if r.Available() < rr.Quantity {
		_ = rr.Reject(fmt.Sprintf("over-allocation: need %d, available %d", rr.Quantity, r.Available()))
		rs.store.SaveResourceRequest(rr)
		return fmt.Errorf("over-allocation rejected: need %d, available %d", rr.Quantity, r.Available())
	}
	if err := rr.Allocate(r, now); err != nil {
		return err
	}
	rs.store.SaveResource(r)
	rs.store.SaveResourceRequest(rr)
	return nil
}

// ArbitrateResource processes all approved requests for a resource in
// priority-then-time order, allocating until capacity is exhausted and
// rejecting the remainder.  This implements the arbitration rule for
// competing simultaneous requests.
func (rs *ResourceService) ArbitrateResource(resourceID string) (allocated, rejected int, err error) {
	rs.allocMu.Lock()
	defer rs.allocMu.Unlock()

	r, ok := rs.store.GetResource(resourceID)
	if !ok {
		return 0, 0, fmt.Errorf("resource %s not found", resourceID)
	}
	pending := rs.store.PendingRequestsForResource(resourceID)
	domain.SortRequestsByPriorityThenTime(pending)
	now := time.Now()
	for _, rr := range pending {
		if r.Available() >= rr.Quantity {
			if err := rr.Allocate(r, now); err != nil {
				continue
			}
			allocated++
		} else {
			_ = rr.Reject("over-allocation: capacity exhausted by higher-priority requests")
			rejected++
		}
		rs.store.SaveResourceRequest(rr)
	}
	rs.store.SaveResource(r)
	return allocated, rejected, nil
}

// ReleaseResource returns allocated units to the pool.
func (rs *ResourceService) ReleaseResource(requestID string) error {
	rs.allocMu.Lock()

	rr, ok := rs.store.GetResourceRequest(requestID)
	if !ok {
		return fmt.Errorf("resource request %s not found", requestID)
	}
	r, ok := rs.store.GetResource(rr.ResourceID)
	if !ok {
		return fmt.Errorf("resource %s not found", rr.ResourceID)
	}
	if err := rr.Release(r); err != nil {
		return err
	}
	rs.store.SaveResource(r)
	rs.store.SaveResourceRequest(rr)
	rs.allocMu.Unlock()
	return nil
}
