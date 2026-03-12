# Product JSON Generator 整合命令清单

## 前提条件

确保 `product-json-generator` 和 `task-processor` 在同一父目录下：
```
d:\code\
├── product-json-generator/
└── task-processor/
```

## 执行步骤

在 `task-processor` 目录下执行以下命令：

### 步骤 1: 创建目录结构

```bash
mkdir -p internal/domain/productjson
mkdir -p internal/infra/llm
mkdir -p internal/infra/scraper
mkdir -p internal/application/productjson/api
mkdir -p internal/pkg/resilience
mkdir -p cmd/productjson-api
mkdir -p cmd/productjson-worker
```

### 步骤 2: 复制 Infra 层 - LLM

```bash
cp ../product-json-generator/internal/repo/llm_client.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_client_cached.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_client_resilient.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_factory.go internal/infra/llm/
cp ../product-json-generator/internal/repo/llm_manager.go internal/infra/llm/
```

### 步骤 3: 复制 Infra 层 - Database

```bash
cp ../product-json-generator/internal/repo/database.go internal/infra/database/productjson_db.go
cp ../product-json-generator/internal/repo/task_repo.go internal/infra/database/productjson_task_repo.go
```

### 步骤 4: 复制 Infra 层 - Scraper

```bash
cp ../product-json-generator/internal/repo/web_scraper.go internal/infra/scraper/
```

### 步骤 5: 复制 Application 层

```bash
# 核心服务
cp ../product-json-generator/internal/service/product_service.go internal/application/productjson/service.go
cp ../product-json-generator/internal/service/product_understanding.go internal/application/productjson/understanding.go
cp ../product-json-generator/internal/service/json_generator.go internal/application/productjson/generator.go
cp ../product-json-generator/internal/service/variant_generator.go internal/application/productjson/variant.go

# 验证相关
cp ../product-json-generator/internal/service/input_validator.go internal/application/productjson/validator.go
cp ../product-json-generator/internal/service/input_parser.go internal/application/productjson/parser.go
cp ../product-json-generator/internal/service/quality_scorer.go internal/application/productjson/scorer.go
cp ../product-json-generator/internal/service/strategy_selector.go internal/application/productjson/strategy.go
cp ../product-json-generator/internal/service/enhancement_suggester.go internal/application/productjson/suggester.go
cp ../product-json-generator/internal/service/result_validator.go internal/application/productjson/result_validator.go
cp ../product-json-generator/internal/service/validation_cache.go internal/application/productjson/validation_cache.go
cp ../product-json-generator/internal/service/llm_score_cache.go internal/application/productjson/llm_score_cache.go
cp ../product-json-generator/internal/service/llm_scorer.go internal/application/productjson/llm_scorer.go

# Worker
cp ../product-json-generator/internal/worker/task_worker.go internal/application/productjson/worker.go

# API
cp ../product-json-generator/internal/api/product_handler.go internal/application/productjson/api/handler.go
cp ../product-json-generator/internal/api/middleware.go internal/application/productjson/api/middleware.go
cp ../product-json-generator/internal/api/middleware_auth.go internal/application/productjson/api/middleware_auth.go
cp ../product-json-generator/internal/api/middleware_metrics.go internal/application/productjson/api/middleware_metrics.go
cp ../product-json-generator/internal/api/middleware_ratelimit.go internal/application/productjson/api/middleware_ratelimit.go
cp ../product-json-generator/internal/api/metrics_handler.go internal/application/productjson/api/metrics_handler.go
cp ../product-json-generator/internal/api/response.go internal/application/productjson/api/response.go
```

### 步骤 6: 复制 Pkg 层

```bash
cp ../product-json-generator/internal/utils/retry.go internal/pkg/resilience/
cp ../product-json-generator/internal/utils/circuit_breaker.go internal/pkg/resilience/
cp ../product-json-generator/internal/utils/parallel.go internal/pkg/resilience/
```

### 步骤 7: 复制 Core 层

```bash
cp ../product-json-generator/internal/utils/config.go internal/core/config/productjson_config.go
cp ../product-json-generator/internal/utils/config_watcher.go internal/core/config/productjson_config_watcher.go
cp ../product-json-generator/internal/utils/errors.go internal/core/errors/productjson_errors.go
cp ../product-json-generator/internal/utils/metrics.go internal/core/metrics/productjson_metrics.go
cp ../product-json-generator/internal/utils/prometheus.go internal/core/metrics/productjson_prometheus.go
```

### 步骤 8: 添加依赖

```bash
go get github.com/gin-gonic/gin@v1.9.1
go get github.com/chromedp/chromedp@v0.14.2
go get go.uber.org/zap@v1.27.1
go get gorm.io/gorm@v1.31.1
go get gorm.io/driver/postgres@v1.6.0
go get github.com/go-redis/redis/v8@v8.11.5
go mod tidy
```

## 后续步骤

文件复制完成后，需要：

1. **替换 Import 路径** - 使用 IDE 的全局替换功能
2. **更新包名** - 手动调整各文件的 package 声明
3. **创建 App 层编排代码** - service、processor、worker
4. **创建 CMD 入口** - main.go
5. **验证编译** - `go build ./...`

详细说明请查看 `INTEGRATION_STEPS.md`
