# ListingKit 项目重构路线图

## 文档状态

- 状态：Active
- 更新日期：2026-06-20
- 参考基线：`master` 近期 SDS/SHEIN 批次与提交实现
- 适用对象：技术负责人、后端、前端、QA、代码审查者

## 1. 重构目标

本轮重构的目标不是追求理想目录结构，而是降低真实业务演进成本，使 ListingKit 能够稳定支持：

- SDS-to-SHEIN 批量生产；
- 多平台 Listing 资料包；
- 可观察、可恢复、幂等的提交；
- 清晰的商品、资产、平台规则和外部集成边界；
- 可独立测试的核心业务逻辑。

最终判断标准是：

> 新能力能够在清晰模块中实现，失败能够定位到明确责任边界，平台扩展不需要继续把规则堆入 `internal/listingkit` 根包。

## 2. 核心策略

采用“能力牵引、小步迁移、兼容收缩”的策略：

```text
先解决生产链路中的真实风险；
在实现能力时切清对应边界；
保留兼容 facade，逐步迁出所有权；
每一步可测试、可回滚；
不进行一次性大搬家。
```

优先级比例原则：

```text
主要精力用于主链路稳定和可恢复性；
其余精力用于与当前能力直接相关的边界收口。
```

## 3. 当前架构判断

项目已经从通用 task processor 演进为跨境商品 Listing 自动化平台，包含：

- HTTP API 与运行时装配；
- Temporal / 队列 / Worker；
- GORM、Redis、RabbitMQ 和对象存储；
- OpenAI 和图片模型；
- 浏览器自动化和外部平台客户端；
- 多租户、权限和配置；
- ListingKit UI；
- SHEIN、Amazon、TEMU、Walmart 等平台流程。

当前主要问题不是功能不足，而是部分所有权仍然交叉。

### 3.1 `internal/listingkit` 仍是复杂度中心

根包同时承载或兼容：

- API-facing service；
- Studio 批次；
- SDS baseline；
- 工作流编排；
- 平台预览；
- 平台提交兼容；
- 设置和运行时协作者；
- 旧 Session 模型。

它应逐步收缩为产品编排和兼容 facade，而不是继续成为所有业务规则的默认位置。

### 3.2 Batch Graph 与 Session 双事实源

Studio 已有 durable `batch -> item -> attempt -> design` 图，但任务归属、旧草稿和部分 UI 状态仍依赖 `SheinStudioSession`。

风险包括：

- 页面显示新状态，任务创建读取旧状态；
- 无 Session 时任务关系无法刷新恢复；
- 重试或重复点击产生重复任务；
- 旧设计与已创建任务失去历史关系。

### 3.3 通用提交与平台规则仍需继续分层

通用提交状态机、attempt、幂等和恢复应属于 listing submission 领域；SHEIN 的图片上传、预校验、远端提交和平台错误映射应属于 SHEIN publishing 领域。

### 3.4 HTTP 和运行时装配仍可能拥有业务判断

`internal/app/httpapi` 和 runtime 包应负责装配、路由、鉴权上下文和生命周期，不应拥有类目、属性、兼容性、readiness 或发布资格规则。

### 3.5 外部客户端需要稳定适配层

SDS、SHEIN、AI、图片、对象存储和历史 management client 应通过小接口隐藏。核心业务不应依赖具体 SDK、HTTP payload 或浏览器实现。

## 4. 目标模块地图

目标结构是方向，不是一次性迁移清单。

```text
internal/app
  runtime/                 进程启动、依赖装配、生命周期
  httpapi/                 路由、handler、认证上下文、DTO 绑定

internal/catalog
  canonical/               标准商品事实
  product/                 商品身份、变体、规格和来源事实

internal/asset
  productimage/            商品图片和衍生资产
  design/                  设计稿、模板、蒙版和版本关系

internal/listing
  studio/                  通用 batch/item/attempt/design 机制
  task/                    Listing 任务领域
  readiness/               通用 readiness 结构与 blocker 契约
  submission/              通用提交 attempt、幂等、阶段和恢复

internal/marketplace/shein
  workspace/               SHEIN 人工审核和修复规则
  readiness/               SHEIN 提交资格和 blocker
  publishing/              SHEIN 草稿、发布和远端结果
  mapping/                 类目、属性、销售属性、SKU 和价格适配

internal/marketplace/amazon
internal/marketplace/temu
internal/marketplace/walmart

internal/integration
  sds/                     SDS API / 登录 / 浏览器适配
  shein/                   SHEIN 远端 client
  ai/                      文案和图片模型适配
  storage/                 对象存储适配

internal/listingkit
  facade/compatibility      产品 API facade、旧契约兼容
  orchestration             跨领域用例编排
```

如果现有代码已经使用 `internal/publishing/shein` 等路径，不要求立即改名。应先完成所有权迁移，再决定目录统一。

## 5. 边界规则

### 5.1 `internal/app/*`

允许：

- 初始化依赖；
- 注册路由；
- 构造 service；
- 管理进程生命周期；
- 注入 tenant/user context。

禁止：

- 平台业务规则；
- 商品字段映射；
- readiness 判定；
- 提交重试策略；
- Studio 兼容性规则。

### 5.2 `internal/listingkit`

允许：

- 产品级用例入口；
- 跨模块编排；
- 旧 API 和旧数据兼容；
- 迁移期 adapter。

禁止新增：

- SHEIN 专属类目、属性、图片、价格和发布规则；
- 通用提交状态机的第二套实现；
- 商品事实和图片资产的重复模型；
- 直接依赖外部 SDK 的核心规则。

### 5.3 `internal/listing/*`

拥有：

- 平台无关的 Listing 生命周期；
- Batch、Item、Attempt、Design 的通用机制；
- 提交 attempt、幂等、锁、事件和恢复；
- 通用 readiness / blocker 契约；
- durable task ownership。

### 5.4 `internal/marketplace/*`

拥有：

- 平台特定字段映射；
- 类目与属性规则；
- 平台图片、SKU、价格和提交限制；
- 远端错误到业务 reason code 的映射；
- 平台工作台修复规则。

### 5.5 `internal/catalog` 与 `internal/asset`

拥有平台无关的商品事实和资产。平台包可以引用它们，但不能反向依赖平台规则。

### 5.6 `internal/integration/*`

拥有外部协议和客户端适配。业务层只依赖窄接口，不依赖具体请求结构。

## 6. 重构工作流优先级

## Stream A：SDS 批量生产闭环

这是当前 P0，也是下一阶段所有边界调整的主要牵引。

需要完成：

```text
design x selection 正确 fan-out；
Batch 与 ListingKit task 的 durable link；
候选级幂等和并发去重；
baseline、store、ownership、compatibility 最终门禁；
严格 baseline 复用；
兼容指纹驱动的生成分组；
created/reused/rejected/failed 结构化结果；
真实 task/submission 状态投影；
已创建任务后的重生成保护；
批量提交部分成功。
```

边界收益：

- Batch Graph 成为任务创建事实源；
- Session 收缩为旧草稿兼容；
- Studio 通用机制与 SHEIN 规则开始分离；
- 提交状态与任务创建状态解耦。

## Stream B：Submission 所有权收口

目标：通用提交机制只保留一套。

迁移方向：

```text
internal/listing/submission
  -> attempt
  -> idempotency
  -> phase transition
  -> lock / retry / recovery
  -> event / persistence orchestration

internal/marketplace/shein/publishing
  -> prepare SHEIN payload
  -> upload SHEIN assets
  -> pre-validate
  -> save draft / publish
  -> map remote result and error
```

`internal/listingkit` 只保留提交用例入口和旧契约 adapter。

## Stream C：Studio 事实源统一

目标：新流程不再依赖 Session 持有 Batch 的核心业务状态。

顺序：

1. 新增 durable batch-task link；
2. Batch detail 优先读取 durable link；
3. 旧 Session CreatedTasks 回填 link；
4. 新任务创建停止写 Session 作为唯一来源；
5. Session 仅保留旧草稿读取和迁移；
6. 删除无调用的 session-centered API。

## Stream D：商品事实与资产迁移

当新增或修改商品、图片逻辑时，将平台无关部分迁入：

```text
internal/catalog/*
internal/asset/*
```

优先迁移高复用内容：

- 商品身份和变体；
- 图片角色和资产版本；
- design revision；
- 来源 trace；
- canonical product cache policy。

## Stream E：平台规则隔离

以 SHEIN 为模板，把以下规则迁出 ListingKit 根包：

- 类目解析；
- 属性和销售属性映射；
- 平台价格和 SKU；
- workspace repair target；
- readiness blocker；
- publishing payload 和错误映射。

完成 SHEIN 发布门槛后，再将同样边界应用到 TEMU、Amazon 和 Walmart。

## Stream F：运行时和外部集成解耦

逐步淘汰业务层对以下具体实现的直接依赖：

- management client；
- Gin context；
- GORM transaction 细节；
- 具体 AI SDK；
- 浏览器自动化对象；
- 外部平台原始 response。

先定义业务所需的小接口，再移动具体 adapter。

## Stream G：测试、CI 与可观测性

所有重构必须有以下安全网：

```text
go test ./... -count=1
npm run lint
npm run typecheck
npm test
npm run build
```

高风险并发模块增加：

```text
go test -race ./internal/listingkit ./internal/listing/studio ./internal/listing/submission
```

关键状态必须记录 tenant、batch、item、attempt、design、selection、candidate 和 task 标识。

## 7. 分阶段执行路线

## Phase 0：基线和守门规则

目标：确保后续迁移可度量、可回滚。

交付：

- 完整测试 baseline；
- 包列表和依赖图；
- project boundaries；
- CI 全量后端和前端测试；
- 禁止新增反向依赖的检查。

退出条件：

- baseline 可重复生成；
- 关键测试进入 CI；
- 新 PR 能说明模块归属。

## Phase 1：完成 SDS 批量生产闭环

目标：消除当前真实生产风险。

交付：

- 正确候选展开；
- durable task link；
- 最终门禁；
- 严格 baseline；
- 兼容分组；
- 状态投影；
- 重生成和部分提交保护。

退出条件：

- 真实 SDS-to-SHEIN 草稿通过；
- 重复请求不产生重复任务；
- Session-less Batch 可刷新恢复；
- 受控失败不阻断无关候选。

## Phase 2：收缩 ListingKit 根包

目标：只迁移已经通过 Phase 1 明确归属的逻辑。

执行方式：

- 每次迁移一个 use case；
- 保留兼容 facade；
- 先转调用，再删旧实现；
- 不把文件移动和行为修改混在同一个 PR。

优先对象：

```text
studio task ownership
submission orchestration
readiness taxonomy
SHEIN publishing adapters
```

退出条件：

- 根包不再拥有这些领域的核心状态机；
- 新能力无需修改多个兼容层；
- 包级测试可独立运行。

## Phase 3：平台边界固化

目标：形成可复制的 SHEIN marketplace 模板。

交付：

- SHEIN mapping / readiness / workspace / publishing 边界；
- 外部 SHEIN client adapter；
- 稳定平台 DTO 和错误代码；
- ListingKit facade 不持有 SHEIN 规则。

退出条件：

- SHEIN 规则可以在不启动 ListingKit HTTP 层的情况下测试；
- 平台包依赖 catalog/listing/asset，而不是反向依赖；
- TEMU/Amazon 可以复用通用 submission 和 readiness 契约。

## Phase 4：外部集成和运行时清理

目标：消除业务逻辑对基础设施的直接耦合。

交付：

- management client 退休计划；
- repository 和 integration ports；
- runtime 只做装配；
- 旧 debug 或 compatibility 入口清理。

退出条件：

- 核心业务测试不依赖真实外部服务；
- 替换客户端不需要修改领域逻辑；
- 进程启动代码不包含业务判断。

## Phase 5：多平台复制

前置条件：SHEIN 发布门槛通过。

目标：把稳定模式复制到 TEMU、Amazon 和 Walmart，而不是复制旧耦合。

每个平台先明确：

- 只生成资料包还是完整工作台；
- readiness 和 blocker；
- 保存草稿 / 发布能力；
- 失败恢复和运营入口；
- 哪些规则可以复用，哪些必须平台独有。

## 8. 迁移方法

### 8.1 Strangler / Facade

新实现放入目标模块，旧入口委托新实现；调用方迁移完成后再删除旧代码。

### 8.2 Additive API

先增加新字段和新状态，保持旧字段可读；前端切换完成后再淘汰旧语义。

### 8.3 双读单写

迁移期允许：

```text
优先读新事实源；
旧数据作为 fallback；
所有新写入只进入新事实源；
读取旧数据时顺手回填新模型。
```

避免两个事实源同时接受新写入。

### 8.4 数据回填

每个数据迁移必须具备：

- 幂等回填；
- tenant scope；
- 统计报告；
- unresolved 记录；
- 可重复执行；
- 回滚或禁用方案。

### 8.5 Feature Gate

对高风险行为切换使用显式配置或按租户灰度，不通过隐含条件改变生产路径。

## 9. PR 规则

每个重构 PR 应满足：

1. 只解决一个明确所有权问题；
2. 说明行为是否改变；
3. 提供迁移前后依赖关系；
4. 添加或迁移测试；
5. 不同时进行大规模重命名和业务修改；
6. 不引入新的平台规则到 `internal/listingkit`；
7. 包含回滚方式；
8. 更新相关边界或 checkpoint 文档。

推荐 PR 描述：

```text
Problem
Ownership before
Ownership after
Behavior change
Compatibility path
Tests
Rollback
Follow-up deletion
```

## 10. 停止条件

以下重构不应执行：

- 只是为了目录更整齐；
- 没有减少任何模块所有权；
- 迁移后仍需通过原包完成核心逻辑；
- 没有测试保护；
- 同时影响多个稳定业务链路但没有真实收益；
- 为未来假设平台提前建设复杂抽象。

出现以下信号时，应先切边界再继续加能力：

- 一个小功能需要修改多个无关模块；
- 同一状态或规则在多个包重复；
- 平台规则回流到 ListingKit；
- 业务测试必须启动 HTTP、数据库和远端服务；
- 修复一个平台问题影响其他平台；
- 团队只能通过全文搜索判断责任归属。

## 11. 风险与控制

| 风险 | 控制方式 |
| --- | --- |
| 大规模迁移破坏已跑通链路 | 小步 use-case 迁移、兼容 facade、真实验收报告。 |
| 新旧事实源不一致 | 双读单写、幂等回填、明确 source of truth。 |
| 重试产生重复远端动作 | candidate / submission 幂等键、唯一索引、恢复状态机。 |
| 平台规则被过度抽象 | 先实现 SHEIN 平台规则，再提取真正通用契约。 |
| 文件移动掩盖行为变更 | 拆分纯移动 PR 和行为 PR。 |
| 测试只覆盖 happy path | 并发、重启、部分失败、超时和真实接口验收。 |
| 旧批次无法读取 | 保留 legacy adapter，按读取回填新模型。 |

## 12. 重构度量

### 12.1 结构指标

- `internal/listingkit` 根包文件数和代码量；
- 平台规则位于目标平台包的比例；
- app/runtime 中业务规则数量；
- 跨领域反向依赖数量；
- legacy Session 新写入路径数量。

### 12.2 工程效率指标

- 新能力平均触碰包数量；
- 核心 use-case 单元测试启动成本；
- CI 时长和不稳定测试数量；
- 真实问题定位到责任模块的时间；
- 需要工程查日志才能恢复的运营问题比例。

### 12.3 可靠性指标

- 重复任务和重复提交数量；
- 孤儿 task / design / attempt 数量；
- 未知状态和空错误数量；
- 恢复后无需人工数据修复的比例；
- 批次部分失败后的继续完成率。

## 13. 近期执行顺序

```text
1. SDS candidate fan-out 正确性
2. durable batch-task ownership
3. candidate 幂等和并发保护
4. baseline/store/ownership 最终门禁
5. strict baseline reuse
6. compatibility-aware generation grouping
7. created/reused/rejected/failed 前后端契约
8. task/submission 真实状态投影
9. regeneration protection
10. partial batch submission
11. 真实环境验收
12. 根据上述实现收缩 ListingKit 根包
```

2026-06-21 执行记录：

| 步骤 | 状态 | 证据 |
| --- | --- | --- |
| 1-10 | 已完成代码层闭环 | 见 `docs/product/validation/runs/2026-06-21-shein-sds-batch-production-closure-regression.md`。 |
| 11. 真实环境验收 | pass | 见 `docs/product/validation/runs/2026-06-21-shein-sds-batch-production-closure.md`。store `870` 复测中真实 SDS fan-out、重复请求幂等、受控拒绝、SDS baseline warmup、readiness 清零和真实 SHEIN `save_draft` 均已验证。 |
| 12. 根据上述实现收缩 ListingKit 根包 | next | Phase 1 退出条件中的“真实 SDS-to-SHEIN 草稿通过”已关闭；下一步可按已验证边界收缩 ListingKit 根包。 |

2026-06-21 Phase 2 启动记录：

| 对象 | 状态 | 证据 |
| --- | --- | --- |
| readiness repair center 组装 | migrated | `internal/listingkit/shein_repair_center.go` 已收缩为 facade；去重、排序、section label、direct apply queue/session 组装迁入 `internal/marketplace/shein/workspace/repair_center_from_readiness.go`，并新增 marketplace 包级测试。 |
| readiness taxonomy 映射 | migrated | `internal/listingkit/shein_submit_readiness_checks_support.go` 中的 key -> taxonomy switch 已收缩为 facade；映射规则迁入 `internal/marketplace/shein/workspace/readiness_taxonomy.go`，并新增 marketplace 包级测试。 |
| SHEIN remote submit 动作分发 | migrated | `internal/listingkit/task_submission_execution_remote.go` 不再直接 switch `save_draft` / `publish` 调用远端 API；动作分发和 response summary 构造迁入 `internal/publishing/shein/submit_remote_action.go`，ListingKit 仅保留执行服务日志和 orchestration。 |
| SHEIN submit 翻译决策 | migrated | `internal/listingkit/task_submission_execution_product.go` 不再组合翻译缺失与区域目标语言规则；该判断迁入 `internal/publishing/shein/submit_prep.go` 的 `SubmitProductTranslationNeeded`，ListingKit 仅传入 task region 并决定是否构造 translate API。 |
| SHEIN submit supplier/publish payload policy | migrated | supplier code 派生和 publish 必需 SKC 图片校验迁入 `internal/publishing/shein/submit_payload_policy.go`；`internal/listingkit/shein_submit_payload_supplier_validation_support.go` 只保留兼容 wrapper。 |
| SHEIN submit image policy | migrated | submit product 深拷贝、图片 URL 计数、待上传计数、已上传/SDS URL 分类和 upload cache 清洗迁入 `internal/publishing/shein/submit_image_policy.go`；ListingKit 图片上传编排仍保留在根包。 |
| SHEIN submit payload transport normalization | migrated | submit payload 空集合、extra 默认值和传输字段补齐迁入 `internal/publishing/shein/submit_payload_normalize.go`；`internal/listingkit/shein_submit_payload.go` 保留提交准备顺序和兼容 wrapper。 |
| SHEIN submit site/SKU normalization | migrated | submit 站点默认值、仓库选择、SKU 库存/数量/尺寸/重量规范化迁入 `internal/publishing/shein/submit_site_sku_policy.go`；ListingKit 只把 `SheinSettings` 适配为 publishing-owned settings 并保留兼容 wrapper。 |
| SHEIN submit image upload orchestration | migrated | submit 图片上传去重、cache key、并发 job、color-block fallback 迁入 `internal/publishing/shein/submit_image_upload.go`；ListingKit 仅注入现有 color-block builder 与 SHEIN image API。 |
| SHEIN submit payload image normalization | migrated | submit SPU/SKC/SKU 图片类型、排序、去重、square/color-block、site detail image 组装迁入 `internal/publishing/shein/submit_payload_images.go`；ListingKit 图片 payload support 只保留兼容 wrapper。 |
| SHEIN studio submit SKU pricing references | migrated | supplier SKU rename 后的手工价格覆盖、SKU price 引用 remap，以及 stale task/request pricing alias reconcile 迁入 `internal/publishing/shein/submit_sku_pricing.go`；ListingKit pricing support 只保留兼容 wrapper。 |
| SHEIN studio submit SKU style tokens | migrated | submit task/request discriminator、style suffix 推导、token classifier 和 discriminator 组合迁入 `internal/publishing/shein/submit_sku_style.go`；ListingKit style support 只保留从 `Task` 取值和兼容 wrapper。 |
| SHEIN studio submit SKU variant rules | migrated | SDS variant 匹配、base SKU 推导、variant discriminator、旧 SKU 反推与是否需要 discriminator 的规则迁入 `internal/publishing/shein/submit_sku_variant.go`；ListingKit variant support 仅适配 `SDSSyncOptions` 到 publishing-owned input。 |
| SHEIN studio submit supplier SKU normalization flow | migrated | studio supplier SKU 主流程、DraftPayload/SkcList/PreviewPayload 同步更新、rename 收集与 pricing reconcile 编排迁入 `internal/publishing/shein/submit_sku_normalization.go`；ListingKit 只从 `Task` 组装 style/discriminator/variant context。 |
| SHEIN submit state transitions | migrated | begin/advance/complete/fail attempt 状态转移、lease 刷新、closeout event 构造迁入 `internal/publishing/shein/submission_state.go`；ListingKit 仅保留兼容 wrapper 并注入统一 TTL。 |
| SHEIN submit sensitive-word retry | migrated | publish validation notes 触发的敏感词清理、retry event 追加和重试响应错误归一迁入 `internal/publishing/shein/submit_sensitive_retry.go`；ListingKit 仅注入现有远端执行函数。 |
| SHEIN submit source-facts readiness | migrated | 1688 来源事实复核规则抽到底层 `internal/listing/sourcefacts`，并由 SHEIN workspace 暴露 `SourceFactsReady`；ListingKit readiness checks 不再直接依赖 `internal/listing/submission`。 |
| SHEIN submit final draft confirmation | migrated | submit 请求确认最终草稿时的 FinalSubmissionDraft 初始化、Confirmed/时间戳/SubmitMode 写入迁入 `internal/publishing/shein/final_draft_submit.go`；ListingKit 仅判断请求是否携带 ConfirmedFinal。 |
| SHEIN submit readiness status predicates | migrated | submit readiness 使用的 SKU、最终图片、SKC/色块图、价格和图片存在性判定迁入 `internal/publishing/shein/submit_readiness_status.go`；ListingKit status support 仅保留兼容 wrapper。 |
| SHEIN submit image upload cache persistence | migrated | 图片上传后的 FinalSubmissionDraft cache 写入和更新时间迁入 `internal/publishing/shein/submit_image_upload_cache.go`；ListingKit 上传服务仅负责 API/runtime orchestration。 |
| SHEIN final draft image application | migrated | 最终图片排序、删除、角色覆盖、SKC/SKU fallback 和 preview SKC image 回写迁入 `internal/publishing/shein/final_draft_images.go`；ListingKit final draft 文件仅保留兼容 wrapper。 |
| SHEIN pricing cache review reconcile | migrated | 缓存命中的旧 SKU/manual price override 按当前 DraftPayload/source_sds_sku 重映射的规则迁入 `internal/publishing/shein/pricing_cache_reconcile.go`；ListingKit pricing cache loader 只负责读取缓存并委托 publishing 规则。 |
| SHEIN pricing cache identity/applicability | migrated | pricing cache key、source identity、SKU facts、SKU alias、review normalization/applicability/decode/clone 迁入 `internal/publishing/shein/pricing_cache_identity.go`；ListingKit pricing cache support 仅保留兼容 wrapper、cache store 读写和日志。 |
| SHEIN revision patch application | migrated | editor revision 的 category/attribute/sale attribute/SKC/SKU patch 应用规则迁入 `internal/marketplace/shein/workspace/revision_apply_patch.go`；ListingKit revision apply support 仅保留兼容 wrapper 和整体 apply 编排。 |
| SHEIN submit freshness readiness checks | migrated | 在线登录态、类目模板、普通属性模板和销售属性 freshness readiness check 的 key/文案/field path 构造迁入 `internal/marketplace/shein/workspace/submit_freshness_readiness.go`；ListingKit freshness flow 仅保留 API 调用、任务上下文和结果持久化。 |
| SHEIN SDS image matching helpers | migrated | SDS mockup 到 ImageSet 转换、SKU/color key 归一、source_sds_sku/SupplierSKU 匹配和 image set merge 迁入 `internal/publishing/shein/sds_images.go`；ListingKit SDS 图片流程仅保留请求 DTO 适配与编排 wrapper。 |
| SHEIN studio variant image matching | migrated | AI 生成的 variant image set 按 source_sds_sku/SupplierSKU/SKC color 匹配 DraftPayload/SkcList 的规则迁入 `internal/publishing/shein/variant_image_sets.go`；ListingKit 仅把前端 `SheinStudioVariantImageSet` DTO 归一为 publishing-owned `VariantImageSet`。 |
| SHEIN variant image coverage guard | migrated | 多 SKC 共享单图的 coverage 阻断判断、SKC group/main image 计数和 metadata 状态读写迁入 `internal/publishing/shein/variant_image_coverage.go`；ListingKit 仅从前端/SDS 输入计算可用变体图片组数并委托 publishing 规则。 |
| SHEIN studio AI product image application | migrated | AI 商品图替换/追加、ImageSet 构造、DraftPayload/SkcList/PreviewPayload 写回迁入 `internal/publishing/shein/studio_ai_images.go`；ListingKit Studio 图片流程仅保留请求策略、source image 聚合和兼容 wrapper。 |
| SHEIN studio size reference image application | migrated | size reference 图片追加、preview Product/SKC size-map 标记和 `ImageType=6`/`SizeImgFlag` 写回迁入 `internal/publishing/shein/studio_size_reference_images.go`；ListingKit 只保留前端/SDS 尺寸图解析与兼容 wrapper。 |
| SHEIN size reference rendered resolution | migrated | raw size reference 与 SDS source/rendered mockup 按位置匹配、variant summary 按 ID/SKU/color 匹配规则迁入 `internal/publishing/shein/size_reference_resolution.go`；ListingKit 只负责从 `GenerateRequest`/`SDSSyncSummary` 适配输入。 |
| SHEIN Studio image compatibility cleanup | migrated | 删除迁移后未调用的 ListingKit 私有 wrapper/dead helper，包括旧 coverage group/main-image helper、clear shared SKC image helper、ImageDraftToSet wrapper 和 size reference detail wrapper。 |

在这条路径完成之前，不启动新的大规模多平台工作台建设，也不进行无业务牵引的目录级重构。

## 14. 完成定义

项目重构不是以“文件全部搬完”为完成标准，而是以以下结果为准：

```text
每个核心领域有一个明确事实源；
ListingKit 根包主要承担 facade 和 orchestration；
平台规则位于平台边界；
通用提交和 Studio 机制可独立测试；
外部客户端可替换；
真实失败能够安全恢复；
新增平台不需要复制已有状态机和技术债。
```

## 15. 相关文档

- `docs/product/listingkit-project-goals.md`
- `docs/architecture/project-boundaries.md`
- `docs/architecture/next-steps.md`
- `docs/refactoring/project-wide-refactoring-plan.md`
- `docs/refactoring/listingkit-boundary-checkpoint.md`
- `docs/product/listingkit-next-execution-plan.md`
- `docs/superpowers/specs/2026-06-20-listingkit-sds-batch-production-closure-requirements.md`
- `docs/superpowers/plans/2026-06-20-listingkit-sds-batch-production-closure.md`
