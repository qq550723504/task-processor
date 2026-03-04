# RabbitMQ 消费者系统技术文档

## 1. 系统概述

RabbitMQ 消费者系统是一个分布式爬虫任务处理平台，支持 Amazon、TEMU、SHEIN 三个电商平台的产品数据采集和处理。系统采用消息队列架构，实现任务的异步处理、负载均衡和高可用性。

### 1.1 核心特性

- **多平台支持**：Amazon、TEMU、SHEIN 三大电商平台
- **异步处理**：基于 RabbitMQ 的消息队列异步任务处理
- **并发控制**：Worker Pool 模式，支持可配置的并发数
- **容错机制**：自动重试、死信队列、优雅降级
- **监控告警**：健康检查、指标监控、负载统计
- **动态配置**：支持配置热更新，无需重启服务

### 1.2 技术栈

- **语言**：Go 1.x
- **消息队列**：RabbitMQ (amqp091-go)
- **日志**：Logrus
- **配置**：YAML
- **监控**：HTTP 健康检查 + Prometheus 指标

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    RabbitMQ Consumer                         │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │              ServiceManager (主协调器)              │    │
│  │  - 生命周期管理                                     │    │
│  │  - 组件协调                                         │    │
│  │  - 信号处理                                         │    │
│  └────────────────────────────────────────────────────┘    │
│           │                │                │               │
│           ▼                ▼                ▼               │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  RabbitMQ   │  │ ResultReporter│  │ LoadMonitor  │      │
│  │   Service   │  │  (结果上报)   │  │  (负载监控)  │      │
│  └─────────────┘  └──────────────┘  └──────────────┘      │
│           │                                                  │
│           ▼                                                  │
│  ┌─────────────────────────────────────────────────┐       │
│  │         MessageConsumer (消息消费者)             │       │
│  │  - 多队列并发消费                                │       │
│  │  - QoS 控制                                      │       │
│  └─────────────────────────────────────────────────┘       │
│           │                                                  │
│           ▼                                                  │
│  ┌─────────────────────────────────────────────────┐       │
│  │      EnhancedTaskHandler (任务处理器)            │       │
│  │  - 消息路由                                      │       │
│  │  - 结果上报集成                                  │       │
│  └─────────────────────────────────────────────────┘       │
│           │                                                  │
│           ▼                                                  │
│  ┌──────────────────────────────────────────────────┐      │
│  │         Platform Processors (平台处理器)          │      │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐       │      │
│  │  │  Amazon  │  │   TEMU   │  │  SHEIN   │       │      │
│  │  │Processor │  │Processor │  │Processor │       │      │
│  │  └──────────┘  └──────────┘  └──────────┘       │      │
│  └──────────────────────────────────────────────────┘      │
│           │                                                  │
│           ▼                                                  │
│  ┌─────────────────────────────────────────────────┐       │
│  │          Worker Pool (工作池)                    │       │
│  │  - 并发任务处理                                  │       │
│  │  - 超时控制                                      │       │
│  └─────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
   ┌──────────┐                        ┌──────────┐
   │ RabbitMQ │                        │ 管理系统  │
   │  Server  │                        │   API    │
   └──────────┘                        └──────────┘
```

### 2.2 核心组件说明

#### ServiceManager (服务管理器)
- **职责**：统一管理所有服务组件的生命周期
- **功能**：
  - 初始化 RabbitMQ 服务、结果上报器、负载监控器
  - 启动 HTTP 健康检查服务器 (8081)
  - 启动 HTTP 指标监控服务器 (8082)
  - 处理系统信号，实现优雅关闭
  - 协调各组件的启动和停止顺序

#### RabbitMQ Service (消息队列服务)
- **职责**：管理 RabbitMQ 连接、消费者、队列初始化
- **功能**：
  - 连接管理：自动重连、心跳检测
  - 队列初始化：声明队列、交换机、绑定关系
  - 消费者管理：注册消费者、QoS 控制
  - 处理器注册：维护平台处理器映射

#### MessageConsumer (消息消费者)
- **职责**：从 RabbitMQ 队列消费消息
- **功能**：
  - 多队列并发消费
  - Prefetch 控制 (默认 1)
  - 消息确认/拒绝机制
  - 自动重试和死信队列

#### EnhancedTaskHandler (增强任务处理器)
- **职责**：处理消息并集成结果上报
- **功能**：
  - 消息转换为任务对象
  - 平台验证和路由
  - 任务处理异常捕获
  - 自动上报处理结果

#### Platform Processors (平台处理器)
- **Amazon Processor**：处理 Amazon 平台产品数据
- **TEMU Processor**：处理 TEMU 平台产品数据
- **SHEIN Processor**：处理 SHEIN 平台产品数据
- **共同特性**：
  - 继承 BaseProcessor 基础功能
  - 使用 Worker Pool 并发处理
  - 集成管理系统客户端
  - 内存管理和资源优化

#### Worker Pool (工作池)
- **职责**：并发执行任务
- **功能**：
  - 可配置的并发数 (Concurrency)
  - 任务队列缓冲 (BufferSize)
  - 超时控制
  - 优雅关闭

#### ResultReporter (结果上报器)
- **职责**：异步上报任务处理结果
- **功能**：
  - 异步上报队列 (Buffer: 1000)
  - 重试机制 (最多 3 次)
  - 指数退避策略
  - 统计信息收集

#### LoadMonitor (负载监控器)
- **职责**：监控系统负载和性能
- **功能**：
  - CPU 使用率监控
  - 内存使用率监控
  - Goroutine 数量统计
  - 任务处理统计
  - 队列性能指标

---

## 3. 业务处理流程

### 3.1 系统启动流程

```
1. main() 入口
   ├─ 解析命令行参数
   │  ├─ config: RabbitMQ 配置文件路径
   │  ├─ app-config: 应用配置文件路径
   │  ├─ log-level: 日志级别
   │  └─ platforms: 启用的平台列表
   │
   ├─ 设置日志器 (setupLogger)
   │
   ├─ 加载应用配置 (loadAppConfig)
   │  └─ 使用 config.LoadConfigFromFile()
   │
   ├─ 创建服务管理器 (NewServiceManager)
   │  ├─ 创建配置管理器
   │  ├─ 加载 RabbitMQ 配置
   │  └─ 初始化 RabbitMQ 服务
   │
   ├─ 注册任务处理器 (registerProcessors)
   │  ├─ 创建管理客户端
   │  ├─ 注册 Amazon 处理器 (可选)
   │  ├─ 注册 TEMU 处理器 (可选)
   │  └─ 注册 SHEIN 处理器 (可选)
   │
   ├─ 启动服务 (serviceManager.Start)
   │  ├─ 启动 RabbitMQ 服务
   │  ├─ 启动结果上报器
   │  ├─ 启动负载监控器
   │  ├─ 启动健康检查服务器 (8081)
   │  └─ 启动指标监控服务器 (8082)
   │
   └─ 等待退出信号 (serviceManager.Wait)
      └─ 优雅关闭所有服务
```

### 3.2 任务处理流程

```
1. RabbitMQ 消息到达
   │
   ├─ MessageConsumer 接收消息
   │  └─ 从队列获取 amqp.Delivery
   │
   ├─ QueueConsumer 处理消息
   │  ├─ 解析消息体 (JSON)
   │  └─ 调用 MessageHandler
   │
   ├─ EnhancedTaskHandler.HandleMessage()
   │  ├─ 消息转换为任务对象
   │  │  └─ TaskMessageAdapter.MessageToTask()
   │  │
   │  ├─ 验证平台匹配
   │  │
   │  ├─ 调用平台处理器
   │  │  └─ processor.ProcessTask(ctx, task)
   │  │
   │  ├─ 处理成功
   │  │  ├─ 上报成功结果
   │  │  └─ ACK 消息
   │  │
   │  └─ 处理失败
   │     ├─ 上报失败结果
   │     ├─ 判断是否重试
   │     │  ├─ 未达到最大重试次数 → NACK (requeue=true)
   │     │  └─ 达到最大重试次数 → NACK (requeue=false, 进入死信队列)
   │     └─ 记录错误日志
   │
   └─ 更新监控指标
```

### 3.3 平台处理器处理流程

以 TEMU 为例：

```
1. TemuProcessor.ProcessTask()
   │
   ├─ 任务验证
   │  ├─ 检查 ProductID
   │  ├─ 检查 StoreID
   │  └─ 检查必要参数
   │
   ├─ 调用 TaskHandler
   │  └─ taskHandler.Handle(ctx, task)
   │
   ├─ 执行管道处理
   │  └─ pipelineExecutor.Execute()
   │     │
   │     ├─ 1. 产品数据获取
   │     │  └─ 调用 TEMU API 获取产品信息
   │     │
   │     ├─ 2. 数据清洗和验证
   │     │  ├─ 敏感词过滤
   │     │  ├─ 违禁品检查
   │     │  └─ 数据格式验证
   │     │
   │     ├─ 3. Amazon 匹配 (可选)
   │     │  └─ 使用共享 AmazonProcessor 查找匹配产品
   │     │
   │     ├─ 4. 数据转换
   │     │  └─ 转换为统一的产品模型
   │     │
   │     └─ 5. 数据上传
   │        └─ 调用管理系统 API 保存产品数据
   │
   ├─ 更新任务状态
   │  └─ managementClient.UpdateTaskStatus()
   │
   └─ 返回处理结果
```

### 3.4 Worker Pool 工作机制

```
1. Worker Pool 初始化
   ├─ 创建任务队列 (buffered channel)
   ├─ 创建指定数量的 Worker
   └─ 启动所有 Worker goroutines
   
2. 任务提交
   ├─ 调用 pool.Submit(job)
   ├─ 将任务放入队列
   └─ 等待 Worker 处理
   
3. Worker 处理循环
   ├─ 从队列获取任务
   ├─ 调用 processor.ProcessTask()
   ├─ 处理完成通知
   └─ 继续等待下一个任务
   
4. 优雅关闭
   ├─ 关闭任务队列
   ├─ 等待所有任务完成
   └─ 退出所有 Worker
```

---

## 4. 数据模型

### 4.1 RabbitMQ 消息结构

```go
type Message struct {
    ID         string                 // 消息唯一标识
    Type       string                 // 消息类型 (固定为 "task")
    Payload    map[string]interface{} // 消息载荷
    Priority   uint8                  // RabbitMQ 优先级 (0-10)
    Timestamp  int64                  // 时间戳
    RetryCount int                    // 当前重试次数
    MaxRetries int                    // 最大重试次数
}
```

### 4.2 任务消息载荷

```go
type TaskMessage struct {
    TaskID        int64  // 任务 ID
    TenantID      int64  // 租户 ID
    StoreID       int64  // 店铺 ID
    Platform      string // 平台 (amazon/temu/shein)
    Region        string // 地区
    CategoryID    int64  // 分类 ID
    ProductID     string // 产品 ID
    Priority      int    // 业务优先级 (1-10)
    RetryCount    int    // 重试次数
    MaxRetryCount int    // 最大重试次数
    CreatedAt     int64  // 创建时间
}
```

### 4.3 任务对象

```go
type Task struct {
    ID            int64  // 任务 ID
    TenantID      int64  // 租户 ID
    StoreID       int64  // 店铺 ID
    Platform      string // 平台
    Region        string // 地区
    CategoryID    int64  // 分类 ID
    ProductID     string // 产品 ID
    Status        int    // 状态 (1:处理中)
    RetryCount    int    // 重试次数
    MaxRetryCount int    // 最大重试次数
    Priority      int    // 优先级
    CreateTime    int64  // 创建时间
    UpdateTime    int64  // 更新时间
}
```

### 4.4 任务结果

```go
type TaskResult struct {
    TaskID       int64                  // 任务 ID
    Status       string                 // 状态 (success/failed/retry)
    Message      string                 // 结果消息
    Data         map[string]interface{} // 结果数据
    ProcessTime  int64                  // 处理时间 (毫秒)
    ErrorCode    string                 // 错误代码
    ErrorMessage string                 // 错误消息
    RetryCount   int                    // 重试次数
    NodeID       string                 // 节点 ID
    Timestamp    int64                  // 时间戳
}
```

---

## 5. 配置说明

### 5.1 RabbitMQ 配置 (rabbitmq-config.yaml)

```yaml
# RabbitMQ 连接配置
rabbitmq:
  url: "amqp://admin:admin123@localhost:5672/"
  reconnect_interval: 5s        # 重连间隔
  max_reconnect_tries: 10       # 最大重连次数
  consumer:
    prefetch_count: 1           # 预取数量
    prefetch_size: 0            # 预取大小
    retry_delay: 5s             # 重试延迟
    max_retries: 3              # 最大重试次数

# 结果上报器配置
result_reporter:
  report_url: "http://localhost:8080/api/task/result"
  node_id: ""                   # 节点 ID (自动生成)
  timeout: 30s                  # 超时时间
  buffer_size: 1000             # 缓冲区大小
  retry:
    max_retries: 3              # 最大重试次数
    initial_delay: 2s           # 初始延迟
    max_delay: 30s              # 最大延迟
    backoff_factor: 2.0         # 退避因子
    timeout: 10s                # 单次请求超时

# 负载监控配置
load_monitor:
  update_interval: 30s          # 更新间隔
  enable_cpu: true              # 启用 CPU 监控
  enable_memory: true           # 启用内存监控
  enable_tasks: true            # 启用任务监控

# 节点配置
node:
  node_id: ""                   # 节点 ID (自动生成)
  max_concurrency: 5            # 最大并发数
  health_check_port: 8081       # 健康检查端口
  metrics_port: 8082            # 指标监控端口
  log_level: "info"             # 日志级别
  shutdown_timeout: 30s         # 关闭超时
```

### 5.2 应用配置 (config-dev.yaml)

```yaml
# Worker 配置
worker:
  concurrency: 5                # 并发数
  buffer_size: 100              # 缓冲区大小
  task_interval: 30             # 任务间隔 (秒)

# 管理系统配置
management:
  base_url: "http://localhost:8080"
  timeout: 30s

# Amazon 配置
amazon:
  data_freshness_days: 7        # 数据新鲜度 (天)

# 浏览器配置
browser:
  pool_size: 1                  # 浏览器池大小
  headless: true                # 无头模式
```

---

## 6. 队列设计

### 6.1 队列命名规范

```
平台队列：{platform}.tasks.queue
- amazon.tasks.queue
- temu.tasks.queue
- shein.tasks.queue

死信队列：{platform}.tasks.dlq
- amazon.tasks.dlq
- temu.tasks.dlq
- shein.tasks.dlq
```

### 6.2 交换机设计

```
主交换机：tasks.exchange (topic)
- 路由键格式：{platform}.{priority}
- 示例：amazon.urgent, temu.high, shein.normal

死信交换机：tasks.dlx (direct)
- 处理失败的消息

延迟交换机：tasks.delay.exchange (x-delayed-message)
- 处理延迟任务
```

### 6.3 路由规则

```
优先级映射：
- 业务优先级 1-3  → urgent  (紧急)
- 业务优先级 4-6  → high    (高)
- 业务优先级 7-8  → normal  (普通)
- 业务优先级 9-10 → low     (低)

路由键示例：
- amazon.urgent  → amazon.tasks.queue
- temu.high      → temu.tasks.queue
- shein.normal   → shein.tasks.queue
```

---

## 7. 监控和运维

### 7.1 健康检查端点

```
HTTP 服务器：http://localhost:8081

端点：
- GET /health  - 健康检查
  返回：{"status": "healthy", "timestamp": 1234567890}
  
- GET /ready   - 就绪检查
  返回：{"ready": true, "services": {...}}
```

### 7.2 指标监控端点

```
HTTP 服务器：http://localhost:8082

端点：
- GET /metrics - Prometheus 格式指标
  
- GET /stats   - 统计信息
  返回：{
    "cpu_usage": 45.2,
    "memory_usage": 512.5,
    "goroutine_count": 120,
    "tasks_processed": 1000,
    "tasks_succeeded": 950,
    "tasks_failed": 50
  }
```

### 7.3 日志级别

```
支持的日志级别：
- debug: 调试信息
- info:  一般信息 (默认)
- warn:  警告信息
- error: 错误信息
- fatal: 致命错误

设置方式：
--log-level=debug
```

### 7.4 优雅关闭

```
关闭流程：
1. 接收 SIGINT/SIGTERM 信号
2. 停止接收新任务
3. 等待正在处理的任务完成 (最多 30 秒)
4. 关闭 RabbitMQ 连接
5. 关闭 HTTP 服务器
6. 退出程序

超时处理：
- 如果 30 秒内任务未完成，强制退出
- 未完成的任务会重新入队
```

---

## 8. 容错机制

### 8.1 重试策略

```
重试条件：
- 网络错误
- 临时性错误
- 超时错误

重试配置：
- 最大重试次数：3 次
- 重试延迟：5 秒
- 重试方式：NACK + requeue

不重试条件：
- 参数错误
- 业务逻辑错误
- 达到最大重试次数
```

### 8.2 死信队列

```
触发条件：
- 消息被拒绝且 requeue=false
- 消息 TTL 过期
- 队列长度超限

处理方式：
- 消息进入死信队列
- 记录详细错误信息
- 人工介入处理
```

### 8.3 连接重连

```
重连策略：
- 自动检测连接断开
- 指数退避重连
- 最大重连次数：10 次
- 重连间隔：5 秒

重连流程：
1. 检测到连接断开
2. 等待重连间隔
3. 尝试重新连接
4. 重新声明队列和交换机
5. 恢复消费者
```

### 8.4 异常处理

```
Panic 恢复：
- 捕获 goroutine panic
- 记录堆栈信息
- 上报失败结果
- 继续处理下一个任务

超时控制：
- 任务处理超时
- HTTP 请求超时
- 数据库操作超时
```

---

## 9. 性能优化

### 9.1 并发控制

```
Worker Pool：
- 可配置的并发数
- 任务队列缓冲
- 避免 goroutine 泄漏

资源限制：
- 浏览器池大小限制
- HTTP 连接池复用
- 内存使用监控
```

### 9.2 消息预取

```
Prefetch 配置：
- prefetch_count: 1
- 避免消息堆积
- 保证负载均衡
```

### 9.3 批量处理

```
结果上报：
- 异步上报队列
- 批量发送 (可选)
- 减少网络开销
```

---

## 10. 部署和运行

### 10.1 启动命令

```bash
# 基本启动
./rabbitmq-consumer

# 指定配置文件
./rabbitmq-consumer \
  --config=config/rabbitmq-config.yaml \
  --app-config=config/config-dev.yaml

# 指定日志级别
./rabbitmq-consumer --log-level=debug

# 指定启用的平台
./rabbitmq-consumer --platforms=amazon,temu

# 完整示例
./rabbitmq-consumer \
  --config=config/rabbitmq-config.yaml \
  --app-config=config/config-dev.yaml \
  --log-level=info \
  --platforms=amazon,temu,shein
```

### 10.2 Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o rabbitmq-consumer cmd/rabbitmq-consumer/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/rabbitmq-consumer .
COPY config/ ./config/
CMD ["./rabbitmq-consumer"]
```

### 10.3 环境变量

```bash
# RabbitMQ 连接
RABBITMQ_URL=amqp://admin:admin123@localhost:5672/

# 管理系统 API
MANAGEMENT_BASE_URL=http://localhost:8080

# 日志级别
LOG_LEVEL=info

# 节点 ID
NODE_ID=node-001
```

---

## 11. 故障排查

### 11.1 常见问题

**问题 1：连接 RabbitMQ 失败**
```
原因：
- RabbitMQ 服务未启动
- 连接配置错误
- 网络不通

解决：
- 检查 RabbitMQ 服务状态
- 验证连接字符串
- 测试网络连通性
```

**问题 2：任务处理失败**
```
原因：
- 业务逻辑错误
- 外部 API 不可用
- 数据格式错误

解决：
- 查看错误日志
- 检查外部依赖
- 验证数据格式
```

**问题 3：内存占用过高**
```
原因：
- 并发数过大
- 浏览器实例未释放
- goroutine 泄漏

解决：
- 降低并发数
- 检查资源释放
- 使用 pprof 分析
```

### 11.2 日志分析

```
关键日志：
- "启动RabbitMQ消费者" - 服务启动
- "注册平台处理器" - 处理器注册
- "开始处理任务" - 任务开始
- "任务处理成功/失败" - 任务结果
- "优雅关闭" - 服务关闭
```

---

## 12. 最佳实践

### 12.1 配置建议

```
生产环境：
- concurrency: 10-20 (根据机器性能)
- buffer_size: 100-500
- prefetch_count: 1
- max_retries: 3
- log_level: info

开发环境：
- concurrency: 2-5
- buffer_size: 10-50
- log_level: debug
```

### 12.2 监控建议

```
关键指标：
- 任务处理速率
- 任务成功率
- 平均处理时间
- 队列长度
- CPU/内存使用率
- Goroutine 数量

告警阈值：
- 任务失败率 > 10%
- 平均处理时间 > 60s
- 队列长度 > 1000
- CPU 使用率 > 80%
- 内存使用率 > 80%
```

### 12.3 安全建议

```
- 使用强密码连接 RabbitMQ
- 启用 TLS 加密传输
- 限制网络访问
- 定期更新依赖
- 敏感信息使用环境变量
```

---

## 13. 技术债务和改进方向

### 13.1 当前限制

- 不支持动态调整并发数
- 缺少分布式追踪
- 监控指标不够完善
- 缺少任务优先级动态调整

### 13.2 改进计划

- 集成 OpenTelemetry 分布式追踪
- 增加更多 Prometheus 指标
- 支持配置热更新
- 实现任务优先级队列
- 增加任务去重机制
- 支持任务依赖关系

---

## 14. 附录

### 14.1 相关文档

- [RabbitMQ 官方文档](https://www.rabbitmq.com/documentation.html)
- [amqp091-go 文档](https://pkg.go.dev/github.com/rabbitmq/amqp091-go)
- [Logrus 文档](https://github.com/sirupsen/logrus)

### 14.2 联系方式

- 技术支持：tech-support@example.com
- 问题反馈：issues@example.com

---

**文档版本**：v1.0  
**最后更新**：2024-03-04  
**维护者**：开发团队
