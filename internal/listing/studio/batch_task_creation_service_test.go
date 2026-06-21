package studio

import (
	"context"
	"errors"
	"testing"
)

func TestBatchTaskCreationServicePrepareTaskCreationBuildsStateThenDelegates(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchTaskCreationService(BatchTaskCreationServiceConfig[string, string, string, string, string]{
		PrepareState: func(ctx context.Context, batchID string, designIDs []string) (BatchTaskPrepareState[string, string], error) {
			calls = append(calls, "prepare-state:"+batchID)
			session := "session"
			batch := "batch"
			return BatchTaskPrepareState[string, string]{
				Session:   &session,
				Batch:     &batch,
				DesignIDs: append([]string(nil), designIDs...),
			}, nil
		},
		PrepareTaskCreation: func(ctx context.Context, batchID string, state BatchTaskPrepareState[string, string]) (*string, error) {
			calls = append(calls, "prepare:"+batchID)
			value := state.DesignIDs[0]
			return &value, nil
		},
	})

	result, err := service.PrepareTaskCreation(context.Background(), " batch-1 ", []string{"design-1"})
	if err != nil {
		t.Fatalf("PrepareTaskCreation() error = %v", err)
	}
	if result == nil || *result != "design-1" {
		t.Fatalf("PrepareTaskCreation() result = %#v", result)
	}
	if got, want := calls, []string{"prepare-state:batch-1", "prepare:batch-1"}; len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("calls = %#v, want %#v", got, want)
			}
		}
	}
}

func TestBatchTaskCreationServiceResumeTaskCreationLoadsResultWhenNoPendingDesigns(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchTaskCreationService(BatchTaskCreationServiceConfig[string, string, string, string, string]{
		LoadSession: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "session:"+batchID)
			value := "session"
			return &value, nil
		},
		PendingDesignIDs: func(session *string) []string {
			calls = append(calls, "pending")
			return nil
		},
		LoadResult: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "result:"+batchID)
			value := "loaded"
			return &value, nil
		},
		CreateTasks: func(ctx context.Context, batchID string, designIDs []string) (*string, error) {
			t.Fatal("CreateTasks should not be called without pending designs")
			return nil, nil
		},
		LoadBatch: func(ctx context.Context, batchID string) (*string, error) {
			t.Fatal("LoadBatch should not be called without pending designs")
			return nil, nil
		},
		FinalizeTaskCreation: func(ctx context.Context, batchID string, state BatchTaskResumeFinalizeState[string, string, string, string]) (*string, error) {
			t.Fatal("FinalizeTaskCreation should not be called without pending designs")
			return nil, nil
		},
		CreatedTasks: func(result *string) []string { return nil },
		FailedTasks:  func(result *string) []string { return nil },
	})

	result, err := service.ResumeTaskCreation(context.Background(), "batch-2")
	if err != nil {
		t.Fatalf("ResumeTaskCreation() error = %v", err)
	}
	if result == nil || *result != "loaded" {
		t.Fatalf("ResumeTaskCreation() result = %#v", result)
	}
	if got, want := calls, []string{"session:batch-2", "pending", "result:batch-2"}; len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("calls = %#v, want %#v", got, want)
			}
		}
	}
}

func TestBatchTaskCreationServiceResumeTaskCreationReturnsCreateResult(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchTaskCreationService(BatchTaskCreationServiceConfig[string, string, []string, string, string]{
		LoadSession: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "session:"+batchID)
			value := "session"
			return &value, nil
		},
		PendingDesignIDs: func(session *string) []string {
			calls = append(calls, "pending")
			return []string{"design-1"}
		},
		LoadResult: func(ctx context.Context, batchID string) (*[]string, error) {
			t.Fatal("LoadResult should not be called when there are pending designs")
			return nil, nil
		},
		CreateTasks: func(ctx context.Context, batchID string, designIDs []string) (*[]string, error) {
			calls = append(calls, "create:"+batchID)
			result := []string{"created"}
			return &result, nil
		},
		LoadBatch: func(ctx context.Context, batchID string) (*string, error) {
			t.Fatal("LoadBatch should not be called because CreateTasks owns task finalization")
			return nil, nil
		},
		FinalizeTaskCreation: func(ctx context.Context, batchID string, state BatchTaskResumeFinalizeState[string, string, string, string]) (*[]string, error) {
			t.Fatal("FinalizeTaskCreation should not be called because CreateTasks owns task finalization")
			return nil, nil
		},
		CreatedTasks: func(result *[]string) []string {
			return append([]string(nil), (*result)...)
		},
		FailedTasks: func(result *[]string) []string {
			return nil
		},
	})

	result, err := service.ResumeTaskCreation(context.Background(), " batch-3 ")
	if err != nil {
		t.Fatalf("ResumeTaskCreation() error = %v", err)
	}
	if result == nil || len(*result) != 1 || (*result)[0] != "created" {
		t.Fatalf("ResumeTaskCreation() result = %#v", result)
	}
	if got, want := calls, []string{"session:batch-3", "pending", "create:batch-3"}; len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("calls = %#v, want %#v", got, want)
			}
		}
	}
}

func TestBatchTaskCreationServiceResumeTaskCreationReturnsSessionError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	service := NewBatchTaskCreationService(BatchTaskCreationServiceConfig[string, string, string, string, string]{
		LoadSession: func(ctx context.Context, batchID string) (*string, error) {
			return nil, wantErr
		},
		PendingDesignIDs: func(session *string) []string { return nil },
		LoadResult:       func(ctx context.Context, batchID string) (*string, error) { return nil, nil },
		CreateTasks:      func(ctx context.Context, batchID string, designIDs []string) (*string, error) { return nil, nil },
		LoadBatch:        func(ctx context.Context, batchID string) (*string, error) { return nil, nil },
		FinalizeTaskCreation: func(ctx context.Context, batchID string, state BatchTaskResumeFinalizeState[string, string, string, string]) (*string, error) {
			return nil, nil
		},
		CreatedTasks: func(result *string) []string { return nil },
		FailedTasks:  func(result *string) []string { return nil },
	})

	_, err := service.ResumeTaskCreation(context.Background(), "batch-4")
	if !errors.Is(err, wantErr) {
		t.Fatalf("ResumeTaskCreation() error = %v, want %v", err, wantErr)
	}
}
