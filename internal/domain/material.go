package domain

import (
	"fmt"
	"sort"
	"time"
)

// Material is a consumable supply tracked in the reserve warehouse.
type Material struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Stock int    `json:"stock"`
}

// MaterialRequestStatus tracks the dual-signature outbound flow.
type MaterialRequestStatus string

const (
	MaterialRequested     MaterialRequestStatus = "requested"
	MaterialManagerSigned MaterialRequestStatus = "manager_signed"
	MaterialCompleted     MaterialRequestStatus = "completed"
	MaterialRejected      MaterialRequestStatus = "rejected"
)

// MaterialItem is one line in an outbound request.
type MaterialItem struct {
	MaterialID string `json:"material_id"`
	Quantity   int    `json:"quantity"`
}

// MaterialRequest is the aggregate root for a material outbound operation.
// Completion requires signatures from both the requesting officer and the
// material manager.  Priority is derived from the associated incident so
// that competing requests can be arbitrated.
type MaterialRequest struct {
	ID              string                `json:"id"`
	IncidentID      string                `json:"incident_id,omitempty"`
	Items           []MaterialItem        `json:"items"`
	RequesterID     string                `json:"requester_id"`
	ManagerID       string                `json:"manager_id,omitempty"`
	Priority        Priority              `json:"priority"`
	Status          MaterialRequestStatus `json:"status"`
	RequestedAt     time.Time             `json:"requested_at"`
	RequesterSigned bool                  `json:"requester_signed"`
	ManagerSigned   bool                  `json:"manager_signed"`
	CompletedAt     *time.Time            `json:"completed_at,omitempty"`
	RejectedReason  string                `json:"rejected_reason,omitempty"`
}

// RequesterSign records the requesting officer's signature on the outbound
// request.  This must happen before the material manager can co-sign.
func (m *MaterialRequest) RequesterSign(now time.Time) error {
	if m.Status != MaterialRequested {
		return fmt.Errorf("cannot sign: request status is %s, expected %s", m.Status, MaterialRequested)
	}
	if m.RequesterSigned {
		return fmt.Errorf("requester has already signed")
	}
	m.RequesterSigned = true
	return nil
}

// ManagerSign records the material manager's signature.  When both parties
// have signed the request is completed and stock is decremented atomically.
func (m *MaterialRequest) ManagerSign(managerID string, materials map[string]*Material, now time.Time) error {
	if m.Status != MaterialRequested {
		return fmt.Errorf("cannot co-sign: request status is %s, expected %s", m.Status, MaterialRequested)
	}
	if !m.RequesterSigned {
		return fmt.Errorf("requester must sign before material manager")
	}
	if m.ManagerSigned {
		return fmt.Errorf("material manager has already signed")
	}
	for _, item := range m.Items {
		mat, ok := materials[item.MaterialID]
		if !ok {
			return fmt.Errorf("material %s not found", item.MaterialID)
		}
		if mat.Stock < item.Quantity {
			return fmt.Errorf("insufficient stock for material %s: need %d, have %d", mat.Name, item.Quantity, mat.Stock)
		}
	}
	for _, item := range m.Items {
		materials[item.MaterialID].Stock -= item.Quantity
	}
	m.ManagerID = managerID
	m.ManagerSigned = true
	m.Status = MaterialCompleted
	m.CompletedAt = &now
	return nil
}

// Reject cancels a material request that cannot be fulfilled.
func (m *MaterialRequest) Reject(managerID, reason string) error {
	if m.Status != MaterialRequested {
		return fmt.Errorf("cannot reject: request status is %s", m.Status)
	}
	m.ManagerID = managerID
	m.Status = MaterialRejected
	m.RejectedReason = reason
	return nil
}

// SortMaterialRequestsByPriorityThenTime orders requests by descending
// priority then ascending request time for arbitration.
func SortMaterialRequestsByPriorityThenTime(reqs []*MaterialRequest) {
	sort.SliceStable(reqs, func(i, j int) bool {
		if reqs[i].Priority != reqs[j].Priority {
			return reqs[i].Priority > reqs[j].Priority
		}
		return reqs[i].RequestedAt.Before(reqs[j].RequestedAt)
	})
}
