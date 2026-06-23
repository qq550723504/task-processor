package consumer

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

func TestProcessingTimeoutWatchdogRunOnceRecoversTimedOutTasks(t *testing.T) {
	repo := &stubProcessingTimeoutRepository{
		count: 2,
		tasks: []listingadmin.ImportTask{
			{ID: 101, Status: model.TaskStatusProcessing.Int16()},
			{ID: 102, Status: model.TaskStatusProcessing.Int16()},
		},
		recovered: 2,
	}
	watchdog := NewProcessingTimeoutWatchdog(ProcessingTimeoutWatchdogConfig{
		Enabled:        true,
		Interval:       time.Minute,
		TimeoutMinutes: 30,
		RecoveryLimit:  100,
		Repository:     repo,
		Logger:         logrus.New(),
	})

	summary, err := watchdog.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if summary.Candidates != 2 || summary.Recovered != 2 {
		t.Fatalf("summary = %+v, want candidates=2 recovered=2", summary)
	}
	if len(repo.recoveredIDs) != 2 || repo.recoveredIDs[0] != 101 || repo.recoveredIDs[1] != 102 {
		t.Fatalf("recovered IDs = %v, want [101 102]", repo.recoveredIDs)
	}
	if repo.recovery.ReasonCode != processingTimeoutReasonCode || repo.recovery.Stage != processingTimeoutStage {
		t.Fatalf("recovery metadata = %+v, want processing timeout reason/stage", repo.recovery)
	}

	status := watchdog.GetStatus()
	if status["last_recovered"] != 2 {
		t.Fatalf("status last_recovered = %v, want 2", status["last_recovered"])
	}
}

func TestProcessingTimeoutWatchdogDisabledSkipsRepository(t *testing.T) {
	repo := &stubProcessingTimeoutRepository{count: 10}
	watchdog := NewProcessingTimeoutWatchdog(ProcessingTimeoutWatchdogConfig{
		Enabled:        false,
		Interval:       time.Minute,
		TimeoutMinutes: 30,
		RecoveryLimit:  100,
		Repository:     repo,
		Logger:         logrus.New(),
	})

	summary, err := watchdog.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if summary.Candidates != 0 || summary.Recovered != 0 {
		t.Fatalf("summary = %+v, want zero summary when disabled", summary)
	}
	if repo.countCalls != 0 || repo.listCalls != 0 || repo.recoverCalls != 0 {
		t.Fatalf("repo calls = count:%d list:%d recover:%d, want no calls", repo.countCalls, repo.listCalls, repo.recoverCalls)
	}
}

type stubProcessingTimeoutRepository struct {
	count        int64
	tasks        []listingadmin.ImportTask
	recovered    int
	countCalls   int
	listCalls    int
	recoverCalls int
	recoveredIDs []int64
	recovery     listingadmin.ProcessingTimeoutRecovery
}

func (s *stubProcessingTimeoutRepository) CountTimedOutProcessingTasks(_ context.Context, _ time.Time) (int64, error) {
	s.countCalls++
	return s.count, nil
}

func (s *stubProcessingTimeoutRepository) ListTimedOutProcessingTasks(_ context.Context, _ time.Time, _ int) ([]listingadmin.ImportTask, error) {
	s.listCalls++
	return s.tasks, nil
}

func (s *stubProcessingTimeoutRepository) RecoverTimedOutProcessingTasks(_ context.Context, ids []int64, recovery listingadmin.ProcessingTimeoutRecovery) (int, error) {
	s.recoverCalls++
	s.recoveredIDs = append([]int64(nil), ids...)
	s.recovery = recovery
	return s.recovered, nil
}
