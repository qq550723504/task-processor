package listingkit

import (
	"context"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestTaskSubmissionStateServicePersistSheinSubmitPhasePersistsRunningPhase(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	state := newTaskSubmissionStateService(taskSubmissionStateServiceConfig{
		repo: repo,
	})

	err := state.persistSheinSubmitPhase(
		context.Background(),
		task.ID,
		task.Result,
		task.Result.Shein,
		"publish",
		"req-1",
		sheinpub.SubmissionPhasePrepareProduct,
	)
	if err != nil {
		t.Fatalf("persistSheinSubmitPhase() error = %v", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.Submission == nil {
		t.Fatalf("saved result = %+v, want submission state", saved.Result)
	}
	if saved.Result.Shein.Submission.CurrentPhase != sheinpub.SubmissionPhasePrepareProduct {
		t.Fatalf("current phase = %q, want %q", saved.Result.Shein.Submission.CurrentPhase, sheinpub.SubmissionPhasePrepareProduct)
	}
	if len(saved.Result.Shein.SubmissionEvents) == 0 {
		t.Fatal("expected phase event to be persisted")
	}
	lastEvent := saved.Result.Shein.SubmissionEvents[0]
	if lastEvent.Phase != sheinpub.SubmissionPhasePrepareProduct || lastEvent.Status != sheinpub.SubmissionStatusRunning {
		t.Fatalf("last event = %+v, want running prepare-product event", lastEvent)
	}
}
