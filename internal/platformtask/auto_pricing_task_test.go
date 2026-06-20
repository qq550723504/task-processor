package platformtask

import (
	"context"
	"testing"
	"time"

	appscheduler "task-processor/internal/scheduler"
)

// MockAutoPricingService 模拟自动核价服务
type MockAutoPricingService struct {
	FetchPendingPriceProductsFunc func(ctx context.Context, startDate, endDate string) ([]any, error)
	ApplyPricingRulesFunc         func(ctx context.Context, products []any, storeID int64, enableRebargain bool) ([]any, error)
	SubmitPricingResultsFunc      func(ctx context.Context, results []any) (*PricingStats, error)
}

func (m *MockAutoPricingService) FetchPendingPriceProducts(ctx context.Context, startDate, endDate string) ([]any, error) {
	if m.FetchPendingPriceProductsFunc != nil {
		return m.FetchPendingPriceProductsFunc(ctx, startDate, endDate)
	}
	return []any{}, nil
}

func (m *MockAutoPricingService) ApplyPricingRules(ctx context.Context, products []any, storeID int64, enableRebargain bool) ([]any, error) {
	if m.ApplyPricingRulesFunc != nil {
		return m.ApplyPricingRulesFunc(ctx, products, storeID, enableRebargain)
	}
	return products, nil
}

func (m *MockAutoPricingService) SubmitPricingResults(ctx context.Context, results []any) (*PricingStats, error) {
	if m.SubmitPricingResultsFunc != nil {
		return m.SubmitPricingResultsFunc(ctx, results)
	}
	return &PricingStats{}, nil
}

type MockAutoPricingStoreConfigProvider struct {
	GetAutoPricingStoreConfigFunc func(ctx context.Context, storeID int64) (*AutoPricingStoreConfig, error)
}

func (m *MockAutoPricingStoreConfigProvider) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*AutoPricingStoreConfig, error) {
	if m.GetAutoPricingStoreConfigFunc != nil {
		return m.GetAutoPricingStoreConfigFunc(ctx, storeID)
	}
	return &AutoPricingStoreConfig{}, nil
}

func TestNewAutoPricingTask(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "auto_pricing",
		TenantID: 1,
		StoreID:  100,
		Interval: 3600 * time.Second,
	}

	mockService := &MockAutoPricingService{}
	task := NewAutoPricingTask(AutoPricingTaskConfig{
		TaskConfig:          config,
		StoreConfigProvider: &MockAutoPricingStoreConfigProvider{},
		PricingService:      mockService,
		PlatformName:        "Test",
	})

	if task == nil {
		t.Fatal("NewAutoPricingTask returned nil")
	}

	if task.GetPlatform() != "test" {
		t.Errorf("Expected platform 'test', got '%s'", task.GetPlatform())
	}

	if task.GetType() != "auto_pricing" {
		t.Errorf("Expected type 'auto_pricing', got '%s'", task.GetType())
	}
}

func TestAutoPricingTask_GetStoreConfigProvider(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "auto_pricing",
		TenantID: 1,
		StoreID:  100,
	}

	mockProvider := &MockAutoPricingStoreConfigProvider{}
	mockService := &MockAutoPricingService{}

	task := NewAutoPricingTask(AutoPricingTaskConfig{
		TaskConfig:          config,
		StoreConfigProvider: mockProvider,
		PricingService:      mockService,
		PlatformName:        "Test",
	})

	provider := task.GetStoreConfigProvider()
	if provider != mockProvider {
		t.Error("GetStoreConfigProvider should return the same provider")
	}
}

func TestAutoPricingTask_GetPricingService(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "auto_pricing",
		TenantID: 1,
		StoreID:  100,
	}

	mockService := &MockAutoPricingService{}

	task := NewAutoPricingTask(AutoPricingTaskConfig{
		TaskConfig:          config,
		StoreConfigProvider: &MockAutoPricingStoreConfigProvider{},
		PricingService:      mockService,
		PlatformName:        "Test",
	})

	service := task.GetPricingService()
	if service != mockService {
		t.Error("GetPricingService should return the same service")
	}
}

func TestAutoPricingTask_StatusTransition(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "auto_pricing",
		TenantID: 1,
		StoreID:  100,
	}

	mockService := &MockAutoPricingService{}

	task := NewAutoPricingTask(AutoPricingTaskConfig{
		TaskConfig:          config,
		StoreConfigProvider: &MockAutoPricingStoreConfigProvider{},
		PricingService:      mockService,
		PlatformName:        "Test",
	})

	// 初始状态应该是Stopped
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected initial status %s, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}

	// 手动设置状态
	task.SetStatus(appscheduler.TaskStatusRunning)
	if task.GetStatus() != appscheduler.TaskStatusRunning {
		t.Errorf("Expected status %s, got %s", appscheduler.TaskStatusRunning, task.GetStatus())
	}

	// 恢复状态
	task.SetStatus(appscheduler.TaskStatusStopped)
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected status %s, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}
}

// 注意：Execute方法的测试需要完整的ClientManager mock，
// 这超出了单元测试的范围，应该在集成测试中进行
