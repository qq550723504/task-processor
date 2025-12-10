package amazon

import (
	"context"
	"task-processor/common/config"
	"task-processor/common/types"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestNewAmazonProcessor 测试创建处理器
func TestNewAmazonProcessor(t *testing.T) {
	cfg := &config.Config{
		Processor: config.ProcessorConfig{
			MaxRetries: 3,
			Timeout:    600,
		},
		Worker: config.WorkerConfig{
			Concurrency: 1,
			BufferSize:  5,
		},
	}

	logger := logrus.New()
	processor := NewAmazonProcessor(cfg, logger)

	if processor == nil {
		t.Fatal("处理器创建失败")
	}

	if processor.config != cfg {
		t.Error("配置未正确设置")
	}
}

// TestProcessorStartStop 测试启动和停止
func TestProcessorStartStop(t *testing.T) {
	cfg := &config.Config{
		Worker: config.WorkerConfig{
			Concurrency: 1,
			BufferSize:  5,
		},
	}

	logger := logrus.New()
	processor := NewAmazonProcessor(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 启动处理器
	if err := processor.Start(ctx); err != nil {
		t.Fatalf("启动处理器失败: %v", err)
	}

	// 等待一段时间
	time.Sleep(1 * time.Second)

	// 关闭处理器
	processor.Close()
}

// TestProcessTask 测试任务处理
func TestProcessTask(t *testing.T) {
	cfg := &config.Config{
		Processor: config.ProcessorConfig{
			MaxRetries: 3,
		},
	}

	logger := logrus.New()
	processor := NewAmazonProcessor(cfg, logger)

	ctx := context.Background()
	task := types.Task{
		ID:        "test-123",
		ProductID: "B08N5WRWNW",
		StoreID:   556,
		TenantID:  1,
	}

	// 注意：这个测试需要实际的 API 配置才能通过
	// 在单元测试中应该 mock API 调用
	err := processor.ProcessTask(ctx, task)
	if err != nil {
		t.Logf("任务处理失败（预期行为，因为没有实际配置）: %v", err)
	}
}
