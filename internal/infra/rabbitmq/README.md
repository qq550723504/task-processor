# rabbitmq 目录

## 用途

提供 RabbitMQ 消息队列的完整封装，包括连接管理、消息生产、消息消费、任务提交、结果上报、负载监控等功能。

## 目录结构

```
rabbitmq/
├── README.md              # 本文件
├── client.go              # RabbitMQ 客户端
├── config.go              # 配置结构定义
├── connection.go          # 连接管理器
├── consumer.go            # 消息消费者
├── deduplicator.go        # 消息去重器
├── initializer.go         # 初始化器
├── load_monitor.go        # 负载监控器
├── result_reporter.go     # 结果上报器
├── service_manager.go     # 服务管理器
├── service.go             # RabbitMQ 服务
├── task_adapter.go        # 任务消息适配器
├── task_handler.go        # 任务处理器
└── task_submitter.go      # 任务提交器
```

## 核心组件

### 1. ServiceManager（服务管理器）

**职责：**
- 统一管理所有 RabbitMQ 相关服务
- 协调服务启动和停止
- 提供健康检查和指标接口
- 处理优雅关闭

**主要方法：**
```go
// 创建服务管理器
func NewServiceManager(configPath string, logger *logrus.Logger) (*ServiceManager, error)

// 注册任务处理器
func (sm *ServiceManager) RegisterProcessor(platform string, processor worker.Processor) error

// 启动所有服务
func (sm *ServiceManager) Start(ctx context.Context) error

// 停止所有服务
func (sm *ServiceManager) Stop(ctx context.Context) error

// 获取统计信息
func (sm *ServiceManager) GetStats() map[string]interface{}
```

**HTTP 端点：**
- `GET /health` - 健康检查
- `GET /ready` - 就绪检查
- `GET /metrics` - Prometheus 格式指标
- `GET /stats` - 详细统计信息

### 2. Client（RabbitMQ 客户端）

**职责：**
- 封装 RabbitMQ 连接和通道操作
- 提供消息发布和消费接口
- 管理队列和交换机

**主要方法：**
```go
// 创建客户端
func NewClient(config *config.RabbitMQConfig, logger *logrus.Logger) *Client

// 连接到 RabbitMQ
func (c *Client) Connect(ctx context.Context) error

// 发布消息
func (c *Client) Publish(ctx context.Context, opts PublishOptions) error

// 消费消息
func (c *Client) Consume(ctx context.Context, opts ConsumeOptions) (<-chan amqp.Delivery, error)

// 声明队列
func (c *Client) DeclareQueue(ctx context.Context, opts QueueOptions) error
```

### 3. ConnectionManager（连接管理器）

**职责：**
- 管理 RabbitMQ 连接和通道
- 自动重连机制
- 连接池管理

**特性：**
- 自动重连
- 连接健康检查
- 通道复用
- 错误恢复

### 4. MessageConsumer（消息消费者）

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

**使用示例：**
```go
// 创建消费者
consumer := NewMessageConsumer(client, config, logger)

// 设置队列配置
consumer.SetQueueConfigs([]ConsumerQueueConfig{
    {Name: "task.temu", Priority: 10, Prefetch: 5},
    {Name: "task.shein", Priority: 5, Prefetch: 10},
})

// 注册处理器
consumer.RegisterHandler("task.temu", temuHandler)
consumer.RegisterHandler("task.shein", sheinHandler)

// 启动消费
err := consumer.Start(ctx)
```

### 5. TaskSubmitter（任务提交器）

**职责：**
- 提交任务到 RabbitMQ 队列
- 批量提交变体任务
- 任务去重（5分钟缓存）
- 消息格式转换

**主要方法：**
```go
// 提交单个任务
func (ts *TaskSubmitter) SubmitTask(ctx context.Context, task *model.Task) error

// 批量提交变体任务（带去重）
func (ts *TaskSubmitter) SubmitVariantTasks(
    ctx context.Context, 
    parentTask *model.Task, 
    variations []model.Variation, 
    parentAsin string,
) (successCount, failCount int)
```

**去重规则：**
1. 跳过父 ASIN 本身
2. 跳过当前任务的 ProductID
3. 跳过 5 分钟内已提交的任务

**使用示例：**
```go
// 创建提交器
submitter := NewTaskSubmitter(client, logger)

// 提交单个任务
task := &model.Task{
    ID:        123,
    Platform:  "temu",
    ProductID: "abc123",
    // ...
}
err := submitter.SubmitTask(ctx, task)

// 批量提交变体
successCount, failCount := submitter.SubmitVariantTasks(
    ctx, 
    parentTask, 
    variations, 
    parentAsin,
)
```

### 6. ResultReporter（结果上报器）

**职责：**
- 异步上报任务处理结果
- 批量上报优化
- 失败重试机制
- 缓冲队列管理

**配置：**
```yaml
result_reporter:
  report_url: "http://management-api/tasks/results"
  node_id: "node-001"
  timeout: 30s
  buffer_size: 1000
  retry:
    max_retries: 3
    retry_delay: 5s
```

**使用示例：**
```go
// 创建上报器
reporter := NewResultReporter(config, logger)

// 启动上报器
err := reporter.Start(ctx)

// 上报结果
result := &TaskResult{
    TaskID:   123,
    Status:   "completed",
    Data:     productData,
    Error:    "",
}
reporter.ReportResult(result)
```

### 7. LoadMonitor（负载监控器）

**职责：**
- 监控系统资源使用
- 收集任务处理统计
- 提供健康状态检查
- 导出 Prometheus 指标

**监控指标：**
- CPU 使用率
- 内存使用率
- Goroutine 数量
- 任务处理统计（成功/失败/总数）
- 队列消息数量

**使用示例：**
```go
// 创建监控器
monitor := NewLoadMonitor(config, logger)

// 启动监控
err := monitor.Start(ctx)

// 获取统计信息
stats := monitor.GetStats()

// 获取健康状态
health := monitor.GetHealthStatus()
```

### 8. TaskMessageAdapter（任务消息适配器）

**职责：**
- 任务模型与消息格式转换
- 队列路由规则
- 优先级映射

**队列映射：**
```go
platform -> queue
─────────────────
temu    -> task.temu
shein   -> task.shein
amazon  -> task.amazon
```

**优先级映射：**
```go
任务优先级 -> RabbitMQ 优先级
────────────────────────────
1-3      -> 1 (低)
4-6      -> 5 (中)
7-10     -> 10 (高)
```

### 9. Deduplicator（消息去重器）

**职责：**
- 防止重复消息处理
- 基于消息 ID 去重
- 自动清理过期记录

**使用示例：**
```go
// 创建去重器
dedup := NewDeduplicator(cacheDuration)

// 检查是否重复
if dedup.IsDuplicate(messageID) {
    // 跳过重复消息
    return
}

// 标记为已处理
dedup.MarkAsProcessed(messageID)
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
    
    // 1. 创建服务管理器
    sm, err := rabbitmq.NewServiceManager("config/config.yaml", logger)
    if err != nil {
        logger.Fatal(err)
    }
    
    // 2. 注册处理器
    err = sm.RegisterProcessor("temu", temuProcessor)
    if err != nil {
        logger.Fatal(err)
    }
    
    err = sm.RegisterProcessor("shein", sheinProcessor)
    if err != nil {
        logger.Fatal(err)
    }
    
    // 3. 启动服务
    err = sm.Start(ctx)
    if err != nil {
        logger.Fatal(err)
    }
    defer sm.Stop(ctx)
    
    // 4. 等待服务运行
    sm.Wait()
}
```

### 自定义消息处理器

```go
type MyMessageHandler struct {
    logger *logrus.Logger
}

func (h *MyMessageHandler) HandleMessage(ctx context.Context, msg *rabbitmq.Message) error {
    h.logger.Infof("处理消息: ID=%s", msg.ID)
    
    // 解析消息
    taskID := msg.Payload["task_id"].(float64)
    platform := msg.Payload["platform"].(string)
    
    // 处理业务逻辑
    err := processTask(int64(taskID), platform)
    if err != nil {
        return fmt.Errorf("处理任务失败: %w", err)
    }
    
    return nil
}
```

## 配置说明

### 完整配置示例

```yaml
rabbitmq:
  # 连接配置
  host: localhost
  port: 5672
  username: guest
  password: guest
  vhost: /
  
  # 连接池配置
  connection_pool:
    max_connections: 10
    max_channels: 100
    
  # 消费者配置
  consumer:
    prefetch_count: 10
    prefetch_size: 0
    retry_delay: 5s
    max_retries: 3
    
  # 队列配置
  queues:
    - name: task.temu
      priority: 10
      prefetch: 5
    - name: task.shein
      priority: 5
      prefetch: 10
      
  # 结果上报配置
  result_reporter:
    report_url: "http://management-api/tasks/results"
    node_id: "node-001"
    timeout: 30s
    buffer_size: 1000
    retry:
      max_retries: 3
      retry_delay: 5s
      
  # 负载监控配置
  load_monitor:
    update_interval: 10s
    enable_cpu: true
    enable_memory: true
    enable_tasks: true
    
  # 节点配置
  node:
    node_id: "node-001"
    health_check_port: 8080
    metrics_port: 9090
    shutdown_timeout: 30s
```

## 监控和运维

### 健康检查

```bash
# 检查服务健康状态
curl http://localhost:8080/health

# 响应示例
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### 就绪检查

```bash
# 检查服务是否就绪
curl http://localhost:8080/ready

# 响应示例
{
  "ready": true,
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Prometheus 指标

```bash
# 获取 Prometheus 格式指标
curl http://localhost:9090/metrics

# 输出示例
# HELP tasks_processed_total Total number of tasks processed
# TYPE tasks_processed_total counter
tasks_processed_total 1234

# HELP tasks_succeeded_total Total number of tasks succeeded
# TYPE tasks_succeeded_total counter
tasks_succeeded_total 1200

# HELP tasks_failed_total Total number of tasks failed
# TYPE tasks_failed_total counter
tasks_failed_total 34

# HELP cpu_usage_percent Current CPU usage percentage
# TYPE cpu_usage_percent gauge
cpu_usage_percent 45.67

# HELP memory_usage_percent Current memory usage percentage
# TYPE memory_usage_percent gauge
memory_usage_percent 62.34
```

### 详细统计

```bash
# 获取详细统计信息
curl http://localhost:9090/stats

# 响应示例
{
  "timestamp": "2024-01-01T00:00:00Z",
  "stats": {
    "load": {
      "cpu_usage": 45.67,
      "memory_usage": 62.34,
      "goroutine_count": 123,
      "tasks_processed": 1234,
      "tasks_succeeded": 1200,
      "tasks_failed": 34
    },
    "rabbitmq": {
      "connected": true,
      "queues": {
        "task.temu": {"messages": 10},
        "task.shein": {"messages": 5}
      }
    },
    "result_reporter": {
      "pending": 3,
      "reported": 1231
    }
  }
}
```

## 错误处理

### 连接失败

```go
// 自动重连机制
// 连接失败时会自动重试，默认重试间隔 5 秒
```

### 消息处理失败

```go
// 重试机制
// 1. 消息处理失败时，会根据 retry_count 判断是否重试
// 2. 未达到最大重试次数：Nack(requeue=true)
// 3. 达到最大重试次数：Nack(requeue=false) -> 进入死信队列
```

### 死信队列处理

```bash
# 查看死信队列
rabbitmqctl list_queues name messages | grep dlq

# 手动处理死信消息
# 1. 从死信队列消费消息
# 2. 分析失败原因
# 3. 修复问题后重新提交
```

## 性能优化

### 1. 连接池

- 复用连接和通道
- 减少连接开销
- 提高并发性能

### 2. 批量操作

- 批量确认消息
- 批量发布消息
- 减少网络往返

### 3. 预取优化

```yaml
# 高优先级队列：低预取
queues:
  - name: task.temu
    priority: 10
    prefetch: 5      # 确保高优先级任务快速处理

# 低优先级队列：高预取
  - name: task.shein
    priority: 5
    prefetch: 10     # 提高吞吐量
```

### 4. 消息持久化

```go
// 只对重要消息持久化
publishing := amqp.Publishing{
    DeliveryMode: 2,  // 2 = persistent
    // ...
}
```

## 最佳实践

1. **连接管理**
   - 使用连接池
   - 监控连接状态
   - 实现自动重连

2. **消息确认**
   - 使用手动确认
   - 处理成功后再确认
   - 失败时合理重试

3. **错误处理**
   - 区分可重试和不可重试错误
   - 使用死信队列
   - 记录详细日志

4. **监控告警**
   - 监控队列长度
   - 监控消费速率
   - 设置告警阈值

5. **优雅关闭**
   - 停止接收新消息
   - 等待当前消息处理完成
   - 关闭连接和通道

## 注意事项

1. 确保 RabbitMQ 服务可用
2. 合理设置预取数量
3. 注意消息大小限制
4. 定期清理死信队列
5. 监控内存使用
6. 避免消息堆积
