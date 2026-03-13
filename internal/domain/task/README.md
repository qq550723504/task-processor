# Task 领域层

## 概述

`internal/domain/task` 包含任务相关的核心业务规则和领域逻辑，独立于具体的技术实现（如 RabbitMQ、HTTP 等）。

## 设计原则

1. **业务规则集中** - 所有任务相关的业务规则都在这里定义
2. **技术无关** - 不依赖具体的基础设施实现
3. **可测试性** - 业务逻辑可以独立测试，无需启动外部服务
4. **可复用性** - 业务规则可以在不同的应用场景中复用

## 文件说明

### deduplicator.go
任务去重器，负责防止重复任务的提交和处理。

**核心功能：**
- 检查任务是否重复（基于 taskID）
- 标记任务为已处理
- 自动清理过期记录（TTL 机制）
- 提供统计信息

**使用示例：**
```go
// 创建去重器（10分钟TTL）
dedup := task.NewDeduplicator(10*time.Minute, logger)

// 检查是否重复
if dedup.IsDuplicate(taskID) {
    log.Info("任务重复，跳过")
    return
}

// 处理任务...

// 标记为已处理
dedup.MarkProcessed(taskID)

// 停止去重器
defer dedup.Stop()
```

### message_adapter.go
任务消息适配器，负责任务对象与消息格式之间的转换。

**核心功能：**
- 任务对象 ↔ 消息格式转换
- 平台到队列的映射（业务规则）
- 优先级计算（业务规则）
- 路由键构建（业务规则）
- 状态码转换（字符串 ↔ int16）

**使用示例：**
```go
// 创建适配器
adapter := task.NewMessageAdapter()

// 任务转消息
taskMsg, err := adapter.TaskToMessage(task)
if err != nil {
    return err
}

// 消息转任务
task, err := adapter.MessageToTask(msg)
if err != nil {
    return err
}

// 获取队列名称
queueName := adapter.GetQueueName("amazon") // "amazon.tasks.normal"

// 计算优先级
priority := adapter.CalculatePriority(1) // 业务优先级1 -> 消息优先级10

// 构建路由键
routingKey := adapter.BuildRoutingKey(task) // "amazon.urgent"
```

## 业务规则

### 1. 平台队列映射
```
amazon -> amazon.tasks.normal
temu   -> temu.tasks.normal
shein  -> shein.tasks.normal
其他   -> amazon.tasks.normal (默认)
```

### 2. 优先级映射
```
业务优先级 1-3  -> urgent (紧急)
业务优先级 4-6  -> high (高)
业务优先级 7-8  -> normal (普通)
业务优先级 9-10 -> low (低)
```

### 3. 优先级计算
```
业务优先级 1 (最高) -> 消息优先级 10
业务优先级 10 (最低) -> 消息优先级 1
公式: 消息优先级 = 11 - 业务优先级
```

### 4. 状态码映射
```go
字符串状态 -> int16状态
"pending"       -> 0  (待处理)
"processing"    -> 1  (处理中)
"crawled"       -> 2  (已爬取)
"failed"        -> 3  (失败)
"pending_retry" -> 4  (待重试)
"queued"        -> 5  (已排队)
"completed"     -> 6  (已完成)
"draft"         -> 8  (草稿)
"paused"        -> 10 (暂停)
"terminated"    -> 13 (终止)
```

### 5. 去重规则
- 基于 taskID 进行去重
- 默认 TTL 为 10 分钟
- 自动清理过期记录（每半个 TTL 清理一次）

## 测试示例

### 测试去重器
```go
func TestDeduplicator(t *testing.T) {
    logger := logrus.New()
    dedup := task.NewDeduplicator(5*time.Minute, logger)
    defer dedup.Stop()

    taskID := int64(12345)

    // 第一次不重复
    assert.False(t, dedup.IsDuplicate(taskID))

    // 标记为已处理
    dedup.MarkProcessed(taskID)

    // 第二次重复
    assert.True(t, dedup.IsDuplicate(taskID))
}
```

### 测试消息适配器
```go
func TestMessageAdapter(t *testing.T) {
    adapter := task.NewMessageAdapter()

    // 测试队列映射
    assert.Equal(t, "amazon.tasks.normal", adapter.GetQueueName("amazon"))
    assert.Equal(t, "temu.tasks.normal", adapter.GetQueueName("temu"))

    // 测试优先级计算
    assert.Equal(t, uint8(10), adapter.CalculatePriority(1)) // 最高优先级
    assert.Equal(t, uint8(1), adapter.CalculatePriority(10)) // 最低优先级

    // 测试任务转换
    task := &model.Task{
        ID:       12345,
        Platform: "amazon",
        Priority: 1,
    }
    
    taskMsg, err := adapter.TaskToMessage(task)
    assert.NoError(t, err)
    assert.Equal(t, int64(12345), taskMsg.TaskID)
}
```

## 依赖关系

```
domain/task (领域层)
    ↓ 依赖
domain/model (领域模型)
    ↑ 被依赖
app/messaging (应用层)
infra/rabbitmq (基础设施层)
```

## 优势

1. **独立测试** - 无需启动 RabbitMQ 即可测试业务规则
2. **易于维护** - 业务规则集中管理，修改影响范围小
3. **可复用** - 可以在不同的消息队列实现中复用（RabbitMQ、Kafka 等）
4. **清晰边界** - 业务逻辑与技术实现分离

## 注意事项

1. 领域层不应该依赖基础设施层
2. 业务规则的修改应该在这里进行
3. 保持领域对象的纯粹性，避免引入技术细节
4. 使用接口定义依赖，而不是具体实现
