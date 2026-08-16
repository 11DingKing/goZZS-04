package worker

import (
	"testing"
	"time"

	"github.com/reserve/patrol-dispatch/internal/config"
	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/service"
	"github.com/reserve/patrol-dispatch/internal/store"
)

func TestEscalationWorkerEscalatesTimedOutHighRisk(t *testing.T) {
	s := store.New()
	ds := service.NewDispatchService(s)

	// Create a high-risk fire incident reported 15 minutes ago (past timeout)
	inc := &domain.Incident{
		ID:         "inc-fire-1",
		Type:       domain.IncidentTypeFire,
		Location:   &domain.Location{Lat: 30, Lng: 120},
		Images:     []string{"fire.jpg"},
		Priority:   domain.PriorityCritical,
		Status:     domain.IncidentStatusReported,
		ReportedAt: time.Now().Add(-15 * time.Minute),
	}
	s.SaveIncident(inc)

	cfg := config.Default()
	cfg.EscalationTimeout = 10 * time.Minute
	w := NewEscalationWorker(ds, s, cfg)

	escalated := w.ScanOnce(time.Now())
	if escalated != 1 {
		t.Fatalf("expected 1 escalation, got %d", escalated)
	}

	got, _ := s.GetIncident("inc-fire-1")
	if got.Status != domain.IncidentStatusEscalated {
		t.Errorf("expected escalated, got %s", got.Status)
	}
	if got.EscalatedTo == "" {
		t.Error("escalated_to should be set")
	}
}

func TestEscalationWorkerSkipsRespondedIncident(t *testing.T) {
	s := store.New()
	ds := service.NewDispatchService(s)

	inc := &domain.Incident{
		ID:         "inc-fire-2",
		Type:       domain.IncidentTypeFire,
		Location:   &domain.Location{Lat: 30, Lng: 120},
		Images:     []string{"fire.jpg"},
		Priority:   domain.PriorityCritical,
		Status:     domain.IncidentStatusResponding,
		ReportedAt: time.Now().Add(-30 * time.Minute),
	}
	s.SaveIncident(inc)

	cfg := config.Default()
	cfg.EscalationTimeout = 10 * time.Minute
	w := NewEscalationWorker(ds, s, cfg)

	escalated := w.ScanOnce(time.Now())
	if escalated != 0 {
		t.Fatalf("expected 0 escalations for responded incident, got %d", escalated)
	}

	got, _ := s.GetIncident("inc-fire-2")
	if got.Status != domain.IncidentStatusResponding {
		t.Errorf("status should remain responding, got %s", got.Status)
	}
}

func TestEscalationWorkerSkipsLowRiskIncident(t *testing.T) {
	s := store.New()
	ds := service.NewDispatchService(s)

	inc := &domain.Incident{
		ID:         "inc-poaching-1",
		Type:       domain.IncidentTypePoaching,
		Location:   &domain.Location{Lat: 30, Lng: 120},
		Images:     []string{"trap.jpg"},
		Priority:   domain.PriorityLow,
		Status:     domain.IncidentStatusReported,
		ReportedAt: time.Now().Add(-60 * time.Minute),
	}
	s.SaveIncident(inc)

	cfg := config.Default()
	cfg.EscalationTimeout = 10 * time.Minute
	w := NewEscalationWorker(ds, s, cfg)

	escalated := w.ScanOnce(time.Now())
	if escalated != 0 {
		t.Fatalf("expected 0 escalations for low-risk incident, got %d", escalated)
	}
}

func TestEscalationWorkerNotYetTimedOut(t *testing.T) {
	s := store.New()
	ds := service.NewDispatchService(s)

	inc := &domain.Incident{
		ID:         "inc-fire-3",
		Type:       domain.IncidentTypeFire,
		Location:   &domain.Location{Lat: 30, Lng: 120},
		Images:     []string{"fire.jpg"},
		Priority:   domain.PriorityCritical,
		Status:     domain.IncidentStatusReported,
		ReportedAt: time.Now().Add(-3 * time.Minute),
	}
	s.SaveIncident(inc)

	cfg := config.Default()
	cfg.EscalationTimeout = 10 * time.Minute
	w := NewEscalationWorker(ds, s, cfg)

	escalated := w.ScanOnce(time.Now())
	if escalated != 0 {
		t.Fatalf("expected 0 escalations for incident within timeout, got %d", escalated)
	}
}
