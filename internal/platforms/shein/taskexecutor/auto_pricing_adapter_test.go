package scheduler

import (
	"context"
	"errors"
	"testing"

	commonscheduler "task-processor/internal/platforms/common/scheduler"
	"task-processor/internal/platforms/shein/api/pricing"
	schedulerservice "task-processor/internal/platforms/shein/operation"
)

// MockSheinAutoPricingService 模拟Shein自动核价服务
type MockSheinAutoPricingService struct {
	FetchPendingPriceProductsFunc func(ctx context.Context, startDate, endDate string) ([]pricing.BargainPageData, error)
	ApplyPricingRulesFunc         func(ctx context.Context, products []pricing.BargainPageData, storeID int64, enableRebargain bool) ([]schedulerservice.PricingDecision, error)
	SubmitPricingResultsFunc      func(ctx context.Context, decisions []schedulerservice.PricingDecision) (*schedulerservice.PricingStatistics, error)
}

func (m *MockSheinAutoPricingService) FetchPendingPriceProducts(ctx context.Context, startDate, endDate string) ([]pricing.BargainPageData, error) {
	if m.FetchPendingPriceProductsFunc != nil {
		return m.FetchPendingPriceProductsFunc(ctx, startDate, endDate)
	}
	return []pricing.BargainPageData{}, nil
}

func (m *MockSheinAutoPricingService) ApplyPricingRules(ctx context.Context, products []pricing.BargainPageData, storeID int64, enableRebargain bool) ([]schedulerservice.PricingDecision, error) {
	if m.ApplyPricingRulesFunc != nil {
		return m.ApplyPricingRulesFunc(ctx, products, storeID, enableRebargain)
	}
	return []schedulerservice.PricingDecision{}, nil
}

func (m *MockSheinAutoPricingService) SubmitPricingResults(ctx context.Context, decisions []schedulerservice.PricingDecision) (*schedulerservice.PricingStatistics, error) {
	if m.SubmitPricingResultsFunc != nil {
		return m.SubmitPricingResultsFunc(ctx, decisions)
	}
	return &schedulerservice.PricingStatistics{}, nil
}

func TestNewSheinAutoPricingAdapter(t *testing.T) {
	mockService := &MockSheinAutoPricingService{}
	adapter := NewSheinAutoPricingAdapter(mockService)

	if adapter == nil {
		t.Fatal("NewSheinAutoPricingAdapter returned nil")
	}

	if adapter.pricingService != mockService {
		t.Error("Adapter should store the pricing service")
	}
}

func TestSheinAutoPricingAdapter_FetchPendingPriceProducts_Success(t *testing.T) {
	mockProducts := []pricing.BargainPageData{
		{BargainSn: "1", ProductTitle: "Product 1"},
		{BargainSn: "2", ProductTitle: "Product 2"},
	}

	mockService := &MockSheinAutoPricingService{
		FetchPendingPriceProductsFunc: func(ctx context.Context, startDate, endDate string) ([]pricing.BargainPageData, error) {
			return mockProducts, nil
		},
	}

	adapter := NewSheinAutoPricingAdapter(mockService)
	ctx := context.Background()

	results, err := adapter.FetchPendingPriceProducts(ctx, "2026-03-01", "2026-03-06")

	if err != nil {
		t.Errorf("FetchPendingPriceProducts should succeed, got error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 products, got %d", len(results))
	}
}

func TestSheinAutoPricingAdapter_FetchPendingPriceProducts_Error(t *testing.T) {
	expectedError := errors.New("fetch failed")

	mockService := &MockSheinAutoPricingService{
		FetchPendingPriceProductsFunc: func(ctx context.Context, startDate, endDate string) ([]pricing.BargainPageData, error) {
			return nil, expectedError
		},
	}

	adapter := NewSheinAutoPricingAdapter(mockService)
	ctx := context.Background()

	_, err := adapter.FetchPendingPriceProducts(ctx, "2026-03-01", "2026-03-06")

	if err == nil {
		t.Error("FetchPendingPriceProducts should fail")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be fetch error, got: %v", err)
	}
}

func TestSheinAutoPricingAdapter_ApplyPricingRules_Success(t *testing.T) {
	mockProducts := []interface{}{
		pricing.BargainPageData{BargainSn: "1", ProductTitle: "Product 1"},
		pricing.BargainPageData{BargainSn: "2", ProductTitle: "Product 2"},
	}

	mockDecisions := []schedulerservice.PricingDecision{
		{Product: pricing.BargainPageData{BargainSn: "1"}, Action: "accept"},
		{Product: pricing.BargainPageData{BargainSn: "2"}, Action: "reject"},
	}

	mockService := &MockSheinAutoPricingService{
		ApplyPricingRulesFunc: func(ctx context.Context, products []pricing.BargainPageData, storeID int64, enableRebargain bool) ([]schedulerservice.PricingDecision, error) {
			return mockDecisions, nil
		},
	}

	adapter := NewSheinAutoPricingAdapter(mockService)
	ctx := context.Background()

	results, err := adapter.ApplyPricingRules(ctx, mockProducts, 100, true)

	if err != nil {
		t.Errorf("ApplyPricingRules should succeed, got error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 decisions, got %d", len(results))
	}
}

func TestSheinAutoPricingAdapter_ApplyPricingRules_Error(t *testing.T) {
	expectedError := errors.New("apply rules failed")

	mockProducts := []interface{}{
		pricing.BargainPageData{BargainSn: "1"},
	}

	mockService := &MockSheinAutoPricingService{
		ApplyPricingRulesFunc: func(ctx context.Context, products []pricing.BargainPageData, storeID int64, enableRebargain bool) ([]schedulerservice.PricingDecision, error) {
			return nil, expectedError
		},
	}

	adapter := NewSheinAutoPricingAdapter(mockService)
	ctx := context.Background()

	_, err := adapter.ApplyPricingRules(ctx, mockProducts, 100, true)

	if err == nil {
		t.Error("ApplyPricingRules should fail")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be apply rules error, got: %v", err)
	}
}

func TestSheinAutoPricingAdapter_SubmitPricingResults_Success(t *testing.T) {
	mockResults := []interface{}{
		schedulerservice.PricingDecision{Product: pricing.BargainPageData{BargainSn: "1"}, Action: "accept"},
		schedulerservice.PricingDecision{Product: pricing.BargainPageData{BargainSn: "2"}, Action: "reject"},
	}

	mockStats := &schedulerservice.PricingStatistics{
		TotalProcessed: 2,
		AcceptCount:    1,
		RejectCount:    1,
		ReappealCount:  0,
		SkipCount:      0,
	}

	mockService := &MockSheinAutoPricingService{
		SubmitPricingResultsFunc: func(ctx context.Context, decisions []schedulerservice.PricingDecision) (*schedulerservice.PricingStatistics, error) {
			return mockStats, nil
		},
	}

	adapter := NewSheinAutoPricingAdapter(mockService)
	ctx := context.Background()

	stats, err := adapter.SubmitPricingResults(ctx, mockResults)

	if err != nil {
		t.Errorf("SubmitPricingResults should succeed, got error: %v", err)
	}

	if stats.TotalProcessed != 2 {
		t.Errorf("Expected TotalProcessed=2, got %d", stats.TotalProcessed)
	}

	if stats.AcceptCount != 1 {
		t.Errorf("Expected AcceptCount=1, got %d", stats.AcceptCount)
	}

	if stats.RejectCount != 1 {
		t.Errorf("Expected RejectCount=1, got %d", stats.RejectCount)
	}
}

func TestSheinAutoPricingAdapter_SubmitPricingResults_Error(t *testing.T) {
	expectedError := errors.New("submit failed")

	mockResults := []interface{}{
		schedulerservice.PricingDecision{Product: pricing.BargainPageData{BargainSn: "1"}},
	}

	mockService := &MockSheinAutoPricingService{
		SubmitPricingResultsFunc: func(ctx context.Context, decisions []schedulerservice.PricingDecision) (*schedulerservice.PricingStatistics, error) {
			return nil, expectedError
		},
	}

	adapter := NewSheinAutoPricingAdapter(mockService)
	ctx := context.Background()

	_, err := adapter.SubmitPricingResults(ctx, mockResults)

	if err == nil {
		t.Error("SubmitPricingResults should fail")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be submit error, got: %v", err)
	}
}

func TestConvertSheinStats(t *testing.T) {
	tests := []struct {
		name     string
		input    *schedulerservice.PricingStatistics
		expected *commonscheduler.PricingStats
	}{
		{
			name: "正常统计",
			input: &schedulerservice.PricingStatistics{
				TotalProcessed: 10,
				AcceptCount:    5,
				RejectCount:    3,
				ReappealCount:  1,
				SkipCount:      1,
			},
			expected: &commonscheduler.PricingStats{
				TotalProcessed: 10,
				AcceptCount:    5,
				RejectCount:    3,
				ReappealCount:  1,
				SkipCount:      1,
			},
		},
		{
			name:  "空统计",
			input: nil,
			expected: &commonscheduler.PricingStats{
				TotalProcessed: 0,
				AcceptCount:    0,
				RejectCount:    0,
				ReappealCount:  0,
				SkipCount:      0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertSheinStats(tt.input)

			if result.TotalProcessed != tt.expected.TotalProcessed {
				t.Errorf("TotalProcessed: expected %d, got %d", tt.expected.TotalProcessed, result.TotalProcessed)
			}
			if result.AcceptCount != tt.expected.AcceptCount {
				t.Errorf("AcceptCount: expected %d, got %d", tt.expected.AcceptCount, result.AcceptCount)
			}
			if result.RejectCount != tt.expected.RejectCount {
				t.Errorf("RejectCount: expected %d, got %d", tt.expected.RejectCount, result.RejectCount)
			}
			if result.ReappealCount != tt.expected.ReappealCount {
				t.Errorf("ReappealCount: expected %d, got %d", tt.expected.ReappealCount, result.ReappealCount)
			}
			if result.SkipCount != tt.expected.SkipCount {
				t.Errorf("SkipCount: expected %d, got %d", tt.expected.SkipCount, result.SkipCount)
			}
		})
	}
}
