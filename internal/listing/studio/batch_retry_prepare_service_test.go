package studio

import (
	"context"
	"errors"
	"testing"
)

func TestBatchRetryPrepareServicePrepareRetryItems(t *testing.T) {
	t.Parallel()

	type sourceDetail struct {
		ID string
	}
	type item struct {
		ID string
	}
	type result struct {
		ID string
	}

	var resetItems []item
	service := NewBatchRetryPrepareService(BatchRetryPrepareServiceConfig[sourceDetail, item, result]{
		LoadDetail: func(context.Context, string) (*sourceDetail, error) {
			return &sourceDetail{ID: "batch-1"}, nil
		},
		SelectItems: func(detail *sourceDetail, itemIDs []string) ([]item, error) {
			if detail == nil || detail.ID != "batch-1" {
				t.Fatalf("unexpected detail = %+v", detail)
			}
			if len(itemIDs) != 1 || itemIDs[0] != "item-2" {
				t.Fatalf("unexpected itemIDs = %+v", itemIDs)
			}
			return []item{{ID: "item-2"}}, nil
		},
		ResetItems: func(_ context.Context, items []item) error {
			resetItems = append([]item(nil), items...)
			return nil
		},
		LoadResult: func(context.Context, string) (*result, error) {
			return &result{ID: "batch-1"}, nil
		},
	})

	got, err := service.PrepareRetryItems(context.Background(), "batch-1", []string{"item-2"})
	if err != nil {
		t.Fatalf("PrepareRetryItems() error = %v", err)
	}
	if got == nil || got.ID != "batch-1" {
		t.Fatalf("PrepareRetryItems() = %+v, want batch-1 result", got)
	}
	if len(resetItems) != 1 || resetItems[0].ID != "item-2" {
		t.Fatalf("resetItems = %+v, want item-2", resetItems)
	}
}

func TestBatchRetryPrepareServicePropagatesSelectionError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("selection failed")
	service := NewBatchRetryPrepareService(BatchRetryPrepareServiceConfig[struct{}, struct{}, struct{}]{
		LoadDetail: func(context.Context, string) (*struct{}, error) {
			return &struct{}{}, nil
		},
		SelectItems: func(*struct{}, []string) ([]struct{}, error) {
			return nil, wantErr
		},
		ResetItems: func(context.Context, []struct{}) error {
			t.Fatal("ResetItems should not be called after selection error")
			return nil
		},
		LoadResult: func(context.Context, string) (*struct{}, error) {
			t.Fatal("LoadResult should not be called after selection error")
			return nil, nil
		},
	})

	_, err := service.PrepareRetryItems(context.Background(), "batch-1", []string{"item-1"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("PrepareRetryItems() error = %v, want %v", err, wantErr)
	}
}
