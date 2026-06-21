package submission

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeRequeueTaskIDs(t *testing.T) {
	t.Parallel()

	got := NormalizeRequeueTaskIDs(&RequeueRequest{TaskIDs: []string{" task-1 ", "", "task-1", "task-2"}})
	if len(got) != 2 || got[0] != "task-1" || got[1] != "task-2" {
		t.Fatalf("NormalizeRequeueTaskIDs() = %#v", got)
	}
}

func TestCanRequeueTaskWithStatus(t *testing.T) {
	t.Parallel()

	if allowed, reason := CanRequeueTaskWithStatus(nil, "pending"); allowed || reason != `task status "" is not processable` {
		t.Fatalf("nil task = (%v, %q)", allowed, reason)
	}

	if allowed, reason := CanRequeueTaskWithStatus(&RequeueTask{ID: "task-review", Status: "needs_review"}, "pending"); allowed || reason != `task status "needs_review" is not processable` {
		t.Fatalf("needs_review = (%v, %q)", allowed, reason)
	}

	if allowed, reason := CanRequeueTaskWithStatus(&RequeueTask{ID: "task-pending", Status: "pending"}, "pending"); !allowed || reason != "" {
		t.Fatalf("pending = (%v, %q)", allowed, reason)
	}
}

func TestRequeueServiceRequeueTasks(t *testing.T) {
	t.Parallel()

	errUnavailable := errors.New("submitter unavailable")
	errInvalid := errors.New("invalid request")
	errNotFound := errors.New("task not found")

	t.Run("requeues only processable tasks", func(t *testing.T) {
		t.Parallel()

		submitted := make([]string, 0, 1)
		service := NewRequeueService(RequeueServiceConfig{
			LoadTask: func(_ context.Context, taskID string) (*RequeueTask, error) {
				switch taskID {
				case "task-pending":
					return &RequeueTask{ID: taskID, Status: "pending"}, nil
				case "task-review":
					return &RequeueTask{ID: taskID, Status: "needs_review"}, nil
				default:
					return nil, errNotFound
				}
			},
			CurrentSubmitter: func() RequeueSubmitFunc {
				return func(taskID string) error {
					submitted = append(submitted, taskID)
					return nil
				}
			},
			IsTaskNotFound: func(err error) bool { return errors.Is(err, errNotFound) },
			CanRequeue: func(task *RequeueTask) (bool, string) {
				return CanRequeueTaskWithStatus(task, "pending")
			},
			SubmitTask: func(_ context.Context, submit RequeueSubmitFunc, taskID string) error {
				return submit(taskID)
			},
			ErrUnavailable:    errUnavailable,
			ErrInvalidRequest: errInvalid,
		})

		result, err := service.RequeueTasks(context.Background(), &RequeueRequest{
			TaskIDs: []string{"task-pending", "task-review", "task-missing", "task-pending"},
		})
		if err != nil {
			t.Fatalf("RequeueTasks() error = %v", err)
		}
		if len(result.RequeuedTaskIDs) != 1 || result.RequeuedTaskIDs[0] != "task-pending" {
			t.Fatalf("RequeuedTaskIDs = %#v", result.RequeuedTaskIDs)
		}
		if len(submitted) != 1 || submitted[0] != "task-pending" {
			t.Fatalf("submitted = %#v", submitted)
		}
		if len(result.Skipped) != 2 {
			t.Fatalf("Skipped = %#v", result.Skipped)
		}
	})

	t.Run("collects submit failures", func(t *testing.T) {
		t.Parallel()

		service := NewRequeueService(RequeueServiceConfig{
			LoadTask: func(_ context.Context, taskID string) (*RequeueTask, error) {
				return &RequeueTask{ID: taskID, Status: "pending"}, nil
			},
			CurrentSubmitter: func() RequeueSubmitFunc {
				return func(string) error { return errors.New("submit failed") }
			},
			CanRequeue:        func(task *RequeueTask) (bool, string) { return true, "" },
			SubmitTask:        func(_ context.Context, submit RequeueSubmitFunc, taskID string) error { return submit(taskID) },
			ErrUnavailable:    errUnavailable,
			ErrInvalidRequest: errInvalid,
		})

		result, err := service.RequeueTasks(context.Background(), &RequeueRequest{TaskIDs: []string{"task-1"}})
		if err != nil {
			t.Fatalf("RequeueTasks() error = %v", err)
		}
		if len(result.Failed) != 1 || result.Failed[0].TaskID != "task-1" {
			t.Fatalf("Failed = %#v", result.Failed)
		}
	})

	t.Run("rejects unavailable submitter", func(t *testing.T) {
		t.Parallel()

		service := NewRequeueService(RequeueServiceConfig{
			LoadTask: func(_ context.Context, taskID string) (*RequeueTask, error) {
				return &RequeueTask{ID: taskID, Status: "pending"}, nil
			},
			ErrUnavailable:    errUnavailable,
			ErrInvalidRequest: errInvalid,
		})

		if _, err := service.RequeueTasks(context.Background(), &RequeueRequest{TaskIDs: []string{"task-1"}}); !errors.Is(err, errUnavailable) {
			t.Fatalf("RequeueTasks() error = %v, want %v", err, errUnavailable)
		}
	})
}
