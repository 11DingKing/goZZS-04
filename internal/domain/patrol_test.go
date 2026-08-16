package domain

import (
	"testing"
	"time"
)

func newAssignedTask() *PatrolTask {
	return &PatrolTask{
		ID:           "task-1",
		GridID:       "grid-A",
		OfficerID:    "officer-1",
		DispatcherID: "disp-1",
		Status:       PatrolTaskAssigned,
		AssignedAt:   time.Now(),
	}
}

func TestPatrolSignInHappyPath(t *testing.T) {
	task := newAssignedTask()
	now := time.Now()
	if err := task.SignIn("officer-1", "grid-A", now); err != nil {
		t.Fatalf("sign in failed: %v", err)
	}
	if task.Status != PatrolTaskSignedIn {
		t.Errorf("expected status %s, got %s", PatrolTaskSignedIn, task.Status)
	}
	if task.SignedInAt == nil || !task.SignedInAt.Equal(now) {
		t.Errorf("signed in at not set correctly")
	}
	if task.MissedSignIn {
		t.Error("should not be marked as missed")
	}
}

func TestPatrolSignInWrongGrid(t *testing.T) {
	task := newAssignedTask()
	err := task.SignIn("officer-1", "grid-B", time.Now())
	if err == nil {
		t.Fatal("expected error signing in at wrong grid")
	}
}

func TestPatrolSignInWrongOfficer(t *testing.T) {
	task := newAssignedTask()
	err := task.SignIn("officer-2", "grid-A", time.Now())
	if err == nil {
		t.Fatal("expected error signing in as wrong officer")
	}
}

func TestPatrolConfirmMissedSignIn(t *testing.T) {
	task := newAssignedTask()
	if err := task.ConfirmMissedSignIn("disp-1", time.Now()); err != nil {
		t.Fatalf("confirm missed sign-in failed: %v", err)
	}
	if task.Status != PatrolTaskSignedIn {
		t.Errorf("expected status %s, got %s", PatrolTaskSignedIn, task.Status)
	}
	if !task.MissedSignIn {
		t.Error("should be marked as missed")
	}
	if task.ConfirmedBy != "disp-1" {
		t.Errorf("expected confirmed_by disp-1, got %s", task.ConfirmedBy)
	}
}

func TestPatrolConfirmMissedSignInWrongDispatcher(t *testing.T) {
	task := newAssignedTask()
	err := task.ConfirmMissedSignIn("disp-2", time.Now())
	if err == nil {
		t.Fatal("expected error: only assigning dispatcher can confirm")
	}
}

func TestPatrolFullLifecycle(t *testing.T) {
	task := newAssignedTask()
	now := time.Now()

	if err := task.SignIn("officer-1", "grid-A", now); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if err := task.StartPatrol(); err != nil {
		t.Fatalf("start patrol: %v", err)
	}
	if task.Status != PatrolTaskPatrolling {
		t.Fatalf("expected patrolling, got %s", task.Status)
	}
	if err := task.AddTrajectory(TrajectoryPoint{Lat: 30.1, Lng: 120.2, At: now}); err != nil {
		t.Fatalf("add trajectory: %v", err)
	}
	if len(task.Trajectory) != 1 {
		t.Errorf("expected 1 trajectory point, got %d", len(task.Trajectory))
	}
	if err := task.Complete(now); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if task.Status != PatrolTaskCompleted {
		t.Errorf("expected completed, got %s", task.Status)
	}
}

func TestPatrolCompleteFromAssignedFails(t *testing.T) {
	task := newAssignedTask()
	err := task.Complete(time.Now())
	if err == nil {
		t.Fatal("expected error completing from assigned state")
	}
}
