package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

// MaterialService manages material outbound operations with dual-signature
// confirmation and priority-based stock arbitration.
type MaterialService struct {
	store *store.Store
	outMu sync.Mutex // serialises all material outbound completions
}

// NewMaterialService creates a material service backed by the given store.
func NewMaterialService(s *store.Store) *MaterialService {
	return &MaterialService{store: s}
}

// CreateMaterial registers a new consumable material in the warehouse.
func (ms *MaterialService) CreateMaterial(m *domain.Material) error {
	if m.Stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}
	if m.ID == "" {
		m.ID = newID("mat")
	}
	ms.store.SaveMaterial(m)
	return nil
}

// CreateRequest creates a material outbound request with the requester's
// signature.  The requester must sign before the material manager can
// co-sign and complete the outbound.
func (ms *MaterialService) CreateRequest(requesterID string, items []domain.MaterialItem, priority domain.Priority, incidentID string) (*domain.MaterialRequest, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("material request must contain at least one item")
	}
	for _, item := range items {
		if _, ok := ms.store.GetMaterial(item.MaterialID); !ok {
			return nil, fmt.Errorf("material %s not found", item.MaterialID)
		}
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("quantity for material %s must be positive", item.MaterialID)
		}
	}
	mr := &domain.MaterialRequest{
		ID:          newID("matreq"),
		IncidentID:  incidentID,
		Items:       items,
		RequesterID: requesterID,
		Priority:    priority,
		Status:      domain.MaterialRequested,
		RequestedAt: time.Now(),
	}
	if err := mr.RequesterSign(time.Now()); err != nil {
		return nil, err
	}
	ms.store.SaveMaterialRequest(mr)
	return mr, nil
}

// ManagerSign completes the dual-signature outbound.  Both the requester
// and the material manager must have signed.  Stock is decremented
// atomically under the outbound mutex.
func (ms *MaterialService) ManagerSign(requestID, managerID string) error {
	ms.outMu.Lock()
	defer ms.outMu.Unlock()

	mr, ok := ms.store.GetMaterialRequest(requestID)
	if !ok {
		return fmt.Errorf("material request %s not found", requestID)
	}
	if !ms.store.IsManager(managerID) {
		return fmt.Errorf("material manager %s is not registered", managerID)
	}
	materials := make(map[string]*domain.Material)
	for _, item := range mr.Items {
		mat, ok := ms.store.GetMaterial(item.MaterialID)
		if !ok {
			return fmt.Errorf("material %s not found", item.MaterialID)
		}
		materials[item.MaterialID] = mat
	}
	if err := mr.ManagerSign(managerID, materials, time.Now()); err != nil {
		return err
	}
	for _, mat := range materials {
		ms.store.SaveMaterial(mat)
	}
	ms.store.SaveMaterialRequest(mr)
	return nil
}

// ArbitrateMaterials processes all pending requester-signed material
// requests in priority-then-time order, completing those with sufficient
// stock and rejecting the remainder as over-allocation.
func (ms *MaterialService) ArbitrateMaterials(managerID string) (completed, rejected int, err error) {
	ms.outMu.Lock()
	defer ms.outMu.Unlock()

	pending := ms.store.PendingMaterialRequests()
	domain.SortMaterialRequestsByPriorityThenTime(pending)
	now := time.Now()
	for _, mr := range pending {
		materials := make(map[string]*domain.Material)
		allocatable := true
		for _, item := range mr.Items {
			mat, ok := ms.store.GetMaterial(item.MaterialID)
			if !ok {
				allocatable = false
				break
			}
			materials[item.MaterialID] = mat
		}
		if !allocatable {
			_ = mr.Reject(managerID, "material not found")
			rejected++
			ms.store.SaveMaterialRequest(mr)
			continue
		}
		// Check stock availability across all items.
		stockOK := true
		for _, item := range mr.Items {
			if materials[item.MaterialID].Stock < item.Quantity {
				stockOK = false
				break
			}
		}
		if stockOK {
			if err := mr.ManagerSign(managerID, materials, now); err != nil {
				continue
			}
			completed++
		} else {
			_ = mr.Reject(managerID, "over-allocation: insufficient stock")
			rejected++
		}
		ms.store.SaveMaterialRequest(mr)
		for _, mat := range materials {
			ms.store.SaveMaterial(mat)
		}
	}
	return completed, rejected, nil
}

// RejectRequest cancels a material request.
func (ms *MaterialService) RejectRequest(requestID, managerID, reason string) error {
	mr, ok := ms.store.GetMaterialRequest(requestID)
	if !ok {
		return fmt.Errorf("material request %s not found", requestID)
	}
	if !ms.store.IsManager(managerID) {
		return fmt.Errorf("material manager %s is not registered", managerID)
	}
	return mr.Reject(managerID, reason)
}

// RegisterManager registers a material manager who can co-sign outbound.
func (ms *MaterialService) RegisterManager(id string) {
	ms.store.RegisterManager(id)
}
