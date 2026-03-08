# RabbitMQ 队列架构文档

## 概述

本文档描述了 task-processor 系统中 RabbitMQ 队列的架构设计，包括上架任务队列和爬虫任务队列。

## 系统架构

### 服务分类

#### 1. 爬虫服务（独立服务）
- **Amazon 爬虫** - 爬取 Amazon 产品数据
- **1688 爬虫** - 爬取 1688 产品数据

#### 2. 上架服务（task-processor）
- **Amazon 平台** - 上架到 Amazon
- **TEMU 平台** - 上架到 TEMU（数据来自 Amazon 爬虫）
- **SHEIN 平台** - 上架到 SHEIN（数据来自 Amazon 爬虫）

## 队列配置

### 1. 上架任务队列

用于接收和处理平台上架任务。

#### Amazon 上架队列
```
amazon.tasks.high    - 高优先级任务（路由键: amazon.high.#）
amazon.tasks.normal  - 普通优先级任务（路由键: amazon.normal.#）
amazon.tasks.low     - 低优先级任务（路由键: amazon.low.#）
```

#### TEMU 上架队列
```
temu.tasks.high      - 高优先级任务（路由键: temu.high.#）
temu.tasks.normal    - 普通优先级任务（路由键: temu.normal.#）
temu.tasks.low       - 低优先级任务（路由键: temu.low.#）
```

#### SHEIN 上架队列
```
shein.tasks.high     - 高优先级任务（路由键: shein.high.#）
shein.tasks.normal   - 普通优先级任务（路由键: shein.normal.#）
shein.tasks.low      - 低优先级任务（路由键: shein.low.#）
```

### 2. 爬虫任务队列

用于接收和处理爬虫请求。

```
amazon.crawler.queue - Amazon 爬虫任务队列
1688.crawler.queue   - 1688 爬虫任务队列
```

### 3. 系统队列

```
tasks.dlq            - 死信队列（失败任务）
tasks.delay.queue    - 延迟队列（重试任务）
tasks.result.queue   - 结果队列
```

## 队列名称映射

在 `MessageAdapter` 中定义的平台到队列的映射关系：

```go
queueMapping: map[string]string{
    // 上架任务队列
    "amazon": "amazon.tasks.queue",
    "temu":   "temu.tasks.queue",
    "shein":  "shein.tasks.queue",
    
    // 爬虫任务队列
    "amazon.crawler": "amazon.crawler.queue",
    "1688.crawler":   "1688.crawler.queue",
}
```

## 路由键格式

### 格式规则
```
{platform}.{priority_level}
```

### 优先级映射
```
业务优先级 1-3  -> "urgent"
业务优先级 4-6  -> "high"
业务优先级 7-8  -> "normal"
业务优先级 9-10 -> "low"
```

### 示例
```
amazon.urgent   - Amazon 紧急任务
temu.high       - TEMU 高优先级任务
shein.normal    - SHEIN 普通优先级任务
```

## 工作流程

### TEMU/SHEIN 上架流程

1. **接收上架任务**
   - 任务发送到 `temu.tasks.queue` 或 `shein.tasks.queue`
   - 根据优先级路由到对应的优先级队列

2. **请求产品数据**
   - 通过 `DistributedCrawlerClient` 发送爬虫请求
   - 请求发送到 `amazon.crawler.queue`

3. **爬虫处理**
   - Amazon 爬虫服务监听 `amazon.crawler.queue`
   - 爬取产品数据并返回结果

4. **继续上架**
   - 接收爬虫结果
   - 完成产品上架流程

## 交换机配置

```
tasks.exchange        - 主任务交换机（topic 类型）
tasks.dlx             - 死信交换机（direct 类型）
tasks.delay.exchange  - 延迟交换机（direct 类型）
tasks.result.exchange - 结果交换机（direct 类型）
```

## 队列特性

### 优先级支持
所有任务队列和爬虫队列都支持优先级（0-10）：
```
x-max-priority: 10
```

### 死信处理
失败的任务会自动路由到死信队列：
```
x-dead-letter-exchange: "tasks.dlx"
x-dead-letter-routing-key: "failed"
```

### TTL 配置
死信队列中的消息保留 24 小时：
```
x-message-ttl: 86400000  # 24小时（毫秒）
```

## 使用示例

### 发送上架任务
```go
// 发送 TEMU 上架任务
adapter := task.NewMessageAdapter()
queueName := adapter.GetQueueName("temu")  // 返回 "temu.tasks.queue"
```

### 发送爬虫请求
```go
// 发送 Amazon 爬虫请求
adapter := task.NewMessageAdapter()
queueName := adapter.GetQueueName("amazon.crawler")  // 返回 "amazon.crawler.queue"
```

## 注意事项

1. **队列名称区分**
   - 上架队列：`{platform}.tasks.queue`
   - 爬虫队列：`{platform}.crawler.queue`

2. **优先级队列**
   - 实际物理队列按优先级分为 high/normal/low
   - 通过路由键自动路由到对应队列

3. **爬虫服务独立**
   - 爬虫服务是独立的服务，不在 task-processor 中
   - 通过 RabbitMQ 进行通信

4. **结果返回**
   - 爬虫结果通过临时队列返回
   - 每个客户端创建唯一的结果队列

## 相关文件

- `internal/domain/task/message_adapter.go` - 队列名称映射
- `internal/app/messaging/queue_initializer.go` - 队列初始化配置
- `internal/application/crawler/distributed_crawler_client.go` - 分布式爬虫客户端
