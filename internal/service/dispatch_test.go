package service

import (
	"testing"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

func setupDispatchService(t *testing.T) (*DispatchService, *store.Store) {
	t.Helper()
	s := store.New()
	ds := NewDispatchService(s)
	ds.RegisterGrid("grid-A", &domain.Location{Lat: 30.0, Lng: 120.0})
	ds.RegisterGrid("grid-B", &domain.Location{Lat: 31.0, Lng: 121.0})
	if err := ds.RegisterOfficer("officer-1", "grid-A"); err != nil {
		t.Fatalf("register officer: %v", err)
	}
	ds.RegisterDispatcher("disp-1")
	return ds, s
}

func TestDispatchAssignTaskAndSignIn(t *testing.T) {
	ds, _ := setupDispatchService(t)

	task, err := ds.AssignTask("disp-1", "officer-1", "grid-A")
	if err != nil {
		t.Fatalf("assign task: %v", err)
	}
	if task.Status != domain.PatrolTaskAssigned {
		t.Errorf("expected assigned, got %s", task.Status)
	}

	if err := ds.SignIn(task.ID, "officer-1", "grid-A"); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	got, _ := ds.GetTask(task.ID)
	if got.Status != domain.PatrolTaskSignedIn {
		t.Errorf("expected signed_in, got %s", got.Status)
	}
}

func TestDispatchSignInWrongGrid(t *testing.T) {
	ds, _ := setupDispatchService(t)
	task, _ := ds.AssignTask("disp-1", "officer-1", "grid-A")

	err := ds.SignIn(task.ID, "officer-1", "grid-B")
	if err == nil {
		t.Fatal("expected error signing in at wrong grid")
	}
}

func TestDispatchConfirmMissedSignIn(t *testing.T) {
	ds, _ := setupDispatchService(t)
	task, _ := ds.AssignTask("disp-1", "officer-1", "grid-A")

	if err := ds.ConfirmMissedSignIn(task.ID, "disp-1"); err != nil {
		t.Fatalf("confirm missed: %v", err)
	}
	got, _ := ds.GetTask(task.ID)
	if !got.MissedSignIn {
		t.Error("expected missed_sign_in to be true")
	}
}

func TestDispatchAssignTaskUnregisteredDispatcher(t *testing.T) {
	ds, _ := setupDispatchService(t)
	_, err := ds.AssignTask("unknown-disp", "officer-1", "grid-A")
	if err == nil {
		t.Fatal("expected error for unregistered dispatcher")
	}
}

func TestDispatchReportIncidentValidation(t *testing.T) {
	ds, _ := setupDispatchService(t)

	// Missing location
	inc := &domain.Incident{
		Type:   domain.IncidentTypeFire,
		Images: []string{"img1.jpg"},
	}
	if _, err := ds.ReportIncident(inc); err == nil {
		t.Fatal("expected error for missing location")
	}

	// Missing images
	inc2 := &domain.Incident{
		Type:     domain.IncidentTypeFire,
		Location: &domain.Location{Lat: 30, Lng: 120},
	}
	if _, err := ds.ReportIncident(inc2); err == nil {
		t.Fatal("expected error for missing images")
	}

	// Valid
	inc3 := &domain.Incident{
		Type:     domain.IncidentTypeFire,
		Location: &domain.Location{Lat: 30, Lng: 120},
		Images:   []string{"img1.jpg"},
	}
	result, err := ds.ReportIncident(inc3)
	if err != nil {
		t.Fatalf("report incident: %v", err)
	}
	if result.Status != domain.IncidentStatusReported {
		t.Errorf("expected reported, got %s", result.Status)
	}
}

func TestDispatchIncidentReviewFlow(t *testing.T) {
	ds, _ := setupDispatchService(t)
	inc, _ := ds.ReportIncident(&domain.Incident{
		Type:     domain.IncidentTypePoaching,
		Location: &domain.Location{Lat: 30, Lng: 120},
		Images:   []string{"evidence.jpg"},
	})

	if err := ds.ReviewIncident(inc.ID, "disp-1"); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := ds.StartDispatch(inc.ID); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if err := ds.RespondIncident(inc.ID); err != nil {
		t.Fatalf("respond: %v", err)
	}
	if err := ds.ResolveIncident(inc.ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got, _ := ds.GetIncident(inc.ID)
	if got.Status != domain.IncidentStatusResolved {
		t.Errorf("expected resolved, got %s", got.Status)
	}
}

func TestDispatchReportIncidentIdempotent(t *testing.T) {
	ds, _ := setupDispatchService(t)
	inc := &domain.Incident{
		Type:           domain.IncidentTypeFire,
		Location:       &domain.Location{Lat: 30, Lng: 120},
		Images:         []string{"img.jpg"},
		IdempotencyKey: "idem-key-1",
		ReportedAt:     time.Now(),
	}
	first, err := ds.ReportIncident(inc)
	if err != nil {
		t.Fatalf("first report: %v", err)
	}

	// Second report with same key should return existing, not create new
	dup := &domain.Incident{
		Type:           domain.IncidentTypeFire,
		Location:       &domain.Location{Lat: 30, Lng: 120},
		Images:         []string{"img.jpg"},
		IdempotencyKey: "idem-key-1",
	}
	second, err := ds.ReportIncident(dup)
	if err != nil {
		t.Fatalf("second report: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("idempotent report should return same incident: %s vs %s", first.ID, second.ID)
	}
}
