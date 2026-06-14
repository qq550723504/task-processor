package studio

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBatchDesignReviewServiceApproveDesigns(t *testing.T) {
	var gotIDs []string
	var gotUpdatedAt time.Time
	service := NewBatchDesignReviewService(BatchDesignReviewServiceConfig[string]{
		EnsureBatchExists: func(context.Context, string) error { return nil },
		ReplaceReviews: func(_ context.Context, _ string, designIDs []string, updatedAt time.Time) error {
			gotIDs = append([]string(nil), designIDs...)
			gotUpdatedAt = updatedAt
			return nil
		},
		LoadDetail: func(context.Context, string) (*string, error) {
			value := "detail"
			return &value, nil
		},
		CurrentTime: func() time.Time { return time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC) },
	})

	detail, err := service.ApproveDesigns(context.Background(), "batch-1", []string{"design-1"})
	if err != nil {
		t.Fatalf("ApproveDesigns() error = %v", err)
	}
	if detail == nil || *detail != "detail" {
		t.Fatalf("detail = %+v, want detail", detail)
	}
	if len(gotIDs) != 1 || gotIDs[0] != "design-1" {
		t.Fatalf("gotIDs = %+v, want design-1", gotIDs)
	}
	if gotUpdatedAt.IsZero() {
		t.Fatal("gotUpdatedAt is zero, want populated timestamp")
	}
}

func TestBatchDesignReviewServiceReturnsEnsureError(t *testing.T) {
	wantErr := errors.New("missing")
	service := NewBatchDesignReviewService(BatchDesignReviewServiceConfig[string]{
		EnsureBatchExists: func(context.Context, string) error { return wantErr },
		ReplaceReviews:    func(context.Context, string, []string, time.Time) error { return nil },
		LoadDetail:        func(context.Context, string) (*string, error) { return nil, nil },
		CurrentTime:       time.Now,
	})

	_, err := service.ApproveDesigns(context.Background(), "batch-1", nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ApproveDesigns() error = %v, want %v", err, wantErr)
	}
}
