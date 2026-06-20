package task

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
)

type stubGuardStoreClient struct {
	store          *StoreInfo
	paused         bool
	pauseDetail    *StorePauseStatusDetail
	storeErr       error
	pauseErr       error
	pauseDetailErr error
	setPauseCalls  []bool
}

func (s *stubGuardStoreClient) GetStore(storeID int64) (*StoreInfo, error) {
	if s.storeErr != nil {
		return nil, s.storeErr
	}
	if s.store != nil {
		return s.store, nil
	}
	return &StoreInfo{ID: storeID, Platform: "temu", Name: "demo"}, nil
}

func (s *stubGuardStoreClient) GetStorePauseStatus(storeID int64) (bool, error) {
	if s.pauseErr != nil {
		return false, s.pauseErr
	}
	return s.paused, nil
}

func (s *stubGuardStoreClient) GetStorePauseStatusDetail(storeID int64) (*StorePauseStatusDetail, error) {
	if s.pauseDetailErr != nil {
		return nil, s.pauseDetailErr
	}
	return s.pauseDetail, nil
}

func (s *stubGuardStoreClient) SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	s.setPauseCalls = append(s.setPauseCalls, pause)
	s.paused = pause
	if !pause && s.pauseDetail != nil {
		s.pauseDetail.Paused = false
	}
	return true, nil
}

func TestTaskDispatchGuardCheckPausedStore(t *testing.T) {
	fetcher := &TaskFetcher{}
	guard := NewTaskDispatchGuard(fetcher, &stubGuardStoreClient{
		store:  &StoreInfo{ID: 1, Platform: "shein", Name: "paused-shop"},
		paused: true,
	})

	storeInfo, isPaused, err := guard.Check(&ImportTaskRecord{
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
		store: &StoreInfo{ID: 2, Platform: "temu", Name: "active-shop"},
	})

	storeInfo, isPaused, err := guard.Check(&ImportTaskRecord{
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

	_, _, err := guard.Check(&ImportTaskRecord{
		ID:      3,
		StoreID: 3,
	})
	if err == nil {
		t.Fatal("Check should return pause status error")
	}
}

func TestTaskDispatchGuardCheckResumesStaleQuotaPause(t *testing.T) {
	limit := 5
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/store/get-daily-listing-count" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": 2,
		})
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("test-token", "1")

	storeClient := &stubGuardStoreClient{
		store:  &StoreInfo{ID: 7, TenantID: 246, Platform: "shein", Name: "quota-shop", DailyLimit: &limit},
		paused: true,
		pauseDetail: &StorePauseStatusDetail{
			Paused:    true,
			PauseType: "",
			Reason:    "quota_limit",
		},
	}
	fetcher := &TaskFetcher{managementClient: clientMgr}
	guard := NewTaskDispatchGuard(fetcher, storeClient)

	storeInfo, isPaused, err := guard.Check(&ImportTaskRecord{
		ID:        10,
		TenantID:  246,
		StoreID:   7,
		ProductID: "P-7",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if isPaused {
		t.Fatal("Check should resume stale quota pause and continue dispatch")
	}
	if storeInfo == nil || storeInfo.ID != 7 {
		t.Fatal("Check should return resolved store info after self-heal")
	}
	if len(storeClient.setPauseCalls) != 1 || storeClient.setPauseCalls[0] {
		t.Fatalf("expected one resume call, got %#v", storeClient.setPauseCalls)
	}
}

func TestTaskDispatchGuardCheckKeepsPausedWhenQuotaCountQueryFails(t *testing.T) {
	limit := 5
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	clientMgr := management.NewClientManager(&config.ManagementConfig{BaseURL: server.URL})
	clientMgr.GetClient()
	clientMgr.SetUserToken("test-token", "1")

	storeClient := &stubGuardStoreClient{
		store:  &StoreInfo{ID: 8, TenantID: 246, Platform: "shein", Name: "quota-shop", DailyLimit: &limit},
		paused: true,
		pauseDetail: &StorePauseStatusDetail{
			Paused:    true,
			PauseType: "quota_limit",
		},
	}
	fetcher := &TaskFetcher{managementClient: clientMgr}
	guard := NewTaskDispatchGuard(fetcher, storeClient)

	storeInfo, isPaused, err := guard.Check(&ImportTaskRecord{
		ID:        11,
		TenantID:  246,
		StoreID:   8,
		ProductID: "P-8",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !isPaused {
		t.Fatal("Check should keep paused when quota count query fails")
	}
	if storeInfo == nil || storeInfo.ID != 8 {
		t.Fatal("Check should return resolved store info")
	}
	if len(storeClient.setPauseCalls) != 0 {
		t.Fatalf("expected no resume call, got %#v", storeClient.setPauseCalls)
	}
}
