package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/reserve/patrol-dispatch/internal/config"
	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/service"
)

// Handler exposes all patrol-dispatch business operations as JSON HTTP
// endpoints.  Each handler validates input, delegates to the appropriate
// service, and returns a structured JSON response or error.
type Handler struct {
	dispatch *service.DispatchService
	resource *service.ResourceService
	material *service.MaterialService
	sync     *service.SyncService
	config   config.Config
}

// NewHandler creates a handler wired to the given services.
func NewHandler(
	ds *service.DispatchService,
	rs *service.ResourceService,
	ms *service.MaterialService,
	ss *service.SyncService,
	cfg config.Config,
) *Handler {
	return &Handler{
		dispatch: ds,
		resource: rs,
		material: ms,
		sync:     ss,
		config:   cfg,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// ---------------------------------------------------------------------------
// actor registration
// ---------------------------------------------------------------------------

type registerGridRequest struct {
	GridID string           `json:"grid_id"`
	Loc    *domain.Location `json:"location"`
}

func (h *Handler) RegisterGrid(w http.ResponseWriter, r *http.Request) {
	var req registerGridRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.GridID == "" {
		writeError(w, http.StatusBadRequest, "grid_id is required")
		return
	}
	h.dispatch.RegisterGrid(req.GridID, req.Loc)
	writeJSON(w, http.StatusCreated, map[string]string{"grid_id": req.GridID})
}

type registerOfficerRequest struct {
	OfficerID string `json:"officer_id"`
	GridID    string `json:"grid_id"`
}

func (h *Handler) RegisterOfficer(w http.ResponseWriter, r *http.Request) {
	var req registerOfficerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.dispatch.RegisterOfficer(req.OfficerID, req.GridID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, req)
}

type registerActorRequest struct {
	ID string `json:"id"`
}

func (h *Handler) RegisterDispatcher(w http.ResponseWriter, r *http.Request) {
	var req registerActorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.dispatch.RegisterDispatcher(req.ID)
	writeJSON(w, http.StatusCreated, req)
}

func (h *Handler) RegisterManager(w http.ResponseWriter, r *http.Request) {
	var req registerActorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.material.RegisterManager(req.ID)
	writeJSON(w, http.StatusCreated, req)
}

// ---------------------------------------------------------------------------
// patrol tasks
// ---------------------------------------------------------------------------

type assignTaskRequest struct {
	DispatcherID string `json:"dispatcher_id"`
	OfficerID    string `json:"officer_id"`
	GridID       string `json:"grid_id"`
}

func (h *Handler) AssignTask(w http.ResponseWriter, r *http.Request) {
	var req assignTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := h.dispatch.AssignTask(req.DispatcherID, req.OfficerID, req.GridID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, ok := h.dispatch.GetTask(id)
	if !ok {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

type signInRequest struct {
	OfficerID string `json:"officer_id"`
	GridID    string `json:"grid_id"`
}

func (h *Handler) SignIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.dispatch.SignIn(r.PathValue("id"), req.OfficerID, req.GridID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "signed_in"})
}

type confirmMissedRequest struct {
	DispatcherID string `json:"dispatcher_id"`
}

func (h *Handler) ConfirmMissedSignIn(w http.ResponseWriter, r *http.Request) {
	var req confirmMissedRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.dispatch.ConfirmMissedSignIn(r.PathValue("id"), req.DispatcherID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

func (h *Handler) StartPatrol(w http.ResponseWriter, r *http.Request) {
	if err := h.dispatch.StartPatrol(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "patrolling"})
}

type trajectoryRequest struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (h *Handler) AddTrajectory(w http.ResponseWriter, r *http.Request) {
	var req trajectoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.dispatch.AddTrajectory(r.PathValue("id"), domain.TrajectoryPoint{
		Lat: req.Lat,
		Lng: req.Lng,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *Handler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	if err := h.dispatch.CompleteTask(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// ---------------------------------------------------------------------------
// incidents
// ---------------------------------------------------------------------------

func (h *Handler) ReportIncident(w http.ResponseWriter, r *http.Request) {
	var inc domain.Incident
	if err := decodeJSON(r, &inc); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.dispatch.ReportIncident(&inc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) GetIncident(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inc, ok := h.dispatch.GetIncident(id)
	if !ok {
		writeError(w, http.StatusNotFound, "incident not found")
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

type reviewRequest struct {
	DispatcherID string `json:"dispatcher_id"`
}

func (h *Handler) ReviewIncident(w http.ResponseWriter, r *http.Request) {
	var req reviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.dispatch.ReviewIncident(r.PathValue("id"), req.DispatcherID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed"})
}

func (h *Handler) StartDispatch(w http.ResponseWriter, r *http.Request) {
	if err := h.dispatch.StartDispatch(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "dispatching"})
}

func (h *Handler) RespondIncident(w http.ResponseWriter, r *http.Request) {
	if err := h.dispatch.RespondIncident(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "responding"})
}

func (h *Handler) ResolveIncident(w http.ResponseWriter, r *http.Request) {
	if err := h.dispatch.ResolveIncident(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// ---------------------------------------------------------------------------
// resources
// ---------------------------------------------------------------------------

func (h *Handler) CreateResource(w http.ResponseWriter, r *http.Request) {
	var res domain.Resource
	if err := decodeJSON(r, &res); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.resource.CreateResource(&res); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

type resourceRequestReq struct {
	IncidentID  string          `json:"incident_id"`
	ResourceID  string          `json:"resource_id"`
	RequestedBy string          `json:"requested_by"`
	Quantity    int             `json:"quantity"`
	Priority    domain.Priority `json:"priority"`
}

func (h *Handler) RequestResource(w http.ResponseWriter, r *http.Request) {
	var req resourceRequestReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rr, err := h.resource.RequestResource(req.IncidentID, r.PathValue("id"), req.RequestedBy, req.Quantity, req.Priority)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rr)
}

type approveRequest struct {
	DispatcherID string `json:"dispatcher_id"`
}

func (h *Handler) ApproveResourceRequest(w http.ResponseWriter, r *http.Request) {
	var req approveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.resource.ApproveRequest(r.PathValue("id"), req.DispatcherID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handler) AllocateResource(w http.ResponseWriter, r *http.Request) {
	if err := h.resource.AllocateResource(r.PathValue("id")); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "allocated"})
}

func (h *Handler) ArbitrateResource(w http.ResponseWriter, r *http.Request) {
	allocated, rejected, err := h.resource.ArbitrateResource(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"allocated": allocated,
		"rejected":  rejected,
	})
}

func (h *Handler) ReleaseResource(w http.ResponseWriter, r *http.Request) {
	if err := h.resource.ReleaseResource(r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "released"})
}

// ---------------------------------------------------------------------------
// materials
// ---------------------------------------------------------------------------

func (h *Handler) CreateMaterial(w http.ResponseWriter, r *http.Request) {
	var mat domain.Material
	if err := decodeJSON(r, &mat); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.material.CreateMaterial(&mat); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mat)
}

type materialRequestReq struct {
	RequesterID string                `json:"requester_id"`
	Items       []domain.MaterialItem `json:"items"`
	Priority    domain.Priority       `json:"priority"`
	IncidentID  string                `json:"incident_id"`
}

func (h *Handler) CreateMaterialRequest(w http.ResponseWriter, r *http.Request) {
	var req materialRequestReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mr, err := h.material.CreateRequest(req.RequesterID, req.Items, req.Priority, req.IncidentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mr)
}

type managerSignRequest struct {
	ManagerID string `json:"manager_id"`
}

func (h *Handler) MaterialManagerSign(w http.ResponseWriter, r *http.Request) {
	var req managerSignRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.material.ManagerSign(r.PathValue("id"), req.ManagerID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *Handler) ArbitrateMaterials(w http.ResponseWriter, r *http.Request) {
	var req managerSignRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	completed, rejected, err := h.material.ArbitrateMaterials(req.ManagerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"completed": completed,
		"rejected":  rejected,
	})
}

type rejectMaterialRequest struct {
	ManagerID string `json:"manager_id"`
	Reason    string `json:"reason"`
}

func (h *Handler) RejectMaterialRequest(w http.ResponseWriter, r *http.Request) {
	var req rejectMaterialRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.material.RejectRequest(r.PathValue("id"), req.ManagerID, req.Reason); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// ---------------------------------------------------------------------------
// offline sync
// ---------------------------------------------------------------------------

func (h *Handler) CacheReport(w http.ResponseWriter, r *http.Request) {
	var entry domain.SyncEntry
	if err := decodeJSON(r, &entry); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.sync.CacheReport(&entry)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) ReplayPending(w http.ResponseWriter, r *http.Request) {
	uploaded, skipped, failed, err := h.sync.ReplayPending(h.config.SyncMaxAttempts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{
		"uploaded": uploaded,
		"skipped":  skipped,
		"failed":   failed,
	})
}

func (h *Handler) GetSyncEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, ok := h.sync.GetEntry(id)
	if !ok {
		writeError(w, http.StatusNotFound, "sync entry not found")
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// ---------------------------------------------------------------------------
// health
// ---------------------------------------------------------------------------

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"port":   strconv.Itoa(53649),
	})
}
