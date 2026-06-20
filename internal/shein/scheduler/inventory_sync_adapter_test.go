package scheduler

import (
	"context"
	"errors"
	"testing"

	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/shein/inventory"
)

// mockInventorySyncService 模拟 SHEIN 库存同步服务
type mockInventorySyncService struct {
	fetchFunc   func(ctx context.Context, tenantID, storeID int64) ([]*inventory.InventoryProductSnapshot, error)
	monitorFunc func(ctx context.Context, products []*inventory.InventoryProductSnapshot, tenantID, storeID int64) (*inventory.MonitorResult, error)
}

func (m *mockInventorySyncService) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*inventory.InventoryProductSnapshot, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, tenantID, storeID)
	}
	return []*inventory.InventoryProductSnapshot{}, nil
}

func (m *mockInventorySyncService) MonitorInventoryChanges(ctx context.Context, products []*inventory.InventoryProductSnapshot, tenantID, storeID int64) (*inventory.MonitorResult, error) {
	if m.monitorFunc != nil {
		return m.monitorFunc(ctx, products, tenantID, storeID)
	}
	return &inventory.MonitorResult{}, nil
}

func TestNewInventorySyncServiceAdapter(t *testing.T) {
	mock := &mockInventorySyncService{}
	adapter := newInventorySyncServiceAdapter(mock)
	if adapter == nil {
		t.Fatal("newInventorySyncServiceAdapter returned nil")
	}
}

// TestInventorySyncAdapter_FetchProducts 验证 FetchProductsForInventorySync 的转换
func TestInventorySyncAdapter_FetchProducts(t *testing.T) {
	tests := []struct {
		name      string
		fetchFunc func(ctx context.Context, tenantID, storeID int64) ([]*inventory.InventoryProductSnapshot, error)
		wantLen   int
		wantErr   bool
	}{
		{
			name: "成功获取产品列表",
			fetchFunc: func(_ context.Context, _, _ int64) ([]*inventory.InventoryProductSnapshot, error) {
				return []*inventory.InventoryProductSnapshot{
					{ProductID: "p1"},
					{ProductID: "p2"},
				}, nil
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "空产品列表",
			fetchFunc: func(_ context.Context, _, _ int64) ([]*inventory.InventoryProductSnapshot, error) {
				return []*inventory.InventoryProductSnapshot{}, nil
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "服务返回错误",
			fetchFunc: func(_ context.Context, _, _ int64) ([]*inventory.InventoryProductSnapshot, error) {
				return nil, errors.New("fetch failed")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := newInventorySyncServiceAdapter(&mockInventorySyncService{fetchFunc: tt.fetchFunc})
			results, err := adapter.FetchProductsForInventorySync(context.Background(), 1, 100)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tt.wantLen {
				t.Errorf("len(results) = %d, want %d", len(results), tt.wantLen)
			}
			// 验证元素类型可以断言回 *InventoryProductSnapshot
			for i, r := range results {
				if _, ok := r.(*inventory.InventoryProductSnapshot); !ok {
					t.Errorf("results[%d] type assertion failed: got %T", i, r)
				}
			}
		})
	}
}

// TestInventorySyncAdapter_MonitorInventoryChanges 验证 MonitorInventoryChanges 的类型转换和结果映射
func TestInventorySyncAdapter_MonitorInventoryChanges(t *testing.T) {
	tests := []struct {
		name        string
		products    []any
		monitorFunc func(ctx context.Context, products []*inventory.InventoryProductSnapshot, tenantID, storeID int64) (*inventory.MonitorResult, error)
		wantResult  *platformtask.InventorySyncResult
		wantErr     bool
	}{
		{
			name: "成功监控并映射结果字段",
			products: []any{
				&inventory.InventoryProductSnapshot{ProductID: "p1"},
				&inventory.InventoryProductSnapshot{ProductID: "p2"},
			},
			monitorFunc: func(_ context.Context, _ []*inventory.InventoryProductSnapshot, _, _ int64) (*inventory.MonitorResult, error) {
				return &inventory.MonitorResult{
					TotalProducts:     2,
					ProcessedProducts: 2,
					SkippedProducts:   0,
					PriceChanges:      1,
					StockChanges:      1,
					AmazonFetched:     2,
					AmazonFailed:      0,
				}, nil
			},
			wantResult: &platformtask.InventorySyncResult{
				TotalProducts:     2,
				ProcessedProducts: 2,
				SkippedProducts:   0,
				PriceChanges:      1,
				StockChanges:      1,
				AmazonFetched:     2,
				AmazonFailed:      0,
			},
		},
		{
			name:     "空产品列表",
			products: []any{},
			monitorFunc: func(_ context.Context, _ []*inventory.InventoryProductSnapshot, _, _ int64) (*inventory.MonitorResult, error) {
				return &inventory.MonitorResult{}, nil
			},
			wantResult: &platformtask.InventorySyncResult{},
		},
		{
			name: "类型断言失败返回错误",
			products: []any{
				"not-a-product-dto", // 错误类型
			},
			wantErr: true,
		},
		{
			name: "服务返回错误",
			products: []any{
				&inventory.InventoryProductSnapshot{ProductID: "p1"},
			},
			monitorFunc: func(_ context.Context, _ []*inventory.InventoryProductSnapshot, _, _ int64) (*inventory.MonitorResult, error) {
				return nil, errors.New("monitor failed")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := newInventorySyncServiceAdapter(&mockInventorySyncService{monitorFunc: tt.monitorFunc})
			result, err := adapter.MonitorInventoryChanges(context.Background(), tt.products, 1, 100)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TotalProducts != tt.wantResult.TotalProducts {
				t.Errorf("TotalProducts: want %d, got %d", tt.wantResult.TotalProducts, result.TotalProducts)
			}
			if result.ProcessedProducts != tt.wantResult.ProcessedProducts {
				t.Errorf("ProcessedProducts: want %d, got %d", tt.wantResult.ProcessedProducts, result.ProcessedProducts)
			}
			if result.PriceChanges != tt.wantResult.PriceChanges {
				t.Errorf("PriceChanges: want %d, got %d", tt.wantResult.PriceChanges, result.PriceChanges)
			}
			if result.StockChanges != tt.wantResult.StockChanges {
				t.Errorf("StockChanges: want %d, got %d", tt.wantResult.StockChanges, result.StockChanges)
			}
			if result.AmazonFetched != tt.wantResult.AmazonFetched {
				t.Errorf("AmazonFetched: want %d, got %d", tt.wantResult.AmazonFetched, result.AmazonFetched)
			}
		})
	}
}
