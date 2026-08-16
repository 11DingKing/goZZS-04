package domain

import "time"

// SyncEntryStatus tracks a locally-cached report awaiting retransmission.
type SyncEntryStatus string

const (
	SyncPending  SyncEntryStatus = "pending"
	SyncUploaded SyncEntryStatus = "uploaded"
	SyncFailed   SyncEntryStatus = "failed"
)

// SyncEntry represents a field report cached on the terminal when
// communication is interrupted.  Entries are replayed in CachedAt order
// once connectivity is restored, and the IdempotencyKey prevents
// duplicate processing.
type SyncEntry struct {
	ID             string          `json:"id"`
	IdempotencyKey string          `json:"idempotency_key"`
	IncidentID     string          `json:"incident_id"`
	Payload        []byte          `json:"payload"`
	CachedAt       time.Time       `json:"cached_at"`
	ReportedAt     time.Time       `json:"reported_at"`
	Status         SyncEntryStatus `json:"status"`
	Attempts       int             `json:"attempts"`
	LastError      string          `json:"last_error,omitempty"`
	UploadedAt     *time.Time      `json:"uploaded_at,omitempty"`
}

// MarkUploaded records a successful replay.
func (e *SyncEntry) MarkUploaded(now time.Time) {
	e.Status = SyncUploaded
	e.UploadedAt = &now
	e.LastError = ""
}

// MarkFailed records a failed replay attempt with the error message.
func (e *SyncEntry) MarkFailed(err string, maxAttempts int) {
	e.Attempts++
	e.LastError = err
	if e.Attempts >= maxAttempts {
		e.Status = SyncFailed
	}
}
