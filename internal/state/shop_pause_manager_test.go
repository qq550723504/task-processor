package state

import (
	"errors"
	"testing"
	"time"
)

type stubPauseStoreClient struct {
	resumeCalls int
	resumeErr   error
}

func (s *stubPauseStoreClient) SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error) {
	if pause {
		return true, nil
	}
	s.resumeCalls++
	if s.resumeErr != nil {
		return false, s.resumeErr
	}
	return true, nil
}

func (s *stubPauseStoreClient) GetStorePauseStatus(id int64) (bool, error) {
	return false, nil
}

func TestShopPauseManagerResumeShopRetriesFailedRemoteResume(t *testing.T) {
	manager := NewShopPauseManager()
	storeClient := &stubPauseStoreClient{resumeErr: errors.New("timeout")}
	manager.SetStoreClient(storeClient)

	manager.ResumeShop(246, 712)

	if storeClient.resumeCalls != 1 {
		t.Fatalf("expected first remote resume attempt, got %d", storeClient.resumeCalls)
	}

	storeClient.resumeErr = nil
	manager.CleanupExpired()

	if storeClient.resumeCalls != 2 {
		t.Fatalf("expected cleanup to retry remote resume, got %d calls", storeClient.resumeCalls)
	}
}

func TestShopPauseManagerCleanupExpiredRetriesFailedQuotaResume(t *testing.T) {
	manager := NewShopPauseManager()
	storeClient := &stubPauseStoreClient{resumeErr: errors.New("timeout")}
	manager.SetStoreClient(storeClient)

	manager.pauses["246:942"] = &ShopPauseInfo{
		Reason:    "达到每日限额",
		PausedAt:  time.Now().Add(-2 * time.Hour),
		ResumeAt:  time.Now().Add(-time.Minute),
		IsPaused:  true,
		PauseType: "quota_limit",
	}

	manager.CleanupExpired()
	if storeClient.resumeCalls != 1 {
		t.Fatalf("expected cleanup to attempt remote resume once, got %d", storeClient.resumeCalls)
	}

	storeClient.resumeErr = nil
	manager.CleanupExpired()

	if storeClient.resumeCalls != 2 {
		t.Fatalf("expected cleanup to retry failed remote resume, got %d calls", storeClient.resumeCalls)
	}
}
