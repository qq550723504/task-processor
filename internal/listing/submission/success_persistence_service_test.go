package submission

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSuccessPersistenceServicePersistSuccess(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	var calls []string
	service := NewSuccessPersistenceService(SuccessPersistenceServiceConfig[string, string, string]{
		PersistResultAndPhase: func(context.Context, SuccessPersistenceInput[string, string, string]) error {
			calls = append(calls, "persist_result")
			return nil
		},
		CompleteAttempt: func(in SuccessPersistenceInput[string, string, string], gotFinishedAt time.Time) {
			calls = append(calls, "complete_attempt")
			if in.TaskID != "task-1" || in.Action != "publish" || in.RequestID != "req-1" || in.Response != "response" {
				t.Fatalf("unexpected success input: %+v", in)
			}
			if !gotFinishedAt.Equal(finishedAt) {
				t.Fatalf("finishedAt = %v, want %v", gotFinishedAt, finishedAt)
			}
		},
		RememberSubmitted: func(task string, action string) {
			calls = append(calls, "remember")
			if task != "task" || action != "publish" {
				t.Fatalf("remember args = %q/%q", task, action)
			}
		},
		PersistSuccessfulSubmission: func(context.Context, string, string, string) error {
			calls = append(calls, "persist_success")
			return nil
		},
		CurrentTime: func() time.Time { return finishedAt },
	})

	err := service.PersistSuccess(context.Background(), SuccessPersistenceInput[string, string, string]{
		TaskID:    "task-1",
		Task:      "task",
		Package:   "pkg",
		Action:    "publish",
		RequestID: "req-1",
		Response:  "response",
	})
	if err != nil {
		t.Fatalf("PersistSuccess() error = %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"persist_result", "complete_attempt", "remember", "persist_success"}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestSuccessPersistenceServiceReturnsPersistError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("save failed")
	service := NewSuccessPersistenceService(SuccessPersistenceServiceConfig[string, string, string]{
		PersistResultAndPhase: func(context.Context, SuccessPersistenceInput[string, string, string]) error {
			return expectedErr
		},
		CompleteAttempt: func(SuccessPersistenceInput[string, string, string], time.Time) {
			t.Fatal("CompleteAttempt should not be called")
		},
	})

	err := service.PersistSuccess(context.Background(), SuccessPersistenceInput[string, string, string]{TaskID: "task-1"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("PersistSuccess() error = %v, want %v", err, expectedErr)
	}
}
