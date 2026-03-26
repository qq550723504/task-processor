# productenrich-api

商品信息增强服务，接收商品图片、文本描述或 1688 商品详情页链接，只支持 `https://detail.1688.com/offer/...`，通过 LLM 多模态分析异步生成结构化商品 JSON。

## 当前架构

### 1. 接入层
- `POST /api/v1/products/generate`
  负责参数绑定、基础校验、创建任务。
- `GET /api/v1/products/tasks/:task_id`
  负责查询任务状态和结果。

### 2. 任务状态机
任务状态由 [state_machine.go](/D:/code/task-processor/internal/productenrich/pipeline/state_machine.go) 收口，当前主状态为：
- `pending`
- `processing`
- `completed`
- `failed`

处理器只会消费 `pending` 任务；拒绝类错误不会重试；可重试错误最多自动重试 3 次。

### 3. 执行层
[processor.go](/D:/code/task-processor/internal/productenrich/pipeline/processor.go) 负责：
- 从仓储加载任务
- 调用 `ProductService.ProcessProduct`
- 失败分类
- 增加重试计数
- `PrepareRetry` 后重新入队

### 4. 编排层
[service_process.go](/D:/code/task-processor/internal/productenrich/service_process.go) 是薄编排入口，只做：
- `MarkProcessing`
- 运行 pipeline
- `MarkCompleted` / `MarkFailed`

### 5. 流水线层
[pipeline.go](/D:/code/task-processor/internal/productenrich/pipeline.go) 将主流程拆成 5 个显式阶段：

1. `parse_input`
2. `validate_strategy`
3. `analyze_product`
4. `generate_json`
5. `validate_result`

每个 stage 都会记录统一日志字段：
- `task_id`
- `stage`
- `duration_ms`
- `outcome`
- `failure_disposition`

### 6. 能力边界
[service.go](/D:/code/task-processor/internal/productenrich/service.go) 里支持两种 capability 模式：
- `compat`
  允许简单 fallback，适合测试或联调。
- `strict`
  缺少关键组件时直接失败，不静默降级。

API 装配入口 [main.go](/D:/code/task-processor/cmd/productenrich-api/main.go) 默认使用 `strict`。

### 7. 包结构
- 根包 [internal/productenrich](/D:/code/task-processor/internal/productenrich) 保留领域模型、接口契约、service 编排和通用规则
- [api](/D:/code/task-processor/internal/productenrich/api) 负责 HTTP handler
- [pipeline](/D:/code/task-processor/internal/productenrich/pipeline) 负责异步执行、状态机和重试
- [store](/D:/code/task-processor/internal/productenrich/store) 负责仓储实现
- [enrich](/D:/code/task-processor/internal/productenrich/enrich) 负责输入解析、理解、JSON 生成和变体生成等富化能力实现

## 业务流程

```text
Client
  -> POST /api/v1/products/generate
  -> api.Handler
  -> ProductService.CreateGenerateTask
     - 校验 image_urls / text / product_url
     - product_url 必须是 1688 商品详情页
     - 创建 Task(status=pending)
     - 提交到 WorkerPool

WorkerPool
  -> pipeline.Processor.ProcessTask
     - 仅处理 pending
     - 调用 ProductService.ProcessProduct
     - 失败时按状态机判断是否重试

ProductService.ProcessProduct
  -> MarkProcessing
  -> PipelineRunner
     1. parse_input
     2. validate_strategy
     3. analyze_product
     4. generate_json
     5. validate_result
  -> MarkCompleted

若任一步失败：
  -> MarkFailed(status=failed, error=...)
  -> Processor 决定 no-retry / retryable
```

## Pipeline 说明

### 1. `parse_input`
- 收集图片 URL
- 清洗文本
- 若存在 `product_url`，抓取 1688 商品详情页
- 合并 scraped 标题、描述、图片和属性

### 2. `validate_strategy`
- 校验输入质量
- 计算质量分
- 选择处理策略：
  - `full`
  - `basic`
  - `minimal`
  - `reject`

### 3. `analyze_product`
- 分析图片属性
- 提取文本属性
- 融合多模态信息为统一 `ProductAnalysis`

### 4. `generate_json`
- 调用 `enrich.JSONGenerator` 实现生成结果
- 根据策略决定是否生成规格和变体
- 图片列表直接来自 `ParsedInput.Images`

### 5. `validate_result`
- 校验结果完整性
- 检查图片一致性
- 检查关键词匹配
- 无效结果直接失败，不保存 completed 结果

## 数据模型

### 请求：`GenerateRequest`

| 字段 | 类型 | 说明 |
|------|------|------|
| `image_urls` | `[]string` | 商品图片 URL，最多 10 张 |
| `text` | `string` | 商品文本描述，最多 10000 字符 |
| `product_url` | `string` | 1688 商品详情页 URL，仅支持 `https://detail.1688.com/offer/...` |

三个字段至少提供一个，可组合使用。非法 `product_url` 会在创建任务阶段直接被拒绝。

### 结果：`ProductJSON`

| 字段 | 说明 |
|------|------|
| `title` | 商品标题 |
| `category` | 分类路径 |
| `attributes` | 商品属性键值对 |
| `specifications` | 规格信息 |
| `variants` | SKU 变体列表 |
| `selling_points` | 卖点列表 |
| `seo_keywords` | SEO 关键词 |
| `description` | 商品详情描述 |
| `images` | 图片 URL 列表 |

## API

```text
POST /api/v1/products/generate
GET  /api/v1/products/tasks/:task_id
GET  /health
```

## 启动

```bash
go run ./cmd/productenrich-api \
  -config config/config-dev.yaml \
  -port 8085 \
  -log-level info
```

依赖配置项：
- `openai` 必填
- `database` 可选，缺省使用内存仓储
- `redis` 可选，缺省使用内存实现
- `worker.concurrency` 控制并发度
