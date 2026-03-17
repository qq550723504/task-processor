// Package main 提供任务提交的示例程序
package main

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/app/messaging"
	"task-processor/internal/model"
	"task-processor/internal/domain/task"
	"task-processor/internal/infra/rabbitmq"

	"github.com/sirupsen/logrus"
)

func main() {
	// 1. 设置日志
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logger.Info("🚀 任务提交示例程序启动")

	// 2. 创建 RabbitMQ 连接配置
	connConfig := rabbitmq.ConnectionConfig{
		URL:               "amqp://guest:guest@localhost:5672/",
		ReconnectInterval: 5 * time.Second,
		MaxReconnectTries: 3,
	}

	// 3. 创建连接管理器
	connManager := rabbitmq.NewConnectionManager(connConfig, logger)

	// 4. 创建 RabbitMQ 客户端
	mqClient := rabbitmq.NewClient(connManager, logger)

	// 5. 创建任务提交服务（应用层）
	submitter := messaging.NewTaskSubmitter(mqClient, logger)

	// 6. 创建示例任务
	exampleTask := &model.Task{
		ID:            time.Now().UnixNano(),
		TenantID:      1,
		StoreID:       100,
		Platform:      "amazon",
		Region:        "US",
		CategoryID:    200,
		ProductID:     "B08N5WRWNW",
		Priority:      1, // 最高优先级
		RetryCount:    0,
		MaxRetryCount: 3,
		CreateTime:    time.Now().Unix(),
		UpdateTime:    time.Now().Unix(),
		Remark:        "example",
	}

	logger.Info("📝 准备提交任务")
	logger.Infof("   任务ID: %d", exampleTask.ID)
	logger.Infof("   平台: %s", exampleTask.Platform)
	logger.Infof("   区域: %s", exampleTask.Region)
	logger.Infof("   产品ID: %s", exampleTask.ProductID)
	logger.Infof("   优先级: %d", exampleTask.Priority)

	// 7. 提交任务
	ctx := context.Background()
	err := submitter.SubmitTask(ctx, exampleTask)
	if err != nil {
		logger.Fatalf("❌ 提交任务失败: %v", err)
	}

	logger.Info("✅ 任务提交成功")

	// 8. 演示领域层功能
	demonstrateDomainLayer(logger)

	// 9. 演示批量提交
	demonstrateBatchSubmission(submitter, exampleTask, logger)

	logger.Info("🎉 示例程序执行完成")
}

// demonstrateDomainLayer 演示领域层功能
func demonstrateDomainLayer(logger *logrus.Logger) {
	logger.Info("\n📚 演示领域层功能")

	// 1. 消息适配器
	adapter := task.NewMessageAdapter()

	// 获取队列名称
	queueName := adapter.GetQueueName("amazon")
	logger.Infof("   Amazon 队列名称: %s", queueName)

	// 计算优先级
	priority := adapter.CalculatePriority(1)
	logger.Infof("   业务优先级 1 -> 消息优先级: %d", priority)

	// 构建路由键
	exampleTask := &model.Task{
		Platform: "amazon",
		Priority: 1,
	}
	routingKey := adapter.BuildRoutingKey(exampleTask)
	logger.Infof("   路由键: %s", routingKey)

	// 2. 去重器
	dedup := task.NewDeduplicator(5*time.Minute, logger)
	defer dedup.Stop()

	taskID := int64(12345)

	// 检查是否重复
	isDup := dedup.IsDuplicate(taskID)
	logger.Infof("   任务 %d 是否重复: %v", taskID, isDup)

	// 标记为已处理
	dedup.MarkProcessed(taskID)
	logger.Infof("   任务 %d 已标记为已处理", taskID)

	// 再次检查
	isDup = dedup.IsDuplicate(taskID)
	logger.Infof("   任务 %d 是否重复: %v", taskID, isDup)

	// 获取统计信息
	stats := dedup.GetStats()
	logger.Infof("   去重器统计: %+v", stats)
}

// demonstrateBatchSubmission 演示批量提交
func demonstrateBatchSubmission(submitter *messaging.TaskSubmitter, parentTask *model.Task, logger *logrus.Logger) {
	logger.Info("\n📦 演示批量变体任务提交")

	// 创建变体列表
	variations := []model.Variation{
		{Asin: "B08N5WRWN1", Name: "变体1 - 红色"},
		{Asin: "B08N5WRWN2", Name: "变体2 - 蓝色"},
		{Asin: "B08N5WRWN3", Name: "变体3 - 绿色"},
	}

	logger.Infof("   准备提交 %d 个变体任务", len(variations))

	// 批量提交
	ctx := context.Background()
	successCount, failCount := submitter.SubmitVariantTasks(
		ctx,
		parentTask,
		variations,
		parentTask.ProductID,
	)

	logger.Infof("   提交结果: 成功=%d, 失败=%d", successCount, failCount)
}

// 使用说明
func printUsage() {
	fmt.Println(`任务提交示例程序

功能：
1. 演示如何使用新的分层架构提交任务
2. 演示领域层的业务规则（消息适配、去重）
3. 演示应用层的流程编排（单个提交、批量提交）

使用前提：
- RabbitMQ 服务已启动（localhost:5672）
- 已创建相应的队列

运行方式：
  go run examples/task_submission_example.go

架构说明：
  应用层 (app/messaging)
    ↓ 使用
  领域层 (domain/task)
    ↓ 使用
  基础设施层 (infra/rabbitmq)

更多信息：
- 快速入门: docs/QUICK_START_NEW_STRUCTURE.md
- 领域层文档: internal/domain/task/README.md
- 应用层文档: internal/app/messaging/README.md`)
}

