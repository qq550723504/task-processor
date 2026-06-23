package consumer

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

func TestStaleQueuedWatchdogRunOnceRecoversQueuedTasks(t *testing.T) {
	repo := &stubStaleQueuedRepository{
		count: 2,
		tasks: []listingadmin.ImportTask{
			{ID: 201, Status: model.TaskStatusQueued.Int16()},
			{ID: 202, Status: model.TaskStatusQueued.Int16()},
		},
		recovered: 2,
	}
	watchdog := NewStaleQueuedWatchdog(StaleQueuedWatchdogConfig{
		Enabled:        true,
		Interval:       time.Minute,
		TimeoutMinutes: 120,
		RecoveryLimit:  500,
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
	if len(repo.recoveredIDs) != 2 || repo.recoveredIDs[0] != 201 || repo.recoveredIDs[1] != 202 {
		t.Fatalf("recovered IDs = %v, want [201 202]", repo.recoveredIDs)
	}
	if repo.recovery.ReasonCode != staleQueuedReasonCode || repo.recovery.Stage != staleQueuedStage {
		t.Fatalf("recovery metadata = %+v, want stale queued reason/stage", repo.recovery)
	}
}

type stubStaleQueuedRepository struct {
	count        int64
	tasks        []listingadmin.ImportTask
	recovered    int
	countCalls   int
	listCalls    int
	recoverCalls int
	recoveredIDs []int64
	recovery     listingadmin.StaleQueuedRecovery
}

func (s *stubStaleQueuedRepository) CountStaleQueuedTasks(_ context.Context, _ time.Time) (int64, error) {
	s.countCalls++
	return s.count, nil
}

func (s *stubStaleQueuedRepository) ListStaleQueuedTasks(_ context.Context, _ time.Time, _ int) ([]listingadmin.ImportTask, error) {
	s.listCalls++
	return s.tasks, nil
}

func (s *stubStaleQueuedRepository) RecoverStaleQueuedTasks(_ context.Context, ids []int64, recovery listingadmin.StaleQueuedRecovery) (int, error) {
	s.recoverCalls++
	s.recoveredIDs = append([]int64(nil), ids...)
	s.recovery = recovery
	return s.recovered, nil
}
