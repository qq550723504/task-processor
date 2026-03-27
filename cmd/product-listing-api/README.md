# product-listing-api

`product-listing-api` 当前通过统一的 [`internal/app/httpapi`](/D:/code/task-processor/internal/app/httpapi) 入口提供三条异步流水线：

- `productenrich`
  负责商品理解与结构化 JSON 生成
- `productimage`
  负责 1688 图片处理、Amazon 主图/白底图生成、审核和资产发布
- `amazonlisting`
  负责聚合商品信息与图片资产，生成 Amazon Listing 草稿、审核工作台和提交动作

## API

```text
POST /api/v1/products/generate
GET  /api/v1/products/tasks/:task_id

POST /api/v1/images/process
GET  /api/v1/images/tasks/:task_id
POST /api/v1/images/tasks/:task_id/review

POST /api/v1/amazon/listings/generate
GET  /api/v1/amazon/listings/tasks/:task_id
GET  /api/v1/amazon/listings/tasks/:task_id/workbench
POST /api/v1/amazon/listings/tasks/:task_id/review
POST /api/v1/amazon/listings/tasks/:task_id/submit

GET  /health
```

## productenrich

### 请求

`POST /api/v1/products/generate`

```json
{
  "image_urls": ["https://example.com/a.jpg"],
  "text": "Bluetooth headphones with ANC",
  "product_url": "https://detail.1688.com/offer/123456789.html"
}
```

约束：
- `image_urls` 最多 10 张
- `text` 最多 10000 字符
- `product_url` 仅支持 `https://detail.1688.com/offer/...`
- 三者至少提供一个

### 状态

- `pending`
- `processing`
- `completed`
- `failed`

### 主流程

```text
parse_input
  -> validate_strategy
  -> analyze_product
  -> generate_json
  -> validate_result
```

步骤说明：
- `parse_input`：解析输入源（图片、文本、1688 链接），统一为内部处理结构。
- `validate_strategy`：根据输入完整度与可用能力校验可执行策略，避免进入无效流程。
- `analyze_product`：对商品进行语义理解与信息抽取，生成后续结构化数据的上下文。
- `generate_json`：基于分析结果生成目标 JSON 草稿（包含关键字段与结构）。
- `validate_result`：对输出 JSON 做完整性与一致性校验，确保可被下游消费。

## productimage

### 请求

`POST /api/v1/images/process`

```json
{
  "image_urls": [
    "https://example.com/hero.jpg",
    "https://example.com/detail.jpg"
  ],
  "product_url": "https://detail.1688.com/offer/123456789.html",
  "text": "Ceramic mug",
  "marketplace": "amazon",
  "country": "US"
}
```

约束：
- `marketplace` 当前只支持 `amazon`
- `image_urls` 最多 20 张
- `image_urls` 和 `product_url` 至少提供一个

### 状态

- `pending`
- `processing`
- `needs_review`
- `completed`
- `rejected`
- `failed`

说明：
- `needs_review`
  自动处理完成，但因质量分、fallback 或营销覆盖物风险需要人工确认
- `rejected`
  人工审核后驳回

### 主流程

```text
parse_source
  -> analyze_context
  -> audit_images
  -> rank_candidates
  -> extract_subject
  -> cleanup_image
  -> render_white_bg
  -> render_gallery
  -> assess_quality
  -> validate_marketplace
  -> assess_review
  -> publish_assets
```

步骤说明：
- `parse_source`：解析请求输入，聚合图片、文本、商品链接、站点与国家信息。
- `analyze_context`：分析商品语境（品类、用途、卖点等），为后续图像处理提供上下文。
- `audit_images`：对输入图片做基础审计（清晰度、质量、可用性）并记录审计结果。
- `rank_candidates`：按审计与语境结果筛选主图候选与场景图候选。
- `extract_subject`：从主候选图中提取主体（抠图/主体定位），产出主体资产。
- `cleanup_image`：对主体图做清理与优化（例如去干扰元素），生成主图资产。
- `render_white_bg`：基于主图渲染白底图，满足平台主图规范要求。
- `render_gallery`：生成或整理辅图/场景图集合，用于详情展示。
- `assess_quality`：对主图、白底图、辅图做质量评分并汇总问题项。
- `validate_marketplace`：按目标平台规则校验图片合规性，不通过时拦截后续提交。
- `assess_review`：评估是否需要人工审核（低分、fallback、风险信号等）。
- `publish_assets`：发布图片资产（本地或远端），写入可访问地址与发布元数据。

### 审核接口

`POST /api/v1/images/tasks/:task_id/review`

```json
{
  "action": "approve",
  "reason": "optional"
}
```

支持动作：
- `approve`
- `reject`
- `retry`

### 结果结构

`GET /api/v1/images/tasks/:task_id` 返回 `TaskResult`，其中 `result` 是 `ImageProcessResult`。

关键字段：
- `main_image`
- `white_bg_image`
- `subject_cutout`
- `gallery_images`
- `compliance`
- `ip_risk`
- `quality`
- `review`
- `stage_summaries`
- `image_traces`

其中：
- `quality`
  包含 `overall_score / main_score / white_bg_score / issues`
- `ip_risk`
  包含 `level / score / reasons`，表示图片侧侵权风险信号
- `review`
  包含 `needs_review / reasons`
- `stage_summaries`
  记录每个 stage 的执行结果和耗时
- `image_traces`
  记录每张图在各阶段的执行轨迹，可能出现 `success / failed / fallback / reused`

### 资产地址语义

- `url`
  当前可直接使用的资产地址
- `metadata.local_path`
  本地临时产物路径
- `metadata.published_path`
  稳定发布后的本地产物路径
- `metadata.published_url`
  本地发布器带公共前缀时生成的访问地址
- `metadata.uploaded_url`
  上传到 Amazon 后的地址
- `metadata.uploaded_image_id`
  Amazon 图片 ID

### 生命周期与复用

当前已支持：
- 临时处理文件清理
- 保留稳定发布产物
- 重试时复用已有主体图、主图、白底图和已发布结果

## productimage 配置

配置项位于 `productimage` 段，关键字段：

```yaml
productimage:
  workDir: "./tmp/productimage"
  segmenter:
    enabled: false
    endpoint: ""
    apiKey: ""
    timeout: 45
  whiteBackground:
    enabled: false
    endpoint: ""
    apiKey: ""
    timeout: 45
  publisher:
    enabled: true
    provider: "local"
    outputDir: "./tmp/productimage-published"
    publicBase: ""
  lifecycle:
    cleanupTemporaryFiles: true
    reuseExistingAssets: true
```

### 发布策略

`productimage.publisher.provider` 支持：
- `local`
- `amazon`
- `hybrid`

## amazonlisting

### 请求

`POST /api/v1/amazon/listings/generate`

```json
{
  "marketplace": "amazon",
  "country": "US",
  "language": "en_US",
  "image_urls": ["https://example.com/hero.jpg"],
  "text": "Bluetooth headphones with ANC",
  "product_url": "https://detail.1688.com/offer/123456789.html",
  "target_category_hint": "Electronics",
  "brand_hint": "DemoBrand",
  "options": {
    "process_images": true,
    "publish_images": true,
    "strict_validation": true
  }
}
```

说明：
- 输入源和 `productenrich` / `productimage` 一样，支持 `image_urls`、`text`、`product_url` 组合输入
- `marketplace` 当前主目标是 Amazon
- `target_category_hint` 是可选提示字段，不填时走自动类目推断；填写后会优先影响生成草稿里的 `category_path` 和 `product_type`
  支持单值或路径形式，例如：`Electronics`、`Electronics > Headphones`、`Home & Kitchen / Drinkware`
- `options.process_images` 控制是否触发图片处理
- `options.publish_images` 控制是否发布图片资产
- `options.strict_validation` 控制是否严格校验导出结果

### 状态

- `pending`
- `processing`
- `needs_review`
- `completed`
- `rejected`
- `failed`

### 结果接口

- `GET /api/v1/amazon/listings/tasks/:task_id`
  返回最终任务结果，核心字段包括 `title`、`bullet_points`、`description`、`attributes`、`variants`、`images`、`pricing`、`compliance`、`ip_risk`、`listing_ip_risk`、`review`、`export`、`submission`
- `GET /api/v1/amazon/listings/tasks/:task_id/workbench`
  返回审核工作台视图，包含 `ready`、`needs_review`、`top_action`、`action_buckets`

补充说明：
- `ip_risk`
  主要表示文案侧侵权风险，例如品牌词、`compatible with`、`replacement for`
- `listing_ip_risk`
  表示最终汇总风险，会把文案侧风险和 `productimage` 的图片侧风险合并后再决策
- 当前审核/阻断更应该看 `listing_ip_risk`
  - `medium`：进入 `needs_review`
  - `high`：阻断生成结果

### 审核接口

`POST /api/v1/amazon/listings/tasks/:task_id/review`

```json
{
  "action": "approve",
  "reason": "optional"
}
```

### 提交接口

`POST /api/v1/amazon/listings/tasks/:task_id/submit`

```json
{
  "action": "preview"
}
```

说明：
- `review` 和 `submit` 的可选动作由服务端按当前任务状态校验
- `submit` 会把当前草稿导出为 Amazon 提交载荷，并记录提交结果/预览结果

## 包结构

### productenrich

- [internal/productenrich](/D:/code/task-processor/internal/productenrich)
- [api](/D:/code/task-processor/internal/productenrich/api)
- [pipeline](/D:/code/task-processor/internal/productenrich/pipeline)
- [store](/D:/code/task-processor/internal/productenrich/store)
- [enrich](/D:/code/task-processor/internal/productenrich/enrich)

### productimage

- [internal/productimage](/D:/code/task-processor/internal/productimage)
- [api](/D:/code/task-processor/internal/productimage/api)
- [pipeline](/D:/code/task-processor/internal/productimage/pipeline)
- [store](/D:/code/task-processor/internal/productimage/store)

### amazonlisting

- [internal/amazonlisting](/D:/code/task-processor/internal/amazonlisting)
- [api](/D:/code/task-processor/internal/amazonlisting/api)
- [store](/D:/code/task-processor/internal/amazonlisting/store)

## 启动

当前二进制入口位于 [`cmd/product-listing-api/main.go`](/D:/code/task-processor/cmd/product-listing-api/main.go)，内部通过 [`internal/app/httpapi`](/D:/code/task-processor/internal/app/httpapi) 统一装配三个模块。

```bash
go run ./cmd/product-listing-api \
  -config config/config-dev.yaml \
  -port 8085 \
  -log-level info
```

## 建议联调顺序

1. 先不接外部分割和白底模型，验证 fallback 流程
2. 再接 `segmenter`
3. 再接 `whiteBackground`
4. 再验证 `publisher`
5. 最后验证 `needs_review -> approve/reject/retry` 闭环
