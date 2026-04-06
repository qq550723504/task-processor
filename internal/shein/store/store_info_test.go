package store

import (
	"context"
	"sync"
	"testing"

	managementAPI "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	sheinctx "task-processor/internal/shein/context"
)

type mockStoreClient struct {
	store *managementAPI.StoreRespDTO
	err   error
}

func (m *mockStoreClient) GetStore(id int64) (*managementAPI.StoreRespDTO, error) {
	return m.store, m.err
}

func boolPtr(v bool) *bool {
	return &v
}

func TestStoreInfoHandlerSyncsTenantIDFromStoreInfo(t *testing.T) {
	storeCache = sync.Map{}

	handler := NewStoreInfoHandler(&mockStoreClient{
		store: &managementAPI.StoreRespDTO{
			ID:                181,
			TenantID:          227,
			Name:              "test-store",
			EnableAutoListing: boolPtr(true),
		},
	})

	task := &model.Task{
		ID:       1001,
		TenantID: 246,
		StoreID:  181,
	}
	ctx := sheinctx.NewTaskContext(context.Background(), task)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if ctx.Task.TenantID != 227 {
		t.Fatalf("tenant id not synced, got %d want %d", ctx.Task.TenantID, 227)
	}
	if ctx.StoreInfo == nil || ctx.StoreInfo.TenantID != 227 {
		t.Fatalf("store info tenant id not loaded correctly")
	}
}
