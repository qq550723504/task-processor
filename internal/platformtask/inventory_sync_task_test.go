package platformtask

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/infra/clients/management"
	appscheduler "task-processor/internal/scheduler"
)

// MockInventorySyncService 模拟库存同步服务
type MockInventorySyncService struct {
	FetchProductsForInventorySyncFunc func(ctx context.Context, tenantID, storeID int64) ([]any, error)
	MonitorInventoryChangesFunc       func(ctx context.Context, products []any, tenantID, storeID int64) (*InventorySyncResult, error)
}

func (m *MockInventorySyncService) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]any, error) {
	if m.FetchProductsForInventorySyncFunc != nil {
		return m.FetchProductsForInventorySyncFunc(ctx, tenantID, storeID)
	}
	return []any{}, nil
}

func (m *MockInventorySyncService) MonitorInventoryChanges(ctx context.Context, products []any, tenantID, storeID int64) (*InventorySyncResult, error) {
	if m.MonitorInventoryChangesFunc != nil {
		return m.MonitorInventoryChangesFunc(ctx, products, tenantID, storeID)
	}
	return &InventorySyncResult{}, nil
}

func TestNewInventorySyncTask(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
		Interval: 3600 * time.Second,
	}

	mockService := &MockInventorySyncService{}
	mockManagement := &management.ClientManager{}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: mockManagement,
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	if task == nil {
		t.Fatal("NewInventorySyncTask returned nil")
	}

	if task.GetPlatform() != "test" {
		t.Errorf("Expected platform 'test', got '%s'", task.GetPlatform())
	}

	if task.GetType() != "inventory_sync" {
		t.Errorf("Expected type 'inventory_sync', got '%s'", task.GetType())
	}
}

func TestInventorySyncTask_Execute_Success(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	// 模拟成功的服务调用
	mockService := &MockInventorySyncService{
		FetchProductsForInventorySyncFunc: func(ctx context.Context, tenantID, storeID int64) ([]any, error) {
			return []any{"product1", "product2"}, nil
		},
		MonitorInventoryChangesFunc: func(ctx context.Context, products []any, tenantID, storeID int64) (*InventorySyncResult, error) {
			return &InventorySyncResult{
				TotalProducts:     2,
				ProcessedProducts: 2,
				SkippedProducts:   0,
				PriceChanges:      1,
				StockChanges:      1,
				AmazonFetched:     2,
				AmazonFailed:      0,
			}, nil
		},
	}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err != nil {
		t.Errorf("Execute should succeed, got error: %v", err)
	}

	// 验证状态已恢复为Stopped
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected status %s after execution, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}
}

func TestInventorySyncTask_Execute_FetchError(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	expectedError := errors.New("fetch failed")

	// 模拟获取产品失败
	mockService := &MockInventorySyncService{
		FetchProductsForInventorySyncFunc: func(ctx context.Context, tenantID, storeID int64) ([]any, error) {
			return nil, expectedError
		},
	}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err == nil {
		t.Error("Execute should fail when FetchProductsForInventorySync fails")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to contain fetch error, got: %v", err)
	}
}

func TestInventorySyncTask_Execute_MonitorError(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	expectedError := errors.New("monitor failed")

	// 模拟监控失败
	mockService := &MockInventorySyncService{
		FetchProductsForInventorySyncFunc: func(ctx context.Context, tenantID, storeID int64) ([]any, error) {
			return []any{"product1"}, nil
		},
		MonitorInventoryChangesFunc: func(ctx context.Context, products []any, tenantID, storeID int64) (*InventorySyncResult, error) {
			return nil, expectedError
		},
	}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err == nil {
		t.Error("Execute should fail when MonitorInventoryChanges fails")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to contain monitor error, got: %v", err)
	}
}

func TestInventorySyncTask_Execute_EmptyProductList(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	// 模拟空产品列表
	mockService := &MockInventorySyncService{
		FetchProductsForInventorySyncFunc: func(ctx context.Context, tenantID, storeID int64) ([]any, error) {
			return []any{}, nil
		},
	}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	// 空列表应该成功执行
	if err != nil {
		t.Errorf("Execute should succeed with empty list, got error: %v", err)
	}
}

func TestInventorySyncTask_Execute_ResultStatistics(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	expectedResult := &InventorySyncResult{
		TotalProducts:     10,
		ProcessedProducts: 8,
		SkippedProducts:   2,
		PriceChanges:      3,
		StockChanges:      2,
		AmazonFetched:     7,
		AmazonFailed:      1,
	}

	// 模拟返回详细统计
	mockService := &MockInventorySyncService{
		FetchProductsForInventorySyncFunc: func(ctx context.Context, tenantID, storeID int64) ([]any, error) {
			products := make([]any, 10)
			for i := 0; i < 10; i++ {
				products[i] = i
			}
			return products, nil
		},
		MonitorInventoryChangesFunc: func(ctx context.Context, products []any, tenantID, storeID int64) (*InventorySyncResult, error) {
			return expectedResult, nil
		},
	}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err != nil {
		t.Errorf("Execute should succeed, got error: %v", err)
	}
}

func TestInventorySyncTask_GetManagementClient(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	mockManagement := &management.ClientManager{}
	mockService := &MockInventorySyncService{}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: mockManagement,
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	client := task.GetManagementClient()
	if client != mockManagement {
		t.Error("GetManagementClient should return the same client")
	}
}

func TestInventorySyncTask_GetInventoryService(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "inventory_sync",
		TenantID: 1,
		StoreID:  100,
	}

	mockService := &MockInventorySyncService{}

	task := NewInventorySyncTask(InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		InventoryService: mockService,
		PlatformName:     "Test",
	})

	service := task.GetInventoryService()
	if service != mockService {
		t.Error("GetInventoryService should return the same service")
	}
}
