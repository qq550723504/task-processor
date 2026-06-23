package listingcontrol

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"task-processor/internal/listingadmin"
)

func TestRecoveryCoordinatorDisabledSkipsRepositoryCalls(t *testing.T) {
	repo := &fakeRecoveryRepository{}
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:    false,
		Repository: repo,
	})

	summary, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if !reflect.DeepEqual(summary, RecoverySummary{}) {
		t.Fatalf("summary = %+v, want zero", summary)
	}
	if repo.totalCalls() != 0 {
		t.Fatalf("repository calls = %d, want 0", repo.totalCalls())
	}
}

func TestRecoveryCoordinatorProcessingTimeoutCountsListsAndRecoversSelectedTasks(t *testing.T) {
	now := time.Date(2026, 6, 23, 9, 30, 0, 0, time.UTC)
	repo := &fakeRecoveryRepository{
		processingCount: 7,
		processingTasks: []listingadmin.ImportTask{
			{ID: 101},
			{ID: 102},
		},
		processingRecovered: 2,
	}
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:                  true,
		ProcessingTimeoutEnabled: true,
		ProcessingTimeoutMinutes: 45,
		ProcessingRecoveryLimit:  25,
		Repository:               repo,
		Now:                      func() time.Time { return now },
	})

	summary, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if summary.ProcessingCandidates != 7 || summary.ProcessingRecovered != 2 {
		t.Fatalf("processing summary = %+v, want 7 candidates and 2 recovered", summary)
	}
	if !reflect.DeepEqual(summary.ProcessingTaskIDs, []int64{101, 102}) {
		t.Fatalf("processing task IDs = %v, want [101 102]", summary.ProcessingTaskIDs)
	}
	if repo.countProcessingCalls != 1 || !repo.countProcessingBefore.Equal(now.Add(-45*time.Minute)) {
		t.Fatalf("count processing before = %v calls=%d, want now-45m once", repo.countProcessingBefore, repo.countProcessingCalls)
	}
	if repo.listProcessingCalls != 1 || repo.listProcessingLimit != 25 || !repo.listProcessingBefore.Equal(now.Add(-45*time.Minute)) {
		t.Fatalf("list processing before=%v limit=%d calls=%d, want now-45m limit 25 once", repo.listProcessingBefore, repo.listProcessingLimit, repo.listProcessingCalls)
	}
	if repo.recoverProcessingCalls != 1 || !reflect.DeepEqual(repo.recoverProcessingIDs, []int64{101, 102}) {
		t.Fatalf("recover processing IDs = %v calls=%d, want [101 102] once", repo.recoverProcessingIDs, repo.recoverProcessingCalls)
	}
	wantRemark := "Recovered after processing timeout by listing control plane (45 minutes)"
	if repo.processingRecovery.TimeoutMinutes != 45 ||
		repo.processingRecovery.ErrorMessage != "Task processing lease expired, recovered by listing control plane" ||
		repo.processingRecovery.ReasonCode != "PROCESSING_TIMEOUT" ||
		repo.processingRecovery.Stage != "processing_timeout_recovery" ||
		repo.processingRecovery.Remark != wantRemark {
		t.Fatalf("processing recovery payload = %+v", repo.processingRecovery)
	}
}

func TestRecoveryCoordinatorStaleQueuedCountsListsAndRecoversSelectedTasks(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	repo := &fakeRecoveryRepository{
		staleQueuedCount: 3,
		staleQueuedTasks: []listingadmin.ImportTask{
			{ID: 201},
			{ID: 202},
			{ID: 203},
		},
		staleQueuedRecovered: 3,
	}
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:                   true,
		StaleQueuedEnabled:        true,
		StaleQueuedTimeoutMinutes: 90,
		StaleQueuedRecoveryLimit:  40,
		Repository:                repo,
		Now:                       func() time.Time { return now },
	})

	summary, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if summary.StaleQueuedCandidates != 3 || summary.StaleQueuedRecovered != 3 {
		t.Fatalf("stale queued summary = %+v, want 3 candidates and 3 recovered", summary)
	}
	if !reflect.DeepEqual(summary.StaleQueuedTaskIDs, []int64{201, 202, 203}) {
		t.Fatalf("stale queued task IDs = %v, want [201 202 203]", summary.StaleQueuedTaskIDs)
	}
	if repo.countStaleCalls != 1 || !repo.countStaleBefore.Equal(now.Add(-90*time.Minute)) {
		t.Fatalf("count stale before = %v calls=%d, want now-90m once", repo.countStaleBefore, repo.countStaleCalls)
	}
	if repo.listStaleCalls != 1 || repo.listStaleLimit != 40 || !repo.listStaleBefore.Equal(now.Add(-90*time.Minute)) {
		t.Fatalf("list stale before=%v limit=%d calls=%d, want now-90m limit 40 once", repo.listStaleBefore, repo.listStaleLimit, repo.listStaleCalls)
	}
	if repo.recoverStaleCalls != 1 || !reflect.DeepEqual(repo.recoverStaleIDs, []int64{201, 202, 203}) {
		t.Fatalf("recover stale IDs = %v calls=%d, want [201 202 203] once", repo.recoverStaleIDs, repo.recoverStaleCalls)
	}
	wantRemark := "Recovered from stale queued state by listing control plane (90 minutes)"
	if repo.staleRecovery.TimeoutMinutes != 90 ||
		repo.staleRecovery.ErrorMessage != "Task stayed queued too long, recovered by listing control plane" ||
		repo.staleRecovery.ReasonCode != "STALE_QUEUED" ||
		repo.staleRecovery.Stage != "queued_timeout_recovery" ||
		repo.staleRecovery.Remark != wantRemark {
		t.Fatalf("stale queued recovery payload = %+v", repo.staleRecovery)
	}
}

func TestRecoveryCoordinatorDoesNotRecoverWhenNoIDsAreListed(t *testing.T) {
	repo := &fakeRecoveryRepository{
		processingCount: 5,
	}
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:                  true,
		ProcessingTimeoutEnabled: true,
		Repository:               repo,
	})

	summary, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if summary.ProcessingCandidates != 5 || summary.ProcessingRecovered != 0 || len(summary.ProcessingTaskIDs) != 0 {
		t.Fatalf("summary = %+v, want candidates only", summary)
	}
	if repo.recoverProcessingCalls != 0 {
		t.Fatalf("recover processing calls = %d, want 0", repo.recoverProcessingCalls)
	}
}

func TestRecoveryCoordinatorReturnsRepositoryErrorWithPartialSummary(t *testing.T) {
	recoverErr := errors.New("recover failed")
	repo := &fakeRecoveryRepository{
		processingCount: 4,
		processingTasks: []listingadmin.ImportTask{
			{ID: 301},
		},
		recoverProcessingErr: recoverErr,
	}
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:                  true,
		ProcessingTimeoutEnabled: true,
		StaleQueuedEnabled:       true,
		Repository:               repo,
	})

	summary, err := coordinator.RunOnce(context.Background())
	if err == nil {
		t.Fatal("RunOnce returned nil error, want repository error")
	}
	if !errors.Is(err, recoverErr) || !strings.Contains(err.Error(), "recover timed out processing tasks") {
		t.Fatalf("error = %v, want wrapped recover error", err)
	}
	if summary.ProcessingCandidates != 4 || summary.ProcessingRecovered != 0 || !reflect.DeepEqual(summary.ProcessingTaskIDs, []int64{301}) {
		t.Fatalf("partial summary = %+v, want processing candidates and IDs preserved", summary)
	}
	if repo.countStaleCalls != 0 {
		t.Fatalf("stale queued calls = %d, want 0 after processing error", repo.countStaleCalls)
	}
}

func TestRecoveryCoordinatorDisabledSubRecoveriesSkipOnlyThatPart(t *testing.T) {
	t.Run("processing disabled", func(t *testing.T) {
		repo := &fakeRecoveryRepository{
			staleQueuedCount:     1,
			staleQueuedTasks:     []listingadmin.ImportTask{{ID: 401}},
			staleQueuedRecovered: 1,
		}
		coordinator := NewRecoveryCoordinator(RecoveryConfig{
			Enabled:                  true,
			ProcessingTimeoutEnabled: false,
			StaleQueuedEnabled:       true,
			Repository:               repo,
		})

		summary, err := coordinator.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("RunOnce returned error: %v", err)
		}
		if repo.countProcessingCalls != 0 || repo.listProcessingCalls != 0 || repo.recoverProcessingCalls != 0 {
			t.Fatalf("processing calls count/list/recover = %d/%d/%d, want 0/0/0", repo.countProcessingCalls, repo.listProcessingCalls, repo.recoverProcessingCalls)
		}
		if summary.StaleQueuedRecovered != 1 {
			t.Fatalf("summary = %+v, want stale queued recovery", summary)
		}
	})

	t.Run("stale queued disabled", func(t *testing.T) {
		repo := &fakeRecoveryRepository{
			processingCount:     1,
			processingTasks:     []listingadmin.ImportTask{{ID: 501}},
			processingRecovered: 1,
		}
		coordinator := NewRecoveryCoordinator(RecoveryConfig{
			Enabled:                  true,
			ProcessingTimeoutEnabled: true,
			StaleQueuedEnabled:       false,
			Repository:               repo,
		})

		summary, err := coordinator.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("RunOnce returned error: %v", err)
		}
		if repo.countStaleCalls != 0 || repo.listStaleCalls != 0 || repo.recoverStaleCalls != 0 {
			t.Fatalf("stale calls count/list/recover = %d/%d/%d, want 0/0/0", repo.countStaleCalls, repo.listStaleCalls, repo.recoverStaleCalls)
		}
		if summary.ProcessingRecovered != 1 {
			t.Fatalf("summary = %+v, want processing recovery", summary)
		}
	})
}

func TestRecoveryCoordinatorAppliesDefaultMinutesAndLimits(t *testing.T) {
	now := time.Date(2026, 6, 23, 11, 0, 0, 0, time.UTC)
	repo := &fakeRecoveryRepository{
		processingCount:      1,
		processingTasks:      []listingadmin.ImportTask{{ID: 601}},
		processingRecovered:  1,
		staleQueuedCount:     1,
		staleQueuedTasks:     []listingadmin.ImportTask{{ID: 701}},
		staleQueuedRecovered: 1,
	}
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:                  true,
		ProcessingTimeoutEnabled: true,
		StaleQueuedEnabled:       true,
		Repository:               repo,
		Now:                      func() time.Time { return now },
	})

	_, err := coordinator.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	if repo.listProcessingLimit != 100 || !repo.listProcessingBefore.Equal(now.Add(-30*time.Minute)) {
		t.Fatalf("processing before=%v limit=%d, want now-30m limit 100", repo.listProcessingBefore, repo.listProcessingLimit)
	}
	if repo.processingRecovery.TimeoutMinutes != 30 {
		t.Fatalf("processing recovery timeout = %d, want 30", repo.processingRecovery.TimeoutMinutes)
	}
	if repo.listStaleLimit != 500 || !repo.listStaleBefore.Equal(now.Add(-120*time.Minute)) {
		t.Fatalf("stale before=%v limit=%d, want now-120m limit 500", repo.listStaleBefore, repo.listStaleLimit)
	}
	if repo.staleRecovery.TimeoutMinutes != 120 {
		t.Fatalf("stale recovery timeout = %d, want 120", repo.staleRecovery.TimeoutMinutes)
	}
}

func TestRecoveryCoordinatorEnabledWithoutRepositoryReturnsClearError(t *testing.T) {
	coordinator := NewRecoveryCoordinator(RecoveryConfig{
		Enabled:                  true,
		ProcessingTimeoutEnabled: true,
	})

	_, err := coordinator.RunOnce(context.Background())
	if err == nil {
		t.Fatal("RunOnce returned nil error, want repository error")
	}
	if !strings.Contains(err.Error(), "recovery repository is nil") {
		t.Fatalf("error = %v, want clear nil repository error", err)
	}
}

type fakeRecoveryRepository struct {
	processingCount     int64
	processingTasks     []listingadmin.ImportTask
	processingRecovered int

	countProcessingCalls   int
	countProcessingBefore  time.Time
	countProcessingErr     error
	listProcessingCalls    int
	listProcessingBefore   time.Time
	listProcessingLimit    int
	listProcessingErr      error
	recoverProcessingCalls int
	recoverProcessingIDs   []int64
	processingRecovery     listingadmin.ProcessingTimeoutRecovery
	recoverProcessingErr   error

	staleQueuedCount     int64
	staleQueuedTasks     []listingadmin.ImportTask
	staleQueuedRecovered int

	countStaleCalls   int
	countStaleBefore  time.Time
	countStaleErr     error
	listStaleCalls    int
	listStaleBefore   time.Time
	listStaleLimit    int
	listStaleErr      error
	recoverStaleCalls int
	recoverStaleIDs   []int64
	staleRecovery     listingadmin.StaleQueuedRecovery
	recoverStaleErr   error
}

func (r *fakeRecoveryRepository) CountTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time) (int64, error) {
	r.countProcessingCalls++
	r.countProcessingBefore = timeoutBefore
	return r.processingCount, r.countProcessingErr
}

func (r *fakeRecoveryRepository) ListTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]listingadmin.ImportTask, error) {
	r.listProcessingCalls++
	r.listProcessingBefore = timeoutBefore
	r.listProcessingLimit = limit
	return r.processingTasks, r.listProcessingErr
}

func (r *fakeRecoveryRepository) RecoverTimedOutProcessingTasks(ctx context.Context, ids []int64, recovery listingadmin.ProcessingTimeoutRecovery) (int, error) {
	r.recoverProcessingCalls++
	r.recoverProcessingIDs = append([]int64(nil), ids...)
	r.processingRecovery = recovery
	return r.processingRecovered, r.recoverProcessingErr
}

func (r *fakeRecoveryRepository) CountStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time) (int64, error) {
	r.countStaleCalls++
	r.countStaleBefore = timeoutBefore
	return r.staleQueuedCount, r.countStaleErr
}

func (r *fakeRecoveryRepository) ListStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]listingadmin.ImportTask, error) {
	r.listStaleCalls++
	r.listStaleBefore = timeoutBefore
	r.listStaleLimit = limit
	return r.staleQueuedTasks, r.listStaleErr
}

func (r *fakeRecoveryRepository) RecoverStaleQueuedTasks(ctx context.Context, ids []int64, recovery listingadmin.StaleQueuedRecovery) (int, error) {
	r.recoverStaleCalls++
	r.recoverStaleIDs = append([]int64(nil), ids...)
	r.staleRecovery = recovery
	return r.staleQueuedRecovered, r.recoverStaleErr
}

func (r *fakeRecoveryRepository) totalCalls() int {
	return r.countProcessingCalls + r.listProcessingCalls + r.recoverProcessingCalls +
		r.countStaleCalls + r.listStaleCalls + r.recoverStaleCalls
}
