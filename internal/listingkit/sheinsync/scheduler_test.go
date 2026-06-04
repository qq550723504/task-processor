package sheinsync

import (
	"context"
	"errors"
	"testing"
)

type stubSheinSchedulerSyncService struct {
	calls       *[]string
	job         *SheinSyncJobRecord
	err         error
	tenantID    int64
	storeID     int64
	triggerMode SheinSyncTriggerMode
}

func (s *stubSheinSchedulerSyncService) SyncSheinOnShelfProducts(_ context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	if s.calls != nil {
		*s.calls = append(*s.calls, "sync")
	}
	s.tenantID = tenantID
	s.storeID = storeID
	s.triggerMode = triggerMode
	return s.job, s.err
}

func (s *stubSheinSchedulerSyncService) ListSyncedProducts(context.Context, *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	return nil, 0, nil
}

func (s *stubSheinSchedulerSyncService) UpdateManualCostPrice(context.Context, int64, *float64) error {
	return nil
}

type stubSheinSchedulerCandidateService struct {
	calls        *[]string
	result       *SheinCandidateRefreshResult
	err          error
	tenantID     int64
	storeID      int64
	activityType string
}

func (s *stubSheinSchedulerCandidateService) RefreshCandidates(_ context.Context, tenantID, storeID int64, activityType string) (*SheinCandidateRefreshResult, error) {
	if s.calls != nil {
		*s.calls = append(*s.calls, "refresh")
	}
	s.tenantID = tenantID
	s.storeID = storeID
	s.activityType = activityType
	return s.result, s.err
}

func (s *stubSheinSchedulerCandidateService) ListCandidates(context.Context, *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error) {
	return nil, 0, nil
}

func (s *stubSheinSchedulerCandidateService) ReviewCandidate(context.Context, int64, int64, int64, SheinCandidateReviewStatus, *bool, *bool) (*SheinActivityCandidateRecord, error) {
	return nil, nil
}

type stubSheinSchedulerEnrollmentService struct {
	calls        *[]string
	run          *SheinActivityEnrollmentRunRecord
	err          error
	tenantID     int64
	storeID      int64
	activityType string
	activityKey  string
}

func (s *stubSheinSchedulerEnrollmentService) ExecuteSheinActivityEnrollment(context.Context, int64, int64, string, string, SheinEnrollmentRunTriggerMode, ...int64) (*SheinActivityEnrollmentRunRecord, error) {
	return nil, nil
}

func (s *stubSheinSchedulerEnrollmentService) ExecuteAutoSheinActivityEnrollment(_ context.Context, tenantID, storeID int64, activityType string, activityKey string) (*SheinActivityEnrollmentRunRecord, error) {
	if s.calls != nil {
		*s.calls = append(*s.calls, "enroll")
	}
	s.tenantID = tenantID
	s.storeID = storeID
	s.activityType = activityType
	s.activityKey = activityKey
	return s.run, s.err
}

func TestRunScheduledSheinEnrollmentSyncsThenRefreshesThenEnrolls(t *testing.T) {
	t.Parallel()

	calls := make([]string, 0, 3)
	syncSvc := &stubSheinSchedulerSyncService{calls: &calls, job: &SheinSyncJobRecord{}}
	candidateSvc := &stubSheinSchedulerCandidateService{calls: &calls, result: &SheinCandidateRefreshResult{EligibleCount: 2}}
	enrollmentSvc := &stubSheinSchedulerEnrollmentService{calls: &calls, run: &SheinActivityEnrollmentRunRecord{}}

	scheduler := NewSheinEnrollmentScheduler(syncSvc, candidateSvc, enrollmentSvc)
	err := scheduler.Run(context.Background(), 11, 22, "PROMOTION")
	if err != nil {
		t.Fatalf("run scheduler: %v", err)
	}

	want := []string{"sync", "refresh", "enroll"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
	}
	if syncSvc.triggerMode != SheinSyncTriggerModeSchedule {
		t.Fatalf("trigger mode = %q, want %q", syncSvc.triggerMode, SheinSyncTriggerModeSchedule)
	}
}

func TestRunScheduledSheinEnrollmentStopsWhenSyncFails(t *testing.T) {
	t.Parallel()

	calls := make([]string, 0, 3)
	syncSvc := &stubSheinSchedulerSyncService{calls: &calls, err: errors.New("sync failed")}
	candidateSvc := &stubSheinSchedulerCandidateService{calls: &calls}
	enrollmentSvc := &stubSheinSchedulerEnrollmentService{calls: &calls}

	scheduler := NewSheinEnrollmentScheduler(syncSvc, candidateSvc, enrollmentSvc)
	err := scheduler.Run(context.Background(), 1, 2, "PROMOTION")
	if err == nil || err.Error() != "sync failed" {
		t.Fatalf("err = %v, want sync failed", err)
	}
	if len(calls) != 1 || calls[0] != "sync" {
		t.Fatalf("calls = %v, want [sync]", calls)
	}
}
