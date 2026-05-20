# productenrich-api

`productenrich-api` 当前是共享 `product-listing-api` HTTP 装配的一条兼容入口，不再维护独立的 API 组装逻辑。

`productenrich-api` 当前同时提供两条异步流水线：

- `productenrich`
  负责商品理解与结构化 JSON 生成
- `productimage`
  负责 1688 图片处理、Amazon 主图/白底图生成、审核和资产发布

## API

```text
POST /api/v1/products/generate
GET  /api/v1/products/tasks/:task_id

POST /api/v1/images/process
GET  /api/v1/images/tasks/:task_id
POST /api/v1/images/tasks/:task_id/review

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
- `quality`
- `review`
- `stage_summaries`
- `image_traces`

其中：
- `quality`
  包含 `overall_score / main_score / white_bg_score / issues`
- `review`
  包含 `needs_review / reasons`
- `stage_summaries`
  记录每个 stage 的执行结果和耗时
- `image_traces`
  记录每张图在各阶段的执行轨迹，可能出现 `success / failed / fallback / reused`

补充说明：
- `audit_images` 阶段会结合商品标题上下文，为图片审计补充 `PrimaryObject`
- `quality.issues` 和 `review.reasons` 可能会带上商品上下文，例如 `... for Running Shoes`
- 这类上下文优先来自 1688 抓取标题，其次来自输入文本和商品分析结果

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
  workDir: "./.local/tmp/productimage"
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
    outputDir: "./.local/tmp/productimage-published"
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

## 启动

```bash
go run ./cmd/productenrich-api \
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
