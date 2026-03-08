# Messaging 应用层

## 概述

`internal/app/messaging` 包含消息处理相关的应用层服务，负责协调领域层和基础设施层，实现完整的业务流程。

## 设计原则

1. **应用编排** - 协调多个领域服务和基础设施服务
2. **流程控制** - 实现完整的业务流程（如任务提交、结果上报）
3. **依赖管理** - 管理对领域层和基础设施层的依赖
4. **事务边界** - 定义应用级的事务边界

## 文件说明

### task_submitter.go
任务提交服务，负责协调任务提交的完整流程。

**核心功能：**
- 单个任务提交
- 批量变体任务提交
- 提交去重（5分钟缓存）
- 自动清理过期缓存

**使用示例：**
```go
// 创建任务提交服务
submitter := messaging.NewTaskSubmitter(mqClient, logger)

// 提交单个任务
err := submitter.SubmitTask(ctx, task)
if err != nil {
    log.Errorf("提交任务失败: %v", err)
}

// 批量提交变体任务
successCount, failCount := submitter.SubmitVariantTasks(
    ctx, 
    parentTask, 
    variations, 
    parentAsin,
)
log.Infof("提交完成: 成功=%d, 失败=%d", successCount, failCount)
```

**提交流程：**
```
1. 使用领域层适配器转换任务
   ↓
2. 序列化为JSON格式
   ↓
3. 获取队列名称和优先级（领域规则）
   ↓
4. 获取RabbitMQ通道（基础设施）
   ↓
5. 构建发布消息
   ↓
6. 发布到队列
```

**变体任务过滤规则：**
1. 跳过父ASIN本身
2. 跳过当前任务的ProductID
3. 跳过5分钟内已提交的任务

## 依赖关系

```
app/messaging (应用层)
    ↓ 依赖
domain/task (领域层) - 业务规则
domain/model (领域模型) - 数据模型
infra/rabbitmq (基础设施层) - 技术实现
```

## 架构分层

```
┌─────────────────────────────────────┐
│      app/messaging (应用层)          │
│  - 流程编排                          │
│  - 服务协调                          │
│  - 事务管理                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│      domain/task (领域层)            │
│  - 业务规则                          │
│  - 领域逻辑                          │
│  - 消息适配                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│   infra/rabbitmq (基础设施层)        │
│  - RabbitMQ客户端                    │
│  - 连接管理                          │
│  - 消息发布/消费                     │
└─────────────────────────────────────┘
```

## 测试示例

### 单元测试（Mock基础设施）
```go
func TestTaskSubmitter_SubmitTask(t *testing.T) {
    // 创建Mock客户端
    mockClient := &MockRabbitMQClient{}
    logger := logrus.New()
    
    submitter := messaging.NewTaskSubmitter(mockClient, logger)
    
    task := &model.Task{
        ID:       12345,
        Platform: "amazon",
        Priority: 1,
    }
    
    err := submitter.SubmitTask(context.Background(), task)
    assert.NoError(t, err)
    
    // 验证Mock调用
    assert.Equal(t, 1, mockClient.PublishCallCount)
}
```

### 集成测试（真实RabbitMQ）
```go
func TestTaskSubmitter_Integration(t *testing.T) {
    // 连接真实RabbitMQ
    client := rabbitmq.NewClient(config, logger)
    err := client.Connect(context.Background())
    require.NoError(t, err)
    defer client.Close()
    
    submitter := messaging.NewTaskSubmitter(client, logger)
    
    task := &model.Task{
        ID:       12345,
        Platform: "amazon",
        Priority: 1,
    }
    
    err = submitter.SubmitTask(context.Background(), task)
    assert.NoError(t, err)
    
    // 验证消息已发送到队列
    // ...
}
```

## 最佳实践

### 1. 错误处理
```go
err := submitter.SubmitTask(ctx, task)
if err != nil {
    // 记录详细错误信息
    logger.WithFields(logrus.Fields{
        "task_id":  task.ID,
        "platform": task.Platform,
        "error":    err,
    }).Error("提交任务失败")
    
    // 返回业务错误
    return fmt.Errorf("提交任务失败: %w", err)
}
```

### 2. 上下文传递
```go
// 使用带超时的上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := submitter.SubmitTask(ctx, task)
```

### 3. 批量操作
```go
// 批量提交时使用goroutine池
const maxWorkers = 10
sem := make(chan struct{}, maxWorkers)

for _, task := range tasks {
    sem <- struct{}{} // 获取信号量
    
    go func(t *model.Task) {
        defer func() { <-sem }() // 释放信号量
        
        if err := submitter.SubmitTask(ctx, t); err != nil {
            logger.Errorf("提交任务失败: %v", err)
        }
    }(task)
}

// 等待所有任务完成
for i := 0; i < maxWorkers; i++ {
    sem <- struct{}{}
}
```

### 4. 监控和指标
```go
// 记录提交指标
start := time.Now()
err := submitter.SubmitTask(ctx, task)
duration := time.Since(start)

metrics.RecordTaskSubmission(task.Platform, err == nil, duration)
```

## 扩展点

### 1. 添加新的消息服务
```go
// result_reporter.go - 结果上报服务
type ResultReporter struct {
    httpClient *http.Client
    reportURL  string
    logger     *logrus.Logger
}

func (r *ResultReporter) ReportResult(ctx context.Context, result *model.Result) error {
    // 实现结果上报逻辑
}
```

### 2. 添加消息消费服务
```go
// task_consumer.go - 任务消费服务
type TaskConsumer struct {
    client    *rabbitmq.Client
    adapter   *task.MessageAdapter
    processor worker.Processor
    logger    *logrus.Logger
}

func (c *TaskConsumer) Start(ctx context.Context) error {
    // 实现消息消费逻辑
}
```

## 注意事项

1. 应用层不应该包含业务规则，业务规则应该在领域层
2. 应用层负责协调，不负责具体实现
3. 保持应用服务的薄层特性，避免过度膨胀
4. 使用依赖注入，便于测试和替换实现
5. 合理使用上下文传递超时和取消信号
