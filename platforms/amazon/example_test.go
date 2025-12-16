// Package amazon 提供Amazon平台架构测试示例
package amazon

import (
	"context"
	"task-processor/common/types"
	"task-processor/platforms/amazon/internal/model"
	"task-processor/platforms/amazon/internal/service"
	"testing"
)

// TestNewArchitecture 测试新架构的基本功能
func TestNewArchitecture(t *testing.T) {
	// 创建任务上下文
	ctx := context.Background()
	task := &types.Task{
		ID:        "test-123",
		ProductID: "B08N5WRWNW",
		StoreID:   1,
		TenantID:  1,
	}

	taskCtx := NewTaskContext(ctx, task)
	if taskCtx == nil {
		t.Fatal("TaskContext创建失败")
	}

	// 测试数据存储
	taskCtx.SetData("test_key", "test_value")
	value, exists := taskCtx.GetData("test_key")
	if !exists || value != "test_value" {
		t.Fatal("数据存储测试失败")
	}

	t.Log("✅ TaskContext测试通过")
}

// TestPipelineService 测试管道服务
func TestPipelineService(t *testing.T) {
	// 创建管道服务
	pipeline := service.NewPipelineService()
	if pipeline == nil {
		t.Fatal("PipelineService创建失败")
	}

	// 测试处理器数量
	if pipeline.GetHandlerCount() != 0 {
		t.Fatal("初始处理器数量应为0")
	}

	t.Log("✅ PipelineService测试通过")
}

// TestPipelineBuilder 测试管道构建器
func TestPipelineBuilder(t *testing.T) {
	// 创建服务集合
	services := model.NewServices()

	// 创建管道构建器
	builder := service.NewPipelineBuilder(services)
	if builder == nil {
		t.Fatal("PipelineBuilder创建失败")
	}

	// 构建Amazon管道
	pipeline := builder.BuildAmazonPipeline()
	if pipeline == nil {
		t.Fatal("Amazon管道构建失败")
	}

	// 验证管道包含处理器
	if pipeline.GetHandlerCount() == 0 {
		t.Fatal("管道应包含处理器")
	}

	t.Logf("✅ 管道构建成功，包含 %d 个处理器", pipeline.GetHandlerCount())
}
