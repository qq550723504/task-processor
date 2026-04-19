# productimage

`productimage` 负责商品图片处理，目标是把 1688 或用户提供的原始商品图处理成更适合 Amazon 使用的图片资产。

## 主要能力

- 解析图片来源
  - 直接图片 URL
  - 1688 商品详情页
- 复用 `productenrich` 的商品上下文分析
- 审计图片质量并筛选主图候选
- 主体提取
- 覆盖物清洗
  - promo 角标
  - overlay text
  - logo / watermark
- 白底图生成
- Amazon 合规校验
- 质量评分
- 自动拦截高风险任务到 `needs_review`
- 发布处理后的图片资产
- 任务重试时复用已有资产

## 主流程

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

主入口：
- [service_process.go](/D:/code/task-processor/internal/productimage/service_process.go)
- [pipeline.go](/D:/code/task-processor/internal/productimage/pipeline.go)

异步执行和重试：
- [pipeline/processor.go](/D:/code/task-processor/internal/productimage/pipeline/processor.go)
- [pipeline/state_machine.go](/D:/code/task-processor/internal/productimage/pipeline/state_machine.go)

## 任务状态

- `pending`
- `processing`
- `needs_review`
- `completed`
- `rejected`
- `failed`

其中：
- `needs_review` 表示自动处理完成，但因质量分数、fallback、营销覆盖物等原因需要人工确认
- `rejected` 表示人工审核后驳回

## 审核闭环

图片任务支持人工审核动作：

- `approve`
  把 `needs_review` 任务转成 `completed`
- `reject`
  把 `needs_review` 任务转成 `rejected`
- `retry`
  把 `needs_review / rejected / failed` 任务转回 `pending` 并重新入队

相关实现：
- [service_task.go](/D:/code/task-processor/internal/productimage/service_task.go)
- [api/handler.go](/D:/code/task-processor/internal/productimage/api/handler.go)

## 结果结构

核心结果类型在 [model.go](/D:/code/task-processor/internal/productimage/model.go)：

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
- `audit_images`
  会结合商品标题上下文，为每张图记录 `PrimaryObject`
- `stage_summaries`
  记录每个 stage 的 `outcome / duration_ms`
- `image_traces`
  记录每张图在各阶段的执行轨迹

补充说明：
- `quality.issues`
  现在可能带上商品上下文，例如 `primary image contains overlay text for Running Shoes`
- `review.reasons`
  现在可能带上商品上下文，例如 `primary image contains promo badge and was auto-cleaned for Running Shoes`
- 这类上下文优先来自 1688 抓取标题，其次来自输入文本和商品分析结果
- `ip_risk.reasons`
  当前会描述图片里的 logo / watermark / overlay text / promo badge / 1688 原图来源等风险信号

## 当前策略

## Model-backed production path

生产主路径现在按能力优先级运行：

- `subject extraction`
  优先走远端模型 provider
- `white background rendering`
  优先走远端模型 provider
- `scene generation`
  只有在显式配置 scene model 时才走生成模型
  未配置时不会再把本地 `SceneRenderer` 当作模型主路径
- `review`
  优先走 LLM review model，再由规则做 guard

本地 scene canvas 只允许作为显式 fallback，不应再被视为默认生产成功路径。

当前配置入口：

- `productimage.segmenter`
- `productimage.whiteBackground`
- `productimage.scene`
- `openai.clients.image`

其中：
- `productimage.scene.enabled=true`
- `productimage.scene.endpoint=<remote-scene-model-url>`

后，bootstrap 会为 gallery/scene 资产启用远端 `SceneGenerator`。

如果使用 OpenAI-compatible image API（例如统一成 OpenAI 协议的 `nanobanana` 服务），推荐直接配置：

- `openai.clients.image.model`
- `openai.clients.image.baseURL`
- `openai.clients.image.apiKey`

这条路径会优先于 `productimage.segmenter / whiteBackground / scene` 的独立 HTTP endpoint 配置。

### 主体提取

- 优先走外部分割模型
- 模型不可用时回退到本地规则裁切

相关文件：
- [real_components.go](/D:/code/task-processor/internal/productimage/real_components.go)
- [segmenter_client.go](/D:/code/task-processor/internal/productimage/segmenter_client.go)

### 白底图

- 优先走外部白底模型
- 模型不可用时回退到本地白画布合成

相关文件：
- [real_components.go](/D:/code/task-processor/internal/productimage/real_components.go)
- [background_client.go](/D:/code/task-processor/internal/productimage/background_client.go)

### 覆盖物清洗

- 使用仓库内 `watermark` 能力做检测和修复
- 对明显 promo/logo/text 风险图做规则兜底清洗

### 质量评分

当前是规则版质量评分器：
- 主图质量分
- 白底图质量分
- 总分
- 质量问题列表

默认实现：
- [default_components.go](/D:/code/task-processor/internal/productimage/default_components.go)
- [marketplace_profile.go](/D:/code/task-processor/internal/productimage/marketplace_profile.go)

### Amazon/US Family Profiles

当前 `amazon/us` 会先把 `ProductType` 映射到商品 family，再决定：
- 主图待审阈值
- 白底图待审阈值
- `white_canvas` fallback 的扣分力度

当前内置 family：
- `footwear`
- `apparel`
- `bags_accessories`
- `home_textiles`
- `electronics`
- `jewelry_watch`
- `beauty_bottle`
- `default`

示例映射：
- `Slippers` -> `footwear`
- `Running Shoes` -> `footwear`
- `Shirt` -> `apparel`
- `Backpack` -> `bags_accessories`
- `Blanket` -> `home_textiles`
- `Bluetooth Speaker` -> `electronics`
- `Watch` -> `jewelry_watch`
- `Cosmetic Bottle` -> `beauty_bottle`

当前策略方向：
- 软商品更宽松
  - `footwear / apparel / bags_accessories / home_textiles`
- 硬商品更严格
  - `electronics / jewelry_watch / beauty_bottle`

未命中的类型会回退到 `default`。

### 审核判断

当前会因为这些原因进入 `needs_review`：
- 主图存在明显营销覆盖物风险
- 关键阶段走了 fallback
- 质量评分低于阈值

## 资产发布与生命周期

发布器实现：
- [asset_publisher.go](/D:/code/task-processor/internal/productimage/asset_publisher.go)

当前支持：
- `local`
- `amazon`
- `hybrid`

生命周期管理：
- [lifecycle.go](/D:/code/task-processor/internal/productimage/lifecycle.go)

当前已经支持：
- 清理临时处理文件
- 保留稳定发布产物
- 重试时复用已有主体图 / 主图 / 白底图 / 已发布结果

## 目录职责

- [api](/D:/code/task-processor/internal/productimage/api)
  HTTP handler
- [pipeline](/D:/code/task-processor/internal/productimage/pipeline)
  异步执行和状态机
- [store](/D:/code/task-processor/internal/productimage/store)
  内存仓储和数据库仓储

## 关键文件

- [model.go](/D:/code/task-processor/internal/productimage/model.go)
- [interfaces.go](/D:/code/task-processor/internal/productimage/interfaces.go)
- [service.go](/D:/code/task-processor/internal/productimage/service.go)
- [service_task.go](/D:/code/task-processor/internal/productimage/service_task.go)
- [service_process.go](/D:/code/task-processor/internal/productimage/service_process.go)
- [pipeline.go](/D:/code/task-processor/internal/productimage/pipeline.go)
- [default_components.go](/D:/code/task-processor/internal/productimage/default_components.go)
- [real_components.go](/D:/code/task-processor/internal/productimage/real_components.go)
- [asset_publisher.go](/D:/code/task-processor/internal/productimage/asset_publisher.go)
- [lifecycle.go](/D:/code/task-processor/internal/productimage/lifecycle.go)

## 当前边界

已经具备：
- 处理链路
- 审核闭环
- 数据库存储
- 资产生命周期管理
- 重试复用
- 质量评分

还偏弱的部分：
- 更强的模型级质量评估
- 更精细的类目化图片规则
- 管理系统内的人审记录和审计归档

## 与 productenrich 的边界

`productimage` 和 `productenrich` 有上游共享，但职责不同。

### 应留在 productenrich 的能力

- 输入解析标准
- 1688 商品页抓取与输入合并
- 商品上下文理解
- 多模态商品分析
- 商品结构化输出
- 面向商品数据的完整性校验

也就是：
`productenrich` 负责“懂商品”。

### 应留在 productimage 的能力

- 图片审计
- 图片清洗
- 主体提取
- 白底图生成
- 图片质量评分
- Amazon 图片合规校验
- 审核流转
- 图片资产发布
- 生命周期管理

也就是：
`productimage` 负责“产图片”。

### 允许共享但不应重复实现的能力

- 1688 抓取和输入清洗
  主实现应放在 `productenrich`
- 商品上下文分析
  主实现应放在 `productenrich`

`productimage` 应复用这些能力，而不是自己再维护一套规则。
