# product-listing-api

`product-listing-api` 当前通过统一的 [`internal/app/httpapi`](/D:/code/task-processor/internal/app/httpapi) 入口提供四条异步流水线：

- `productenrich`
  负责商品理解与结构化 JSON 生成
- `productimage`
  负责 1688 图片处理、Amazon 主图/白底图生成、审核和资产发布
- `amazonlisting`
  负责聚合商品信息与图片资产，生成 Amazon Listing 草稿、审核工作台和提交动作
- `listingkit`
  负责聚合商品结构化结果与图片资产，生成可供 SHEIN / Amazon / TEMU / Walmart 使用的多平台资料包

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

POST /api/v1/listing-kits/generate
GET  /api/v1/listing-kits/tasks/:task_id
GET  /api/v1/listing-kits/tasks/:task_id/preview
GET  /api/v1/listing-kits/tasks/:task_id/revision-history
GET  /api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id
GET  /api/v1/listing-kits/tasks/:task_id/export
POST /api/v1/listing-kits/tasks/:task_id/revision
POST /api/v1/listing-kits/tasks/:task_id/revision/validate

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

## listingkit

### 请求

`POST /api/v1/listing-kits/generate`

```json
{
  "image_urls": ["https://example.com/hero.jpg"],
  "text": "Bluetooth headphones with ANC",
  "product_url": "https://detail.1688.com/offer/123456789.html",
  "platforms": ["amazon", "shein", "temu", "walmart"],
  "country": "US",
  "language": "en_US",
  "shein_store_id": 12345,
  "target_category_hint": "Electronics > Headphones",
  "brand_hint": "DemoBrand",
  "options": {
    "process_images": true
  }
}
```

说明：
- 输入源和 `productenrich` 一样，支持 `image_urls`、`text`、`product_url` 组合输入
- `platforms` 支持：`amazon`、`shein`、`temu`、`walmart`
- 不传 `platforms` 时默认生成四个平台资料包
- `options.process_images=true` 时会串联 `productimage` 产出主图、白底图和辅图地址
- `options.sds` 是可选字段；仅当服务端已配置有效 SDS 登录态，且 `variant_id>0` 时才会在图片处理成功后触发 SDS 设计同步
- `shein_store_id` 是可选字段；当需要启用 SHEIN 在线类目解析和属性模板加载时应传入有效店铺 ID，否则会降级为离线解析

### 带 SDS 同步的请求示例

下面这条请求会在 `productimage` 成功后，自动把选中的设计图同步到 SDS 设计页：

```json
{
  "image_urls": ["https://example.com/hero.jpg"],
  "text": "Bluetooth headphones with ANC",
  "platforms": ["amazon"],
  "options": {
    "process_images": true,
    "sds": {
      "variant_id": 89764,
      "parent_product_id": 89763,
      "prototype_group_id": 14555,
      "layer_id": "698744758333792256",
      "design_type": "material",
      "fit_level": 1,
      "resize_mode": 0
    }
  }
}
```

字段说明：
- `variant_id`
  必填。SDS 子 SKU ID，`listingkit` 只有在这个值大于 `0` 时才会触发同步
- `parent_product_id`
  可选。父商品 ID；不传时会由 SDS 设计初始化链路自动补足
- `prototype_group_id`
  可选。模板组 ID；不传时会优先使用设计页默认模板组
- `layer_id`
  可选。目标印花层 ID；不传时会自动选择当前模板的主设计层
- `design_type`
  可选。默认建议传 `material`，与当前 SDS 设计页真实请求保持一致
- `fit_level`
  可选。素材缩放级别；默认 `1`
- `resize_mode`
  可选。素材缩放模式；默认 `0`

已验证可直接联调的一组参数：
- `variant_id = 89764`
- `parent_product_id = 89763`
- `prototype_group_id = 14555`
- `layer_id = 698744758333792256`

### 最短联调步骤

启动本地服务后，可以直接用下面两步验证 `listingkit -> SDS` 是否打通。

1. 提交生成任务

```bash
curl -X POST "http://127.0.0.1:8080/api/v1/listing-kits/generate" \
  -H "Content-Type: application/json" \
  -d '{
    "image_urls": ["https://example.com/hero.jpg"],
    "text": "Bluetooth headphones with ANC",
    "platforms": ["amazon"],
    "options": {
      "process_images": true,
      "sds": {
        "variant_id": 89764,
        "parent_product_id": 89763,
        "prototype_group_id": 14555,
        "layer_id": "698744758333792256",
        "design_type": "material",
        "fit_level": 1,
        "resize_mode": 0
      }
    }
  }'
```

典型返回：

```json
{
  "task_id": "6d4028d9-6a5a-4e1a-9db7-4a1aef7b43b0",
  "status": "pending"
}
```

2. 轮询任务结果

```bash
curl "http://127.0.0.1:8080/api/v1/listing-kits/tasks/6d4028d9-6a5a-4e1a-9db7-4a1aef7b43b0"
```

判定是否成功，优先看这几个字段：
- `status = completed`
- `result.sds_sync.status = completed`
- `result.sds_sync.material_id > 0`
- `result.child_tasks` 里存在 `kind = sds_design_sync` 且 `status = completed`

如果只想看 SDS 结果，最小关注片段大致如下：

```json
{
  "status": "completed",
  "result": {
    "sds_sync": {
      "variant_id": 89764,
      "product_id": 89764,
      "prototype_group_id": 14555,
      "layer_id": "698744758333792256",
      "material_id": 459421935,
      "status": "completed"
    }
  }
}
```

### 状态

- `pending`
- `processing`
- `completed`
- `failed`

### 结果接口

- `GET /api/v1/listing-kits/tasks/:task_id`
  返回统一结果，核心字段包括：
  - `canonical_product`
  - `image_assets`
  - `sds_sync`
  - `amazon`
  - `shein`
  - `temu`
  - `walmart`
  - `summary`
  - `child_tasks`
- `GET /api/v1/listing-kits/tasks/:task_id/preview`
  返回前端专用预览视图，支持可选查询参数 `platform`
  - 例如：`/api/v1/listing-kits/tasks/:task_id/preview?platform=shein`
  - 当前会返回 `overview` 和各平台压缩后的预览 payload
  - `shein` 预览会直接携带 `inspection`、`submit_readiness`、`submit_checklist`、`status_overview`、`editor_context`、`request_draft`、`preview_product`
- `GET /api/v1/listing-kits/tasks/:task_id/revision-history`
  返回 revision 历史列表，适合客户端单独做“编辑轨迹”加载
  - 支持 `limit`，默认 `10`，最大 `20`
  - 支持 `before`，格式为 RFC3339，例如 `2026-04-17T10:00:00Z`
  - 支持 `action_type=edit|restore`
  - 返回的 `items` 按 `updated_at` 倒序排列
  - 每条记录都会带 `revision_id`
  - 每条记录都会带 `timeline.headline / timeline.badge / timeline.relation_text / timeline.change_count`
  - `meta.next_before` 可直接用于继续拉取更早一页
  - `meta.action_type` 会回显当前过滤条件
  - `meta.counts.all / meta.counts.edit / meta.counts.restore`
    会返回当前保留窗口内的时间线统计
  - `meta.has_more` 只表示当前保留窗口内是否还有下一页
  - `meta.is_truncated=true` 表示更早历史已经被服务端裁剪
- `GET /api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id`
  返回某一条 revision 历史的完整详情
  - 适合客户端时间线点开查看 `applied_changes` 和 `editor_context_snapshot`
  - 当前恢复确认相关数据统一收在 `restore_payload` 里，不再平铺很多恢复字段
  - 原始恢复数据优先读取 `restore_payload.core`
  - `restore_payload.core.draft`
    可直接作为下一次 `revision` 提交的回放草稿基础
  - `restore_payload.core.revision_payload`
    这是可直接提交到 `POST /api/v1/listing-kits/tasks/:task_id/revision` 的最小回滚请求
  - `restore_payload.core.context / safety / compare`
    分别用于恢复上下文、恢复安全提示和差异比较
  - 展示相关字段优先读取 `restore_payload.presentation`
  - `navigation.prev_revision_id / navigation.next_revision_id`
    方便详情页直接切换前后记录
  - 支持 `compare_to=prev|next|<revision_id>|current`
    比较结果统一放在 `restore_payload.core.compare`
  - 对旧历史记录也支持读取，服务端会自动生成兼容的 `revision_id`
- `GET /api/v1/listing-kits/tasks/:task_id/export`
  返回导出用 JSON 载荷，并附带下载文件名
  - 例如：`/api/v1/listing-kits/tasks/:task_id/export?platform=shein`
  - 默认返回多平台 bundle
  - 按平台导出时会只保留对应平台的资料结构
- `POST /api/v1/listing-kits/tasks/:task_id/revision`
  保存客户端编辑稿，并直接返回更新后的 preview
  - 当前优先支持 `shein` 深度编辑
  - `amazon / temu / walmart` 先支持标题、描述、图片等基础字段回写
  - 当请求里带 `restore_from_revision_id` 时
    服务端会直接取对应历史记录里的 `restore_draft` 执行回滚式保存
  - revision 成功返回的 preview 会附带 `apply_result`
    当前这是一个薄壳，主要语义是“本次是否执行成功”
    普通保存成功后的完整成功态协议统一读取 `apply_result.success_payload`
    原始领域数据优先读取 `apply_result.success_payload.core`
    其中展示相关字段优先读取 `apply_result.success_payload.presentation`
    `presentation.scene=apply_success`
    会统一提供卡片、文案、下一步动作和推荐视图
  - revision 成功返回的 preview 现在会附带 `applied_changes`
    可直接用于保存后提示“本次实际更新了哪些字段”
  - preview 也会附带 `revision_history`
    其中会记录每次 revision 的 `updated_at / updated_by / reason / platform / applied_changes`
    并在 `shein` 场景下附带只读的 `editor_context_snapshot`
    同时会带 `action_type=edit|restore`
    并附带可直接用于列表渲染的 `timeline`
    如果这次保存来自历史回滚，还会带 `restored_from_revision_id`
  - preview 还会附带 `revision_history_meta`
    当前服务端会最多保留最近 `20` 条历史记录，并通过
    `total_records / returned_records / has_more / is_truncated / max_records`
    告诉客户端当前是否已经裁剪
  - 当这次保存是一次历史回滚时，preview 还会额外附带 `restore_result`
    当前这也是一个薄壳，主要语义是“这次是否发生了回滚式保存”
    历史回滚成功后的完整成功态协议统一读取 `restore_result.success_payload`
    原始领域数据优先读取 `restore_result.success_payload.core`
    其中展示相关字段优先读取 `restore_result.success_payload.presentation`
    `presentation.scene=restore_success`
    会统一提供卡片、文案、下一步动作和推荐视图
  - 当 revision 校验失败时，会返回 `field_errors`
    其中包含 `field_path / code / message`，可直接用于前端字段级错误提示
- `POST /api/v1/listing-kits/tasks/:task_id/revision/validate`
  只做预校验，不保存修改
  - 当前会返回 `valid / field_errors`
  - `shein` 还会额外返回 `dirty_hints` 和各分区 `preview_effects`
  - `shein` 还会返回 `suggested_minimal_revision`
    这是基于当前 skeleton 自动裁剪后的最小 revision payload，适合前端直接做增量提交
  - `shein` 还会返回 `revision_diff_preview`
    可直接用于保存前确认，展示这次提交相对当前任务结果会改动哪些字段
  - 当请求里带 `restore_from_revision_id` 时
    `shein` 还会返回 `restore_preview`
    这和 history detail 里的 `restore_payload` 使用同一套恢复预览协议
    原始领域数据优先读取 `restore_preview.core`
    展示相关字段优先读取 `restore_preview.presentation`
    `presentation.scene=restore_preview`
    适合客户端在执行历史回滚前先做一次确认
  - 适合客户端在真正保存前先做一次表单检查

### SDS 同步结果示例

当 `options.sds` 生效时，`GET /api/v1/listing-kits/tasks/:task_id` 的 `result` 里会多出 `sds_sync`，同时 `child_tasks` 会出现 `kind=sds_design_sync`：

```json
{
  "task_id": "6d4028d9-6a5a-4e1a-9db7-4a1aef7b43b0",
  "status": "completed",
  "result": {
    "sds_sync": {
      "variant_id": 89764,
      "product_id": 89764,
      "prototype_group_id": 14555,
      "layer_id": "698744758333792256",
      "material_id": 459421935,
      "status": "completed"
    },
    "child_tasks": [
      {
        "kind": "sds_design_sync",
        "status": "completed"
      }
    ]
  }
}
```

失败时的行为：
- 不会打断 `listingkit` 主流程
- `result.sds_sync.status` 会变成 `failed`
- `result.sds_sync.error` 会带错误信息
- `result.summary.warnings` 会追加一条 `sds design sync failed: ...`

### 编辑稿请求示例

```json
{
  "platform": "shein",
  "actor": "desktop-client",
  "reason": "manual adjustment",
  "shein": {
    "spu_name": "Updated Travel Bottle",
    "product_name_en": "Updated Travel Bottle",
    "brand_name": "Updated Brand",
    "description": "Updated description",
    "images": {
      "main_image": "https://cdn.example.com/updated-main.jpg",
      "gallery": ["https://cdn.example.com/updated-gallery.jpg"]
    },
    "product_attributes": [
      {"name": "material", "value": "stainless steel"}
    ],
    "category_resolution": {
      "status": "resolved",
      "source": "manual_revision",
      "matched_path": ["Home", "Kitchen", "Bottle"],
      "category_id": 7788,
      "category_id_list": [100, 200, 7788],
      "product_type_id": 8899
    },
    "attribute_resolution": {
      "status": "resolved",
      "source": "manual_revision",
      "resolved_attributes": [
        {
          "name": "material",
          "value": "stainless steel",
          "attribute_id": 7001,
          "attribute_value_id": 301
        }
      ]
    },
    "sale_attribute_resolution": {
      "primary_attribute_id": 501,
      "secondary_attribute_id": 502,
      "selection_summary": ["颜色作为 SKC，尺码作为 SKU"]
    },
    "skc_patches": [
      {
        "supplier_code": "SKC-1",
        "skc_name": "Matte Black",
        "main_image_url": "https://cdn.example.com/new-skc.jpg",
        "sale_attribute": {
          "scope": "skc",
          "name": "Color",
          "value": "Matte Black",
          "attribute_id": 501,
          "attribute_value_id": 101
        },
        "sku_patches": [
          {
            "supplier_sku": "SKU-1",
            "base_price": "21.99",
            "stock_count": 26,
            "sale_attributes": [
              {
                "scope": "sku",
                "name": "Size",
                "value": "L",
                "attribute_id": 502,
                "attribute_value_id": 202
              }
            ]
          }
        ]
      }
    ],
    "review_notes": ["confirm category again"]
  }
}
```

历史回滚执行示例：

```json
{
  "platform": "shein",
  "restore_from_revision_id": "rev-restore-1"
}
```

补充说明：
- `amazon`
  当前直接复用 Amazon draft 组装能力
- `shein`
  当前输出已贴近 `SPU / SKC / SKU` 结构，核心字段包括 `spu_name`、`product_name_en`、`brand_name`、`skc_list`、`request_draft`、`preview_product`、`category_resolution`、`attribute_resolution`、`sale_attribute_resolution`
  其中 `sale_attribute_resolution` 现在会附带候选销售属性、主副属性评分和选择说明，便于客户端预览与人工确认
  同时会产出 `inspection` 视图，把类目、普通属性、销售属性和 review note 汇总成更适合前端直接展示的结构
  另外 `submit_readiness` 会单独给出提交前校验结果，区分 `blocking_items / warning_items / checks`，便于客户端直接展示“可提交 / 仍阻塞 / 带提醒”状态
- `temu`
  当前输出已贴近 `goods_basic / skc_list / batch_sku_info` 结构，核心字段包括 `goods_name`、`category_path`、`skc_list`、`batch_sku_info`
- `walmart`
  当前是占位适配器，重点先保证字段聚合和扩展位
- `child_tasks`
  记录底层 `productenrich` / `productimage` 子任务状态，便于客户端轮询和排查
- `preview`
  面向客户端渲染场景，`overview.platform_cards` 会汇总每个平台的预览状态与是否需要人工确认
  - `shein.inspection.sections[].actions`
    现在会返回结构化操作建议，客户端可以直接据此渲染“确认类目 / 确认属性 / 确认规格”按钮，并回写到 `revision` 接口
  - `shein.inspection.sections[].actions[].category / attributes / sale`
    提供稳定 typed payload；原来的 `payload` map 继续保留用于兼容
  - `shein.submit_readiness`
    用于提交前校验视图
    - `status=blocked`
      当前仍有关键阻塞项，不能直接提交
    - `status=ready_with_warnings`
      关键骨架已齐，但仍有人工备注待确认
    - `status=ready`
      当前关键字段已满足提交前要求
  - `shein.status_overview`
    用于客户端顶部状态条，已经把 `inspection + submit_readiness` 收口成更适合直接展示的 `headline / subheadline / primary_action / next_actions`
  - `shein.submit_checklist`
    用于客户端编辑页字段分组，已经按 `required / recommended / optional` 三组整理好，可直接映射到表单分区
  - `shein.editor_context`
    用于客户端编辑器默认值和 patch 草稿，已经把基础信息、类目、属性、销售属性以及 `skc/sku` 建议 patch 收口成稳定结构
    - `category / attributes / sale_attributes`
      现在都会附带 `recommendation.source / confidence / reason`
    - `attributes.suggestions / sale_attributes.candidate_suggestions`
      可直接用于候选值列表和“为什么推荐这个默认值”的说明
    - `category.preview_effects / attributes.preview_effects / sale_attributes.preview_effects`
      可直接用于“这次修改会影响哪些预览区块和字段”的提示
    - `revision_skeleton`
      直接对齐 `POST /api/v1/listing-kits/tasks/:task_id/revision` 的请求结构，客户端只需要在 skeleton 上填入变更值即可提交
    - `dirty_hints`
      提供 `editable_fields / default_changed_fields / sections`，可直接用于前端字段级 dirty 标记和增量提交
    - `progress`
      提供整体和分区级的 `completed / total / unresolved / status`，可直接用于编辑页进度条和完成度提示
- `export`
  面向下载/归档场景，当前统一导出 JSON
  - `amazon` 导出 `draft`
  - `shein` 导出 `inspection + request_draft + preview_product`
  - `temu / walmart` 导出当前结构化资料包
- `revision`
  记录最近一次客户端编辑稿保存信息，包括 `updated_at / updated_by / reason / platform`

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

### listingkit

- [internal/listingkit](/D:/code/task-processor/internal/listingkit)
- [api](/D:/code/task-processor/internal/listingkit/api)
- [store](/D:/code/task-processor/internal/listingkit/store)

## 启动

当前二进制入口位于 [`cmd/product-listing-api/main.go`](/D:/code/task-processor/cmd/product-listing-api/main.go)，内部通过 [`internal/app/httpapi`](/D:/code/task-processor/internal/app/httpapi) 统一装配四个模块。

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
