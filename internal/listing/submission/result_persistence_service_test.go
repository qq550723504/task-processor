package submission

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResultPersistenceServiceFinishPersistsSuccess(t *testing.T) {
	t.Parallel()

	var gotTaskID string
	var completed bool
	var remembered bool
	var persisted bool
	service := NewResultPersistenceService(ResultPersistenceServiceConfig[string, string, string, string]{
		SuccessRunner: NewSuccessPersistenceService(SuccessPersistenceServiceConfig[string, string, string]{
			PersistResultAndPhase: func(context.Context, SuccessPersistenceInput[string, string, string]) error {
				persisted = true
				return nil
			},
			CompleteAttempt: func(in SuccessPersistenceInput[string, string, string], _ time.Time) {
				gotTaskID = in.TaskID
				completed = true
			},
			RememberSubmitted: func(string, string) {
				remembered = true
			},
		}),
		BuildSuccessInput: func(in ResultPersistenceInput[string, string, string, string]) SuccessPersistenceInput[string, string, string] {
			return SuccessPersistenceInput[string, string, string]{
				TaskID:    in.TaskID,
				Task:      in.Task,
				Package:   in.Package,
				Action:    in.Action,
				RequestID: in.RequestID,
				Response:  in.Response,
				StartedAt: in.StartedAt,
			}
		},
	})

	err := service.Finish(context.Background(), ResultPersistenceInput[string, string, string, string]{
		TaskID:    "task-1",
		Task:      "task",
		Package:   "pkg",
		Action:    "publish",
		RequestID: "req-1",
		Response:  "ok",
	})
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if gotTaskID != "task-1" || !completed || !remembered || !persisted {
		t.Fatalf("success runner not fully invoked: taskID=%q completed=%v remembered=%v persisted=%v", gotTaskID, completed, remembered, persisted)
	}
}

func TestResultPersistenceServiceFinishReturnsOriginalFailureAfterPersistence(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("remote failed")
	var beforeFailureCalled bool
	var gotFailure FailurePersistenceInput[string, string]
	service := NewResultPersistenceService(ResultPersistenceServiceConfig[string, string, string, string]{
		FailureRunner: NewFailurePersistenceService(FailurePersistenceServiceConfig[string, string]{
			RecordFailure: func(_ context.Context, in FailurePersistenceInput[string, string]) error {
				gotFailure = in
				return nil
			},
		}),
		BuildFailureInput: func(in ResultPersistenceInput[string, string, string, string]) FailurePersistenceInput[string, string] {
			return FailurePersistenceInput[string, string]{
				TaskID:    in.TaskID,
				Result:    in.Result,
				Package:   in.Package,
				Action:    in.Action,
				RequestID: in.RequestID,
				Phase:     in.Phase,
				Err:       in.Err,
			}
		},
		BeforeFailure: func(ResultPersistenceInput[string, string, string, string]) {
			beforeFailureCalled = true
		},
		ReturnOriginalFailure: true,
	})

	err := service.Finish(context.Background(), ResultPersistenceInput[string, string, string, string]{
		TaskID:    "task-2",
		Result:    "result",
		Package:   "pkg",
		Action:    "publish",
		RequestID: "req-2",
		Phase:     "persist_result",
		Err:       originalErr,
	})
	if !errors.Is(err, originalErr) {
		t.Fatalf("Finish() error = %v, want %v", err, originalErr)
	}
	if !beforeFailureCalled {
		t.Fatal("expected before failure hook to run")
	}
	if gotFailure.TaskID != "task-2" || gotFailure.Result != "result" || !errors.Is(gotFailure.Err, originalErr) {
		t.Fatalf("failure input = %+v", gotFailure)
	}
}

func TestResultPersistenceServicePersistSuccessFallsBackWithoutRunner(t *testing.T) {
	t.Parallel()

	var fellBack bool
	service := NewResultPersistenceService(ResultPersistenceServiceConfig[string, string, string, string]{
		FallbackSuccess: func(context.Context, ResultPersistenceInput[string, string, string, string]) error {
			fellBack = true
			return nil
		},
	})

	if err := service.PersistSuccess(context.Background(), ResultPersistenceInput[string, string, string, string]{}); err != nil {
		t.Fatalf("PersistSuccess() error = %v", err)
	}
	if !fellBack {
		t.Fatal("expected success fallback to run")
	}
}

func TestResultPersistenceServicePersistFailureUsesFallbackWithoutReturningOriginalWhenDisabled(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("remote warning")
	var fellBack bool
	service := NewResultPersistenceService(ResultPersistenceServiceConfig[string, string, string, string]{
		FallbackFailure: func(context.Context, ResultPersistenceInput[string, string, string, string]) error {
			fellBack = true
			return nil
		},
	})

	if err := service.PersistFailure(context.Background(), ResultPersistenceInput[string, string, string, string]{Err: originalErr}); err != nil {
		t.Fatalf("PersistFailure() error = %v", err)
	}
	if !fellBack {
		t.Fatal("expected failure fallback to run")
	}
}
