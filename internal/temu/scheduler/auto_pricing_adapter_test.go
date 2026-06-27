package scheduler

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/temu/api"
	temupricing "task-processor/internal/temu/api/pricing"
	"task-processor/internal/temu/pricing"
)

// MockTemuAPIClient 模拟Temu API客户端
type MockTemuAPIClient struct {
	api.APIClientInterface
	storeID int64
}

func (m *MockTemuAPIClient) GetStoreID() int64 {
	return m.storeID
}

// MockTemuAutoPricingService 模拟Temu自动核价服务
type MockTemuAutoPricingService struct {
	AutoProcessPendingPricesWithRulesFunc func(runtime pricing.PricingRuntime) (*temupricing.Statistics, error)
}

func (m *MockTemuAutoPricingService) AutoProcessPendingPricesWithRules(runtime pricing.PricingRuntime) (*temupricing.Statistics, error) {
	if m.AutoProcessPendingPricesWithRulesFunc != nil {
		return m.AutoProcessPendingPricesWithRulesFunc(runtime)
	}
	return &temupricing.Statistics{}, nil
}

func TestNewTemuAutoPricingAdapter(t *testing.T) {
	mockAPIClient := &MockTemuAPIClient{storeID: 100}

	adapter := NewTemuAutoPricingAdapter(mockAPIClient, pricing.NewPricingRuntime(stubAutoPricingRuntimeSource{}))

	if adapter == nil {
		t.Fatal("NewTemuAutoPricingAdapter returned nil")
	}
}

func TestTemuAutoPricingAdapter_FetchPendingPriceProducts(t *testing.T) {
	mockAPIClient := &MockTemuAPIClient{storeID: 100}

	adapter := NewTemuAutoPricingAdapter(mockAPIClient, pricing.NewPricingRuntime(stubAutoPricingRuntimeSource{}))
	ctx := context.Background()

	// Temu平台的实现返回空切片
	results, err := adapter.FetchPendingPriceProducts(ctx, "2026-03-01", "2026-03-06")

	if err != nil {
		t.Errorf("FetchPendingPriceProducts should succeed, got error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty slice, got %d items", len(results))
	}
}

func TestTemuAutoPricingAdapter_ApplyPricingRules(t *testing.T) {
	mockAPIClient := &MockTemuAPIClient{storeID: 100}

	adapter := NewTemuAutoPricingAdapter(mockAPIClient, pricing.NewPricingRuntime(stubAutoPricingRuntimeSource{}))
	ctx := context.Background()

	mockProducts := []any{"product1", "product2"}

	// Temu平台的实现直接返回输入
	results, err := adapter.ApplyPricingRules(ctx, mockProducts, 100, true)

	if err != nil {
		t.Errorf("ApplyPricingRules should succeed, got error: %v", err)
	}

	if len(results) != len(mockProducts) {
		t.Errorf("Expected %d items, got %d", len(mockProducts), len(results))
	}
}

type stubAutoPricingRuntimeSource struct{}

func (stubAutoPricingRuntimeSource) GetStoreAPI() listingadmin.StoreAPI                { return nil }
func (stubAutoPricingRuntimeSource) GetPricingRuleClient() listingadmin.PricingRuleAPI { return nil }
func (stubAutoPricingRuntimeSource) GetProductImportMappingAPI() listingadmin.ProductImportMappingAPI {
	return nil
}
func (stubAutoPricingRuntimeSource) GetLocalStoreRepository() *listingadmin.GormStoreRepository {
	return nil
}
func (stubAutoPricingRuntimeSource) GetLocalPricingRuleRepository() *listingadmin.GormPricingRuleRepository {
	return nil
}
func (stubAutoPricingRuntimeSource) GetLocalProductImportMappingRepository() *listingadmin.GormProductImportMappingRepository {
	return nil
}
func (stubAutoPricingRuntimeSource) GetRuntimeOperationStrategy(int64) (*listingruntime.OperationStrategy, error) {
	return nil, nil
}

func TestTemuAutoPricingAdapter_SubmitPricingResults_Success(t *testing.T) {
	mockStats := &temupricing.Statistics{
		TotalProcessed: 10,
		AcceptCount:    5,
		RejectCount:    3,
		ReappealCount:  1,
		SkipCount:      1,
	}

	// 由于无法直接mock AutoPricingService（它是具体类型），
	// 这个测试主要验证转换逻辑
	// 实际的服务调用应该在集成测试中验证
	stats := convertTemuStats(mockStats)

	if stats.TotalProcessed != 10 {
		t.Errorf("Expected TotalProcessed=10, got %d", stats.TotalProcessed)
	}

	if stats.AcceptCount != 5 {
		t.Errorf("Expected AcceptCount=5, got %d", stats.AcceptCount)
	}

	if stats.RejectCount != 3 {
		t.Errorf("Expected RejectCount=3, got %d", stats.RejectCount)
	}

	if stats.ReappealCount != 1 {
		t.Errorf("Expected ReappealCount=1, got %d", stats.ReappealCount)
	}

	if stats.SkipCount != 1 {
		t.Errorf("Expected SkipCount=1, got %d", stats.SkipCount)
	}
}

func TestConvertTemuStats(t *testing.T) {
	tests := []struct {
		name     string
		input    *temupricing.Statistics
		expected *platformtask.PricingStats
	}{
		{
			name: "正常统计",
			input: &temupricing.Statistics{
				TotalProcessed: 10,
				AcceptCount:    5,
				RejectCount:    3,
				ReappealCount:  1,
				SkipCount:      1,
			},
			expected: &platformtask.PricingStats{
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
			expected: &platformtask.PricingStats{
				TotalProcessed: 0,
				AcceptCount:    0,
				RejectCount:    0,
				ReappealCount:  0,
				SkipCount:      0,
			},
		},
		{
			name: "全部接受",
			input: &temupricing.Statistics{
				TotalProcessed: 5,
				AcceptCount:    5,
				RejectCount:    0,
				ReappealCount:  0,
				SkipCount:      0,
			},
			expected: &platformtask.PricingStats{
				TotalProcessed: 5,
				AcceptCount:    5,
				RejectCount:    0,
				ReappealCount:  0,
				SkipCount:      0,
			},
		},
		{
			name: "全部拒绝",
			input: &temupricing.Statistics{
				TotalProcessed: 5,
				AcceptCount:    0,
				RejectCount:    5,
				ReappealCount:  0,
				SkipCount:      0,
			},
			expected: &platformtask.PricingStats{
				TotalProcessed: 5,
				AcceptCount:    0,
				RejectCount:    5,
				ReappealCount:  0,
				SkipCount:      0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTemuStats(tt.input)

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
