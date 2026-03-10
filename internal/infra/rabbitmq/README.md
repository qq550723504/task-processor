# rabbitmq 目录

## 用途

提供 RabbitMQ 消息队列的核心封装，包括连接管理、消息生产、消息消费、负载监控等功能。

## 目录结构

```
rabbitmq/
├── README.md              # 本文件
├── client.go              # RabbitMQ 客户端封装
├── config.go              # 配置结构定义
├── connection.go          # 连接管理器（支持自动重连）
├── consumer.go            # 消息消费者管理器
├── load_monitor.go        # 负载监控器
├── message.go             # 消息处理器接口
└── queue_consumer.go      # 队列消费者（Worker Pool模式）
```

## 核心组件

### 1. Client（RabbitMQ 客户端）

**职责：**
- 封装 RabbitMQ 连接和通道操作
- 提供消息发布和消费接口
- 管理队列和交换机声明

**主要方法：**
```go
// 创建客户端
func NewClient(connManager *ConnectionManager, logger *logrus.Logger) *Client

// 声明队列
func (c *Client) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error

// 声明交换机
func (c *Client) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error

// 发布消息
func (c *Client) Publish(ctx context.Context, msg *Message, opts PublishOptions) error

// 消费消息
func (c *Client) Consume(ctx context.Context, opts ConsumeOptions) (<-chan amqp.Delivery, error)

// 解析消息（支持多种格式）
func (c *Client) ParseMessage(delivery amqp.Delivery) (*Message, error)
```

### 2. ConnectionManager（连接管理器）

**职责：**
- 管理 RabbitMQ 连接和通道
- 自动重连机制
- 连接健康监控

**特性：**
- 自动重连（可配置重连间隔和最大次数）
- 连接健康检查
- 支持重连回调注册
- 错误恢复

**主要方法：**
```go
// 创建连接管理器
func NewConnectionManager(config ConnectionConfig, logger *logrus.Logger) *ConnectionManager

// 建立连接
func (cm *ConnectionManager) Connect(ctx context.Context) error

// 获取通道
func (cm *ConnectionManager) GetChannel() (*amqp.Channel, error)

// 创建新通道（用于消费者独立通道）
func (cm *ConnectionManager) CreateChannel() (*amqp.Channel, error)

// 注册重连回调
func (cm *ConnectionManager) RegisterReconnectCallback(callback func() error)

// 检查连接状态
func (cm *ConnectionManager) IsConnected() bool
```

### 3. MessageConsumer（消息消费者管理器）

**职责：**
- 从队列消费消息
- 多队列并发消费
- 消息确认和重试
- 优先级队列支持

**配置示例：**
```yaml
consumer:
  prefetch_count: 10        # 预取数量
  prefetch_size: 0          # 预取大小
  retry_delay: 5s           # 重试延迟
  max_retries: 3            # 最大重试次数

queues:
  - name: task.temu
    priority: 10            # 高优先级
    prefetch: 5
  - name: task.shein
    priority: 5             # 中优先级
    prefetch: 10
```

**主要方法：**
```go
// 创建消费者管理器
func NewMessageConsumer(client *Client, config ConsumerConfig, logger *logrus.Logger) *MessageConsumer

// 设置队列配置
func (mc *MessageConsumer) SetQueueConfigs(configs []ConsumerQueueConfig)

// 注册消息处理器
func (mc *MessageConsumer) RegisterHandler(queueName string, handler MessageHandler)

// 启动消费者
func (mc *MessageConsumer) Start(ctx context.Context) error

// 重启所有消费者（用于重连后恢复）
func (mc *MessageConsumer) Restart() error

// 停止消费者
func (mc *MessageConsumer) Stop(ctx context.Context) error
```

### 4. QueueConsumer（队列消费者）

**职责：**
- 单个队列的消息消费
- Worker Pool 并发处理
- 消息重试和死信处理

**特性：**
- 根据 prefetch 数量创建对应的 worker
- 自动消息重试（可配置最大次数）
- 失败消息发送到死信队列
- Panic 恢复机制

### 5. LoadMonitor（负载监控器）

**职责：**
- 监控系统资源使用
- 收集任务处理统计
- 提供健康状态检查
- 集成通用指标收集器

**监控指标：**
- CPU 使用率
- 内存使用率
- Goroutine 数量
- 任务处理统计（成功/失败/总数）
- 队列消息处理统计

**主要方法：**
```go
// 创建监控器
func NewLoadMonitor(cfg config.LoadMonitorConfig, logger *logrus.Logger) *LoadMonitor

// 启动监控
func (lm *LoadMonitor) Start(ctx context.Context) error

// 记录任务处理
func (lm *LoadMonitor) RecordTaskProcessed(queueName string, success bool, processingTime time.Duration)

// 获取统计信息
func (lm *LoadMonitor) GetStats() LoadStats

// 获取健康状态
func (lm *LoadMonitor) GetHealthStatus() map[string]any
```

## 消息格式

### 任务消息格式

```json
{
  "task_id": 123,
  "tenant_id": 1,
  "store_id": 1,
  "platform": "temu",
  "region": "US",
  "category_id": 100,
  "product_id": "abc123",
  "priority": 5,
  "retry_count": 0,
  "max_retry_count": 3,
  "created_at": 1640000000,
  "remark": ""
}
```

### 结果消息格式

```json
{
  "task_id": 123,
  "status": "completed",
  "data": {
    "product": {...}
  },
  "error": "",
  "processed_at": 1640000000
}
```

## 队列设计

### 队列结构

```
Exchange: task.exchange (topic)
├── Queue: task.temu
│   ├── Routing Key: task.temu.#
│   ├── Priority: 10
│   └── DLX: task.dlx
├── Queue: task.shein
│   ├── Routing Key: task.shein.#
│   ├── Priority: 10
│   └── DLX: task.dlx
└── Queue: task.dlq (Dead Letter Queue)
    └── Routing Key: task.dlq.#
```

### 队列属性

```go
QueueOptions{
    Name:       "task.temu",
    Durable:    true,              // 持久化
    AutoDelete: false,             // 不自动删除
    Exclusive:  false,             // 非独占
    NoWait:     false,
    Args: amqp.Table{
        "x-max-priority":           10,                    // 最大优先级
        "x-dead-letter-exchange":   "task.dlx",           // 死信交换机
        "x-dead-letter-routing-key": "task.dlq",          // 死信路由键
        "x-message-ttl":            3600000,              // 消息TTL (1小时)
    },
}
```

## 使用示例

### 完整启动流程

```go
package main

import (
    "context"
    "task-processor/internal/infra/rabbitmq"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()
    ctx := context.Background()
    
    // 1. 创建连接管理器
    connConfig := rabbitmq.ConnectionConfig{
        URL:               "amqp://guest:guest@localhost:5672/",
        ReconnectInterval: 5 * time.Second,
        MaxReconnectTries: 10,
    }
    connManager := rabbitmq.NewConnectionManager(connConfig, logger)
    
    // 2. 建立连接
    err := connManager.Connect(ctx)
    if err != nil {
        logger.Fatal(err)
    }
    defer connManager.Close()
    
    // 3. 创建客户端
    client := rabbitmq.NewClient(connManager, logger)
    
    // 4. 创建消费者
    consumerConfig := rabbitmq.ConsumerConfig{
        PrefetchCount: 10,
        PrefetchSize:  0,
        RetryDelay:    5 * time.Second,
        MaxRetries:    3,
    }
    consumer := rabbitmq.NewMessageConsumer(client, consumerConfig, logger)
    
    // 5. 设置队列配置
    consumer.SetQueueConfigs([]rabbitmq.ConsumerQueueConfig{
        {Name: "task.temu", Priority: 10, Prefetch: 5},
        {Name: "task.shein", Priority: 5, Prefetch: 10},
    })
    
    // 6. 注册处理器
    consumer.RegisterHandler("task.temu", &MyTemuHandler{logger: logger})
    consumer.RegisterHandler("task.shein", &MySheinHandler{logger: logger})
    
    // 7. 注册重连回调
    connManager.RegisterReconnectCallback(func() error {
        return consumer.Restart()
    })
    
    // 8. 启动消费者
    err = consumer.Start(ctx)
    if err != nil {
        logger.Fatal(err)
    }
    defer consumer.Stop(ctx)
    
    // 9. 等待信号
    <-ctx.Done()
}
```

### 自定义消息处理器

```go
type MyMessageHandler struct {
    logger *logrus.Logger
}

func (h *MyMessageHandler) HandleMessage(ctx context.Context, msg *rabbitmq.Message) error {
    h.logger.Infof("处理消息: ID=%s, Type=%s", msg.ID, msg.Type)
    
    // 解析消息
    taskID, ok := msg.Payload["task_id"].(float64)
    if !ok {
        return fmt.Errorf("无效的 task_id")
    }
    
    platform, ok := msg.Payload["platform"].(string)
    if !ok {
        return fmt.Errorf("无效的 platform")
    }
    
    // 处理业务逻辑
    err := processTask(int64(taskID), platform)
    if err != nil {
        return fmt.Errorf("处理任务失败: %w", err)
    }
    
    h.logger.Infof("任务处理成功: TaskID=%d", int64(taskID))
    return nil
}

func processTask(taskID int64, platform string) error {
    // 实现具体的业务逻辑
    return nil
}
```

## 配置说明

### 完整配置示例

```yaml
rabbitmq:
  # 连接配置
  connection:
    url: "amqp://guest:guest@localhost:5672/"
    reconnect_interval: 5s
    max_reconnect_tries: 10
    
  # 消费者配置
  consumer:
    prefetch_count: 10
    prefetch_size: 0
    retry_delay: 5s
    max_retries: 3
    
  # 队列配置
  queues:
    - name: task.temu
      priority: 10      # 高优先级
      prefetch: 5       # 低预取，快速处理
    - name: task.shein
      priority: 5       # 中优先级
      prefetch: 10      # 高预取，提高吞吐量
    - name: task.amazon
      priority: 3       # 低优先级
      prefetch: 15
```

### 配置项说明

#### connection（连接配置）
- `url`: RabbitMQ 连接地址
- `reconnect_interval`: 重连间隔（已弃用，使用指数退避策略）
- `max_reconnect_tries`: 最大重连次数

#### consumer（消费者配置）
- `prefetch_count`: 预取消息数量（默认值）
- `prefetch_size`: 预取消息大小（字节）
- `retry_delay`: 消息重试延迟
- `max_retries`: 消息最大重试次数

#### queues（队列配置）
- `name`: 队列名称
- `priority`: 队列优先级（1-10，数字越大优先级越高）
- `prefetch`: 该队列的预取数量（覆盖默认值）

### 配置验证

配置结构提供了自动验证功能：

```go
config := &rabbitmq.Config{
    Connection: rabbitmq.ConnectionConfig{
        URL: "amqp://guest:guest@localhost:5672/",
    },
    Consumer: rabbitmq.ConsumerConfig{
        PrefetchCount: 10,
    },
    Queues: []rabbitmq.QueueConfig{
        {Name: "task.temu", Priority: 10, Prefetch: 5},
    },
}

// 设置默认值
config.SetDefaults()

// 验证配置
if err := config.Validate(); err != nil {
    log.Fatalf("配置验证失败: %v", err)
}
```

## 监控和运维

### 负载监控

LoadMonitor 集成了通用的 MetricsCollector，提供以下功能：

```go
// 创建监控器
monitor := rabbitmq.NewLoadMonitor(config.LoadMonitorConfig, logger)

// 启动监控
err := monitor.Start(ctx)

// 记录任务处理
monitor.RecordTaskProcessed("task.temu", true, 100*time.Millisecond)

// 获取统计信息
stats := monitor.GetStats()
fmt.Printf("已处理: %d, 成功: %d, 失败: %d\n", 
    stats.TasksProcessed, stats.TasksSucceeded, stats.TasksFailed)

// 获取健康状态
health := monitor.GetHealthStatus()
fmt.Printf("状态: %v\n", health["status"])
```

### 统计信息

```go
stats := monitor.GetStats()
// stats 包含:
// - TasksProcessed: 处理的任务总数
// - TasksSucceeded: 成功的任务数
// - TasksFailed: 失败的任务数
// - TasksRetried: 重试的任务数
// - AvgProcessingTime: 平均处理时间
// - MaxProcessingTime: 最大处理时间
// - MinProcessingTime: 最小处理时间
// - QueueStats: 各队列的统计信息
```

## 错误处理

### 连接失败

ConnectionManager 实现了自动重连机制，使用指数退避策略：

```go
// 重连策略配置
// - 初始延迟: 1秒
// - 最大延迟: 30秒
// - 倍数: 2.0
// - 最大重试次数: 配置的 max_reconnect_tries

// 重连延迟示例:
// 第1次: 1秒
// 第2次: 2秒
// 第3次: 4秒
// 第4次: 8秒
// 第5次: 16秒
// 第6次: 30秒（达到最大延迟）
```

### 消息处理失败

QueueConsumer 实现了消息重试机制：

1. 消息处理失败时，递增 RetryCount
2. 如果 RetryCount < MaxRetries：重新发布消息到队列
3. 如果 RetryCount >= MaxRetries：发送到死信队列

```go
// 重试流程:
// 1. 处理失败 -> RetryCount++
// 2. 判断是否应该重试
// 3. 是: 重新发布消息（带更新的 RetryCount）
// 4. 否: Nack(requeue=false) -> 进入死信队列
```

### Panic 恢复

所有消息处理都有 panic 恢复机制：

```go
defer func() {
    if r := recover(); r != nil {
        logger.Errorf("处理消息发生panic: %v", r)
        // Panic时拒绝消息并重新排队
        delivery.Nack(false, true)
    }
}()
```

## 性能优化

### 1. 独立通道

每个队列消费者使用独立的通道，避免 QoS 设置冲突：

```go
// 每个队列创建独立通道
channel, err := connManager.CreateChannel()

// 在独立通道上设置 QoS
channel.Qos(prefetch, 0, false)
```

### 2. Worker Pool

QueueConsumer 根据 prefetch 数量创建对应的 worker：

```go
// prefetch=5 -> 创建5个worker并发处理
for i := 0; i < prefetch; i++ {
    go worker(i)
}
```

### 3. 预取优化

根据队列优先级设置不同的预取数量：

```yaml
queues:
  - name: task.temu
    priority: 10
    prefetch: 5      # 高优先级：低预取，快速处理
  - name: task.shein
    priority: 5
    prefetch: 10     # 中优先级：中等预取
  - name: task.amazon
    priority: 3
    prefetch: 15     # 低优先级：高预取，提高吞吐量
```

### 4. 消息持久化

只对重要消息持久化：

```go
publishing := amqp.Publishing{
    DeliveryMode: 2,  // 2 = persistent
    // ...
}
```

## 最佳实践

1. **连接管理**
   - 使用 ConnectionManager 统一管理连接
   - 注册重连回调以恢复消费者
   - 监控连接状态

2. **消息确认**
   - 使用手动确认模式
   - 处理成功后再确认
   - 失败时合理重试

3. **错误处理**
   - 区分可重试和不可重试错误
   - 使用死信队列处理失败消息
   - 记录详细日志

4. **性能优化**
   - 根据业务设置合理的 prefetch
   - 高优先级队列使用低 prefetch
   - 使用独立通道避免 QoS 冲突

5. **监控告警**
   - 使用 LoadMonitor 收集统计信息
   - 监控队列长度和消费速率
   - 集成到监控系统

6. **优雅关闭**
   - 停止接收新消息
   - 等待当前消息处理完成
   - 关闭连接和通道

## 架构特点

### 优点

1. **职责分离清晰**: ConnectionManager、Client、Consumer 各司其职
2. **并发安全**: 正确使用 RWMutex 保护共享状态
3. **自动重连**: 使用指数退避策略，智能重连
4. **独立通道**: 每个队列独立通道，避免 QoS 冲突
5. **Worker Pool**: 根据 prefetch 创建对应数量的 worker
6. **配置灵活**: 支持队列级别的优先级和 prefetch 配置
7. **监控完善**: 集成通用 MetricsCollector

### 注意事项

1. 确保 RabbitMQ 服务可用
2. 合理设置预取数量
3. 注意消息大小限制
4. 定期清理死信队列
5. 监控内存使用
6. 避免消息堆积

## 分层说明

本 infra 层只负责基础设施相关的功能：

- **连接管理**: ConnectionManager
- **消息收发**: Client
- **消费管理**: MessageConsumer, QueueConsumer
- **负载监控**: LoadMonitor
- **重试策略**: RetryStrategy

业务逻辑相关的功能在其他层：

- **消息适配**: `internal/domain/task/message_adapter.go`（领域层）
- **任务处理**: `internal/app/messaging/task_handler.go`（应用层）
- **结果上报**: `internal/app/messaging/result_reporter.go`（应用层）

这样的分层确保了关注点分离和代码的可维护性。
