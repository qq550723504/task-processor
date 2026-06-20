package store

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	sheinother "task-processor/internal/shein/api/other"
	shein "task-processor/internal/shein/context"
)

type stubStoreIDRepo struct {
	updateIDCalls     int
	updateStatusCalls int
	lastStoreID       string
	lastStatus        int16
	lastRemark        string
	lastTenantID      int64
}

func (s *stubStoreIDRepo) UpdateStoreID(_ context.Context, _ int64, storeID string) (*listingadmin.Store, error) {
	s.updateIDCalls++
	s.lastStoreID = storeID
	return &listingadmin.Store{StoreID: storeID}, nil
}

func (s *stubStoreIDRepo) UpdateStoreStatus(_ context.Context, tenantID, _ int64, status int16, remark string) (*listingadmin.Store, error) {
	s.updateStatusCalls++
	s.lastTenantID = tenantID
	s.lastStatus = status
	s.lastRemark = remark
	return &listingadmin.Store{Status: status, Remark: remark}, nil
}

func TestStoreIDHandlerUpdatesMissingStoreID(t *testing.T) {
	repo := &stubStoreIDRepo{}
	handler := NewStoreIDHandler(repo)
	taskCtx := &shein.TaskContext{
		RuntimeState: shein.RuntimeState{
			Context:   context.Background(),
			Task:      &model.Task{TenantID: 12, StoreID: 34},
			StoreInfo: &listingruntime.StoreInfo{ID: 34},
		},
		ProductState: shein.ProductState{
			SupplierInfo: &sheinother.SupplierOperateInfo{StoreID: 5566},
		},
	}

	if err := handler.Handle(taskCtx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repo.updateIDCalls != 1 {
		t.Fatalf("UpdateStoreID calls = %d, want 1", repo.updateIDCalls)
	}
	if repo.lastStoreID != "5566" {
		t.Fatalf("updated store id = %q, want 5566", repo.lastStoreID)
	}
}

func TestStoreIDHandlerDisablesMismatchedStore(t *testing.T) {
	repo := &stubStoreIDRepo{}
	handler := NewStoreIDHandler(repo)
	taskCtx := &shein.TaskContext{
		RuntimeState: shein.RuntimeState{
			Context:   context.Background(),
			Task:      &model.Task{TenantID: 88, StoreID: 34},
			StoreInfo: &listingruntime.StoreInfo{ID: 34, StoreID: "1234"},
		},
		ProductState: shein.ProductState{
			SupplierInfo: &sheinother.SupplierOperateInfo{StoreID: 5566},
		},
	}

	if err := handler.Handle(taskCtx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repo.updateStatusCalls != 1 {
		t.Fatalf("UpdateStoreStatus calls = %d, want 1", repo.updateStatusCalls)
	}
	if repo.lastTenantID != 88 || repo.lastStatus != 1 {
		t.Fatalf("unexpected status update: tenant=%d status=%d", repo.lastTenantID, repo.lastStatus)
	}
}
