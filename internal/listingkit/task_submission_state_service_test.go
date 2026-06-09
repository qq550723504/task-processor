package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestTaskSubmissionStateServicePersistSuccessfulSheinDirectResponsePersistsRemoteResponseAndPhase(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "req-success", sheinpub.SubmissionPhaseSubmitRemote, startedAt)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	state := newTaskSubmissionStateService(taskSubmissionStateServiceConfig{
		repo: repo,
	})
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-123",
	}
	opts := sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-success",
		startedAt: startedAt,
	}

	if err := state.persistSuccessfulSheinDirectResponse(context.Background(), task.ID, task, task.Result.Shein, opts, "SUP-1", response); err != nil {
		t.Fatalf("persistSuccessfulSheinDirectResponse() error = %v", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	record := sheinSubmissionRecordForAction(saved.Result.Shein.Submission, "publish")
	if record == nil {
		t.Fatalf("publish record = nil, want persisted record")
	}
	if record.Result == nil || record.Result.SPUName != "SPU-123" || !record.Result.Success {
		t.Fatalf("publish result = %+v, want persisted remote response", record.Result)
	}
	if record.SupplierCode != "SUP-1" {
		t.Fatalf("supplier code = %q, want SUP-1", record.SupplierCode)
	}
	if saved.Result.Shein.Submission.LastResult == nil || saved.Result.Shein.Submission.LastResult.SPUName != "SPU-123" {
		t.Fatalf("last result = %+v, want persisted response summary", saved.Result.Shein.Submission.LastResult)
	}
	if saved.Result.Shein.Submission.CurrentPhase != sheinpub.SubmissionPhasePersistResult {
		t.Fatalf("current phase = %q, want %q", saved.Result.Shein.Submission.CurrentPhase, sheinpub.SubmissionPhasePersistResult)
	}
	if !repo.hasSavedSubmissionPhase(sheinpub.SubmissionPhasePersistResult) {
		t.Fatalf("saved phases = %+v, want persist_result", repo.savedSubmissionPhases)
	}
	if len(saved.Result.Shein.SubmissionEvents) == 0 {
		t.Fatal("expected submission events to be persisted")
	}
	if saved.Result.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhasePersistResult || saved.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionStatusRunning {
		t.Fatalf("last event = %+v, want running persist_result event", saved.Result.Shein.SubmissionEvents[0])
	}
}

func TestTaskSubmissionStateServiceFinishSheinDirectSubmitAttemptReturnsSubmitErrorAfterPersistence(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	startedAt := time.Now().Add(-2 * time.Minute)
	beginSheinSubmitAttempt(task.Result.Shein, "publish", "req-finish", sheinpub.SubmissionPhasePersistResult, startedAt)
	response := &sheinpub.SubmissionResponse{
		Code:    "0",
		Message: "success",
		Success: true,
		SPUName: "SPU-456",
	}
	setSheinSubmitRemoteResponse(task.Result.Shein, "publish", "req-finish", "SUP-2", response)
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var rememberedAction string
	var rememberedTaskID string
	state := newTaskSubmissionStateService(taskSubmissionStateServiceConfig{
		repo: repo,
		rememberSheinSubmitted: func(gotTask *Task, action string) {
			rememberedTaskID = gotTask.ID
			rememberedAction = action
		},
	})
	submitErr := errors.New("confirm remote later")
	opts := sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-finish",
		startedAt: startedAt,
	}

	err := state.finishSheinDirectSubmitAttempt(context.Background(), task.ID, task, task.Result.Shein, opts, response, submitErr)
	if !errors.Is(err, submitErr) {
		t.Fatalf("finishSheinDirectSubmitAttempt() err = %v, want %v", err, submitErr)
	}
	if rememberedAction != "" || rememberedTaskID != "" {
		t.Fatalf("remember called = %q/%q, want no remember on failed attempt", rememberedTaskID, rememberedAction)
	}

	saved, getErr := repo.GetTask(context.Background(), task.ID)
	if getErr != nil {
		t.Fatalf("get task: %v", getErr)
	}
	if saved.Result.Shein.Submission.LastStatus != sheinpub.SubmissionStatusFailed {
		t.Fatalf("last status = %q, want failed", saved.Result.Shein.Submission.LastStatus)
	}
	if saved.Result.Shein.Submission.CurrentPhase != "" || saved.Result.Shein.Submission.CurrentAction != "" || saved.Result.Shein.Submission.CurrentRequestID != "" {
		t.Fatalf("current submit state = %+v, want cleared in-flight fields", saved.Result.Shein.Submission)
	}
	if len(saved.Result.Shein.SubmissionEvents) == 0 {
		t.Fatal("expected completion event to be appended")
	}
	if saved.Result.Shein.SubmissionEvents[0].Status != sheinpub.SubmissionStatusFailed || saved.Result.Shein.SubmissionEvents[0].Phase != sheinpub.SubmissionPhasePersistResult {
		t.Fatalf("completion event = %+v, want failed persist_result event", saved.Result.Shein.SubmissionEvents[0])
	}
}
