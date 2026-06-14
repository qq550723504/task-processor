package submission

import (
	"context"
	"errors"
	"testing"
)

func TestFailurePersistenceServicePersistFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("record failed")
	service := NewFailurePersistenceService(FailurePersistenceServiceConfig[string, string]{
		RecordFailure: func(_ context.Context, in FailurePersistenceInput[string, string]) error {
			if in.TaskID != "task-1" || in.Result != "result" || in.Package != "pkg" || in.Action != "publish" || in.RequestID != "req-1" || in.Phase != "submit_remote" {
				t.Fatalf("unexpected failure input: %+v", in)
			}
			if in.Err == nil || in.Err.Error() != "remote rejected" {
				t.Fatalf("err = %v, want remote rejected", in.Err)
			}
			return expectedErr
		},
	})

	err := service.PersistFailure(context.Background(), FailurePersistenceInput[string, string]{
		TaskID:    "task-1",
		Result:    "result",
		Package:   "pkg",
		Action:    "publish",
		RequestID: "req-1",
		Phase:     "submit_remote",
		Err:       errors.New("remote rejected"),
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("PersistFailure() error = %v, want %v", err, expectedErr)
	}
}
