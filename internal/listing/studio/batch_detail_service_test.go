package studio

import (
	"context"
	"errors"
	"testing"
)

func TestBatchDetailServiceReturnsProjectedGraph(t *testing.T) {
	service := NewBatchDetailService(BatchDetailServiceConfig[string, string]{
		LoadGraph: func(ctx context.Context, batchID string) (*string, error) {
			value := "graph"
			return &value, nil
		},
		IsGraphMissing: func(err error) bool { return false },
		ResolveWithoutGraph: func(ctx context.Context, batchID string) (*string, bool, error) {
			return nil, false, nil
		},
		EnsureGraph: func(ctx context.Context, batchID string) error { return nil },
		ProjectDetail: func(ctx context.Context, batchID string, graph *string) (*string, error) {
			value := "detail:" + *graph
			return &value, nil
		},
	})

	result, err := service.GetDetail(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if result == nil || *result != "detail:graph" {
		t.Fatalf("result = %+v, want detail:graph", result)
	}
}

func TestBatchDetailServiceReturnsFallbackWithoutSync(t *testing.T) {
	missingErr := errors.New("missing")
	service := NewBatchDetailService(BatchDetailServiceConfig[string, string]{
		LoadGraph:      func(ctx context.Context, batchID string) (*string, error) { return nil, missingErr },
		IsGraphMissing: func(err error) bool { return errors.Is(err, missingErr) },
		ResolveWithoutGraph: func(ctx context.Context, batchID string) (*string, bool, error) {
			value := "fallback"
			return &value, false, nil
		},
		EnsureGraph: func(ctx context.Context, batchID string) error {
			t.Fatal("EnsureGraph() should not be called")
			return nil
		},
		ProjectDetail: func(ctx context.Context, batchID string, graph *string) (*string, error) {
			t.Fatal("ProjectDetail() should not be called")
			return nil, nil
		},
	})

	result, err := service.GetDetail(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if result == nil || *result != "fallback" {
		t.Fatalf("result = %+v, want fallback", result)
	}
}

func TestBatchDetailServiceSyncsAndReloadsWhenRequired(t *testing.T) {
	missingErr := errors.New("missing")
	loadCalls := 0
	service := NewBatchDetailService(BatchDetailServiceConfig[string, string]{
		LoadGraph: func(ctx context.Context, batchID string) (*string, error) {
			loadCalls++
			if loadCalls == 1 {
				return nil, missingErr
			}
			value := "graph"
			return &value, nil
		},
		IsGraphMissing: func(err error) bool { return errors.Is(err, missingErr) },
		ResolveWithoutGraph: func(ctx context.Context, batchID string) (*string, bool, error) {
			return nil, true, nil
		},
		EnsureGraph: func(ctx context.Context, batchID string) error { return nil },
		ProjectDetail: func(ctx context.Context, batchID string, graph *string) (*string, error) {
			value := "detail:" + *graph
			return &value, nil
		},
	})

	result, err := service.GetDetail(context.Background(), "batch-1")
	if err != nil {
		t.Fatalf("GetDetail() error = %v", err)
	}
	if loadCalls != 2 {
		t.Fatalf("loadCalls = %d, want 2", loadCalls)
	}
	if result == nil || *result != "detail:graph" {
		t.Fatalf("result = %+v, want detail:graph", result)
	}
}
