# ListingKit 下一阶段执行计划

## 目的

这份计划把 ListingKit 接下来一段时间的工作从“方向性 roadmap”落成可执行的阶段、交付物和验收标准。

它补充而不是替代：

- [`listingkit-product-roadmap.md`](./listingkit-product-roadmap.md)
- [`listingkit-product-overview.md`](./listingkit-product-overview.md)
- [`../architecture/project-boundaries.md`](../architecture/project-boundaries.md)
- [`../refactoring/project-wide-refactoring-plan.md`](../refactoring/project-wide-refactoring-plan.md)

当后续具体实现与本计划冲突时，按以下优先级判断：

1. 真实 SHEIN 主链路稳定性。
2. 运营人员是否能独立理解、修复和继续任务。
3. 是否减少工程介入和重复查日志。
4. 是否让模块边界更清晰，而不是继续扩大 `internal/listingkit` 的复杂度。
5. 是否能沉淀为 TEMU / Amazon / Walmart 后续复用模式。

## 总原则

### 1. 能力牵引重构

下一阶段不做一次性大规模目录迁移，也不继续无边界堆功能。

每个关键能力都应顺手切清一块边界：

- 提交状态机推动 `internal/listing/submission` 或迁移期等价包成型。
- SHEIN 远端提交规则进入 `internal/marketplace/shein/publishing`。
- SHEIN 工作台、检查、修复和 blocker 规则进入 `internal/marketplace/shein/workspace`。
- `internal/listingkit` 保持为兼容 facade、编排层和 API-facing shell。
- `internal/app/httpapi` 只做路由、handler 和依赖装配，不承载业务规则。

### 2. 先稳 SHEIN，再扩平台

SHEIN 是当前最完整的产品闭环。TEMU、Amazon、Walmart 不应在 SHEIN 的状态、失败恢复、readiness blocker、提交幂等和批量运营模式沉淀前进入完整工作台扩展。

### 3. 先让运营能独立完成，再追求更多自动化

P0 目标不是多生成几个字段，而是保证一个运营人员能独立完成 SHEIN 任务：

```text
选择来源素材或 SDS 商品
  -> 创建 ListingKit 任务
  -> 等待异步生成
  -> 进入工作台审核
  -> 修复阻断项
  -> 确认最终稿
  -> 保存 SHEIN 草稿或发布
  -> 理解提交结果或失败原因
```

## 阶段总览

| 阶段 | 建议周期 | 主题 | 核心产出 |
| --- | --- | --- | --- |
| 0 | 2-3 天 | 安全网和边界基线 | 测试 / 包 / 依赖 baseline，边界规则确认 |
| 1 | 1 周 | 真实接口验收 + 错误恢复 SOP | 验收报告模板、真实运行报告、错误恢复手册 |
| 2 | 2-3 周 | SHEIN 提交状态机 + 幂等 | SubmitAttempt、提交阶段、幂等、状态查询、UI 展示 |
| 3 | 1-2 周 | readiness blocker taxonomy | 稳定 blocker 分类、修复跳转、unknown 兜底 |
| 4 | 2 周 | 任务列表 / 队列页运营化 | 筛选、next action、Review / Retry / Inspect、失败复盘 |
| 5 | 1 周 | 设置页配置健康检查 | AI / SHEIN / SDS / 图片 / 价格配置体检 |
| 6 | 1-2 周 | SHEIN Studio 批量效率 | 部分成功处理、失败项重试、批量结果回流 |
| 7 | SHEIN 稳定后 | TEMU / Amazon / Walmart 产品化 | 平台边界和产品流程定义 |

## P0：SHEIN 主链路稳定

### Epic 1：真实接口验收沉淀

目标：每轮真实任务联调都能沉淀成可复盘的验收证据。

交付物：

- `docs/product/validation/listingkit-real-api-validation-report-template.md`
- `docs/product/validation/runs/README.md`
- 至少 1 条成功路径报告。
- 至少 1 条失败路径报告。
- 未知状态和未知 blocker key 清单。

验收标准：

- 至少一条真实任务从创建到保存草稿或发布完整通过。
- 至少一条失败路径能被用户从 UI 理解并恢复。
- 没有未知任务状态。
- 没有未知 readiness blocker key；如果仍存在，必须进入待关闭清单。
- 工程收到问题时能拿到 task_id、接口响应和页面表现。

建议任务：

```text
LK-NEXT-001 建立真实接口验收报告模板
LK-NEXT-002 建立 validation runs 目录和记录规范
LK-NEXT-003 跑通 1 条 SDS -> SHEIN 草稿成功路径
LK-NEXT-004 跑通 1 条失败恢复路径
LK-NEXT-005 建立 unknown state / unknown blocker 待关闭清单
```

### Epic 2：错误恢复 SOP

目标：失败后运营知道下一步，而不是只能找工程查日志。

交付物：

- `docs/product/ops/listingkit-error-recovery-sop.md`
- 错误场景矩阵。
- 运营可处理动作和工程介入边界。
- QA 可主动触发的失败样例。

优先覆盖场景：

- 图片上传失败。
- SDS 同步失败。
- 类目解析失败。
- 属性缺失。
- workspace 缺数据。
- 保存草稿失败。
- 发布失败。
- SHEIN 远端校验失败。
- AI 生成失败。
- 图片模型配置缺失。
- 店铺 token / 权限失效。

验收标准：

- 运营看到失败后知道下一步去哪。
- QA 能按 SOP 主动触发至少一个失败样例。
- 工程收到问题时能拿到 task_id、接口响应、页面表现和失败阶段。

建议任务：

```text
LK-NEXT-006 建立错误恢复 SOP 初版
LK-NEXT-007 补图片 / SDS / 类目 / 属性失败恢复说明
LK-NEXT-008 补保存草稿 / 发布 / 远端校验失败恢复说明
LK-NEXT-009 建立运营可处理与工程介入边界
LK-NEXT-010 建立失败样例清单
```

### Epic 3：SHEIN 提交状态机 + 幂等保护

目标：保存草稿 / 发布过程可见、可重试、可恢复，并且不会重复调用 SHEIN 远端接口。

建议状态模型：

```text
SubmitAttempt
SubmitAction
SubmitPhase
SubmitStatus
SubmitError
SubmitRemoteRecord
SubmitIdempotencyKey
```

建议字段：

```text
attempt_id
task_id
tenant_id
target_platform
action: save_draft / publish
status: pending / running / succeeded / failed / recovering
phase: validate / prepare_product / upload_images / pre_validate / submit_remote / persist_result
idempotency_key
remote_product_id
remote_draft_id
remote_publish_id
error_code
error_message
recoverable
created_at
updated_at
finished_at
```

建议阶段：

```text
validate
prepare_product
upload_images
pre_validate
submit_remote
persist_result
```

模块边界目标：

```text
internal/listing/submission
  通用提交状态机、attempt、幂等、阶段流转、恢复入口

internal/marketplace/shein/publishing
  SHEIN 远端提交、图片上传、pre-validate、保存草稿、发布规则

internal/listingkit
  只保留提交入口 facade，负责兼容旧 API 和编排

internal/app/httpapi
  只做 handler / route / service wiring
```

验收标准：

- 保存草稿或发布进行中时，UI 能显示当前阶段。
- 失败后能看到失败阶段和原因。
- 同一 idempotency key 重放不会重复提交。
- 同任务同动作并发提交不会重复调用 SHEIN 远端接口。
- 至少一条真实任务保存草稿通过。
- 至少一条真实任务发布通过。
- 至少一个失败阶段能被恢复或明确提示不可恢复。

建议任务：

```text
LK-NEXT-011 设计 SubmitAttempt / SubmitPhase 数据模型
LK-NEXT-012 实现提交 attempt 创建和状态流转
LK-NEXT-013 实现 idempotency key 检查和并发提交保护
LK-NEXT-014 抽通用 submission 包或迁移期 facade
LK-NEXT-015 收敛 SHEIN 远端提交规则到 marketplace/shein/publishing
LK-NEXT-016 提供提交状态查询接口
LK-NEXT-017 UI 展示提交阶段、失败原因和下一步动作
LK-NEXT-018 真实任务验证保存草稿、发布和重复点击保护
```

### Epic 4：readiness blocker taxonomy

目标：所有阻断项都能稳定映射到修复动作；未知 blocker 不会让用户无路可走。

建议 taxonomy 字段：

```text
blocker_key
severity: blocker / warning / info
domain: category / attribute / sale_attribute / image / price / sku / store / remote / system
repair_target
repair_route
message_template
recoverable
requires_engineering
```

建议修复映射：

```text
missing_category -> 类目修复区
missing_required_attribute -> 普通属性区
missing_sale_attribute -> 销售属性区
image_upload_failed -> 图片区
price_invalid -> 价格区
sku_invalid -> SKU 区
shein_remote_validation_failed -> 提交报告区
unknown -> 阻断项详情兜底区
```

验收标准：

- 所有已知 blocker 都能跳到明确修复区域。
- 未知 blocker 有可理解兜底。
- 未知 blocker 数量可统计。
- 新增 blocker 必须补 taxonomy。
- readiness 一次通过率可以开始统计。

建议任务：

```text
LK-NEXT-019 建立 blocker taxonomy 初版
LK-NEXT-020 后端稳定输出 blocker key / domain / repair target
LK-NEXT-021 前端补 blocker 跳转映射
LK-NEXT-022 unknown blocker 兜底展示和记录
LK-NEXT-023 将 unknown blocker 纳入真实接口验收报告
```

## P1：运营效率和诊断能力

### Epic 5：任务列表 / 队列页运营化

目标：任务列表成为“恢复和继续工作”的工作台，队列页能支持运营负责人排查卡点。

任务列表建议筛选：

```text
平台：SHEIN / Amazon / TEMU / Walmart
来源：SDS / 1688 / manual / image
任务状态：processing / needs_review / failed / completed
readiness 状态：ready / blocked / warning
提交状态：not_submitted / submitting / draft_saved / published / publish_failed
阻断类型：category / attribute / image / price / sku / remote
更新时间
创建时间
店铺
批次
```

任务行 next action：

```text
继续生成
进入工作台
修复阻断项
查看提交状态
重试保存草稿
重试发布
查看失败原因
进入队列排查
```

队列页动作语义：

```text
Review：人工检查资料或阻断项
Retry：可重试的系统动作
Inspect：需要查看详情或工程介入
```

验收标准：

- 运营能从列表快速找到当天失败任务。
- 运营能从列表快速找到可提交任务。
- 不需要复制 task_id 到其他页面才能继续处理。
- 运营负责人能用队列页判断任务卡住原因。
- 队列页能支持每日失败任务复盘。

建议任务：

```text
LK-NEXT-024 任务列表增加运营筛选
LK-NEXT-025 任务行增加 next action
LK-NEXT-026 任务列表支持失败任务和可提交任务快捷入口
LK-NEXT-027 队列页定义 Review / Retry / Inspect
LK-NEXT-028 队列页增加失败复盘字段
```

### Epic 6：设置页配置健康检查

目标：配置问题前置暴露，不要等任务失败后才知道 token、权限、模型或 SDS 登录态有问题。

健康检查范围：

```text
AI 模型配置
SHEIN 店铺连接
SDS 连接
图片处理配置
价格规则
对象存储
队列 / Worker
```

影响范围示例：

```text
AI client 不可用 -> 标题、卖点、描述、属性改写失败
SHEIN token 失效 -> 保存草稿和发布失败
SDS 登录态失效 -> SDS 商品库和 Studio 创建任务失败
图片模型缺失 -> 主图、白底图、mockup 生成失败
价格规则缺失 -> readiness price blocker
```

验收标准：

- 新建任务前可以确认关键配置是否可用。
- 配置错误能在设置页暴露。
- 任务创建前能提示关键配置风险。

建议任务：

```text
LK-NEXT-029 设计配置健康检查接口
LK-NEXT-030 实现 AI / SHEIN / SDS / 图片 / 价格健康检查
LK-NEXT-031 设置页展示配置状态和影响范围
LK-NEXT-032 任务创建前展示关键配置风险
```

### Epic 7：SHEIN Studio 批量任务效率

目标：一批任务里部分失败不阻断其他可提交任务，运营能按状态分组处理。

批量状态分组：

```text
可提交
需修复
处理中
生成失败
提交失败
已保存草稿
已发布
```

验收标准：

- 一批任务中部分失败不会阻断其他可提交任务。
- 用户能清楚知道哪些任务需要单独进入工作台。
- 批量保存草稿失败时可以只重试失败项。
- 成功项不会重复提交。

建议任务：

```text
LK-NEXT-033 批量任务按状态分组
LK-NEXT-034 支持部分成功继续提交
LK-NEXT-035 支持失败项单独重试
LK-NEXT-036 批量结果回流任务列表
LK-NEXT-037 输出批量保存草稿 / 发布报告
```

## P2：扩展平台产品化

在 SHEIN 主链路稳定后，再启动其他平台产品化定义。

建议任务：

```text
LK-NEXT-038 TEMU 产品流程定义
LK-NEXT-039 Amazon ListingKit 边界定义
LK-NEXT-040 Walmart 产品化评估
```

要先回答：

- 该平台是只生成资料包，还是提供完整工作台？
- 是否支持人工审核？
- 是否支持保存草稿或发布？
- blocker 如何定义？
- 失败后如何恢复？
- 哪些能力复用 SHEIN？
- 哪些能力平台独有？

## 建议 10 周排期

| 周期 | 工作重点 | 产出 |
| --- | --- | --- |
| 第 1 周 | baseline、验收模板、SOP 骨架 | 测试 / 包 / 依赖 baseline，验收模板，SOP 初版 |
| 第 2 周 | 真实链路闭环验证 | 1 条成功报告，1 条失败报告，unknown 清单 |
| 第 3-4 周 | SHEIN 提交状态机后端 | attempt、phase、idempotency、状态查询、模块边界收敛 |
| 第 5 周 | 提交状态机 UI + 真实验证 | UI 阶段展示，真实保存草稿 / 发布验证 |
| 第 6 周 | readiness blocker taxonomy | blocker 分类、修复跳转、unknown 兜底 |
| 第 7-8 周 | 任务列表和队列页运营化 | 筛选、next action、Review / Retry / Inspect、失败复盘 |
| 第 9 周 | 设置页健康检查 | AI / SHEIN / SDS / 图片 / 价格体检 |
| 第 10 周 | SHEIN Studio 批量效率 | 状态分组、部分成功、失败项重试、批量报告 |

## 每阶段必须记录的指标

建议从第一阶段开始记录：

```text
从选品到任务创建耗时
从任务完成到进入工作台耗时
从任务完成到可提交耗时
readiness 一次通过率
平均每任务修复次数
保存草稿成功率
发布成功率
失败后无需工程介入恢复率
未知状态数量
未知 blocker key 数量
```

## 暂缓事项

下一阶段暂缓：

- 一次性大规模目录迁移。
- 在 SHEIN 稳定前做完整 TEMU / Amazon / Walmart 工作台。
- 继续把平台规则塞进 root `internal/listingkit`。
- 只优化 AI 文案生成但不补审核、修复、提交、恢复、批量运营闭环。

## Definition of Done

本计划完成时，应满足：

- 一个运营人员可以独立完成 SHEIN 任务，不需要工程解释状态。
- 保存草稿 / 发布过程可见、幂等、可恢复。
- blocker 都能指向修复区域，unknown blocker 可记录和关闭。
- 任务列表和队列页能支撑日常失败任务排查。
- 设置页能在任务失败前暴露关键配置问题。
- 批量任务可以处理部分成功、部分失败、部分需修复。
- 新增能力没有继续扩大 root `internal/listingkit` 的业务规则复杂度。
