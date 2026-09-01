# 阶段三产品域目标架构设计

**状态：** 已批准设计，等待实施计划

**日期：** 2026-09-01

**基线：** `origin/main` @ `c638c53f2864f65741174d7b4b2bd5550bbf7a60`
**前置阶段：** `docs/superpowers/plans/2026-08-30-internal-target-architecture-phase2.md`

## 1. 背景

阶段二已经把配置、日志、数据库、Redis、RabbitMQ、worker pool、Temporal、Feature Flag、可观测性和外部 Provider Adapter 收敛到 `internal/app`、`internal/platform`、`internal/integration` 与 `internal/shared` 的明确所有者。阶段三负责把仍分散在旧根目录中的产品事实、来源、丰富化、资产和图片能力归入产品域。

当前产品相关代码存在以下结构性缺陷：

1. `internal/productenrich` 同时承担来源数据归一化、产品校验、AI 评分、任务状态、Queue、Worker、HTTP API 和持久化，领域能力与运行时编排没有边界。
2. `internal/productimage` 同时承担图片能力、任务状态、Queue、Worker、HTTP API、人工审核和 Provider 接线；与此同时，ImageAgent 已经拥有预算、计划、重试、恢复、审批和 Temporal 工作流，形成两个图片编排所有者。
3. `internal/asset` 反向导入 `internal/productimage`，导致资产事实层依赖图片执行实现。
4. `internal/asset/repository`、`internal/productenrich/store` 和 `internal/productimage/store` 直接持有 GORM，实现细节位于领域包内部。
5. ListingKit、SDS 与未投产的 AmazonListing 直接调用旧 ProductEnrich/ProductImage Service，使未采用的兼容路径持续约束新架构。
6. 旧 ProductEnrich/ProductImage 在数据库或 Redis 缺失时可回退到内存实现，生产装配错误可能被静默隐藏。

本设计不以兼容旧 ProductEnrich、ProductImage、AmazonListing 或 ListingKit 编排接口为目标。目标是建立唯一、明确、可自动验证的产品域和图片工作流所有权。

## 2. 已批准决策

### 2.1 单 PR 硬切

阶段三使用同一分支、同一最终 PR 完成目标架构切换。实施过程可以拆成多个可测试提交，但合并结果必须一次性删除旧产品根目录；主分支不得保留转发包、Deprecated Facade 或双写实现。

### 2.2 ProductEnrich 退化为领域能力

保留丰富化、校验、评分和 Proposal 生成能力，删除 ProductEnrich 的 Task、Queue、Worker、Redis fallback、HTTP Task API 与 GORM Task Repository。

阶段三不实现 ProductAgent。`internal/product/enrichment` 只提供可注入、可测试、无持久化副作用的 Proposal 能力，未来由 ProductAgent 决定何时调用、如何预算以及是否应用结果。

### 2.3 ImageAgent 是唯一图片工作流

ImageAgent 是产品图片计划、预算、执行、重试、恢复、审批和外部副作用状态的唯一所有者。删除 ProductImage 的 Task、Queue、Worker、HTTP Task API、GORM Task Repository 及 ListingKit/SDS/AmazonListing 对旧 ProductImage Service 的调用。

`internal/product/image` 只保留图片分析与生成能力。它不拥有任务、工作流、审批状态或资产持久化。

### 2.4 不让兼容调用者决定新架构

ListingKit、SDS 和 AmazonListing 不再触发产品丰富化或产品图片工作流。它们只能通过本地读取 Port 获取规范产品事实与已批准资产。没有已批准资产时必须返回明确的未就绪结果，不得回退旧 ProductImage、自动生成、来源图或“第一张图”。

### 2.5 不在本阶段物理删表

阶段三停止读写并停止装配 `product_enrich_tasks` 与 `product_image_tasks`，但不在架构 PR 中执行不可恢复的物理删表。物理删除必须在单独的数据保留决策中明确备份、保留周期、执行窗口和回滚方式。

## 3. 目标目录

```text
internal/product/
├── catalog/                # 规范产品事实
├── sourcing/               # 来源身份、SourceEnvelope、lineage、warnings
├── enrichment/             # 丰富化、校验、评分、Proposal 能力
├── asset/                  # 资产事实、清单、策略、配方、Repository Port
└── image/                  # 抠图、白底、场景、审核等图片能力 Port

internal/imageagent/
├── httpapi/                # 唯一图片任务 API
├── temporal/               # 唯一图片工作流
├── tools/                  # 调用 product/image 能力
└── store/                  # ImageAgent 运行状态；目录治理属于 Agent 阶段

internal/integration/
├── persistence/product/asset/   # 产品资产 GORM Adapter
├── crawler/a1688/
├── openai/
├── grsai/
├── httpimage/
└── s3/

internal/app/
├── httpapi/                # 模块装配
├── worker/imageagent/      # ImageAgent Worker 装配
└── schema/                 # 数据库迁移执行
```

最终必须删除：

- `internal/catalog`
- `internal/asset`
- `internal/imageasset`
- `internal/productenrich`
- `internal/productimage`

`internal/pipeline` 不属于产品域。代码证据表明其生产消费者几乎全部位于 TEMU/Marketplace；阶段二清单把它归入产品阶段的判断不准确。阶段三只冻结其增长，阶段四再按 Marketplace 所有权拆分。

## 4. 所有权和依赖规则

### 4.1 产品域

`internal/product/*` 可以依赖标准库、明确批准的第三方纯库、`internal/shared` 以及同一产品域中依赖方向更底层的包。它不得导入：

- `internal/app`
- `internal/platform`
- `internal/integration`
- GORM
- Temporal SDK
- Redis 或 RabbitMQ Client
- OpenAI、GRSAI、S3 等 Provider SDK 或具体 Adapter

依赖方向为：

```text
product/sourcing ──> product/catalog
product/enrichment ──> product/catalog + product/sourcing
product/image ──> product/catalog + product/asset
product/asset ──> product/catalog
```

`product/asset` 不得导入 `product/image`。图片候选转换为已批准资产的映射发生在 ImageAgent Tool 或应用适配层。

### 4.2 ImageAgent

ImageAgent 可以通过明确 Port 调用 `product/image` 能力、读取规范产品事实并提交已批准资产。ImageAgent 继续拥有：

- Run、Plan、Slot 和 Revision
- Budget 与 Usage Quote/Receipt
- Temporal 历史和恢复
- Provider dispatch 状态
- Staging、Publication 与 Effect lifecycle
- 人工审批、取消与重试命令

阶段三只修改 ImageAgent 与产品能力之间的边界，不重排 `imageagent/store` 或 `imageagent/temporal`；这些目录由 Agent 阶段治理。

### 4.3 Integration

具体 Provider、Crawler、对象存储和 GORM Adapter 位于 `internal/integration`。Adapter 实现消费方定义的窄接口；不得把 Provider SDK 类型暴露给产品领域或 ImageAgent 公共模型。

### 4.4 App

`internal/app` 只负责装配：

- ImageAgent Tool 与 `product/image` 能力
- ImageAgent Publisher 与 S3 Adapter
- ImageAgent Asset Catalog 与 `product/asset.Repository`
- 产品领域需要的 Provider-neutral Port 与具体 Integration Adapter

缺少生产依赖时装配必须失败。内存 Repository 只能由测试显式创建。

## 5. 领域能力

### 5.1 Sourcing

来源适配器输出 `product/sourcing.SourceEnvelope`。Envelope 必须保留：

- 来源类型和稳定来源身份
- 原始证据引用
- lineage
- warnings
- 获取时间和可验证的来源元数据

Sourcing 不调用 ImageAgent、ListingKit 或 Marketplace 服务，也不生成平台发布 Payload。

### 5.2 Catalog

`product/catalog.ProductSnapshot` 是规范产品事实。Catalog 不包含工作流状态、Provider 响应或 ListingKit UI 状态。

来源 Envelope 经确定性归一化后产生 ProductSnapshot。相同有效输入必须产生等价事实；警告和证据不能在归一化时丢失。

### 5.3 Enrichment

丰富化能力的核心契约为：

```go
type Proposer interface {
	Propose(context.Context, Request) (Proposal, error)
}
```

`Request` 包含 ProductSnapshot、SourceEnvelope 中允许使用的证据以及显式策略快照。`Proposal` 包含：

- 建议字段变更
- 每项建议的证据
- 质量评分
- 验证结果
- 可解释的警告和拒绝原因

Proposer 不写 ProductSnapshot、不创建任务、不持久化 Proposal、不选择 Provider，也不决定重试。未来 ProductAgent 负责调用和应用决策。

### 5.4 Image

`product/image` 定义图片领域模型和窄能力接口，例如 SubjectExtractor、WhiteBackgroundRenderer、SceneRenderer 与 ImageReviewer。具体模型调用由 Integration Adapter 实现并通过 App 注入。

图片能力返回候选图片及可验证元数据，不创建 ImageAgent Run、不写资产 Repository、不发布到对象存储，也不决定重试。

### 5.5 Asset

`product/asset` 保存产品资产事实、类型、角色、血缘、审批来源和清单规则。Repository Port 只暴露产品资产所需操作；GORM 实现位于 `internal/integration/persistence/product/asset`。

批准写入的幂等身份至少包含 tenant、ImageAgent run、plan revision、slot、attempt 和批准动作。重复审批或恢复不得生成重复资产记录。

## 6. 数据流

### 6.1 产品事实和丰富化

```text
integration/crawler/*
        ↓
product/sourcing.SourceEnvelope
        ↓
product/catalog.ProductSnapshot
        ↓
product/enrichment.Proposer
        ↓
EnrichmentProposal
        ↓
ProductAgent（未来）进行预算、人工确认与应用
```

阶段三不会为了调用 Proposer 新建临时 Scheduler、Queue、Worker 或 Agent。

### 6.2 产品图片

```text
ProductSnapshot + Approved Source Assets
        ↓
ImageAgent Plan
        ↓
ImageAgent Tool
        ↓
product/image capabilities
        ↓
integration/openai | grsai | httpimage
        ↓
ImageAgent candidate/review/effect lifecycle
        ↓ 人工批准
product/asset.Repository
```

ImageAgent 是这条链路中唯一可以决定并发、预算、重试、恢复和批准的组件。

### 6.3 消费方

ListingKit、SDS 和 AmazonListing 通过各自包内定义的窄读取 Port 获取 ProductSnapshot 与 ApprovedAssetInventory。它们不得导入 ImageAgent Store/Temporal，也不得创建丰富化或图片任务。

## 7. 错误、状态和安全

### 7.1 领域错误

产品能力只返回稳定语义：

- 输入无效
- 证据不足
- 能力不支持
- 策略拒绝
- 外部能力不可用
- 输出验证失败

领域错误不包含重试次数、Queue 状态、Temporal 状态或 Provider SDK 错误类型。

### 7.2 编排映射

ImageAgent Tool 把图片能力错误映射为现有 ImageAgent 状态，包括 provider not dispatched、provider outcome unknown、publication unknown、budget exhausted 和 validation blocked。产品图片能力不得反向导入 ImageAgent 类型。

### 7.3 原子性

- Enrichment Proposal 失败时不得修改 ProductSnapshot。
- 图片生成成功但发布失败时由 ImageAgent effect lifecycle 恢复。
- 只有人工批准结果可以写入产品资产。
- 写入失败不能把 ImageAgent Run 标记为已完成。

### 7.4 身份和租户

- 删除匿名 ProductImage 提交路径。
- ImageAgent 必须携带 tenant、user、business task 和 trace identity。
- Repository 查询和写入必须显式限定 tenant。
- 调用方不得伪造或补全缺失身份。

### 7.5 禁止隐式降级

生产环境禁止：

- 数据库缺失时自动使用内存 Repository
- Redis 缺失时自动使用内存 Queue
- ImageAgent 不可用时回退 ProductImage
- 没有批准资产时选择来源图或第一张图顶替
- Provider 失败时静默切换到未授权 Provider

## 8. API 和持久化退休

以下路由直接删除，不提供转发或 deprecated 响应：

- `POST /api/v1/products/generate`
- `GET /api/v1/products/tasks/{task_id}`
- `POST /api/v1/images/process`
- `GET /api/v1/images/tasks/{task_id}`
- `POST /api/v1/images/tasks/{task_id}/review`

OpenAPI 描述、生成客户端类型、命令 README 和路由测试必须同步删除旧契约，并新增旧路由无法注册的测试。

以下运行时资源停止使用：

- `product_enrich_tasks`
- `product_image_tasks`
- `product_enrich` worker pool
- `product_image` worker pool
- `product_enrich_tasks` Queue
- `product_image_tasks` Queue

数据库中已有表可以继续存在，但应用启动、迁移基线、Repository 和业务查询不得依赖它们。

## 9. 实施策略

实施采用契约优先的单 PR 硬切：

1. 先添加目标目录和依赖护栏。
2. 迁移 Catalog、Sourcing、Asset 等稳定事实与 Port。
3. 从 ProductEnrich 提取纯 Enrichment Proposal 能力，删除任务语义。
4. 从 ProductImage 提取纯 Image 能力并改接 ImageAgent Tool。
5. 把 Asset GORM Adapter 移到 Integration。
6. 删除 ListingKit、SDS、AmazonListing 的旧编排调用，改为只读 Port。
7. 删除旧 HTTP API、Queue、Worker、Repository 与运行时装配。
8. 删除旧根目录并启用全仓禁止导入护栏。

每个中间提交必须可编译并通过对应聚焦测试。可以在分支内部短暂同时存在新旧目录以完成迁移，但最终 PR 不得保留兼容 Facade、双写或旧路径。

## 10. 测试和验收

### 10.1 架构护栏

新增测试保证：

- 五个旧产品根目录不存在。
- 全仓没有旧产品路径 import。
- `internal/product/*` 不导入 app、platform、integration、GORM、Temporal、Redis、RabbitMQ 或 Provider SDK。
- 只有 ImageAgent 可以编排产品图片能力。
- ListingKit、SDS、AmazonListing 不创建图片或丰富化任务。
- 生产装配不自动使用内存 Repository/Queue。
- 旧 HTTP 路由无法注册。

### 10.2 行为测试

覆盖：

- SourceEnvelope 保留来源身份、lineage 和 warnings。
- Catalog 归一化保持确定性。
- Enrichment 输入不变、输出 Proposal、无持久化副作用。
- Enrichment 失败不修改 ProductSnapshot。
- ImageAgent Tool 调用新的 `product/image` Port。
- ImageAgent 审批前不写产品资产。
- 重复审批和恢复不重复写资产。
- 内存和 GORM Asset Repository 的 tenant/idempotency 契约一致。
- 消费方只能读取 approved asset；缺失时返回明确未就绪状态。
- 旧 ProductEnrich/ProductImage API 返回 404。
- 缺少 ImageAgent 或生产持久化依赖时应用启动失败。

### 10.3 验证命令

```powershell
go test ./internal/product/... ./internal/imageagent/... ./internal/listingkit/... -count=1
go test ./tests -run 'Test(Product|ImageAgent|LegacyProductRoots|TargetDomains|.*Depguard.*)' -count=1
golangci-lint run ./internal/product/... ./internal/imageagent/... ./internal/app/...
go test ./tests -count=1
go test ./... -count=1
git diff --check
```

同时运行源码扫描，确认旧路径、旧队列名、旧路由和旧运行时模块引用为零。

## 11. 完成标准

阶段三仅在以下条件全部满足时完成：

1. 目标产品目录成为唯一产品领域入口。
2. 五个旧产品根目录已删除。
3. ProductEnrich 和 ProductImage 的任务体系、API 与持久化接线已删除。
4. ImageAgent 是唯一图片工作流所有者。
5. ListingKit、SDS 与 AmazonListing 不再编排丰富化或图片任务。
6. 产品域没有具体基础设施依赖。
7. 所有聚焦测试、架构护栏、lint 与全仓测试通过。
8. 最终 PR 不含兼容 Facade、双写或“后续再迁移”的旧实现。

## 12. 明确不在本阶段处理

- ProductAgent 实现
- ImageAgent Store/Temporal 的 Agent 目录治理
- 旧任务表的物理删除
- Marketplace 全面目录迁移
- Crawler 全面目录迁移
- `internal/pipeline` 的 TEMU/Marketplace 拆分

这些工作不能反向要求阶段三保留旧 ProductEnrich/ProductImage 边界。
