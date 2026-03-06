# Docker 环境搭建指南

本文档说明如何使用 Docker 快速搭建 rabbitmq-consumer 所需的依赖服务。

## 📦 包含的服务

- **RabbitMQ 3.12** (带管理界面)
  - AMQP 端口: 5672
  - 管理界面: 15672
  - 用户名: `admin`
  - 密码: `admin123`

- **Redis 7** (可选，用于分布式锁)
  - 端口: 6379

## 🚀 快速启动

### 方法 1: 使用启动脚本（推荐）

```bash
# 启动所有服务
bash scripts/start-services.sh

# 停止所有服务
bash scripts/stop-services.sh
```

### 方法 2: 使用 docker compose 命令

```bash
# 启动服务（后台运行）
docker compose -f scripts/docker-compose.yml up -d

# 查看服务状态
docker compose -f scripts/docker-compose.yml ps

# 查看日志
docker compose -f scripts/docker-compose.yml logs -f

# 停止服务
docker compose -f scripts/docker-compose.yml down

# 停止服务并删除数据卷
docker compose -f scripts/docker-compose.yml down -v
```

## 🔍 验证服务

### 1. 检查容器状态

```bash
docker compose -f scripts/docker-compose.yml ps
```

应该看到两个容器都是 `Up` 状态。

### 2. 访问 RabbitMQ 管理界面

打开浏览器访问: http://localhost:15672

- 用户名: `admin`
- 密码: `admin123`

### 3. 测试 RabbitMQ 连接

```bash
# 使用 telnet 测试端口
telnet localhost 5672

# 或使用 nc
nc -zv localhost 5672
```

### 4. 测试 Redis 连接

```bash
# 使用 redis-cli（如果已安装）
redis-cli ping

# 或使用 docker exec
docker exec -it task-processor-redis redis-cli ping
```

## 📊 查看日志

```bash
# 查看所有服务日志
docker compose -f scripts/docker-compose.yml logs -f

# 只查看 RabbitMQ 日志
docker compose -f scripts/docker-compose.yml logs -f rabbitmq

# 只查看 Redis 日志
docker compose -f scripts/docker-compose.yml logs -f redis
```

## 🔧 配置说明

### RabbitMQ 配置

配置文件位置: `config/rabbitmq-config.yaml`

确保连接 URL 与 Docker 配置一致：

```yaml
rabbitmq:
  url: "amqp://admin:admin123@localhost:5672/"
```

### 数据持久化

数据会保存在 Docker 卷中：
- `rabbitmq_data`: RabbitMQ 数据
- `rabbitmq_logs`: RabbitMQ 日志
- `redis_data`: Redis 数据

## 🛠️ 常见问题

### 端口被占用

如果端口已被占用，可以修改 `scripts/docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "15672:15672"  # 改为 "15673:15672"
  - "5672:5672"    # 改为 "5673:5672"
```

### 重置所有数据

```bash
# 停止服务并删除所有数据
docker compose -f scripts/docker-compose.yml down -v

# 重新启动
docker compose -f scripts/docker-compose.yml up -d
```

### 查看容器内部

```bash
# 进入 RabbitMQ 容器
docker exec -it task-processor-rabbitmq bash

# 进入 Redis 容器
docker exec -it task-processor-redis sh
```

## 🎯 下一步

服务启动后，你可以：

1. 启动 rabbitmq-consumer 程序：
   ```bash
   go run cmd/rabbitmq-consumer/main.go
   ```

2. 在 RabbitMQ 管理界面中查看队列和消息

3. 测试发送任务消息到队列

## 📝 注意事项

- 首次启动可能需要下载镜像，请耐心等待
- 确保 Docker Desktop 正在运行
- 建议至少分配 2GB 内存给 Docker
- 生产环境请修改默认密码
