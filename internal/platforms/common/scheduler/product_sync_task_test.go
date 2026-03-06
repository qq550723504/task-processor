package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
)

// MockProductSyncService 模拟产品同步服务
type MockProductSyncService struct {
	FetchProductListFunc func(ctx context.Context) ([]interface{}, error)
	ConvertProductsFunc  func(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error)
	SaveProductsFunc     func(ctx context.Context, products []interface{}) (int, error)
}

func (m *MockProductSyncService) FetchProductList(ctx context.Context) ([]interface{}, error) {
	if m.FetchProductListFunc != nil {
		return m.FetchProductListFunc(ctx)
	}
	return []interface{}{}, nil
}

func (m *MockProductSyncService) ConvertProducts(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error) {
	if m.ConvertProductsFunc != nil {
		return m.ConvertProductsFunc(ctx, products, tenantID, storeID)
	}
	return products, nil
}

func (m *MockProductSyncService) SaveProducts(ctx context.Context, products []interface{}) (int, error) {
	if m.SaveProductsFunc != nil {
		return m.SaveProductsFunc(ctx, products)
	}
	return len(products), nil
}

func TestNewProductSyncTask(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
		Interval: 3600 * time.Second,
	}

	mockService := &MockProductSyncService{}
	mockManagement := &management.ClientManager{}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: mockManagement,
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	if task == nil {
		t.Fatal("NewProductSyncTask returned nil")
	}

	if task.GetPlatform() != "test" {
		t.Errorf("Expected platform 'test', got '%s'", task.GetPlatform())
	}

	if task.GetType() != "product_sync" {
		t.Errorf("Expected type 'product_sync', got '%s'", task.GetType())
	}
}

func TestProductSyncTask_Execute_Success(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	// 模拟成功的服务调用
	mockService := &MockProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{"product1", "product2", "product3"}, nil
		},
		ConvertProductsFunc: func(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error) {
			return products, nil
		},
		SaveProductsFunc: func(ctx context.Context, products []interface{}) (int, error) {
			return len(products), nil
		},
	}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
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

func TestProductSyncTask_Execute_FetchError(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	expectedError := errors.New("fetch failed")

	// 模拟获取产品列表失败
	mockService := &MockProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]interface{}, error) {
			return nil, expectedError
		},
	}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err == nil {
		t.Error("Execute should fail when FetchProductList fails")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to contain fetch error, got: %v", err)
	}
}

func TestProductSyncTask_Execute_ConvertError(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	expectedError := errors.New("convert failed")

	// 模拟转换产品失败
	mockService := &MockProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{"product1"}, nil
		},
		ConvertProductsFunc: func(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error) {
			return nil, expectedError
		},
	}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err == nil {
		t.Error("Execute should fail when ConvertProducts fails")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to contain convert error, got: %v", err)
	}
}

func TestProductSyncTask_Execute_SaveError(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	expectedError := errors.New("save failed")

	// 模拟保存产品失败
	mockService := &MockProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{"product1"}, nil
		},
		ConvertProductsFunc: func(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error) {
			return products, nil
		},
		SaveProductsFunc: func(ctx context.Context, products []interface{}) (int, error) {
			return 0, expectedError
		},
	}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	if err == nil {
		t.Error("Execute should fail when SaveProducts fails")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to contain save error, got: %v", err)
	}
}

func TestProductSyncTask_Execute_EmptyProductList(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	// 模拟空产品列表
	mockService := &MockProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{}, nil
		},
	}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	ctx := context.Background()
	err := task.Execute(ctx)

	// 空列表应该成功执行
	if err != nil {
		t.Errorf("Execute should succeed with empty list, got error: %v", err)
	}
}

func TestProductSyncTask_Execute_StatusTransition(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	mockService := &MockProductSyncService{
		FetchProductListFunc: func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{"product1"}, nil
		},
	}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	// 初始状态应该是Stopped
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected initial status %s, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}

	ctx := context.Background()
	_ = task.Execute(ctx)

	// 执行后状态应该恢复为Stopped
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected status %s after execution, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}
}

func TestProductSyncTask_GetManagementClient(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	mockManagement := &management.ClientManager{}
	mockService := &MockProductSyncService{}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: mockManagement,
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	client := task.GetManagementClient()
	if client != mockManagement {
		t.Error("GetManagementClient should return the same client")
	}
}

func TestProductSyncTask_GetSyncService(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	mockService := &MockProductSyncService{}

	task := NewProductSyncTask(ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		SyncService:      mockService,
		PlatformName:     "Test",
	})

	service := task.GetSyncService()
	if service != mockService {
		t.Error("GetSyncService should return the same service")
	}
}
