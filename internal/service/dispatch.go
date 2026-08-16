package service

import (
	"fmt"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

// DispatchService orchestrates patrol task assignment, sign-in, trajectory
// reporting, and incident lifecycle management.
type DispatchService struct {
	store *store.Store
}

// NewDispatchService creates a dispatch service backed by the given store.
func NewDispatchService(s *store.Store) *DispatchService {
	return &DispatchService{store: s}
}

// AssignTask creates a patrol task assigning an officer to a grid.
// The dispatcher must be a registered dispatcher and the officer must be
// registered to the specified grid.
func (ds *DispatchService) AssignTask(dispatcherID, officerID, gridID string) (*domain.PatrolTask, error) {
	if !ds.store.IsDispatcher(dispatcherID) {
		return nil, fmt.Errorf("dispatcher %s is not registered", dispatcherID)
	}
	officerGrid, ok := ds.store.OfficerGrid(officerID)
	if !ok {
		return nil, fmt.Errorf("officer %s is not registered", officerID)
	}
	if officerGrid != gridID {
		return nil, fmt.Errorf("officer %s is responsible for grid %s, not %s", officerID, officerGrid, gridID)
	}
	task := &domain.PatrolTask{
		ID:           newID("task"),
		GridID:       gridID,
		OfficerID:    officerID,
		DispatcherID: dispatcherID,
		Status:       domain.PatrolTaskAssigned,
		AssignedAt:   time.Now(),
	}
	ds.store.SaveTask(task)
	return task, nil
}

// SignIn records a patrol officer signing in at their responsible grid.
// The officer must sign in at the grid they are assigned to; signing in
// at the wrong grid is rejected.
func (ds *DispatchService) SignIn(taskID, officerID, gridID string) error {
	task, ok := ds.store.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	return task.SignIn(officerID, gridID, time.Now())
}

// ConfirmMissedSignIn allows the assigning dispatcher to retroactively
// confirm a sign-in the officer failed to record.
func (ds *DispatchService) ConfirmMissedSignIn(taskID, dispatcherID string) error {
	task, ok := ds.store.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	return task.ConfirmMissedSignIn(dispatcherID, time.Now())
}

// StartPatrol transitions a signed-in task to the patrolling state.
func (ds *DispatchService) StartPatrol(taskID string) error {
	task, ok := ds.store.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	return task.StartPatrol()
}

// AddTrajectory appends a GPS reading to the task's trajectory.
func (ds *DispatchService) AddTrajectory(taskID string, point domain.TrajectoryPoint) error {
	task, ok := ds.store.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if point.At.IsZero() {
		point.At = time.Now()
	}
	return task.AddTrajectory(point)
}

// CompleteTask finalises a patrol task.
func (ds *DispatchService) CompleteTask(taskID string) error {
	task, ok := ds.store.GetTask(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	return task.Complete(time.Now())
}

// ReportIncident creates a new anomaly report.  The report must include
// a location and at least one image; reports missing either are rejected
// for correction.  If the idempotency key matches an existing incident,
// the existing incident is returned without re-processing.
func (ds *DispatchService) ReportIncident(inc *domain.Incident) (*domain.Incident, error) {
	if inc.IdempotencyKey != "" && ds.store.IncidentExistsByKey(inc.IdempotencyKey) {
		existing, _ := ds.findIncidentByKey(inc.IdempotencyKey)
		if existing != nil {
			return existing, nil
		}
	}
	if err := inc.Validate(); err != nil {
		return nil, err
	}
	if inc.ID == "" {
		inc.ID = newID("incident")
	}
	if inc.ReportedAt.IsZero() {
		inc.ReportedAt = time.Now()
	}
	if inc.Status == "" {
		inc.Status = domain.IncidentStatusReported
	}
	ds.store.SaveIncident(inc)
	return inc, nil
}

// ReviewIncident transitions an incident from reported to reviewed.
func (ds *DispatchService) ReviewIncident(incidentID, dispatcherID string) error {
	inc, ok := ds.store.GetIncident(incidentID)
	if !ok {
		return fmt.Errorf("incident %s not found", incidentID)
	}
	if !ds.store.IsDispatcher(dispatcherID) {
		return fmt.Errorf("dispatcher %s is not registered", dispatcherID)
	}
	return inc.Review(dispatcherID, time.Now())
}

// StartDispatch transitions a reviewed incident to dispatching.
func (ds *DispatchService) StartDispatch(incidentID string) error {
	inc, ok := ds.store.GetIncident(incidentID)
	if !ok {
		return fmt.Errorf("incident %s not found", incidentID)
	}
	return inc.StartDispatch()
}

// RespondIncident marks the incident as responding, satisfying the
// ten-minute high-risk response requirement.
func (ds *DispatchService) RespondIncident(incidentID string) error {
	inc, ok := ds.store.GetIncident(incidentID)
	if !ok {
		return fmt.Errorf("incident %s not found", incidentID)
	}
	return inc.Respond(time.Now())
}

// ResolveIncident closes out an incident after resolution.
func (ds *DispatchService) ResolveIncident(incidentID string) error {
	inc, ok := ds.store.GetIncident(incidentID)
	if !ok {
		return fmt.Errorf("incident %s not found", incidentID)
	}
	return inc.Resolve(time.Now())
}

// EscalateIncident auto-escalates an incident to the management bureau.
func (ds *DispatchService) EscalateIncident(incidentID, bureau string) error {
	inc, ok := ds.store.GetIncident(incidentID)
	if !ok {
		return fmt.Errorf("incident %s not found", incidentID)
	}
	return inc.Escalate(time.Now(), bureau)
}

func (ds *DispatchService) findIncidentByKey(key string) (*domain.Incident, bool) {
	for _, inc := range ds.store.AllIncidents() {
		if inc.IdempotencyKey == key {
			return inc, true
		}
	}
	return nil, false
}

// RegisterGrid registers a management grid with its centre location.
func (ds *DispatchService) RegisterGrid(id string, loc *domain.Location) {
	ds.store.RegisterGrid(id, loc)
}

// RegisterOfficer registers a patrol officer to a grid.
func (ds *DispatchService) RegisterOfficer(id, gridID string) error {
	return ds.store.RegisterOfficer(id, gridID)
}

// RegisterDispatcher registers a dispatcher who can assign tasks and
// review incidents.
func (ds *DispatchService) RegisterDispatcher(id string) {
	ds.store.RegisterDispatcher(id)
}

// GetTask retrieves a patrol task by ID.
func (ds *DispatchService) GetTask(id string) (*domain.PatrolTask, bool) {
	return ds.store.GetTask(id)
}

// GetIncident retrieves an incident by ID.
func (ds *DispatchService) GetIncident(id string) (*domain.Incident, bool) {
	return ds.store.GetIncident(id)
}

// ListHighRiskIncidents returns all high-risk incidents (for the
// escalation worker).
func (ds *DispatchService) ListHighRiskIncidents() []*domain.Incident {
	return ds.store.ListHighRiskIncidents()
}
