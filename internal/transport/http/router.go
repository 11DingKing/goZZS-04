package http

import (
	"net/http"
)

// NewRouter builds the HTTP mux with all patrol-dispatch API routes.
// Route patterns use Go 1.22+ method-prefixed syntax so each path is
// bound to exactly one HTTP verb.
func (h *Handler) NewRouter() http.Handler {
	mux := http.NewServeMux()

	// health
	mux.HandleFunc("GET /api/v1/health", h.Health)

	// actor registration
	mux.HandleFunc("POST /api/v1/actors/grids", h.RegisterGrid)
	mux.HandleFunc("POST /api/v1/actors/officers", h.RegisterOfficer)
	mux.HandleFunc("POST /api/v1/actors/dispatchers", h.RegisterDispatcher)
	mux.HandleFunc("POST /api/v1/actors/managers", h.RegisterManager)

	// patrol tasks
	mux.HandleFunc("POST /api/v1/tasks", h.AssignTask)
	mux.HandleFunc("GET /api/v1/tasks/{id}", h.GetTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/sign-in", h.SignIn)
	mux.HandleFunc("POST /api/v1/tasks/{id}/confirm-missed", h.ConfirmMissedSignIn)
	mux.HandleFunc("POST /api/v1/tasks/{id}/start", h.StartPatrol)
	mux.HandleFunc("POST /api/v1/tasks/{id}/trajectory", h.AddTrajectory)
	mux.HandleFunc("POST /api/v1/tasks/{id}/complete", h.CompleteTask)

	// incidents
	mux.HandleFunc("POST /api/v1/incidents", h.ReportIncident)
	mux.HandleFunc("GET /api/v1/incidents/{id}", h.GetIncident)
	mux.HandleFunc("POST /api/v1/incidents/{id}/review", h.ReviewIncident)
	mux.HandleFunc("POST /api/v1/incidents/{id}/dispatch", h.StartDispatch)
	mux.HandleFunc("POST /api/v1/incidents/{id}/respond", h.RespondIncident)
	mux.HandleFunc("POST /api/v1/incidents/{id}/resolve", h.ResolveIncident)

	// resources
	mux.HandleFunc("POST /api/v1/resources", h.CreateResource)
	mux.HandleFunc("POST /api/v1/resources/{id}/requests", h.RequestResource)
	mux.HandleFunc("POST /api/v1/resource-requests/{id}/approve", h.ApproveResourceRequest)
	mux.HandleFunc("POST /api/v1/resource-requests/{id}/allocate", h.AllocateResource)
	mux.HandleFunc("POST /api/v1/resource-requests/{id}/release", h.ReleaseResource)
	mux.HandleFunc("POST /api/v1/resources/{id}/arbitrate", h.ArbitrateResource)

	// materials
	mux.HandleFunc("POST /api/v1/materials", h.CreateMaterial)
	mux.HandleFunc("POST /api/v1/material-requests", h.CreateMaterialRequest)
	mux.HandleFunc("POST /api/v1/material-requests/{id}/sign", h.MaterialManagerSign)
	mux.HandleFunc("POST /api/v1/material-requests/arbitrate", h.ArbitrateMaterials)
	mux.HandleFunc("POST /api/v1/material-requests/{id}/reject", h.RejectMaterialRequest)

	// offline sync
	mux.HandleFunc("POST /api/v1/sync/cache", h.CacheReport)
	mux.HandleFunc("POST /api/v1/sync/replay", h.ReplayPending)
	mux.HandleFunc("GET /api/v1/sync/{id}", h.GetSyncEntry)

	return mux
}
