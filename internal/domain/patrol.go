package domain

import (
	"fmt"
	"time"
)

// PatrolTaskStatus represents the lifecycle state of a patrol task.
type PatrolTaskStatus string

const (
	PatrolTaskAssigned   PatrolTaskStatus = "assigned"
	PatrolTaskSignedIn   PatrolTaskStatus = "signed_in"
	PatrolTaskPatrolling PatrolTaskStatus = "patrolling"
	PatrolTaskCompleted  PatrolTaskStatus = "completed"
)

// TrajectoryPoint is a single GPS reading collected during a patrol.
type TrajectoryPoint struct {
	Lat float64   `json:"lat"`
	Lng float64   `json:"lng"`
	At  time.Time `json:"at"`
}

// PatrolTask is the aggregate root for a daily patrol assignment.
type PatrolTask struct {
	ID           string            `json:"id"`
	GridID       string            `json:"grid_id"`
	OfficerID    string            `json:"officer_id"`
	DispatcherID string            `json:"dispatcher_id"`
	Status       PatrolTaskStatus  `json:"status"`
	AssignedAt   time.Time         `json:"assigned_at"`
	SignedInAt   *time.Time        `json:"signed_in_at,omitempty"`
	MissedSignIn bool              `json:"missed_sign_in"`
	ConfirmedBy  string            `json:"confirmed_by,omitempty"`
	Trajectory   []TrajectoryPoint `json:"trajectory,omitempty"`
	Images       []string          `json:"images,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
}

// SignIn records the patrol officer signing in at their responsible grid.
// The officer must be the assignee and must be physically at the assigned grid.
func (t *PatrolTask) SignIn(officerID, gridID string, now time.Time) error {
	if t.Status != PatrolTaskAssigned {
		return fmt.Errorf("cannot sign in: task status is %s, expected %s", t.Status, PatrolTaskAssigned)
	}
	if t.OfficerID != officerID {
		return fmt.Errorf("officer %s is not assigned to task %s", officerID, t.ID)
	}
	if t.GridID != gridID {
		return fmt.Errorf("officer must sign in at grid %s, attempted grid %s", t.GridID, gridID)
	}
	t.Status = PatrolTaskSignedIn
	t.SignedInAt = &now
	return nil
}

// ConfirmMissedSignIn allows the assigning dispatcher to retroactively
// confirm a sign-in that the patrol officer failed to record.
func (t *PatrolTask) ConfirmMissedSignIn(dispatcherID string, now time.Time) error {
	if t.Status != PatrolTaskAssigned {
		return fmt.Errorf("cannot confirm missed sign-in: task status is %s, expected %s", t.Status, PatrolTaskAssigned)
	}
	if t.DispatcherID != dispatcherID {
		return fmt.Errorf("only the assigning dispatcher %s can confirm missed sign-in", t.DispatcherID)
	}
	t.Status = PatrolTaskSignedIn
	t.MissedSignIn = true
	t.ConfirmedBy = dispatcherID
	t.SignedInAt = &now
	return nil
}

// StartPatrol transitions a signed-in task to the patrolling state.
func (t *PatrolTask) StartPatrol() error {
	if t.Status != PatrolTaskSignedIn {
		return fmt.Errorf("cannot start patrol: task status is %s, expected %s", t.Status, PatrolTaskSignedIn)
	}
	t.Status = PatrolTaskPatrolling
	return nil
}

// AddTrajectory appends a trajectory point during an active patrol.
func (t *PatrolTask) AddTrajectory(point TrajectoryPoint) error {
	if t.Status != PatrolTaskPatrolling && t.Status != PatrolTaskSignedIn {
		return fmt.Errorf("cannot add trajectory: task status is %s", t.Status)
	}
	if point.At.IsZero() {
		return fmt.Errorf("trajectory point timestamp is required")
	}
	t.Trajectory = append(t.Trajectory, point)
	if t.Status == PatrolTaskSignedIn {
		t.Status = PatrolTaskPatrolling
	}
	return nil
}

// Complete finalises the patrol task after the officer finishes their route.
func (t *PatrolTask) Complete(now time.Time) error {
	if t.Status != PatrolTaskPatrolling && t.Status != PatrolTaskSignedIn {
		return fmt.Errorf("cannot complete: task status is %s", t.Status)
	}
	t.Status = PatrolTaskCompleted
	t.CompletedAt = &now
	return nil
}
