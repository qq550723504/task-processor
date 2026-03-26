# productenrich-api

商品信息增强服务，接收商品图片、文本描述或 1688 商品链接，通过 LLM 多模态分析，异步生成结构化的商品 JSON 数据。

## 业务流程

```
客户端
  │
  ▼
POST /api/v1/products/generate
  │  接收 image_urls / text / product_url（三选一或组合）
  │
  ▼
Handler（handler.go）
  │  参数绑定与基础校验
  │
  ▼
ProductService.CreateGenerateTask（service_task.go）
  │  1. 校验请求（至少一种输入，图片数 ≤ 10，文本 ≤ 10000 字符）
  │  2. 生成 UUID 任务 ID，写入数据库（status=pending）
  │  3. 提交 taskID 到 WorkerPool（降级时写 Redis 队列）
  │  4. 立即返回 task_id 给客户端（异步模式）
  │
  ▼
WorkerPool（infra/worker）
  │  并发消费队列，调用 Processor.ProcessTask
  │
  ▼
Processor.ProcessTask（processor.go）
  │  从数据库加载 Task，调用 ProductService.ProcessProduct
  │  失败时自动重试（最多 3 次），拒绝类错误不重试
  │
  ▼
ProductService.ProcessProduct（service_process.go）
  │
  ├─ 步骤 1：InputParser.ParseInput（parser.go）
  │    ├─ 收集图片 URL
  │    ├─ 清洗文本（去除特殊字符、多余空白）
  │    └─ 若有 product_url → WebScraper 抓取 1688 页面
  │         获取标题、描述、图片、规格、价格，合并去重
  │
  ├─ 步骤 2：InputValidator + QualityScorer + StrategySelector（validator.go / scorer.go / strategy.go）
  │    ├─ 验证图片可访问性（并发检测，超时 5s）
  │    ├─ 验证文本长度与关键词
  │    ├─ 计算质量分（图片 40% + 文本 30% + 抓取数据 30%，可选 LLM 重评分）
  │    └─ 按分数选择处理策略：
  │         ≥ 80 → full    （完整处理：图片分析 + 文本提取 + 变体生成 + SEO）
  │         ≥ 60 → basic   （跳过变体生成和详细规格）
  │         ≥ 50 → minimal （仅基础信息提取）
  │         < 50 → reject  （数据质量不足，直接失败）
  │
  ├─ 步骤 3：ProductUnderstanding.AnalyzeProduct（understanding.go）
  │    ├─ AnalyzeImage：调用 LLM vision 接口，提取颜色、材质、场景、用途
  │    │   多张图片逐一分析，后续图片补充空白字段
  │    ├─ ExtractTextAttributes：调用 LLM fast 接口，提取标题、属性、卖点
  │    │   若有抓取描述，合并属性并去重卖点
  │    └─ FuseMultimodal：调用 LLM default 接口，融合图文信息
  │         生成统一的 ProductRepresentation（产品类型 + 属性 + 特性）
  │
  ├─ 步骤 4：JSONGenerator + VariantGenerator（generator_json.go / variant.go）
  │    ├─ 调用 LLM 生成完整 ProductJSON
  │    │   包含：标题、分类、属性、规格、卖点、SEO 关键词、描述
  │    ├─ minimal 策略跳过变体生成
  │    └─ 图片列表直接使用 ParsedInput.Images（不由 LLM 生成）
  │
  ├─ 步骤 5：ResultValidator（result_validator.go）
  │    ├─ 校验图片一致性
  │    ├─ 关键词匹配度评分
  │    └─ 完整性检查（必填字段覆盖率）
  │
  └─ 保存结果到数据库（status=completed）
       失败时更新 error 字段（status=failed）

客户端轮询
  │
  ▼
GET /api/v1/products/tasks/:task_id
  │  返回 status + product_json（completed 时）或 error（failed 时）
```

## 数据模型

**请求（GenerateRequest）**

| 字段 | 类型 | 说明 |
|------|------|------|
| image_urls | []string | 商品图片 URL，最多 10 张 |
| text | string | 商品文本描述，最多 10000 字符 |
| product_url | string | 1688 商品页面 URL，自动抓取 |

三个字段至少提供一个，可组合使用。

**结果（ProductJSON）**

| 字段 | 说明 |
|------|------|
| title | 商品标题 |
| category | 分类路径 |
| attributes | 商品属性键值对 |
| specifications | 规格（尺寸、重量、包装） |
| variants | SKU 变体列表（颜色/尺码等） |
| selling_points | 卖点列表 |
| seo_keywords | SEO 关键词 |
| description | 商品详情描述 |
| images | 图片 URL 列表 |

## API 端点

```
POST /api/v1/products/generate       提交生成任务，立即返回 task_id
GET  /api/v1/products/tasks/:task_id 查询任务状态和结果
GET  /health                         健康检查
```

## 启动

```bash
go run ./cmd/productenrich-api \
  -config config/config-dev.yaml \
  -port 8085 \
  -log-level info
```

依赖配置项：`openai`（必须）、`database`（可选，缺省用内存）、`redis`（可选，缺省用内存）、`worker.concurrency`。
