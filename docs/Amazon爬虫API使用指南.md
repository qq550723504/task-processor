# Amazon 爬虫 API 使用指南

## 概述

Amazon 爬虫 API 服务是一个独立的 HTTP API 服务，不依赖 RabbitMQ，可以通过 REST API 直接调用爬虫功能。

## 服务特点

- ✅ 独立运行，不依赖 RabbitMQ
- ✅ RESTful API 接口
- ✅ 异步任务处理
- ✅ 任务状态查询
- ✅ 支持并发爬取
- ✅ 健康检查和监控
- ✅ CORS 支持

## 启动服务

### 编译
```bash
go build -o bin/amazon-crawler-api.exe ./cmd/amazon-crawler-api
```

### 运行
```bash
# 使用默认配置
./amazon-crawler-api.exe

# 指定配置文件和端口
./amazon-crawler-api.exe --config=config/config-prod.yaml --port=8080 --log-level=info
```

### 参数说明
- `--config`: 配置文件路径（默认: config/config-prod.yaml）
- `--port`: API 服务端口（默认: 8080）
- `--log-level`: 日志级别（debug/info/warn/error，默认: info）

## API 端点

### 1. 提交爬虫任务

**端点**: `POST /api/v1/crawl`

**请求体**:
```json
{
  "url": "https://www.amazon.com/dp/B08N5WRWNW",
  "asin": "B08N5WRWNW",
  "priority": 1
}
```

**参数说明**:
- `url`: Amazon 商品 URL（url 和 asin 至少提供一个）
- `asin`: Amazon 商品 ASIN（可选，如果只提供 ASIN 会自动构造 URL）
- `priority`: 优先级（可选，默认 0）

**响应**:
```json
{
  "success": true,
  "message": "任务已提交",
  "data": {
    "task_id": "task-1234567890",
    "url": "https://www.amazon.com/dp/B08N5WRWNW"
  }
}
```

**示例**:
```bash
# 使用 URL
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.amazon.com/dp/B08N5WRWNW"}'

# 使用 ASIN
curl -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"asin": "B08N5WRWNW"}'
```

### 2. 查询任务状态

**端点**: `GET /api/v1/tasks/{task_id}`

**响应**:
```json
{
  "success": true,
  "message": "查询成功",
  "data": {
    "task_id": "task-1234567890",
    "status": "success",
    "product_data": {
      "title": "商品标题",
      "price": "$29.99",
      "asin": "B08N5WRWNW",
      ...
    },
    "started_at": "2024-03-10T10:00:00Z",
    "completed_at": "2024-03-10T10:00:15Z",
    "duration": "15.5s"
  }
}
```

**状态说明**:
- `pending`: 等待处理
- `processing`: 正在处理
- `success`: 处理成功
- `failed`: 处理失败

**示例**:
```bash
curl http://localhost:8080/api/v1/tasks/task-1234567890
```

### 3. 查询所有任务

**端点**: `GET /api/v1/tasks`

**响应**:
```json
{
  "success": true,
  "message": "查询成功",
  "data": {
    "total": 10,
    "tasks": [
      {
        "task_id": "task-1234567890",
        "status": "success",
        ...
      },
      ...
    ]
  }
}
```

**示例**:
```bash
curl http://localhost:8080/api/v1/tasks
```

### 4. 删除任务

**端点**: `DELETE /api/v1/tasks/{task_id}`

**响应**:
```json
{
  "success": true,
  "message": "任务已删除"
}
```

**示例**:
```bash
curl -X DELETE http://localhost:8080/api/v1/tasks/task-1234567890
```

### 5. 查询统计信息

**端点**: `GET /api/v1/stats`

**响应**:
```json
{
  "success": true,
  "message": "查询成功",
  "data": {
    "worker_count": 5,
    "active_workers": 5,
    "queue_size": 10,
    "queue_capacity": 1000,
    "total_processed": 100,
    "total_success": 95,
    "total_failed": 5,
    "status_count": {
      "pending": 10,
      "processing": 5,
      "success": 80,
      "failed": 5
    }
  }
}
```

**示例**:
```bash
curl http://localhost:8080/api/v1/stats
```

### 6. 健康检查

**端点**: `GET /health`

**响应**:
```json
{
  "success": true,
  "message": "healthy",
  "data": {
    "timestamp": "2024-03-10T10:00:00Z"
  }
}
```

**示例**:
```bash
curl http://localhost:8080/health
```

### 7. 就绪检查

**端点**: `GET /ready`

**响应**:
```json
{
  "success": true,
  "message": "ready",
  "data": {
    "timestamp": "2024-03-10T10:00:00Z"
  }
}
```

**示例**:
```bash
curl http://localhost:8080/ready
```

## 使用场景

### 场景 1: 单个商品爬取

```bash
# 1. 提交任务
TASK_ID=$(curl -s -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"asin": "B08N5WRWNW"}' | jq -r '.data.task_id')

echo "任务 ID: $TASK_ID"

# 2. 等待几秒
sleep 10

# 3. 查询结果
curl http://localhost:8080/api/v1/tasks/$TASK_ID | jq
```

### 场景 2: 批量爬取

```bash
# 批量提交任务
for asin in B08N5WRWNW B07XJ8C8F5 B09G9FPHY6; do
  curl -X POST http://localhost:8080/api/v1/crawl \
    -H "Content-Type: application/json" \
    -d "{\"asin\": \"$asin\"}"
  echo ""
done

# 查询所有任务
curl http://localhost:8080/api/v1/tasks | jq
```

### 场景 3: 轮询任务状态

```bash
#!/bin/bash

# 提交任务
TASK_ID=$(curl -s -X POST http://localhost:8080/api/v1/crawl \
  -H "Content-Type: application/json" \
  -d '{"asin": "B08N5WRWNW"}' | jq -r '.data.task_id')

echo "任务 ID: $TASK_ID"
echo "等待任务完成..."

# 轮询状态
while true; do
  STATUS=$(curl -s http://localhost:8080/api/v1/tasks/$TASK_ID | jq -r '.data.status')
  echo "当前状态: $STATUS"
  
  if [ "$STATUS" = "success" ] || [ "$STATUS" = "failed" ]; then
    break
  fi
  
  sleep 2
done

# 获取结果
curl http://localhost:8080/api/v1/tasks/$TASK_ID | jq
```

## 集成示例

### Python 客户端

```python
import requests
import time

class AmazonCrawlerClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url
    
    def crawl(self, url=None, asin=None, priority=0):
        """提交爬虫任务"""
        data = {"priority": priority}
        if url:
            data["url"] = url
        if asin:
            data["asin"] = asin
        
        response = requests.post(
            f"{self.base_url}/api/v1/crawl",
            json=data
        )
        return response.json()
    
    def get_task(self, task_id):
        """查询任务状态"""
        response = requests.get(
            f"{self.base_url}/api/v1/tasks/{task_id}"
        )
        return response.json()
    
    def wait_for_task(self, task_id, timeout=60):
        """等待任务完成"""
        start_time = time.time()
        while time.time() - start_time < timeout:
            result = self.get_task(task_id)
            if result["success"]:
                status = result["data"]["status"]
                if status in ["success", "failed"]:
                    return result
            time.sleep(2)
        raise TimeoutError("任务超时")
    
    def get_stats(self):
        """查询统计信息"""
        response = requests.get(f"{self.base_url}/api/v1/stats")
        return response.json()

# 使用示例
client = AmazonCrawlerClient()

# 提交任务
result = client.crawl(asin="B08N5WRWNW")
task_id = result["data"]["task_id"]
print(f"任务 ID: {task_id}")

# 等待完成
result = client.wait_for_task(task_id)
if result["data"]["status"] == "success":
    print("爬取成功!")
    print(result["data"]["product_data"])
else:
    print("爬取失败:", result["data"]["error"])
```

### Go 客户端

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type CrawlerClient struct {
	BaseURL string
	Client  *http.Client
}

func NewCrawlerClient(baseURL string) *CrawlerClient {
	return &CrawlerClient{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CrawlerClient) Crawl(url, asin string, priority int) (string, error) {
	data := map[string]interface{}{
		"url":      url,
		"asin":     asin,
		"priority": priority,
	}
	
	body, _ := json.Marshal(data)
	resp, err := c.Client.Post(
		c.BaseURL+"/api/v1/crawl",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	taskID := result["data"].(map[string]interface{})["task_id"].(string)
	return taskID, nil
}

func (c *CrawlerClient) GetTask(taskID string) (map[string]interface{}, error) {
	resp, err := c.Client.Get(c.BaseURL + "/api/v1/tasks/" + taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func main() {
	client := NewCrawlerClient("http://localhost:8080")
	
	// 提交任务
	taskID, err := client.Crawl("", "B08N5WRWNW", 0)
	if err != nil {
		panic(err)
	}
	fmt.Println("任务 ID:", taskID)
	
	// 等待完成
	for {
		result, _ := client.GetTask(taskID)
		data := result["data"].(map[string]interface{})
		status := data["status"].(string)
		
		fmt.Println("状态:", status)
		if status == "success" || status == "failed" {
			break
		}
		time.Sleep(2 * time.Second)
	}
}
```

## 配置说明

服务使用与其他爬虫服务相同的配置文件，主要配置项：

```yaml
amazon:
  enabled: true
  concurrency: 5  # 并发工作线程数
  timeout: 30     # 爬取超时时间（秒）
  # 其他 Amazon 配置...
```

## 部署建议

### Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o amazon-crawler-api ./cmd/amazon-crawler-api

FROM alpine:latest
RUN apk --no-cache add ca-certificates chromium
WORKDIR /root/
COPY --from=builder /app/amazon-crawler-api .
COPY config/ ./config/
EXPOSE 8080
CMD ["./amazon-crawler-api", "--port=8080"]
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: amazon-crawler-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: amazon-crawler-api
  template:
    metadata:
      labels:
        app: amazon-crawler-api
    spec:
      containers:
      - name: amazon-crawler-api
        image: your-registry/amazon-crawler-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            cpu: "1000m"
            memory: "1Gi"
          limits:
            cpu: "4000m"
            memory: "4Gi"
---
apiVersion: v1
kind: Service
metadata:
  name: amazon-crawler-api
spec:
  selector:
    app: amazon-crawler-api
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

## 监控和告警

### Prometheus 监控

可以通过 `/api/v1/stats` 端点获取指标，然后配置 Prometheus 抓取：

```yaml
scrape_configs:
  - job_name: 'amazon-crawler-api'
    metrics_path: '/api/v1/stats'
    static_configs:
      - targets: ['amazon-crawler-api:8080']
```

### 关键指标

- `worker_count`: 工作线程数
- `active_workers`: 活跃工作线程数
- `queue_size`: 队列中等待的任务数
- `total_processed`: 总处理任务数
- `total_success`: 成功任务数
- `total_failed`: 失败任务数

## 常见问题

### Q: 任务队列满了怎么办？
A: 默认队列容量是 1000，如果满了会返回 503 错误。可以：
1. 增加工作线程数（修改配置中的 concurrency）
2. 等待队列消化后再提交
3. 部署多个实例

### Q: 如何提高爬取速度？
A: 
1. 增加并发工作线程数
2. 部署多个服务实例
3. 使用负载均衡分发请求

### Q: 任务结果会保存多久？
A: 当前版本任务结果保存在内存中，服务重启后会丢失。建议：
1. 及时获取结果
2. 或者自己实现结果持久化

### Q: 支持代理吗？
A: 支持，在配置文件中配置代理设置即可。

## 总结

Amazon 爬虫 API 服务提供了简单易用的 REST API 接口，适合：
- 需要按需爬取的场景
- 需要同步获取结果的场景
- 不想部署 RabbitMQ 的场景
- 需要与其他系统集成的场景

如果需要大规模异步任务处理，建议使用基于 RabbitMQ 的版本。
