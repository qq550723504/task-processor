package studio

import (
	"context"
	"errors"
	"testing"
)

func TestBatchGenerationServiceStartGenerationRefreshesThenContinues(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchGenerationService(BatchGenerationServiceConfig[string, string]{
		RefreshGraph: func(ctx context.Context, batchID string) error {
			calls = append(calls, "refresh:"+batchID)
			return nil
		},
		ContinueGeneration: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "continue:"+batchID)
			result := "detail:" + batchID
			return &result, nil
		},
	})

	result, err := service.StartGeneration(context.Background(), "  batch-1  ")
	if err != nil {
		t.Fatalf("StartGeneration() error = %v", err)
	}
	if result == nil || *result != "detail:batch-1" {
		t.Fatalf("StartGeneration() result = %#v", result)
	}
	if got, want := len(calls), 2; got != want {
		t.Fatalf("call count = %d, want %d", got, want)
	}
	if calls[0] != "refresh:batch-1" || calls[1] != "continue:batch-1" {
		t.Fatalf("call order = %#v", calls)
	}
}

func TestBatchGenerationServiceResumeGenerationUsesTaskResumeBranch(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchGenerationService(BatchGenerationServiceConfig[string, int]{
		EnsureGraphForResume: func(ctx context.Context, batchID string) error {
			calls = append(calls, "ensure:"+batchID)
			return nil
		},
		ShouldResumeTaskCreation: func(ctx context.Context, batchID string) bool {
			calls = append(calls, "should:"+batchID)
			return true
		},
		ResumeTaskCreation: func(ctx context.Context, batchID string) (*int, error) {
			calls = append(calls, "resume:"+batchID)
			value := 7
			return &value, nil
		},
		AdaptResumeResult: func(result *int) *string {
			if result == nil {
				return nil
			}
			value := "resumed"
			return &value
		},
		ContinueGeneration: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "continue:"+batchID)
			value := "continued"
			return &value, nil
		},
	})

	result, err := service.ResumeGeneration(context.Background(), "batch-2")
	if err != nil {
		t.Fatalf("ResumeGeneration() error = %v", err)
	}
	if result == nil || *result != "resumed" {
		t.Fatalf("ResumeGeneration() result = %#v", result)
	}
	if got, want := calls, []string{"ensure:batch-2", "should:batch-2", "resume:batch-2"}; len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("calls = %#v, want %#v", got, want)
			}
		}
	}
}

func TestBatchGenerationServiceRetryItemsPreparesThenContinues(t *testing.T) {
	t.Parallel()

	var calls []string
	service := NewBatchGenerationService(BatchGenerationServiceConfig[string, string]{
		PrepareRetryItems: func(ctx context.Context, batchID string, itemIDs []string) (*string, error) {
			calls = append(calls, "prepare:"+batchID)
			value := "prepared"
			return &value, nil
		},
		ContinueGeneration: func(ctx context.Context, batchID string) (*string, error) {
			calls = append(calls, "continue:"+batchID)
			value := "continued"
			return &value, nil
		},
	})

	result, err := service.RetryItems(context.Background(), " batch-3 ", []string{"item-1"})
	if err != nil {
		t.Fatalf("RetryItems() error = %v", err)
	}
	if result == nil || *result != "continued" {
		t.Fatalf("RetryItems() result = %#v", result)
	}
	if got, want := calls, []string{"prepare:batch-3", "continue:batch-3"}; len(got) != len(want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("calls = %#v, want %#v", got, want)
			}
		}
	}
}

func TestBatchGenerationServicePrepareGenerationReturnsRefreshError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	service := NewBatchGenerationService(BatchGenerationServiceConfig[string, string]{
		RefreshGraph: func(ctx context.Context, batchID string) error {
			return wantErr
		},
		LoadDetail: func(ctx context.Context, batchID string) (*string, error) {
			t.Fatal("LoadDetail should not be called when refresh fails")
			return nil, nil
		},
	})

	_, err := service.PrepareGeneration(context.Background(), "batch-4")
	if !errors.Is(err, wantErr) {
		t.Fatalf("PrepareGeneration() error = %v, want %v", err, wantErr)
	}
}
