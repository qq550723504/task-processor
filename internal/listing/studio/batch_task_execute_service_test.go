package studio

import (
	"context"
	"errors"
	"testing"
)

func TestBatchTaskExecuteServiceExecuteReusesExistingAndFinalizes(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchTaskExecuteService(BatchTaskExecuteServiceConfig[string, string, string, string, []string]{
		LoadSession: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "session:"+batchID)
			value := "session"
			return &value, nil
		},
		LoadItems: func(ctx context.Context, batchID string, designIDs []string) ([]string, error) {
			calls = append(calls, "items:"+batchID)
			return []string{"existing", "new"}, nil
		},
		FindExisting: func(ctx context.Context, session *string, candidate string) (string, bool) {
			calls = append(calls, "find:"+candidate)
			if candidate == "existing" {
				return "task-existing", true
			}
			return "", false
		},
		CreateTask: func(ctx context.Context, candidate string) (string, error) {
			calls = append(calls, "create:"+candidate)
			return "task-" + candidate, nil
		},
		BuildFailed: func(candidate string, err error) string {
			return candidate + ":" + err.Error()
		},
		Finalize: func(ctx context.Context, batchID string, session *string, created []string, failed []string) (*[]string, error) {
			calls = append(calls, "finalize:"+batchID)
			result := append([]string(nil), created...)
			return &result, nil
		},
	})

	result, err := service.Execute(context.Background(), " batch-1 ", []string{"design-1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || len(*result) != 2 || (*result)[0] != "task-existing" || (*result)[1] != "task-new" {
		t.Fatalf("Execute() result = %#v", result)
	}
	if got, want := calls, []string{"session:batch-1", "items:batch-1", "find:existing", "find:new", "create:new", "finalize:batch-1"}; len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("calls = %#v, want %#v", got, want)
			}
		}
	}
}

func TestBatchTaskExecuteServiceExecuteBuildsFailedTasks(t *testing.T) {
	t.Parallel()

	service := NewBatchTaskExecuteService(BatchTaskExecuteServiceConfig[string, string, string, string, []string]{
		LoadSession: func(ctx context.Context, batchID string) (*string, error) {
			value := "session"
			return &value, nil
		},
		LoadItems: func(ctx context.Context, batchID string, designIDs []string) ([]string, error) {
			return []string{"bad"}, nil
		},
		CreateTask: func(ctx context.Context, candidate string) (string, error) {
			return "", errors.New("boom")
		},
		BuildFailed: func(candidate string, err error) string {
			return candidate + ":" + err.Error()
		},
		Finalize: func(ctx context.Context, batchID string, session *string, created []string, failed []string) (*[]string, error) {
			result := append([]string(nil), failed...)
			return &result, nil
		},
	})

	result, err := service.Execute(context.Background(), "batch-2", []string{"design-1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || len(*result) != 1 || (*result)[0] != "bad:boom" {
		t.Fatalf("Execute() result = %#v", result)
	}
}

func TestBatchTaskExecuteServiceExecuteReturnsLoadItemsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	service := NewBatchTaskExecuteService(BatchTaskExecuteServiceConfig[string, string, string, string, string]{
		LoadSession: func(ctx context.Context, batchID string) (*string, error) {
			value := "session"
			return &value, nil
		},
		LoadItems: func(ctx context.Context, batchID string, designIDs []string) ([]string, error) {
			return nil, wantErr
		},
		CreateTask:  func(ctx context.Context, candidate string) (string, error) { return "", nil },
		BuildFailed: func(candidate string, err error) string { return "" },
		Finalize: func(ctx context.Context, batchID string, session *string, created []string, failed []string) (*string, error) {
			return nil, nil
		},
	})

	_, err := service.Execute(context.Background(), "batch-3", []string{"design-1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
}
