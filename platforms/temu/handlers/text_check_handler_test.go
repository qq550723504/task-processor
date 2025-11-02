package handlers

import (
	"context"
	"strings"
	"task-processor/common/pipeline"
	"task-processor/common/types"
	"testing"
)

func TestTextCheckHandler_Handle_NoAPIClient(t *testing.T) {
	// 创建处理器
	handler := NewTextCheckHandler()

	// 创建任务上下文（不设置API客户端）
	task := &types.Task{
		ID:        "test-001",
		ProductID: "B08N5WRWNW",
		StoreID:   12345,
		Platform:  "amazon",
	}
	ctx := context.Background()
	taskCtx := pipeline.NewTaskContext(ctx, task)

	// 执行处理器
	err := handler.Handle(taskCtx)

	// 验证返回错误
	if err == nil {
		t.Fatal("期望返回错误，但得到nil")
	}

	// 验证错误消息包含预期内容
	expectedMsg := "API客户端未初始化"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("期望错误消息包含: %s, 但得到: %s", expectedMsg, err.Error())
	}
}
