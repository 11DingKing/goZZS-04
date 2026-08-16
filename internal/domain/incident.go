package domain

import (
	"fmt"
	"time"
)

// IncidentType categorises the kind of anomaly reported in the field.
type IncidentType string

const (
	IncidentTypeFire     IncidentType = "fire"
	IncidentTypePoaching IncidentType = "poaching"
	IncidentTypeBoundary IncidentType = "boundary_violation"
)

// IncidentStatus tracks the lifecycle of an anomaly from report to resolution.
type IncidentStatus string

const (
	IncidentStatusReported    IncidentStatus = "reported"
	IncidentStatusReviewed    IncidentStatus = "reviewed"
	IncidentStatusDispatching IncidentStatus = "dispatching"
	IncidentStatusResponding  IncidentStatus = "responding"
	IncidentStatusResolved    IncidentStatus = "resolved"
	IncidentStatusEscalated   IncidentStatus = "escalated"
)

// Priority is used to arbitrate competing resource requests.
type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Location is a GPS coordinate attached to an incident or trajectory.
type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Incident is the aggregate root for field-reported anomalies.
type Incident struct {
	ID             string         `json:"id"`
	Type           IncidentType   `json:"type"`
	Location       *Location      `json:"location,omitempty"`
	Images         []string       `json:"images,omitempty"`
	Description    string         `json:"description,omitempty"`
	Priority       Priority       `json:"priority"`
	Status         IncidentStatus `json:"status"`
	ReportedAt     time.Time      `json:"reported_at"`
	ReportedBy     string         `json:"reported_by"`
	TaskID         string         `json:"task_id,omitempty"`
	ReviewedBy     string         `json:"reviewed_by,omitempty"`
	RespondedAt    *time.Time     `json:"responded_at,omitempty"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty"`
	EscalatedAt    *time.Time     `json:"escalated_at,omitempty"`
	EscalatedTo    string         `json:"escalated_to,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}

// Validate ensures that an anomaly report has the mandatory location and imagery.
// Reports missing either are returned to the patrol officer for correction.
func (i *Incident) Validate() error {
	if i.Location == nil {
		return fmt.Errorf("anomaly report requires a location")
	}
	if i.Location.Lat == 0 && i.Location.Lng == 0 {
		return fmt.Errorf("anomaly report location coordinates are required")
	}
	if len(i.Images) == 0 {
		return fmt.Errorf("anomaly report requires at least one image as evidence")
	}
	return nil
}

// IsHighRisk returns true for fire incidents and high-or-above priority events
// that are subject to the ten-minute response escalation rule.
func (i *Incident) IsHighRisk() bool {
	return i.Type == IncidentTypeFire || i.Priority >= PriorityHigh
}

// Review transitions an incident from reported to reviewed after dispatcher
// verification of the attached location and imagery.
func (i *Incident) Review(dispatcherID string, now time.Time) error {
	if i.Status != IncidentStatusReported {
		return fmt.Errorf("cannot review: incident status is %s, expected %s", i.Status, IncidentStatusReported)
	}
	if err := i.Validate(); err != nil {
		return fmt.Errorf("cannot review: %w", err)
	}
	i.Status = IncidentStatusReviewed
	i.ReviewedBy = dispatcherID
	return nil
}

// StartDispatch transitions a reviewed incident to the dispatching phase
// where emergency resources are being allocated.
func (i *Incident) StartDispatch() error {
	if i.Status != IncidentStatusReviewed {
		return fmt.Errorf("cannot dispatch: incident status is %s, expected %s", i.Status, IncidentStatusReviewed)
	}
	i.Status = IncidentStatusDispatching
	return nil
}

// Respond marks the moment emergency resources arrive on scene, satisfying
// the ten-minute high-risk response requirement.
func (i *Incident) Respond(now time.Time) error {
	if i.Status != IncidentStatusDispatching && i.Status != IncidentStatusReviewed {
		return fmt.Errorf("cannot respond: incident status is %s", i.Status)
	}
	i.Status = IncidentStatusResponding
	i.RespondedAt = &now
	return nil
}

// Resolve closes out an incident after the situation is handled.
func (i *Incident) Resolve(now time.Time) error {
	if i.Status != IncidentStatusResponding {
		return fmt.Errorf("cannot resolve: incident status is %s, expected %s", i.Status, IncidentStatusResponding)
	}
	i.Status = IncidentStatusResolved
	i.ResolvedAt = &now
	return nil
}

// Escalate auto-escalates a high-risk incident that exceeded the response
// timeout to the reserve management bureau.
func (i *Incident) Escalate(now time.Time, bureau string) error {
	if i.Status == IncidentStatusResolved || i.Status == IncidentStatusEscalated {
		return fmt.Errorf("incident already %s", i.Status)
	}
	if !i.IsHighRisk() {
		return fmt.Errorf("only high-risk incidents can be auto-escalated")
	}
	i.Status = IncidentStatusEscalated
	i.EscalatedAt = &now
	i.EscalatedTo = bureau
	return nil
}

// ShouldEscalate checks whether a high-risk incident has exceeded the
// configured response timeout without being marked as responding.
func (i *Incident) ShouldEscalate(now time.Time, timeout time.Duration) bool {
	if !i.IsHighRisk() {
		return false
	}
	if i.Status == IncidentStatusResponding || i.Status == IncidentStatusResolved || i.Status == IncidentStatusEscalated {
		return false
	}
	return now.Sub(i.ReportedAt) >= timeout
}
