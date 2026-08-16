package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reserve/patrol-dispatch/internal/config"
	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/service"
	"github.com/reserve/patrol-dispatch/internal/store"
)

func setupHandler(t *testing.T) (*Handler, *store.Store) {
	t.Helper()
	s := store.New()
	ds := service.NewDispatchService(s)
	rs := service.NewResourceService(s)
	ms := service.NewMaterialService(s)
	ss := service.NewSyncService(s, ds)
	cfg := config.Default()
	h := NewHandler(ds, rs, ms, ss, cfg)
	return h, s
}

func doRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestHTTP_PatrolTaskFullFlow(t *testing.T) {
	h, s := setupHandler(t)
	router := h.NewRouter()

	w := doRequest(t, router, "POST", "/api/v1/actors/grids", map[string]any{
		"grid_id":  "grid-A",
		"location": domain.Location{Lat: 30.0, Lng: 120.0},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register grid: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, router, "POST", "/api/v1/actors/officers", map[string]string{
		"officer_id": "officer-1",
		"grid_id":    "grid-A",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register officer: expected 201, got %d", w.Code)
	}

	w = doRequest(t, router, "POST", "/api/v1/actors/dispatchers", map[string]string{
		"id": "disp-1",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register dispatcher: expected 201, got %d", w.Code)
	}

	w = doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{
		"dispatcher_id": "disp-1",
		"officer_id":    "officer-1",
		"grid_id":       "grid-A",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("assign task: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var task domain.PatrolTask
	json.NewDecoder(w.Body).Decode(&task)
	if task.Status != domain.PatrolTaskAssigned {
		t.Fatalf("expected status %s, got %s", domain.PatrolTaskAssigned, task.Status)
	}

	w = doRequest(t, router, "POST", "/api/v1/tasks/"+task.ID+"/sign-in", map[string]string{
		"officer_id": "officer-1",
		"grid_id":    "grid-A",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("sign in: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = doRequest(t, router, "POST", "/api/v1/tasks/"+task.ID+"/start", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("start patrol: expected 200, got %d", w.Code)
	}

	w = doRequest(t, router, "POST", "/api/v1/tasks/"+task.ID+"/trajectory", map[string]float64{
		"lat": 30.1,
		"lng": 120.2,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("add trajectory: expected 200, got %d", w.Code)
	}

	w = doRequest(t, router, "POST", "/api/v1/tasks/"+task.ID+"/complete", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("complete: expected 200, got %d", w.Code)
	}

	updated, _ := s.GetTask(task.ID)
	if updated.Status != domain.PatrolTaskCompleted {
		t.Fatalf("expected status %s, got %s", domain.PatrolTaskCompleted, updated.Status)
	}
	if len(updated.Trajectory) != 1 {
		t.Fatalf("expected 1 trajectory point, got %d", len(updated.Trajectory))
	}
}

func TestHTTP_SignInWrongGrid(t *testing.T) {
	h, _ := setupHandler(t)
	router := h.NewRouter()

	doRequest(t, router, "POST", "/api/v1/actors/grids", map[string]any{
		"grid_id":  "grid-A",
		"location": domain.Location{Lat: 30.0, Lng: 120.0},
	})
	doRequest(t, router, "POST", "/api/v1/actors/grids", map[string]any{
		"grid_id":  "grid-B",
		"location": domain.Location{Lat: 31.0, Lng: 121.0},
	})
	doRequest(t, router, "POST", "/api/v1/actors/officers", map[string]string{
		"officer_id": "officer-1",
		"grid_id":    "grid-A",
	})
	doRequest(t, router, "POST", "/api/v1/actors/dispatchers", map[string]string{"id": "disp-1"})

	w := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{
		"dispatcher_id": "disp-1",
		"officer_id":    "officer-1",
		"grid_id":       "grid-A",
	})
	var task domain.PatrolTask
	json.NewDecoder(w.Body).Decode(&task)

	w = doRequest(t, router, "POST", "/api/v1/tasks/"+task.ID+"/sign-in", map[string]string{
		"officer_id": "officer-1",
		"grid_id":    "grid-B",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("sign-in at wrong grid should return 400, got %d", w.Code)
	}
}

func TestHTTP_IncidentReport_MissingImages(t *testing.T) {
	h, _ := setupHandler(t)
	router := h.NewRouter()

	w := doRequest(t, router, "POST", "/api/v1/incidents", map[string]any{
		"type":            "fire",
		"location":        domain.Location{Lat: 30.1, Lng: 120.2},
		"priority":        domain.PriorityCritical,
		"reported_by":     "officer-1",
		"idempotency_key": "missing-imgs",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("incident without images should return 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTP_ResourceArbitration(t *testing.T) {
	h, _ := setupHandler(t)
	router := h.NewRouter()

	doRequest(t, router, "POST", "/api/v1/actors/dispatchers", map[string]string{"id": "disp-1"})

	w := doRequest(t, router, "POST", "/api/v1/resources", domain.Resource{
		Type:     domain.ResourceTypeVehicle,
		Name:     "Truck",
		Capacity: 1,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create resource: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var res domain.Resource
	json.NewDecoder(w.Body).Decode(&res)

	w = doRequest(t, router, "POST", "/api/v1/resources/"+res.ID+"/requests", map[string]any{
		"incident_id":  "inc-1",
		"requested_by": "group-A",
		"quantity":     1,
		"priority":     domain.PriorityHigh,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("request resource A: expected 201, got %d", w.Code)
	}
	var rrA domain.ResourceRequest
	json.NewDecoder(w.Body).Decode(&rrA)

	w = doRequest(t, router, "POST", "/api/v1/resources/"+res.ID+"/requests", map[string]any{
		"incident_id":  "inc-2",
		"requested_by": "group-B",
		"quantity":     1,
		"priority":     domain.PriorityLow,
	})
	var rrB domain.ResourceRequest
	json.NewDecoder(w.Body).Decode(&rrB)

	w = doRequest(t, router, "POST", "/api/v1/resource-requests/"+rrA.ID+"/approve", map[string]string{
		"dispatcher_id": "disp-1",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("approve A: expected 200, got %d", w.Code)
	}
	doRequest(t, router, "POST", "/api/v1/resource-requests/"+rrB.ID+"/approve", map[string]string{
		"dispatcher_id": "disp-1",
	})

	w = doRequest(t, router, "POST", "/api/v1/resources/"+res.ID+"/arbitrate", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("arbitrate: expected 200, got %d", w.Code)
	}
	var result map[string]int
	json.NewDecoder(w.Body).Decode(&result)
	if result["allocated"] != 1 || result["rejected"] != 1 {
		t.Fatalf("expected 1 allocated 1 rejected, got %v", result)
	}
}
