# RabbitMQ Consumer 测试指南

本文档说明如何测试 rabbitmq-consumer 程序。

## 📋 前置条件

1. Docker 容器已启动（RabbitMQ 和 Redis）
2. rabbitmq-consumer 程序已编译并运行

## 🚀 快速测试

### 1. 启动服务

```bash
# 启动 Docker 容器
bash scripts/start-services.sh

# 编译程序
go build -o bin/rabbitmq-consumer.exe ./cmd/rabbitmq-consumer

# 启动消费者（在 Kiro 中已启动）
./bin/rabbitmq-consumer.exe --log-level debug
```

### 2. 发送测试消息

#### 方法 1: 使用 Python 脚本（推荐）

```bash
# 安装依赖
pip install pika

# 发送 Amazon 测试消息
python scripts/send-test-message.py amazon normal

# 发送 TEMU 测试消息
python scripts/send-test-message.py temu high

# 发送 SHEIN 测试消息
python scripts/send-test-message.py shein urgent
```

#### 方法 2: 使用 RabbitMQ 管理界面

1. 打开浏览器访问: http://localhost:15672
2. 登录（用户名: admin, 密码: admin123）
3. 进入 "Exchanges" 标签
4. 点击 "tasks.exchange"
5. 在 "Publish message" 部分:
   - Routing key: `amazon.normal` (或 `temu.high`, `shein.urgent`)
   - Payload: 
   ```json
   {
     "task_id": "test-123",
     "platform": "amazon",
     "type": "listing",
     "priority": "normal",
     "data": {
       "source_url": "https://www.amazon.com/dp/B08N5WRWNW",
       "target_store_id": 169,
       "test_mode": true
     },
     "created_at": "2026-03-06T07:00:00Z"
   }
   ```
6. 点击 "Publish message"

### 3. 查看处理结果

#### 查看消费者日志

在 Kiro 中查看 rabbitmq-consumer 进程的输出，应该能看到：

```
INFO[...] 收到消息: 队列=amazon.tasks.queue
INFO[...] 开始处理任务: task_id=test-123, platform=amazon
INFO[...] 任务处理成功: task_id=test-123
```

#### 查看 RabbitMQ 队列状态

1. 访问 http://localhost:15672/#/queues
2. 查看各个队列的消息数量
3. 点击队列名称查看详细信息

#### 查看健康检查

```bash
# 健康检查
curl http://localhost:8081/health

# 就绪检查
curl http://localhost:8081/ready

# 统计信息
curl http://localhost:8082/stats | jq '.'

# 指标监控
curl http://localhost:8082/metrics
```

## 🔍 测试场景

### 场景 1: 正常消息处理

发送一条正常消息，验证消费者能够接收并处理。

```bash
python scripts/send-test-message.py amazon normal test-normal-001
```

预期结果：
- 消息被成功消费
- 日志显示处理成功
- 队列中消息数量减少

### 场景 2: 多平台消息

同时发送多个平台的消息，验证路由正确。

```bash
python scripts/send-test-message.py amazon normal test-amazon-001
python scripts/send-test-message.py temu high test-temu-001
python scripts/send-test-message.py shein urgent test-shein-001
```

预期结果：
- 每个消息路由到对应的队列
- 对应的处理器被调用

### 场景 3: 优先级测试

发送不同优先级的消息。

```bash
python scripts/send-test-message.py amazon urgent test-urgent-001
python scripts/send-test-message.py amazon high test-high-001
python scripts/send-test-message.py amazon normal test-normal-001
python scripts/send-test-message.py amazon low test-low-001
```

### 场景 4: 并发测试

快速发送多条消息，测试并发处理能力。

```bash
for i in {1..10}; do
  python scripts/send-test-message.py amazon normal test-concurrent-$i &
done
wait
```

### 场景 5: 错误处理

发送格式错误的消息，验证错误处理机制。

在 RabbitMQ 管理界面手动发送：
```json
{
  "invalid": "message"
}
```

预期结果：
- 消息被拒绝
- 进入死信队列（tasks.dlq）
- 日志记录错误信息

## 📊 监控指标

### 队列监控

在 RabbitMQ 管理界面查看：
- 消息数量（Ready, Unacked, Total）
- 消费速率（Incoming, Deliver）
- 消费者数量

### 应用监控

```bash
# 查看统计信息
curl http://localhost:8082/stats | jq '.'
```

关键指标：
- `load.goroutines`: 协程数量
- `load.cpu_percent`: CPU 使用率
- `load.memory_mb`: 内存使用（MB）
- `rabbitmq.connected`: RabbitMQ 连接状态
- `rabbitmq.consumer.queue_stats`: 队列统计

## 🐛 故障排查

### 消息未被消费

1. 检查消费者是否运行
   ```bash
   curl http://localhost:8081/health
   ```

2. 检查 RabbitMQ 连接
   ```bash
   docker logs task-processor-rabbitmq
   ```

3. 检查队列绑定
   - 访问 http://localhost:15672/#/queues
   - 确认队列已绑定到交换机

### 消息处理失败

1. 查看消费者日志（在 Kiro 进程输出中）
2. 检查死信队列（tasks.dlq）
3. 查看错误详情

### 性能问题

1. 检查系统资源
   ```bash
   curl http://localhost:8082/stats | jq '.stats.load'
   ```

2. 调整并发配置
   - 修改 `config/rabbitmq-config.yaml` 中的 `max_concurrency`
   - 修改 `consumer.prefetch_count`

3. 查看队列积压
   - 访问 RabbitMQ 管理界面
   - 检查 Ready 消息数量

## 💡 提示

- 测试模式下，任务不会真正执行爬取和上架操作
- 可以通过 `test_mode: true` 标记测试消息
- 建议先用少量消息测试，确认正常后再进行压力测试
- 定期检查死信队列，分析失败原因

## 📝 下一步

测试通过后，你可以：

1. 集成到实际的任务调度系统
2. 配置生产环境的 RabbitMQ 集群
3. 添加更多的监控和告警
4. 优化性能参数
