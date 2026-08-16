package domain

import (
	"testing"
	"time"
)

func newReportedIncident() *Incident {
	return &Incident{
		ID:         "inc-1",
		Type:       IncidentTypeFire,
		Location:   &Location{Lat: 30.5, Lng: 120.3},
		Images:     []string{"img-1.jpg"},
		Priority:   PriorityCritical,
		Status:     IncidentStatusReported,
		ReportedAt: time.Now(),
		ReportedBy: "officer-1",
	}
}

func TestIncidentValidateMissingLocation(t *testing.T) {
	inc := newReportedIncident()
	inc.Location = nil
	if err := inc.Validate(); err == nil {
		t.Fatal("expected validation error for missing location")
	}
}

func TestIncidentValidateMissingImages(t *testing.T) {
	inc := newReportedIncident()
	inc.Images = nil
	if err := inc.Validate(); err == nil {
		t.Fatal("expected validation error for missing images")
	}
}

func TestIncidentValidatePassesWithLocationAndImages(t *testing.T) {
	inc := newReportedIncident()
	if err := inc.Validate(); err != nil {
		t.Fatalf("validation should pass: %v", err)
	}
}

func TestIncidentFullLifecycle(t *testing.T) {
	inc := newReportedIncident()
	now := time.Now()

	if err := inc.Review("disp-1", now); err != nil {
		t.Fatalf("review: %v", err)
	}
	if inc.Status != IncidentStatusReviewed {
		t.Fatalf("expected reviewed, got %s", inc.Status)
	}
	if inc.ReviewedBy != "disp-1" {
		t.Errorf("expected reviewed_by disp-1, got %s", inc.ReviewedBy)
	}
	if err := inc.StartDispatch(); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if err := inc.Respond(now); err != nil {
		t.Fatalf("respond: %v", err)
	}
	if inc.RespondedAt == nil {
		t.Error("responded_at should be set")
	}
	if err := inc.Resolve(now); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if inc.Status != IncidentStatusResolved {
		t.Errorf("expected resolved, got %s", inc.Status)
	}
}

func TestIncidentReviewRejectsMissingLocation(t *testing.T) {
	inc := newReportedIncident()
	inc.Location = nil
	err := inc.Review("disp-1", time.Now())
	if err == nil {
		t.Fatal("expected review to fail for missing location")
	}
	if inc.Status != IncidentStatusReported {
		t.Errorf("status should remain reported, got %s", inc.Status)
	}
}

func TestIncidentShouldEscalateAfterTimeout(t *testing.T) {
	inc := newReportedIncident()
	inc.ReportedAt = time.Now().Add(-11 * time.Minute)
	if !inc.ShouldEscalate(time.Now(), 10*time.Minute) {
		t.Error("high-risk incident past timeout should escalate")
	}
}

func TestIncidentShouldNotEscalateIfResponded(t *testing.T) {
	inc := newReportedIncident()
	inc.ReportedAt = time.Now().Add(-20 * time.Minute)
	if err := inc.Review("disp-1", time.Now()); err != nil {
		t.Fatalf("review: %v", err)
	}
	if err := inc.StartDispatch(); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if err := inc.Respond(time.Now()); err != nil {
		t.Fatalf("respond: %v", err)
	}
	if inc.ShouldEscalate(time.Now(), 10*time.Minute) {
		t.Error("responded incident should not escalate")
	}
}

func TestIncidentLowRiskShouldNotEscalate(t *testing.T) {
	inc := newReportedIncident()
	inc.Type = IncidentTypePoaching
	inc.Priority = PriorityLow
	inc.ReportedAt = time.Now().Add(-30 * time.Minute)
	if inc.ShouldEscalate(time.Now(), 10*time.Minute) {
		t.Error("low-risk incident should not escalate")
	}
}

func TestIncidentEscalateSetsBureau(t *testing.T) {
	inc := newReportedIncident()
	inc.ReportedAt = time.Now().Add(-15 * time.Minute)
	if err := inc.Escalate(time.Now(), "保护区管理局"); err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if inc.Status != IncidentStatusEscalated {
		t.Errorf("expected escalated, got %s", inc.Status)
	}
	if inc.EscalatedTo != "保护区管理局" {
		t.Errorf("expected escalated_to set, got %s", inc.EscalatedTo)
	}
}
