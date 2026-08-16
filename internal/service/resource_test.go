package service

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

func setupResourceService(t *testing.T) *ResourceService {
	t.Helper()
	s := store.New()
	s.RegisterDispatcher("disp-1")
	rs := NewResourceService(s)
	if err := rs.CreateResource(&domain.Resource{
		ID:       "vehicle-1",
		Type:     domain.ResourceTypeVehicle,
		Name:     "patrol-truck",
		Capacity: 1,
	}); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	if err := rs.CreateResource(&domain.Resource{
		ID:       "drone-1",
		Type:     domain.ResourceTypeDrone,
		Name:     "surveillance-drone",
		Capacity: 2,
	}); err != nil {
		t.Fatalf("create resource: %v", err)
	}
	return rs
}

func TestResourceAllocateSuccess(t *testing.T) {
	rs := setupResourceService(t)

	rr, err := rs.RequestResource("inc-1", "vehicle-1", "officer-1", 1, domain.PriorityHigh)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := rs.ApproveRequest(rr.ID, "disp-1"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := rs.AllocateResource(rr.ID); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	got, _ := rs.store.GetResourceRequest(rr.ID)
	if got.Status != domain.ResourceRequestAllocated {
		t.Errorf("expected allocated, got %s", got.Status)
	}
}

func TestResourceAllocateOverCapacityRejected(t *testing.T) {
	rs := setupResourceService(t)

	rr, _ := rs.RequestResource("inc-1", "vehicle-1", "officer-1", 1, domain.PriorityHigh)
	rs.ApproveRequest(rr.ID, "disp-1")
	if err := rs.AllocateResource(rr.ID); err != nil {
		t.Fatalf("first allocate: %v", err)
	}

	rr2, _ := rs.RequestResource("inc-2", "vehicle-1", "officer-2", 1, domain.PriorityMedium)
	rs.ApproveRequest(rr2.ID, "disp-1")
	err := rs.AllocateResource(rr2.ID)
	if err == nil {
		t.Fatal("expected over-allocation rejection")
	}
	got, _ := rs.store.GetResourceRequest(rr2.ID)
	if got.Status != domain.ResourceRequestRejected {
		t.Errorf("expected rejected, got %s", got.Status)
	}
}

func TestResourceAllocateBeforeApprovalFails(t *testing.T) {
	rs := setupResourceService(t)
	rr, _ := rs.RequestResource("inc-1", "vehicle-1", "officer-1", 1, domain.PriorityHigh)
	if err := rs.AllocateResource(rr.ID); err == nil {
		t.Fatal("expected error allocating before approval")
	}
}

func TestResourceArbitrateByPriority(t *testing.T) {
	rs := setupResourceService(t)

	// vehicle-1 has capacity 1.  Two requests: low priority first, then
	// high priority.  Arbitration should give the resource to high priority.
	lowReq, _ := rs.RequestResource("inc-1", "vehicle-1", "officer-1", 1, domain.PriorityLow)
	highReq, _ := rs.RequestResource("inc-2", "vehicle-1", "officer-2", 1, domain.PriorityHigh)
	rs.ApproveRequest(lowReq.ID, "disp-1")
	rs.ApproveRequest(highReq.ID, "disp-1")

	allocated, rejected, err := rs.ArbitrateResource("vehicle-1")
	if err != nil {
		t.Fatalf("arbitrate: %v", err)
	}
	if allocated != 1 {
		t.Errorf("expected 1 allocated, got %d", allocated)
	}
	if rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", rejected)
	}

	highGot, _ := rs.store.GetResourceRequest(highReq.ID)
	lowGot, _ := rs.store.GetResourceRequest(lowReq.ID)
	if highGot.Status != domain.ResourceRequestAllocated {
		t.Errorf("high priority should be allocated, got %s", highGot.Status)
	}
	if lowGot.Status != domain.ResourceRequestRejected {
		t.Errorf("low priority should be rejected, got %s", lowGot.Status)
	}
}

func TestResourceConcurrentAllocationNoOverAllocation(t *testing.T) {
	rs := setupResourceService(t)

	var wg sync.WaitGroup
	var successCount int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rr, err := rs.RequestResource(
				fmt.Sprintf("inc-%d", idx),
				"vehicle-1",
				fmt.Sprintf("officer-%d", idx),
				1,
				domain.Priority(idx%4),
			)
			if err != nil {
				return
			}
			if err := rs.ApproveRequest(rr.ID, "disp-1"); err != nil {
				return
			}
			if err := rs.AllocateResource(rr.ID); err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()

	sc := atomic.LoadInt32(&successCount)
	res, _ := rs.store.GetResource("vehicle-1")
	if sc > int32(res.Capacity) {
		t.Errorf("over-allocation: %d allocated, capacity %d", sc, res.Capacity)
	}
	if res.InUse > res.Capacity {
		t.Errorf("in_use %d exceeds capacity %d", res.InUse, res.Capacity)
	}
}

func TestResourceReleaseReturnsCapacity(t *testing.T) {
	rs := setupResourceService(t)
	rr, _ := rs.RequestResource("inc-1", "vehicle-1", "officer-1", 1, domain.PriorityHigh)
	rs.ApproveRequest(rr.ID, "disp-1")
	rs.AllocateResource(rr.ID)

	res, _ := rs.store.GetResource("vehicle-1")
	if res.Available() != 0 {
		t.Errorf("expected 0 available after allocation, got %d", res.Available())
	}

	if err := rs.ReleaseResource(rr.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	res, _ = rs.store.GetResource("vehicle-1")
	if res.Available() != 1 {
		t.Errorf("expected 1 available after release, got %d", res.Available())
	}
	got, _ := rs.store.GetResourceRequest(rr.ID)
	if got.Status != domain.ResourceRequestReleased {
		t.Errorf("expected released, got %s", got.Status)
	}
}
