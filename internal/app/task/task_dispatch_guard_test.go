package task

import (
	"fmt"
	"testing"

	"task-processor/internal/listingruntime"
)

type stubGuardStoreClient struct {
	store          *listingruntime.StoreInfo
	paused         bool
	pauseDetail    *listingruntime.StorePauseStatusDetail
	storeErr       error
	pauseErr       error
	pauseDetailErr error
	setPauseCalls  []bool
}

type stubDailyListingCountReader struct {
	count *DailyListingCount
	err   error
}

func (s stubDailyListingCountReader) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*DailyListingCount, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.count, nil
}

func (s *stubGuardStoreClient) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	if s.storeErr != nil {
		return nil, s.storeErr
	}
	if s.store != nil {
		return s.store, nil
	}
	return &listingruntime.StoreInfo{ID: storeID, Platform: "temu", Name: "demo"}, nil
}

func (s *stubGuardStoreClient) GetStorePauseStatus(storeID int64) (bool, error) {
	if s.pauseErr != nil {
		return false, s.pauseErr
	}
	return s.paused, nil
}

func (s *stubGuardStoreClient) GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
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
		store:  &listingruntime.StoreInfo{ID: 1, Platform: "shein", Name: "paused-shop"},
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
		store: &listingruntime.StoreInfo{ID: 2, Platform: "temu", Name: "active-shop"},
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
	storeClient := &stubGuardStoreClient{
		store:  &listingruntime.StoreInfo{ID: 7, TenantID: 246, Platform: "shein", Name: "quota-shop", DailyLimit: &limit},
		paused: true,
		pauseDetail: &listingruntime.StorePauseStatusDetail{
			Paused:    true,
			PauseType: "",
			Reason:    "quota_limit",
		},
	}
	fetcher := &TaskFetcher{listingCountReader: stubDailyListingCountReader{count: &DailyListingCount{Count: 2}}}
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
	storeClient := &stubGuardStoreClient{
		store:  &listingruntime.StoreInfo{ID: 8, TenantID: 246, Platform: "shein", Name: "quota-shop", DailyLimit: &limit},
		paused: true,
		pauseDetail: &listingruntime.StorePauseStatusDetail{
			Paused:    true,
			PauseType: "quota_limit",
		},
	}
	fetcher := &TaskFetcher{listingCountReader: stubDailyListingCountReader{err: fmt.Errorf("boom")}}
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
