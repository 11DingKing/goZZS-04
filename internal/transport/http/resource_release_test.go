package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/reserve/patrol-dispatch/internal/domain"
)

// callWithDeadline runs one API call and fails the test if the endpoint does
// not answer within the deadline, so that a stalled dispatch endpoint is
// reported as a test failure instead of hanging the whole run.
func callWithDeadline(t *testing.T, router http.Handler, method, path string, body any, deadline time.Duration, label string) *httptest.ResponseRecorder {
	t.Helper()
	type outcome struct {
		rec *httptest.ResponseRecorder
		err any
	}
	done := make(chan outcome, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- outcome{err: p}
			}
		}()
		done <- outcome{rec: doRequest(t, router, method, path, body)}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("%s: panicked: %v", label, res.err)
		}
		return res.rec
	case <-time.After(deadline):
		t.Fatalf("%s: %s %s did not respond within %s", label, method, path, deadline)
		return nil
	}
}

type resourceFixture struct {
	router   http.Handler
	resource domain.Resource
}

func setupResourceFixture(t *testing.T, capacity int) resourceFixture {
	t.Helper()
	h, _ := setupHandler(t)
	router := h.NewRouter()

	doRequest(t, router, "POST", "/api/v1/actors/dispatchers", map[string]string{"id": "disp-1"})

	w := doRequest(t, router, "POST", "/api/v1/resources", domain.Resource{
		Type:     domain.ResourceTypeDrone,
		Name:     "surveillance-drone",
		Capacity: capacity,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create resource: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var res domain.Resource
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	return resourceFixture{router: router, resource: res}
}

func (f resourceFixture) approvedRequest(t *testing.T, incidentID string, quantity int) domain.ResourceRequest {
	t.Helper()
	w := doRequest(t, f.router, "POST", "/api/v1/resources/"+f.resource.ID+"/requests", map[string]any{
		"incident_id":  incidentID,
		"requested_by": "group-" + incidentID,
		"quantity":     quantity,
		"priority":     domain.PriorityHigh,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("request resource for %s: expected 201, got %d: %s", incidentID, w.Code, w.Body.String())
	}
	var rr domain.ResourceRequest
	if err := json.NewDecoder(w.Body).Decode(&rr); err != nil {
		t.Fatalf("decode resource request: %v", err)
	}
	w = doRequest(t, f.router, "POST", "/api/v1/resource-requests/"+rr.ID+"/approve", map[string]string{
		"dispatcher_id": "disp-1",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("approve request for %s: expected 200, got %d: %s", incidentID, w.Code, w.Body.String())
	}
	return rr
}

// TestHTTP_DispatchStaysAvailableAfterDuplicateRelease covers the double click
// case on the release button: the second release of the same request must be
// rejected, and emergency resource dispatch must keep serving requests
// afterwards.
func TestHTTP_DispatchStaysAvailableAfterDuplicateRelease(t *testing.T) {
	f := setupResourceFixture(t, 1)

	first := f.approvedRequest(t, "inc-1", 1)
	if w := doRequest(t, f.router, "POST", "/api/v1/resource-requests/"+first.ID+"/allocate", nil); w.Code != http.StatusOK {
		t.Fatalf("allocate: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := doRequest(t, f.router, "POST", "/api/v1/resource-requests/"+first.ID+"/release", nil); w.Code != http.StatusOK {
		t.Fatalf("first release: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w := callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/"+first.ID+"/release", nil,
		5*time.Second, "duplicate release")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate release: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	second := f.approvedRequest(t, "inc-2", 1)
	w = callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/"+second.ID+"/allocate", nil,
		5*time.Second, "allocate after duplicate release")
	if w.Code != http.StatusOK {
		t.Fatalf("allocate after duplicate release: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = callWithDeadline(t, f.router, "POST", "/api/v1/resources/"+f.resource.ID+"/arbitrate", nil,
		5*time.Second, "arbitrate after duplicate release")
	if w.Code != http.StatusOK {
		t.Fatalf("arbitrate after duplicate release: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest covers a release
// call carrying a request ID that does not exist: the call must be rejected and
// later resource operations must still be served.
func TestHTTP_DispatchStaysAvailableAfterReleasingUnknownRequest(t *testing.T) {
	f := setupResourceFixture(t, 2)

	w := callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/req_does_not_exist/release", nil,
		5*time.Second, "release unknown request")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("release unknown request: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	rr := f.approvedRequest(t, "inc-3", 1)
	w = callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/"+rr.ID+"/allocate", nil,
		5*time.Second, "allocate after unknown release")
	if w.Code != http.StatusOK {
		t.Fatalf("allocate after unknown release: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest covers a
// release call for a request that was approved but never allocated. It must be
// rejected without taking the dispatch endpoints out of service.
func TestHTTP_DispatchStaysAvailableAfterReleasingUnallocatedRequest(t *testing.T) {
	f := setupResourceFixture(t, 2)

	pending := f.approvedRequest(t, "inc-4", 1)
	w := callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/"+pending.ID+"/release", nil,
		5*time.Second, "release unallocated request")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("release unallocated request: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	w = callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/"+pending.ID+"/allocate", nil,
		5*time.Second, "allocate after unallocated release")
	if w.Code != http.StatusOK {
		t.Fatalf("allocate after unallocated release: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHTTP_ReleaseReturnsCapacityToThePool keeps the happy path pinned: a
// released request frees its units so the next incident can take the resource.
func TestHTTP_ReleaseReturnsCapacityToThePool(t *testing.T) {
	f := setupResourceFixture(t, 1)

	first := f.approvedRequest(t, "inc-5", 1)
	if w := doRequest(t, f.router, "POST", "/api/v1/resource-requests/"+first.ID+"/allocate", nil); w.Code != http.StatusOK {
		t.Fatalf("allocate: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	second := f.approvedRequest(t, "inc-6", 1)
	if w := doRequest(t, f.router, "POST", "/api/v1/resource-requests/"+second.ID+"/allocate", nil); w.Code != http.StatusConflict {
		t.Fatalf("allocate over capacity: expected 409, got %d: %s", w.Code, w.Body.String())
	}

	if w := doRequest(t, f.router, "POST", "/api/v1/resource-requests/"+first.ID+"/release", nil); w.Code != http.StatusOK {
		t.Fatalf("release: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	third := f.approvedRequest(t, "inc-7", 1)
	w := callWithDeadline(t, f.router, "POST", "/api/v1/resource-requests/"+third.ID+"/allocate", nil,
		5*time.Second, "allocate after release")
	if w.Code != http.StatusOK {
		t.Fatalf("allocate after release: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
