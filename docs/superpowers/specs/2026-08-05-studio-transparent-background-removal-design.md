# ListingKit Studio 透明背景后处理设计

## 1. 背景与目标

ListingKit Studio 当前通过提示词要求生图模型直接输出透明背景，并在开启透明背景时固定使用 `gpt-image-2`。这种方式调用链短，但生图模型的主要目标是生成画面，不能稳定保证主体边缘、细线、孔洞和半透明区域的 alpha 质量。

本设计新增一种“生成普通图后调用抠图模型”的处理方式，同时保留现有的原生透明背景方式。目标是：

- 让用户可以在生成时选择透明背景处理策略；
- 让生成模型专注于画面内容，让专用抠图模型负责 alpha 分离；
- 保留原始生成图，支持结果对比和仅重试抠图；
- 抠图失败时不丢失已经成功生成的原图；
- 兼容当前已保存的 `transparent_background` 数据。

## 2. 范围

### 包含

- Studio 生成表单增加透明背景处理模式；
- 后端支持“普通生成 + 抠图”的两阶段流水线；
- 持久化原图、最终图和抠图状态；
- 抠图失败后的单独重试；
- API、草稿、批次和历史结果的模式传递；
- 后端、前端和端到端测试。

### 不包含

- 自研图像分割或 alpha matte 算法；
- 手工画笔修边、局部擦除等编辑器功能；
- 自动判断哪一种模式质量更好的多路竞赛；
- 修改普通非透明背景生成的业务流程。

## 3. 用户体验

表单中的“生成透明背景图案”改为“透明背景处理方式”，提供：

- 不处理：不要求透明背景，使用用户选择的生图模型；
- 模型直接生成：保留现有行为，要求原生 alpha 输出，并继续使用透明背景专用模型；
- 生成后抠图：先使用用户选择的生图模型生成普通图，再调用抠图模型。

当选择“生成后抠图”时，不应强制把生图模型切换为 `gpt-image-2`，因为生图和抠图已经是两个职责不同的阶段。

结果卡片默认展示最终抠图结果，并提供查看原始生成图的入口。抠图处理中显示独立的后处理状态；抠图失败时显示原始图和“重试抠图”操作。

## 4. 数据模型与兼容性

新增规范化字段：

```text
transparent_background_mode: "none" | "native" | "removal"
```

其中：

- `none`：不处理透明背景；
- `native`：由生图模型直接生成透明背景；
- `removal`：普通图片生成完成后进行抠图。

现有 `transparent_background: true` 的记录读取时映射为 `native`；现有 `false` 映射为 `none`。写入新请求时优先使用新字段，旧客户端仍可使用旧布尔字段。

生成图片结果增加以下可选信息：

```text
original_image_url       // 普通生成结果或 native 结果的原始文件
image_url                // 当前默认展示的最终结果
background_removal_status: "not_requested" | "pending" | "succeeded" | "failed"
background_removal_model // removal 模式实际使用的模型或 provider
background_removal_error // 失败时的可读错误摘要，不包含密钥或内部 URL
```

`image_url` 继续作为现有消费者的默认字段，避免历史页面全部改为理解新字段。`original_image_url` 只在需要对比或重试时使用。

## 5. 后端处理链路

### 5.1 生成流程

对每个设计项执行：

1. 根据模式选择生图模型和提示词；
2. 生成图片并下载/规范化为服务端可处理的图片数据；
3. 先持久化原始生成图；
4. `none`：原图作为最终图；
5. `native`：原生透明图作为最终图；
6. `removal`：调用抠图 provider，校验 alpha 输出并持久化最终 PNG；
7. 返回最终图、原图引用和后处理状态。

现有的并行设计项生成可以保留。抠图调用应遵守独立的并发上限，避免批量生成时同时打满抠图 provider。该上限由配置控制，不在业务代码中硬编码不可调整的并发数。

### 5.2 抠图 provider 边界

抠图能力通过独立接口接入，例如：

```go
type BackgroundRemover interface {
    Remove(ctx context.Context, input ImageData) (*ImageData, error)
}
```

业务服务只负责输入、状态和持久化，不负责具体分割算法。优先复用现有 AI client/provider 配置机制，具体抠图实现使用成熟的开源模型服务或已有第三方 provider，保持 provider 可替换。

抠图输出必须满足：

- PNG 或等价的带 alpha 通道格式；
- 原始尺寸和主体位置保持不变；
- 不把棋盘格、白底或其他颜色伪装成透明；
- 能被现有图片存储和前端预览链路读取。

## 6. 状态、错误与重试

- 原图生成失败：设计项生成失败，沿用现有错误处理；
- 原图成功、抠图处理中：设计项为处理中，原图已经可恢复；
- 原图成功、抠图成功：最终图为抠图结果，状态为 `succeeded`；
- 原图成功、抠图失败：保留原图，状态为 `failed`，不自动伪装为成功的透明图；
- 重试抠图：只读取已保存的原图，再次调用 `BackgroundRemover`，不重新消耗生图调用；
- 抠图 provider 超时、限流和可重试错误使用有限次数退避；永久错误直接落库并返回可读状态。

原图和最终图的内部存储标识不能直接泄露给只接受公开 HTTPS URL 的第三方 provider；服务端应按现有上传图片处理约定读取字节并通过 provider 的受支持输入方式提交。

## 7. 代码边界

实现时优先沿用当前边界：

- `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-form-sections.tsx`：模式选择和模型联动；
- `web/listingkit-ui/src/lib/types/*studio*`：请求、草稿、批次和图片结果类型；
- `web/listingkit-ui/src/lib/api/*studio*`：新旧字段映射；
- `internal/listingkit/studio_designs.go`：模式化提示词和生图模型解析；
- `internal/listingkit/task_studio_media_service.go` 及其 support 文件：两阶段生成、抠图调用和结果持久化；
- 现有 batch/session/draft 存储代码：字段传递和历史数据回填。

不新增与现有图片服务平行的临时上传协议，也不把 provider 细节塞进 React 组件或 Studio 业务服务。

## 8. 测试策略

### 后端

- 模式解析：`none`、`native`、`removal` 及旧布尔字段兼容；
- `removal` 调用一次生图、一次抠图，并将抠图结果作为 `image_url`；
- `removal` 失败时保留 `original_image_url` 和失败状态；
- 重试只调用抠图 provider，不重复调用生图 provider；
- 校验无 alpha 或伪透明输出时按失败处理；
- 批量并发受抠图并发上限约束；
- 不同模式下的模型选择和提示词行为。

### 前端

- 三种模式正确提交和回填；
- `removal` 模式不强制切换生图模型；
- 原图/最终图切换展示；
- pending、succeeded、failed 状态展示；
- 重试按钮只提交抠图重试操作；
- 旧批次只含 `transparent_background` 时正确显示。

### 端到端

使用 fake image provider 验证完整链路：普通图生成、抠图、结果持久化、前端回填和失败重试。真实 provider 验证作为部署/验收阶段单独记录，不把 fake provider 结果当成真实质量验收。

## 9. 验收标准

- 用户可以明确选择“模型直接生成”或“生成后抠图”；
- 生成后抠图模式不会强制生图模型输出透明背景；
- 最终结果是真实 alpha PNG，而不是棋盘格或白底模拟；
- 抠图失败不会丢失原图，并可单独重试；
- 旧数据和旧客户端不发生破坏性变化；
- 测试覆盖成功、失败、重试和兼容路径。
