# Amazon 平台部署指南

## 部署前准备

### 1. 环境要求

- Go 1.19+
- 已配置的管理系统 API
- Amazon Seller Central 账号
- Amazon SP-API 开发者权限

### 2. 获取 Amazon SP-API 凭证

#### 步骤 1: 注册开发者账号

1. 登录 [Amazon Seller Central](https://sellercentral.amazon.com)
2. 进入 "设置" → "用户权限"
3. 点击 "开发者中心"

#### 步骤 2: 创建应用

1. 点击 "添加新的应用客户端"
2. 填写应用信息：
   - 应用名称
   - OAuth 重定向 URI
3. 保存并获取：
   - Client ID (LWA Client Identifier)
   - Client Secret (LWA Client Secret)

#### 步骤 3: 授权应用

1. 使用授权 URL 进行授权
2. 获取 Refresh Token

示例授权 URL：
```
https://sellercentral.amazon.com/apps/authorize/consent?
application_id=YOUR_CLIENT_ID&
state=YOUR_STATE&
version=beta
```

## 配置部署

### 1. 配置文件

创建 `config/config-prod.yaml`：

```yaml
# 处理器配置
processor:
  maxRetries: 3
  timeout: 600

# Worker配置
worker:
  concurrency: 5
  bufferSize: 20
  taskInterval: 30
  maxFetchPerCycle: 10

# 管理API配置
management:
  baseURL: "https://api.yourdomain.com"
  clientID: "go-listing"
  clientSecret: "your-secret"
  tokenURL: "https://api.yourdomain.com/oauth2/token"
  tenantID: "1"
  storeIDs: [556, 557, 558]

# Amazon配置
amazon:
  # 爬虫配置
  enabled: true
  headless: true
  browserPath: "/usr/bin/chromium"
  poolSize: 5
  dataFreshnessDays: 15
  
  # SP-API配置
  spapi:
    enabled: true
    region: "us-east-1"
    marketplaceID: "ATVPDKIKX0DER"
    clientID: "amzn1.application-oa2-client.xxxxx"
    clientSecret: "your-client-secret"
    refreshToken: "Atzr|your-refresh-token"
    defaultFulfillmentType: "FBM"
    defaultCondition: "New"

# 自动定价配置
autoPricing:
  amazon:
    enabled: true
    interval: 300
    batchSize: 100
```

### 2. 环境变量

创建 `.env` 文件：

```bash
# Amazon SP-API
AMAZON_CLIENT_ID=amzn1.application-oa2-client.xxxxx
AMAZON_CLIENT_SECRET=your-client-secret
AMAZON_REFRESH_TOKEN=Atzr|your-refresh-token

# 管理系统
MANAGEMENT_BASE_URL=https://api.yourdomain.com
MANAGEMENT_CLIENT_SECRET=your-secret

# 日志
LOG_LEVEL=info
LOG_FORMAT=json
```

## 编译部署

### 方式 1: 直接编译

```bash
# 编译
go build -o amazon-processor cmd/amazon-listing/main.go

# 运行
./amazon-processor -config config/config-prod.yaml
```

### 方式 2: Docker 部署

创建 `Dockerfile`：

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o amazon-processor cmd/amazon-listing/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates chromium

WORKDIR /root/
COPY --from=builder /app/amazon-processor .
COPY config/ ./config/

EXPOSE 8080

CMD ["./amazon-processor", "-config", "config/config-prod.yaml"]
```

构建和运行：

```bash
# 构建镜像
docker build -t amazon-processor:latest .

# 运行容器
docker run -d \
  --name amazon-processor \
  -v $(pwd)/config:/root/config \
  -v $(pwd)/logs:/root/logs \
  --env-file .env \
  amazon-processor:latest
```

### 方式 3: Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  amazon-processor:
    build: .
    container_name: amazon-processor
    restart: unless-stopped
    volumes:
      - ./config:/root/config
      - ./logs:/root/logs
    env_file:
      - .env
    environment:
      - TZ=Asia/Shanghai
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

运行：

```bash
docker-compose up -d
```

## 监控配置

### 1. 日志配置

```go
// 生产环境日志
logger := logrus.New()
logger.SetLevel(logrus.InfoLevel)
logger.SetFormatter(&logrus.JSONFormatter{})

// 输出到文件
file, _ := os.OpenFile("logs/amazon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
logger.SetOutput(file)
```

### 2. Prometheus 指标

添加 Prometheus 指标收集：

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    taskProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "amazon_tasks_processed_total",
            Help: "Total number of processed tasks",
        },
        []string{"status"},
    )
    
    taskDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "amazon_task_duration_seconds",
            Help: "Task processing duration",
        },
        []string{"status"},
    )
)
```

### 3. 健康检查

添加健康检查端点：

```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})

http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
    // 检查处理器状态
    if processor.IsReady() {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Ready"))
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("Not Ready"))
    }
})

go http.ListenAndServe(":8080", nil)
```

## 运维管理

### 1. 启动脚本

创建 `scripts/start.sh`：

```bash
#!/bin/bash

# 设置环境变量
export LOG_LEVEL=info
export CONFIG_FILE=config/config-prod.yaml

# 启动进程
nohup ./amazon-processor -config $CONFIG_FILE > logs/app.log 2>&1 &

echo $! > amazon-processor.pid
echo "Amazon Processor started with PID: $(cat amazon-processor.pid)"
```

### 2. 停止脚本

创建 `scripts/stop.sh`：

```bash
#!/bin/bash

if [ -f amazon-processor.pid ]; then
    PID=$(cat amazon-processor.pid)
    echo "Stopping Amazon Processor (PID: $PID)..."
    kill -TERM $PID
    
    # 等待进程退出
    for i in {1..30}; do
        if ! kill -0 $PID 2>/dev/null; then
            echo "Process stopped"
            rm amazon-processor.pid
            exit 0
        fi
        sleep 1
    done
    
    # 强制停止
    echo "Force stopping..."
    kill -9 $PID
    rm amazon-processor.pid
else
    echo "PID file not found"
fi
```

### 3. 重启脚本

创建 `scripts/restart.sh`：

```bash
#!/bin/bash

./scripts/stop.sh
sleep 2
./scripts/start.sh
```

### 4. Systemd 服务

创建 `/etc/systemd/system/amazon-processor.service`：

```ini
[Unit]
Description=Amazon Processor Service
After=network.target

[Service]
Type=simple
User=app
WorkingDirectory=/opt/amazon-processor
ExecStart=/opt/amazon-processor/amazon-processor -config config/config-prod.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

管理服务：

```bash
# 启动
sudo systemctl start amazon-processor

# 停止
sudo systemctl stop amazon-processor

# 重启
sudo systemctl restart amazon-processor

# 开机自启
sudo systemctl enable amazon-processor

# 查看状态
sudo systemctl status amazon-processor

# 查看日志
sudo journalctl -u amazon-processor -f
```

## 性能优化

### 1. 并发配置

根据服务器性能调整：

```yaml
worker:
  concurrency: 10  # CPU 核心数 * 2
  bufferSize: 50   # 并发数 * 5
```

### 2. 连接池配置

```go
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

### 3. 内存优化

```bash
# 设置 Go 内存限制
export GOMEMLIMIT=2GiB

# 设置 GC 百分比
export GOGC=100
```

## 故障排查

### 1. 查看日志

```bash
# 实时日志
tail -f logs/amazon.log

# 错误日志
grep "ERROR" logs/amazon.log

# 特定任务
grep "TaskID=12345" logs/amazon.log
```

### 2. 检查进程

```bash
# 查看进程
ps aux | grep amazon-processor

# 查看资源使用
top -p $(cat amazon-processor.pid)

# 查看网络连接
netstat -anp | grep amazon-processor
```

### 3. 常见问题

**问题 1: 令牌刷新失败**

```bash
# 检查凭证
curl -X POST https://api.amazon.com/auth/o2/token \
  -d "grant_type=refresh_token&refresh_token=YOUR_TOKEN&client_id=YOUR_ID&client_secret=YOUR_SECRET"
```

**问题 2: API 限流**

调整请求频率：

```yaml
amazon:
  spapi:
    rateLimit:
      requestsPerSecond: 1
      burstSize: 3
```

**问题 3: 内存泄漏**

```bash
# 生成内存分析
go tool pprof http://localhost:8080/debug/pprof/heap

# 查看 goroutine
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

## 备份和恢复

### 1. 配置备份

```bash
# 备份配置
tar -czf config-backup-$(date +%Y%m%d).tar.gz config/

# 恢复配置
tar -xzf config-backup-20250101.tar.gz
```

### 2. 日志归档

```bash
# 归档日志
tar -czf logs-$(date +%Y%m%d).tar.gz logs/*.log

# 清理旧日志
find logs/ -name "*.log" -mtime +30 -delete
```

## 安全建议

1. **凭证管理**
   - 使用环境变量存储敏感信息
   - 定期轮换 API 密钥
   - 使用密钥管理服务（如 AWS Secrets Manager）

2. **网络安全**
   - 使用 HTTPS
   - 配置防火墙规则
   - 限制 API 访问 IP

3. **日志安全**
   - 不记录敏感信息
   - 定期清理日志
   - 加密日志传输

## 升级指南

### 1. 版本升级

```bash
# 备份当前版本
cp amazon-processor amazon-processor.bak

# 下载新版本
wget https://releases.yourdomain.com/amazon-processor-v1.1.0

# 停止服务
./scripts/stop.sh

# 替换二进制
mv amazon-processor-v1.1.0 amazon-processor
chmod +x amazon-processor

# 启动服务
./scripts/start.sh

# 验证
curl http://localhost:8080/health
```

### 2. 配置迁移

```bash
# 比较配置差异
diff config/config-prod.yaml config/config-prod.yaml.new

# 合并配置
# 手动合并新增的配置项
```

## 监控告警

### 1. 告警规则

```yaml
# Prometheus 告警规则
groups:
  - name: amazon_processor
    rules:
      - alert: HighErrorRate
        expr: rate(amazon_tasks_processed_total{status="error"}[5m]) > 0.1
        annotations:
          summary: "High error rate detected"
      
      - alert: ProcessorDown
        expr: up{job="amazon-processor"} == 0
        annotations:
          summary: "Amazon processor is down"
```

### 2. 通知配置

配置告警通知（邮件、钉钉、Slack等）

## 总结

按照本指南完成部署后，Amazon 平台处理器将：

✅ 稳定运行在生产环境
✅ 自动处理上架任务
✅ 提供完整的监控指标
✅ 支持优雅重启和升级
✅ 具备故障自动恢复能力

如有问题，请参考故障排查章节或联系技术支持。
