package scheduler

import (
	"context"
	"testing"
	"time"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
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

func TestNewAutoPricingTask(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "auto_pricing",
		TenantID: 1,
		StoreID:  100,
		Interval: 3600 * time.Second,
	}

	mockService := &MockAutoPricingService{}
	mockManagement := &management.ClientManager{}

	task := NewAutoPricingTask(AutoPricingTaskConfig{
		TaskConfig:       config,
		ManagementClient: mockManagement,
		PricingService:   mockService,
		PlatformName:     "Test",
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

func TestAutoPricingTask_GetManagementClient(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "auto_pricing",
		TenantID: 1,
		StoreID:  100,
	}

	mockManagement := &management.ClientManager{}
	mockService := &MockAutoPricingService{}

	task := NewAutoPricingTask(AutoPricingTaskConfig{
		TaskConfig:       config,
		ManagementClient: mockManagement,
		PricingService:   mockService,
		PlatformName:     "Test",
	})

	client := task.GetManagementClient()
	if client != mockManagement {
		t.Error("GetManagementClient should return the same client")
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
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		PricingService:   mockService,
		PlatformName:     "Test",
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
		TaskConfig:       config,
		ManagementClient: &management.ClientManager{},
		PricingService:   mockService,
		PlatformName:     "Test",
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
