# Product JSON Generator 分层整合步骤

## 整合完成后的目录结构

```
task-processor/
├── internal/
│   ├── domain/
│   │   └── productjson/          # 领域层
│   │       ├── model.go           # 数据模型
│   │       └── service.go         # 服务接口定义
│   ├── infra/
│   │   ├── llm/                   # LLM 客户端实现
│   │   ├── database/              # 数据库访问
│   │   ├── cache/                 # 缓存实现
│   │   └── scraper/               # 网页抓取
│   ├── app/
│   │   └── productjson/           # 应用层
│   │       ├── service/           # 业务服务实现
│   │       ├── validator/         # 输入验证
│   │       ├── worker/            # 任务工作器
│   │       └── api/               # HTTP 处理器
│   ├── core/
│   │   ├── config/                # 配置管理
│   │   └── metrics/               # 指标监控
│   └── pkg/
│       └── resilience/            # 弹性工具（重试、熔断）
└── cmd/
    ├── productjson-api/           # API 服务入口
    └── productjson-worker/        # Worker 服务入口
```

## 执行步骤

### 步骤 1: 创建目录结构

```bash
cd task-processor

# Domain 层
mkdir -p internal/domain/productjson

# Infra 层
mkdir -p internal/infra/llm
mkdir -p internal/infra/scraper

# App 层
mkdir -p internal/app/productjson/service
mkdir -p internal/app/productjson/validator
mkdir -p internal/app/productjson/worker
mkdir -p internal/app/productjson/api

# Pkg 层
mkdir -p internal/pkg/resilience

# CMD 层
mkdir -p cmd/productjson-api
mkdir -p cmd/productjson-worker
```

### 步骤 2: 复制并重命名文件

#### 2.1 Domain 层
```bash
# 复制模型定义
cp ../product-json-generator/internal/model/types.go internal/domain/productjson/model.go
```

#### 2.2 Infra 层 - LLM
```bash
cp ../product-json-generator/internal/repo/llm_client.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_client_cached.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_client_resilient.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_factory.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_manager.go internal/infra/llm/
```

#### 2.3 Infra 层 - Database
```bash
cp ../product-json-generator/internal/repo/database.go internal/infra/database/productjson_db.go
cp ../product-json-generator/internal/repo/task_repo.go internal/infra/database/task_repo.go
```

#### 2.4 Infra 层 - Cache
```bash
cp ../product-json-generator/internal/repo/redis_client.go internal/infra/cache/redis.go
cp ../product-json-generator/internal/repo/cache.go internal/infra/cache/
cp ../product-json-generator/internal/repo/cache_redis.go internal/infra/cache/
```

#### 2.5 Infra 层 - Scraper
```bash
cp ../product-json-generator/internal/repo/web_scraper.go internal/infra/scraper/
```

#### 2.6 App 层 - Service
```bash
cp ../product-json-generator/internal/service/product_service.go internal/app/productjson/service/
cp ../product-json-generator/internal/service/product_understanding.go internal/app/productjson/service/
cp ../product-json-generator/internal/service/json_generator.go internal/app/productjson/service/
cp ../product-json-generator/internal/service/variant_generator.go internal/app/productjson/service/
cp ../product-json-generator/internal/service/llm_score_cache.go internal/app/productjson/service/
cp ../product-json-generator/internal/service/llm_scorer.go internal/app/productjson/service/
```

#### 2.7 App 层 - Validator
```bash
cp ../product-json-generator/internal/service/input_validator.go internal/app/productjson/validator/
cp ../product-json-generator/internal/service/input_parser.go internal/app/productjson/validator/
cp ../product-json-generator/internal/service/quality_scorer.go internal/app/productjson/validator/
cp ../product-json-generator/internal/service/strategy_selector.go internal/app/productjson/validator/
cp ../product-json-generator/internal/service/enhancement_suggester.go internal/app/productjson/validator/
cp ../product-json-generator/internal/service/result_validator.go internal/app/productjson/validator/
cp ../product-json-generator/internal/service/validation_cache.go internal/app/productjson/validator/
```

#### 2.8 App 层 - Worker
```bash
cp ../product-json-generator/internal/worker/task_worker.go internal/app/productjson/worker/
```

#### 2.9 App 层 - API
```bash
cp ../product-json-generator/internal/api/product_handler.go internal/app/productjson/api/
cp ../product-json-generator/internal/api/middleware.go internal/app/productjson/api/
cp ../product-json-generator/internal/api/middleware_auth.go internal/app/productjson/api/
cp ../product-json-generator/internal/api/middleware_metrics.go internal/app/productjson/api/
cp ../product-json-generator/internal/api/middleware_ratelimit.go internal/app/productjson/api/
cp ../product-json-generator/internal/api/metrics_handler.go internal/app/productjson/api/
cp ../product-json-generator/internal/api/response.go internal/app/productjson/api/
```

#### 2.10 Pkg 层 - Resilience
```bash
cp ../product-json-generator/internal/utils/retry.go internal/pkg/resilience/
cp ../product-json-generator/internal/utils/circuit_breaker.go internal/pkg/resilience/
cp ../product-json-generator/internal/utils/parallel.go internal/pkg/resilience/
```

#### 2.11 Core 层 - Config
```bash
cp ../product-json-generator/internal/utils/config.go internal/core/config/productjson_config.go
cp ../product-json-generator/internal/utils/config_watcher.go internal/core/config/
```

#### 2.12 Core 层 - Metrics
```bash
cp ../product-json-generator/internal/utils/metrics.go internal/core/metrics/productjson_metrics.go
cp ../product-json-generator/internal/utils/prometheus.go internal/core/metrics/
```

#### 2.13 Core 层 - Errors
```bash
cp ../product-json-generator/internal/utils/errors.go internal/core/errors/productjson_errors.go
```

#### 2.14 Core 层 - Logger (使用现有的，需要创建适配器)
```bash
# 不需要复制，使用 task-processor 现有的 logrus
# 需要创建 Zap 到 Logrus 的适配器
```

### 步骤 3: 批量替换 Import 路径

使用以下命令或在 IDE 中全局替换：

```bash
# 在 Git Bash 或 Linux 终端中执行
find internal/domain/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/model|task-processor/internal/domain/productjson|g' {} +
find internal/domain/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/repo|task-processor/internal/infra|g' {} +
find internal/domain/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/service|task-processor/internal/app/productjson/service|g' {} +
find internal/domain/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/utils|task-processor/internal/core|g' {} +

find internal/infra -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/model|task-processor/internal/domain/productjson|g' {} +
find internal/infra -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/repo|task-processor/internal/infra|g' {} +
find internal/infra -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/utils|task-processor/internal/core|g' {} +

find internal/app/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/model|task-processor/internal/domain/productjson|g' {} +
find internal/app/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/repo|task-processor/internal/infra|g' {} +
find internal/app/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/service|task-processor/internal/app/productjson/service|g' {} +
find internal/app/productjson -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/utils|task-processor/internal/core|g' {} +

find internal/pkg/resilience -name "*.go" -type f -exec sed -i 's|product-json-generator/internal/utils|task-processor/internal/pkg/resilience|g' {} +

find internal/core -name "*.go" -type f -exec sed -i 's|product-json-generator/internal|task-processor/internal|g' {} +
```

或者在 VSCode 中：
1. 打开搜索替换（Ctrl+Shift+H）
2. 搜索：`product-json-generator/internal/model`
3. 替换为：`task-processor/internal/domain/productjson`
4. 在整个项目中替换

重复以上步骤替换其他路径。

### 步骤 4: 更新包名

需要手动更新以下文件的包名：

```go
// internal/domain/productjson/model.go
package productjson  // 改为 productjson

// internal/infra/llm/*.go
package llm  // 改为 llm

// internal/infra/database/*.go
package database  // 改为 database

// internal/infra/cache/*.go
package cache  // 改为 cache

// internal/infra/scraper/*.go
package scraper  // 改为 scraper

// internal/app/productjson/service/*.go
package service  // 保持 service

// internal/app/productjson/validator/*.go
package validator  // 改为 validator

// internal/app/productjson/worker/*.go
package worker  // 保持 worker

// internal/app/productjson/api/*.go
package api  // 保持 api

// internal/pkg/resilience/*.go
package resilience  // 改为 resilience
```

### 步骤 5: 添加依赖

```bash
cd task-processor
go get github.com/gin-gonic/gin@v1.9.1
go get github.com/chromedp/chromedp@v0.14.2
go get go.uber.org/zap@v1.27.1
go get gorm.io/gorm@v1.31.1
go get gorm.io/driver/postgres@v1.6.0
go get github.com/go-redis/redis/v8@v8.11.5
go mod tidy
```

### 步骤 6: 创建日志适配器

由于 product-json-generator 使用 Zap，task-processor 使用 Logrus，需要创建适配器：

```go
// internal/pkg/logger/zap_adapter.go
package logger

import (
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogrusToZap 将 Logrus Logger 转换为 Zap Logger
func LogrusToZap(l *logrus.Logger) *zap.Logger {
	// 实现适配逻辑
	// 或者直接使用 Logrus，不使用 Zap
	return nil
}
```

### 步骤 7: 创建 CMD 入口

#### 7.1 API 服务入口
```go
// cmd/productjson-api/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-processor/internal/app/productjson/api"
	"task-processor/internal/app/productjson/service"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/infra/cache"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/llm"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()
	
	// 初始化日志
	appLogger := logger.GetLogger()
	
	// 初始化数据库
	db, err := database.NewProductJSONDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	
	// 初始化 Redis
	redisClient, err := cache.NewRedisClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	
	// 初始化 LLM 管理器
	llmManager, err := llm.NewManager(cfg)
	if err != nil {
		log.Fatal(err)
	}
	
	// 创建服务
	productService := service.NewProductService(
		appLogger,
		db,
		redisClient,
		llmManager,
	)
	
	// 创建 API 处理器
	handler := api.NewProductHandler(appLogger, productService)
	
	// 创建路由
	router := gin.Default()
	router.POST("/api/product/generate", handler.GenerateProduct)
	router.GET("/api/product/result/:task_id", handler.GetTaskResult)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ProductJSON.Server.Port),
		Handler: router,
	}
	
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	
	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
```

### 步骤 8: 更新配置文件

在 `config/config-dev.yaml` 中添加：

```yaml
# 产品 JSON 生成配置
productjson:
  server:
    port: 8090
    timeout: 60s
    read_timeout: 30s
    write_timeout: 30s
  
  database:
    host: localhost
    port: 5432
    user: postgres
    password: ${PRODUCTJSON_DB_PASSWORD}
    database: productjson
    max_connections: 20
    max_idle_connections: 5
    connection_max_lifetime: 300s
  
  redis:
    host: localhost
    port: 6379
    password: ""
    db: 1
    pool_size: 10
  
  llm:
    default_client: default
    clients:
      default:
        provider: openai
        api_key: ${OPENAI_API_KEY}
        model: gpt-4
        timeout: 60s
        max_retries: 3
      fast:
        provider: openai
        api_key: ${OPENAI_API_KEY}
        model: gpt-3.5-turbo
        timeout: 30s
        max_retries: 3
      vision:
        provider: openai
        api_key: ${OPENAI_API_KEY}
        model: gpt-4-vision-preview
        timeout: 90s
        max_retries: 3
  
  worker:
    concurrency: 5
    queue_name: productjson:tasks
    task_timeout: 300s
  
  validation:
    quality_weights:
      image: 0.4
      text: 0.3
      scraped: 0.3
    strategy_thresholds:
      full: 80.0
      basic: 60.0
      minimal: 50.0
    image_validation:
      timeout: 10s
      max_concurrent: 10
      enable_cache: true
      cache_ttl: 24h
    llm_scoring:
      enabled: false
      text_client: fast
      vision_client: vision
```

### 步骤 9: 验证编译

```bash
cd task-processor
go build ./...
```

如果有编译错误，根据错误信息调整 import 路径和包名。

### 步骤 10: 运行测试

```bash
go test ./internal/domain/productjson/...
go test ./internal/app/productjson/...
```

### 步骤 11: 启动服务

```bash
# 设置环境变量
export OPENAI_API_KEY="sk-..."
export PRODUCTJSON_DB_PASSWORD="your-password"

# 启动 API 服务
go run cmd/productjson-api/main.go
```

## 注意事项

1. **日志系统**：建议统一使用 Logrus，移除 Zap 依赖
2. **配置系统**：使用 task-processor 的配置系统
3. **错误处理**：统一使用 task-processor 的错误处理方式
4. **测试文件**：测试文件也需要更新 import 路径
5. **数据库表**：需要运行 `product-json-generator/scripts/init_db.sql` 创建表

## 完成后的验证

1. 编译通过：`go build ./...`
2. 测试通过：`go test ./...`
3. API 可访问：`curl http://localhost:8090/health`
4. 功能正常：提交一个生成任务并查询结果

## 后续优化

1. 统一日志格式
2. 添加监控指标
3. 完善错误处理
4. 添加更多测试
5. 优化性能
