package amazon

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/model"
)

type stubBatchRequestProcessor struct {
	calls   int
	results []model.ProductResult
	onCall  func(call int)
}

func (s *stubBatchRequestProcessor) ProcessWithContext(_ context.Context, _ string, _ string) (*model.Product, error) {
	call := s.calls
	s.calls++
	if s.onCall != nil {
		s.onCall(call)
	}
	if call < len(s.results) {
		return s.results[call].Product, s.results[call].Error
	}
	return &model.Product{Asin: "DEFAULT"}, nil
}

func TestBatchProcessorProcessWithContextSequentiallyDelegates(t *testing.T) {
	bp := NewBatchProcessor()
	processor := &stubBatchRequestProcessor{
		results: []model.ProductResult{
			{Product: &model.Product{Asin: "B001"}},
			{Error: errors.New("temporary failure")},
			{Product: &model.Product{Asin: "B003"}},
		},
	}

	results := bp.ProcessWithContext(context.Background(), []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001"},
		{URL: "https://amazon.com/dp/B002"},
		{URL: "https://amazon.com/dp/B003"},
	}, processor)

	if processor.calls != 3 {
		t.Fatalf("expected 3 delegated calls, got %d", processor.calls)
	}
	if results[0].Error != nil || results[0].Product == nil || results[0].Product.Asin != "B001" {
		t.Fatalf("expected first result success, got %+v", results[0])
	}
	if results[1].Error == nil {
		t.Fatalf("expected second result error, got %+v", results[1])
	}
	if results[2].Error != nil || results[2].Product == nil || results[2].Product.Asin != "B003" {
		t.Fatalf("expected third result success, got %+v", results[2])
	}
}

func TestBatchProcessorProcessWithContextStopsOnContextCancel(t *testing.T) {
	bp := NewBatchProcessor()
	ctx, cancel := context.WithCancel(context.Background())
	processor := &stubBatchRequestProcessor{
		results: []model.ProductResult{
			{Product: &model.Product{Asin: "B001"}},
			{Product: &model.Product{Asin: "B002"}},
			{Product: &model.Product{Asin: "B003"}},
		},
		onCall: func(call int) {
			if call == 0 {
				cancel()
			}
		},
	}

	results := bp.ProcessWithContext(ctx, []model.ProductRequest{
		{URL: "https://amazon.com/dp/B001"},
		{URL: "https://amazon.com/dp/B002"},
		{URL: "https://amazon.com/dp/B003"},
	}, processor)

	if processor.calls != 1 {
		t.Fatalf("expected only 1 delegated call before cancellation, got %d", processor.calls)
	}
	if results[0].Error != nil {
		t.Fatalf("expected first result success, got %+v", results[0])
	}
	if !errors.Is(results[1].Error, context.Canceled) {
		t.Fatalf("expected second result context canceled, got %+v", results[1])
	}
	if !errors.Is(results[2].Error, context.Canceled) {
		t.Fatalf("expected third result context canceled, got %+v", results[2])
	}
}

func TestBatchProcessorProcessWithContextReturnsEmptyForNoRequests(t *testing.T) {
	bp := NewBatchProcessor()
	results := bp.ProcessWithContext(context.Background(), nil, &stubBatchRequestProcessor{})
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d", len(results))
	}
}
