package task

import (
	"fmt"
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
)

type stubGuardStoreClient struct {
	store    *managementapi.StoreRespDTO
	paused   bool
	storeErr error
	pauseErr error
}

func (s *stubGuardStoreClient) GetStore(storeID int64) (*managementapi.StoreRespDTO, error) {
	if s.storeErr != nil {
		return nil, s.storeErr
	}
	if s.store != nil {
		return s.store, nil
	}
	return &managementapi.StoreRespDTO{ID: storeID, Platform: "temu", Name: "demo"}, nil
}

func (s *stubGuardStoreClient) GetStorePauseStatus(storeID int64) (bool, error) {
	if s.pauseErr != nil {
		return false, s.pauseErr
	}
	return s.paused, nil
}

func TestTaskDispatchGuardCheckPausedStore(t *testing.T) {
	fetcher := &TaskFetcher{}
	guard := NewTaskDispatchGuard(fetcher, &stubGuardStoreClient{
		store:  &managementapi.StoreRespDTO{ID: 1, Platform: "shein", Name: "paused-shop"},
		paused: true,
	})

	storeInfo, isPaused, err := guard.Check(&managementapi.ProductImportTaskRespDTO{
		ID:        1,
		StoreID:   1,
		ProductID: "P-1",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !isPaused {
		t.Fatal("Check should report paused store")
	}
	if storeInfo == nil || storeInfo.Platform != "shein" {
		t.Fatal("Check should return resolved store info")
	}
}

func TestTaskDispatchGuardCheckActiveStore(t *testing.T) {
	fetcher := &TaskFetcher{}
	guard := NewTaskDispatchGuard(fetcher, &stubGuardStoreClient{
		store: &managementapi.StoreRespDTO{ID: 2, Platform: "temu", Name: "active-shop"},
	})

	storeInfo, isPaused, err := guard.Check(&managementapi.ProductImportTaskRespDTO{
		ID:        2,
		StoreID:   2,
		ProductID: "P-2",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if isPaused {
		t.Fatal("Check should not report paused store")
	}
	if storeInfo == nil || storeInfo.Platform != "temu" {
		t.Fatal("Check should return active store info")
	}
}

func TestTaskDispatchGuardCheckPauseStatusError(t *testing.T) {
	fetcher := &TaskFetcher{}
	guard := NewTaskDispatchGuard(fetcher, &stubGuardStoreClient{
		pauseErr: fmt.Errorf("pause unavailable"),
	})

	_, _, err := guard.Check(&managementapi.ProductImportTaskRespDTO{
		ID:      3,
		StoreID: 3,
	})
	if err == nil {
		t.Fatal("Check should return pause status error")
	}
}
