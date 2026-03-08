# 示例程序

本目录包含使用新架构的示例程序，帮助你快速了解如何使用重构后的代码。

## 📁 示例列表

### task_submission_example.go

演示如何使用新的分层架构提交任务。

**功能：**
- ✅ 单个任务提交
- ✅ 批量变体任务提交
- ✅ 领域层业务规则演示（消息适配、去重）
- ✅ 应用层流程编排演示

**运行前提：**
1. RabbitMQ 服务已启动（默认：localhost:5672）
2. 已创建相应的队列

**运行方式：**
```bash
cd task-processor
go run examples/task_submission_example.go
```

**预期输出：**
```
🚀 任务提交示例程序启动
📝 准备提交任务
   任务ID: 1234567890
   平台: amazon
   区域: US
   产品ID: B08N5WRWNW
   优先级: 1
✅ 任务提交成功

📚 演示领域层功能
   Amazon 队列名称: amazon.tasks.queue
   业务优先级 1 -> 消息优先级: 10
   路由键: amazon.urgent
   任务 12345 是否重复: false
   任务 12345 已标记为已处理
   任务 12345 是否重复: true
   去重器统计: map[total_records:1 ttl_seconds:300]

📦 演示批量变体任务提交
   准备提交 3 个变体任务
   提交结果: 成功=3, 失败=0

🎉 示例程序执行完成
```

## 🏗️ 架构说明

示例程序展示了新的三层架构：

```
┌─────────────────────────────────────┐
│      应用层 (app/messaging)          │
│  - TaskSubmitter                     │
│  - 流程编排                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│      领域层 (domain/task)            │
│  - MessageAdapter                    │
│  - Deduplicator                      │
│  - 业务规则                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│   基础设施层 (infra/rabbitmq)        │
│  - Client                            │
│  - ConnectionManager                 │
│  - 技术实现                          │
└─────────────────────────────────────┘
```

## 📝 代码示例

### 1. 基本任务提交

```go
import (
    "task-processor/internal/app/messaging"
    "task-processor/internal/domain/model"
    "task-processor/internal/infra/rabbitmq"
)

// 创建 RabbitMQ 客户端
connManager := rabbitmq.NewConnectionManager(config, logger)
mqClient := rabbitmq.NewClient(connManager, logger)

// 创建任务提交服务
submitter := messaging.NewTaskSubmitter(mqClient, logger)

// 创建任务
task := &model.Task{
    ID:       12345,
    Platform: "amazon",
    Region:   "US",
    Priority: 1,
}

// 提交任务
err := submitter.SubmitTask(context.Background(), task)
```

### 2. 使用领域层业务规则

```go
import "task-processor/internal/domain/task"

// 消息适配器
adapter := task.NewMessageAdapter()
queueName := adapter.GetQueueName("amazon")
priority := adapter.CalculatePriority(1)

// 去重器
dedup := task.NewDeduplicator(5*time.Minute, logger)
defer dedup.Stop()

if !dedup.IsDuplicate(taskID) {
    // 处理任务
    processTask(taskID)
    dedup.MarkProcessed(taskID)
}
```

### 3. 批量提交变体任务

```go
variations := []model.Variation{
    {Asin: "B001", Name: "红色"},
    {Asin: "B002", Name: "蓝色"},
}

successCount, failCount := submitter.SubmitVariantTasks(
    ctx,
    parentTask,
    variations,
    parentAsin,
)
```

## 🧪 测试示例

查看单元测试了解更多使用方式：

```bash
# 运行领域层测试
go test -v ./internal/domain/task/

# 查看测试覆盖率
go test -cover ./internal/domain/task/

# 运行性能测试
go test -bench=. ./internal/domain/task/
```

## 📚 相关文档

- [新架构快速入门](../docs/QUICK_START_NEW_STRUCTURE.md)
- [领域层文档](../internal/domain/task/README.md)
- [应用层文档](../internal/app/messaging/README.md)
- [重构总结](../docs/REFACTORING_SUMMARY.md)

## 💡 最佳实践

### 1. 依赖注入

```go
// 推荐：使用依赖注入
func NewService(submitter *messaging.TaskSubmitter) *Service {
    return &Service{submitter: submitter}
}

// 不推荐：在内部创建依赖
func NewService() *Service {
    submitter := messaging.NewTaskSubmitter(...)  // ❌
    return &Service{submitter: submitter}
}
```

### 2. 错误处理

```go
err := submitter.SubmitTask(ctx, task)
if err != nil {
    logger.WithFields(logrus.Fields{
        "task_id": task.ID,
        "error":   err,
    }).Error("提交任务失败")
    return fmt.Errorf("提交任务失败: %w", err)
}
```

### 3. 上下文传递

```go
// 使用带超时的上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := submitter.SubmitTask(ctx, task)
```

### 4. 资源清理

```go
dedup := task.NewDeduplicator(ttl, logger)
defer dedup.Stop()  // 确保清理资源

// 或使用 context 控制生命周期
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

## 🔧 故障排查

### 问题：连接 RabbitMQ 失败

**解决方案：**
1. 检查 RabbitMQ 是否启动
2. 检查连接配置是否正确
3. 检查网络连接

### 问题：任务提交失败

**解决方案：**
1. 检查队列是否存在
2. 检查权限配置
3. 查看日志获取详细错误信息

### 问题：去重不生效

**解决方案：**
1. 检查 TTL 配置是否合理
2. 确保 taskID 唯一
3. 检查是否正确调用 MarkProcessed

## 📞 获取帮助

如有问题：
1. 查看相关文档
2. 查看测试代码
3. 提交 Issue
4. 联系团队成员

---

**最后更新：** 2024-01  
**维护者：** 开发团队
