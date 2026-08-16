package worker

import (
	"context"
	"log"
	"time"

	"github.com/reserve/patrol-dispatch/internal/config"
	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/service"
)

// EscalationWorker periodically scans high-risk incidents and auto-escalates
// any that have exceeded the configured response timeout without being
// marked as responding.  Fire and other critical incidents must receive a
// response within ten minutes; otherwise they are escalated to the reserve
// management bureau.
type EscalationWorker struct {
	dispatch *service.DispatchService
	store    incidentLister
	timeout  time.Duration
	interval time.Duration
	bureau   string
}

// incidentLister is the minimal store interface the worker needs to discover
// high-risk incidents.  This keeps the worker decoupled from the concrete
// store implementation and simplifies testing.
type incidentLister interface {
	ListHighRiskIncidents() []*domain.Incident
}

// NewEscalationWorker creates a worker from the dispatch service, store,
// and configuration.
func NewEscalationWorker(ds *service.DispatchService, store incidentLister, cfg config.Config) *EscalationWorker {
	return &EscalationWorker{
		dispatch: ds,
		store:    store,
		timeout:  cfg.EscalationTimeout,
		interval: cfg.EscalationCheckInterval,
		bureau:   cfg.EscalationBureau,
	}
}

// Run starts the escalation loop.  It blocks until the context is cancelled,
// making it suitable for running in a goroutine alongside the HTTP server.
func (w *EscalationWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scanOnce(time.Now())
		}
	}
}

// ScanOnce performs a single escalation pass.  Exported so tests and the
// HTTP layer can trigger an immediate scan without waiting for the ticker.
func (w *EscalationWorker) ScanOnce(now time.Time) int {
	return w.scanOnce(now)
}

func (w *EscalationWorker) scanOnce(now time.Time) int {
	escalated := 0
	for _, inc := range w.store.ListHighRiskIncidents() {
		if !inc.ShouldEscalate(now, w.timeout) {
			continue
		}
		if err := w.dispatch.EscalateIncident(inc.ID, w.bureau); err != nil {
			log.Printf("escalation worker: failed to escalate incident %s: %v", inc.ID, err)
			continue
		}
		escalated++
	}
	return escalated
}
