package service

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/reserve/patrol-dispatch/internal/domain"
	"github.com/reserve/patrol-dispatch/internal/store"
)

func setupMaterialService(t *testing.T) *MaterialService {
	t.Helper()
	s := store.New()
	s.RegisterManager("mgr-1")
	ms := NewMaterialService(s)
	if err := ms.CreateMaterial(&domain.Material{ID: "mat-1", Name: "灭火器", Stock: 5}); err != nil {
		t.Fatalf("create material: %v", err)
	}
	if err := ms.CreateMaterial(&domain.Material{ID: "mat-2", Name: "对讲机", Stock: 3}); err != nil {
		t.Fatalf("create material: %v", err)
	}
	return ms
}

func TestMaterialDualSignCompletes(t *testing.T) {
	ms := setupMaterialService(t)
	mr, err := ms.CreateRequest("officer-1", []domain.MaterialItem{
		{MaterialID: "mat-1", Quantity: 2},
	}, domain.PriorityHigh, "inc-1")
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if !mr.RequesterSigned {
		t.Error("requester should have signed on creation")
	}
	if err := ms.ManagerSign(mr.ID, "mgr-1"); err != nil {
		t.Fatalf("manager sign: %v", err)
	}
	got, _ := ms.store.GetMaterialRequest(mr.ID)
	if got.Status != domain.MaterialCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
	if !got.ManagerSigned {
		t.Error("manager should have signed")
	}
	mat, _ := ms.store.GetMaterial("mat-1")
	if mat.Stock != 3 {
		t.Errorf("expected stock 3, got %d", mat.Stock)
	}
}

func TestMaterialManagerSignBeforeRequesterFails(t *testing.T) {
	ms := setupMaterialService(t)
	mr := &domain.MaterialRequest{
		ID:          "mr-manual",
		Items:       []domain.MaterialItem{{MaterialID: "mat-1", Quantity: 1}},
		RequesterID: "officer-1",
		Status:      domain.MaterialRequested,
	}
	ms.store.SaveMaterialRequest(mr)

	err := ms.ManagerSign(mr.ID, "mgr-1")
	if err == nil {
		t.Fatal("expected error: manager cannot sign before requester")
	}
}

func TestMaterialInsufficientStockFails(t *testing.T) {
	ms := setupMaterialService(t)
	mr, _ := ms.CreateRequest("officer-1", []domain.MaterialItem{
		{MaterialID: "mat-1", Quantity: 100},
	}, domain.PriorityHigh, "inc-1")
	err := ms.ManagerSign(mr.ID, "mgr-1")
	if err == nil {
		t.Fatal("expected error for insufficient stock")
	}
	got, _ := ms.store.GetMaterialRequest(mr.ID)
	if got.Status != domain.MaterialRequested {
		t.Errorf("expected requested, got %s", got.Status)
	}
}

func TestMaterialArbitrationByPriority(t *testing.T) {
	ms := setupMaterialService(t)

	// mat-2 has stock 3.  Create 3 requests with different priorities.
	low, _ := ms.CreateRequest("officer-1", []domain.MaterialItem{
		{MaterialID: "mat-2", Quantity: 2},
	}, domain.PriorityLow, "inc-1")
	high, _ := ms.CreateRequest("officer-2", []domain.MaterialItem{
		{MaterialID: "mat-2", Quantity: 2},
	}, domain.PriorityCritical, "inc-2")

	completed, rejected, err := ms.ArbitrateMaterials("mgr-1")
	if err != nil {
		t.Fatalf("arbitrate: %v", err)
	}
	if completed != 1 {
		t.Errorf("expected 1 completed, got %d", completed)
	}
	if rejected != 1 {
		t.Errorf("expected 1 rejected, got %d", rejected)
	}

	highGot, _ := ms.store.GetMaterialRequest(high.ID)
	lowGot, _ := ms.store.GetMaterialRequest(low.ID)
	if highGot.Status != domain.MaterialCompleted {
		t.Errorf("high priority should be completed, got %s", highGot.Status)
	}
	if lowGot.Status != domain.MaterialRejected {
		t.Errorf("low priority should be rejected, got %s", lowGot.Status)
	}
}

func TestMaterialConcurrentDualSignNoDoubleDecrement(t *testing.T) {
	ms := setupMaterialService(t)
	mr, _ := ms.CreateRequest("officer-1", []domain.MaterialItem{
		{MaterialID: "mat-1", Quantity: 1},
	}, domain.PriorityHigh, "inc-1")

	var wg sync.WaitGroup
	var successCount int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ms.ManagerSign(mr.ID, "mgr-1"); err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful sign, got %d", successCount)
	}
	mat, _ := ms.store.GetMaterial("mat-1")
	if mat.Stock != 4 {
		t.Errorf("expected stock 4 (decremented once), got %d", mat.Stock)
	}
}
