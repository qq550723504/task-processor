package scheduler

import (
	"testing"
	"time"

	appscheduler "task-processor/internal/app/scheduler"
)

func TestNewBaseTask(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "test_task",
		TenantID: 1,
		StoreID:  100,
		Interval: 3600 * time.Second,
	}

	task := NewBaseTask(config)

	if task == nil {
		t.Fatal("NewBaseTask returned nil")
	}

	if task.GetPlatform() != "test" {
		t.Errorf("Expected platform 'test', got '%s'", task.GetPlatform())
	}

	if task.GetType() != "test_task" {
		t.Errorf("Expected type 'test_task', got '%s'", task.GetType())
	}

	if task.GetTenantID() != 1 {
		t.Errorf("Expected tenant ID 1, got %d", task.GetTenantID())
	}

	if task.GetStoreID() != 100 {
		t.Errorf("Expected store ID 100, got %d", task.GetStoreID())
	}

	if task.GetInterval() != 3600*time.Second {
		t.Errorf("Expected interval 3600s, got %v", task.GetInterval())
	}
}

func TestBaseTask_GetID(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "test_task",
		TenantID: 1,
		StoreID:  100,
	}

	task := NewBaseTask(config)
	id := task.GetID()

	// ID格式: platform:taskType:tenantID:storeID
	expectedID := "test:test_task:1:100"
	if id != expectedID {
		t.Errorf("Expected ID '%s', got '%s'", expectedID, id)
	}
}

func TestBaseTask_Status(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "test",
		TaskType: "test_task",
		TenantID: 1,
		StoreID:  100,
	}

	task := NewBaseTask(config)

	// 初始状态应该是 Stopped
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected initial status %s, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}

	// 设置为 Running
	task.SetStatus(appscheduler.TaskStatusRunning)
	if task.GetStatus() != appscheduler.TaskStatusRunning {
		t.Errorf("Expected status %s, got %s", appscheduler.TaskStatusRunning, task.GetStatus())
	}

	// 设置回 Stopped
	task.SetStatus(appscheduler.TaskStatusStopped)
	if task.GetStatus() != appscheduler.TaskStatusStopped {
		t.Errorf("Expected status %s, got %s", appscheduler.TaskStatusStopped, task.GetStatus())
	}
}

func TestBaseTask_Getters(t *testing.T) {
	config := appscheduler.TaskConfig{
		Platform: "shein",
		TaskType: "product_sync",
		TenantID: 5,
		StoreID:  200,
		Interval: 7200 * time.Second,
	}

	task := NewBaseTask(config)

	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{"Platform", task.GetPlatform(), "shein"},
		{"Type", task.GetType(), appscheduler.TaskType("product_sync")},
		{"TenantID", task.GetTenantID(), int64(5)},
		{"StoreID", task.GetStoreID(), int64(200)},
		{"Interval", task.GetInterval(), 7200 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, tt.got)
			}
		})
	}
}

func TestBaseTask_MultipleInstances(t *testing.T) {
	config1 := appscheduler.TaskConfig{
		Platform: "shein",
		TaskType: "product_sync",
		TenantID: 1,
		StoreID:  100,
	}

	config2 := appscheduler.TaskConfig{
		Platform: "temu",
		TaskType: "inventory_sync",
		TenantID: 2,
		StoreID:  200,
	}

	task1 := NewBaseTask(config1)
	task2 := NewBaseTask(config2)

	// 验证两个任务是独立的
	if task1.GetID() == task2.GetID() {
		t.Error("Two different tasks should have different IDs")
	}

	// 修改一个任务的状态不应影响另一个
	task1.SetStatus(appscheduler.TaskStatusRunning)
	if task2.GetStatus() == appscheduler.TaskStatusRunning {
		t.Error("Modifying one task should not affect another")
	}
}

func TestBaseTask_IDFormat(t *testing.T) {
	tests := []struct {
		name     string
		config   appscheduler.TaskConfig
		expected string
	}{
		{
			name: "Shein产品同步",
			config: appscheduler.TaskConfig{
				Platform: "shein",
				TaskType: "product_sync",
				TenantID: 1,
				StoreID:  100,
			},
			expected: "shein:product_sync:1:100",
		},
		{
			name: "Temu库存同步",
			config: appscheduler.TaskConfig{
				Platform: "temu",
				TaskType: "inventory_sync",
				TenantID: 2,
				StoreID:  200,
			},
			expected: "temu:inventory_sync:2:200",
		},
		{
			name: "Shein自动核价",
			config: appscheduler.TaskConfig{
				Platform: "shein",
				TaskType: "auto_pricing",
				TenantID: 3,
				StoreID:  300,
			},
			expected: "shein:auto_pricing:3:300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewBaseTask(tt.config)
			if task.GetID() != tt.expected {
				t.Errorf("Expected ID '%s', got '%s'", tt.expected, task.GetID())
			}
		})
	}
}
