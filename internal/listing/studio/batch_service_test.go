package studio

import (
	"context"
	"slices"
	"testing"
)

func TestBatchServiceNormalizesRequestsBeforeDelegating(t *testing.T) {
	t.Parallel()

	var gotBatchID string
	var gotDesignIDs []string
	service := NewBatchService(BatchServiceConfig[string, []string, []string]{
		ApproveDesigns: func(_ context.Context, batchID string, designIDs []string) (*string, error) {
			gotBatchID = batchID
			gotDesignIDs = append([]string(nil), designIDs...)
			result := "ok"
			return &result, nil
		},
		ApprovedDesignIDs: func(req *[]string) []string {
			if req == nil {
				return []string{"fallback"}
			}
			return *req
		},
	})

	result, err := service.ApproveDesigns(context.Background(), " batch-1 ", &[]string{"design-1", "design-2"})
	if err != nil {
		t.Fatalf("ApproveDesigns() error = %v", err)
	}
	if result == nil || *result != "ok" {
		t.Fatalf("ApproveDesigns() result = %+v, want ok", result)
	}
	if gotBatchID != "batch-1" {
		t.Fatalf("batchID = %q, want batch-1", gotBatchID)
	}
	if !slices.Equal(gotDesignIDs, []string{"design-1", "design-2"}) {
		t.Fatalf("designIDs = %+v, want preserved IDs", gotDesignIDs)
	}
}

func TestBatchServiceReturnsConfigurationErrors(t *testing.T) {
	t.Parallel()

	service := NewBatchService(BatchServiceConfig[string, []string, []string]{})

	if _, err := service.StartGeneration(context.Background(), "batch-1"); err == nil {
		t.Fatal("StartGeneration() error = nil, want configuration error")
	}
	if _, err := service.PrepareRetryItems(context.Background(), "batch-1", nil); err == nil {
		t.Fatal("PrepareRetryItems() error = nil, want configuration error")
	}
}
