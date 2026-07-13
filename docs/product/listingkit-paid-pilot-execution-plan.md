# ListingKit 付费商业试点上线执行计划

> Status: active commercial-readiness execution plan.
>
> Last reviewed: 2026-07-13.
>
> Calibrated against: `master` at `3cb81c9babea8add7c265a6639687abc758e81e8`.
>
> Scope: 将当前已经稳定或接近稳定的 ListingKit / SHEIN 能力整理为可安全收费、可支持、可回滚的邀请制付费试点。
>
> Authority: 当本计划与旧的商业化、订阅、部署或产品扩张计划冲突时，付费试点范围、上线门禁和 PR 顺序以本计划为准；长期架构仍以 `docs/refactoring/current-refactoring-status.md`、`docs/refactoring/next-phase-plan.md` 和架构边界文档为准。

## 1. 商业化决策

首个收费版本不定位为“全平台无人值守自动上架 SaaS”。

首发产品定义为：

```text
邀请制 ListingKit SHEIN 辅助上架服务

受控 SDS / 1688 来源
  -> AI 标题、描述、属性和图片辅助生成
  -> ListingKit 工作台人工审核
  -> readiness 检查
  -> 默认保存 SHEIN 草稿
  -> 白名单租户显式确认后正式发布
```

### 1.1 首发可销售能力

- ZITADEL 登录和租户隔离。
- 受控 SHEIN 店铺接入。
- SDS POD 到 ListingKit 工作台的现有稳定路径。
- 完成受控闭环验证后的 1688 来源导入。
- AI 标题、描述、属性和图片辅助生成。
- 工作台人工审核、readiness blocker 修复。
- 保存 SHEIN 草稿。
- 对白名单租户开放正式发布。
- 人工应用套餐、人工收款和人工开票。
- 工作时间人工支持和明确的故障升级流程。

### 1.2 首发不承诺

- 无人值守批量自动发布。
- 全量自动促销或自动核价。
- TEMU、Amazon、Walmart 完整 ListingKit 工作台。
- 多来源实时同步平台。
- 自助支付、自动续费和复杂按比例计费。
- 7×24 高可用 SLA。
- AI 输出绝对准确、绝对合规或平台必然审核通过。
- 未经演练的数据零丢失承诺。

### 1.3 发布阶段

| 阶段 | 用户范围 | 收费状态 | 发布能力 | 进入/退出条件 |
| --- | --- | --- | --- | --- |
| Internal Readiness | 仅内部账号 | 不收费、不签付费试点订单 | 草稿与测试发布 | 完成 M0；其余门禁在隔离环境持续验证 |
| Paid Pilot A | 2–5 个邀请制租户 | 可按已批准的固定套餐收费 | 默认草稿；逐租户白名单发布 | 进入前满足第 2 节全部收费门禁并完成 PAY-070～PAY-072；退出按 PAY-073 |
| Paid Pilot B | 6–20 个邀请制租户 | 按已批准套餐收费 | 逐租户开放发布 | PAY-073 通过后才可增加第 6 个租户 |
| General Availability | 公开注册 | 另行决策 | 另行决策 | 自助支付、HA、合规和自动化运营闭环完成 |

本计划只覆盖 Internal Readiness、Paid Pilot A 和 Paid Pilot B。

### 1.4 唯一商业放行规则

以下规则是收费和外部准入的唯一权威；单个里程碑完成不自动获得商业放行：

| 动作 | 必须满足 |
| --- | --- |
| 邀请外部客户进入生产或连接真实店铺 | 第 2 节全部条件完成；PAY-070～PAY-072 完成；产品、工程、运维共同 Go/No-Go |
| 签署并开始计费 | 上述条件完成；套餐、价格、退款/暂停政策及合同、隐私文本经人工批准 |
| 为某租户开放正式发布 | 上述条件完成；M3 完成；该租户 preflight、草稿验收和显式业务批准通过 |
| 增加第 6 个租户并进入 Paid Pilot B | PAY-073 全部观察门禁通过 |

内部账号可以在隔离环境提前验证后续能力，但这不构成邀请外部客户、收费或开放正式发布的依据。

## 2. Paid Pilot A 收费 Definition of Done

只有以下条件全部成立，并完成 PAY-070～PAY-072 的逐租户放行，才能对外收取试点费用：

```text
[x] 固定 release SHA 的后端、前端、race、build 和镜像验证结果可见。
[ ] 所有外部 ListingKit / Product Sourcing 路由只信任验证后的身份上下文。
[ ] 零已知跨租户读写路径。
[ ] 零已知重复远端草稿或重复发布路径。
[ ] 保存草稿、正式发布、本地状态和 SHEIN 远端状态可核对。
[ ] 价格、币种、库存、店铺和提交动作在用户确认时可见并被冻结。
[ ] 订阅和配额在所有收费入口一致生效。
[ ] 用量记录具有幂等业务事件，可对账、可补记、可人工调整。
[ ] PostgreSQL、对象存储和关键运行状态有备份及真实恢复演练记录。
[ ] 生产配置、Secret、容器和网络边界经过硬化。
[ ] 生产发布经过 staging smoke、人工批准、post-deploy smoke 和回滚门禁。
[ ] P0/P1 告警能到达实际负责人。
[ ] 至少一条成功草稿、一条成功发布、一条可恢复失败和一条 blocker 修复有真实记录。
[ ] 客户支持、退款/暂停、数据导出/删除和事故通知流程已定义。
[ ] 商业合同、服务条款和隐私文本已经人工审核。
```

## 3. AI 实施规则

AI 执行本计划时必须遵守以下规则：

1. 一次只实现一个任务 ID，不将多个风险域塞进同一 PR。
2. 每个 PR 必须写明精确 base SHA、行为变化、迁移影响和回滚方法。
3. 不做与任务无关的目录重命名、helper shaving 或大范围 package move。
4. 优先复用现有 `internal/listing/submission`、`internal/publishing/shein`、`internal/listingsubscription`、`internal/authz` 和租户上下文，不创建第二套状态机或权限模型。
5. 所有数据库变更必须向前兼容；先扩展、回填、切换，再考虑删除旧字段。
6. 所有外部写操作必须有身份、租户、店铺归属、订阅和幂等检查。
7. 所有收费动作必须有稳定业务事件 ID；重试不能重复扣费。
8. 真实 Token、Cookie、密钥、客户数据和远端完整响应不得进入测试 fixture 或 Git。
9. 测试失败必须分类，禁止用扩大 allowlist、跳过测试或静默忽略错误来“变绿”。
10. 每个阶段完成后更新本计划 checkbox 和对应 dated validation note。

### 3.1 每个 PR 的最低说明

```text
Task ID:
Base SHA:
What changed:
Why:
Behavior change:
Tenant/security impact:
Billing/metering impact:
Database migration:
Rollback:
Validation commands:
Real-flow validation:
Known follow-ups:
```

## 4. 里程碑与依赖顺序

```text
M0 固定基线和发布证据
  -> PAY-040 冻结套餐、entitlement 和用量政策
  -> M1 身份、租户、店铺和资源隔离
  -> M3 SHEIN 提交安全、定价、远端对账和发布策略
  -> M2 来源追溯与 1688 闭环
  -> M4 用量 ledger、入口执行和人工商业台账（PAY-041～PAY-044）
  -> M5 数据保护、生产配置和供应链硬化
  -> M6 可观测性、支持和真实验收
  -> M7 邀请制付费试点发布
```

章节按风险域组织，不代表执行顺序；唯一执行顺序以第 14 节 PR 队列为准。M1 完成只代表隔离基础具备，不单独构成外部准入条件。M3 未完成时不得开放正式发布。PAY-040 未完成人工固定套餐也不得启用；M4 未完成不得开始收费。M5 的恢复演练未完成时不得邀请外部客户或承诺数据 SLA。

## 5. M0：固定基线和发布证据

### PAY-000：建立可重复的 commercial-readiness workflow

**目标**

允许对指定 commit SHA 执行完整验证，而不是依赖“有人运行过 CI”的口头记录。

**建议文件**

- `.github/workflows/ci.yml`
- 可新增 `.github/workflows/commercial-readiness.yml`
- `Makefile`
- `scripts/`
- `docs/product/validation/runs/`

**实现要求**

- 增加 `workflow_dispatch`，允许输入或选择待验证 SHA。
- 输出精确 commit SHA、Go 版本、Node 版本、依赖锁文件摘要。
- 执行：
  - `go test ./... -count=1`
  - Listing Control Plane 和 listingadmin race tests
  - `make build-all`
  - 前端 lint、typecheck、test、build
  - API/UI Docker build
  - Kustomize render 或 server-side dry run
- 将测试摘要、构建清单和失败分类作为 artifact 保存。
- workflow 只采集证据，不在同一次运行中修改代码。

**验收标准**

```text
[x] 可以对一个不可变 SHA 手动触发完整验证。
[x] 四个正式 command 全部构建。
[x] API 和 UI 镜像可以在不 push 的情况下完成 build。
[x] 输出可以关联到具体 workflow run 和 artifact。
[x] 失败不会被通知步骤或非关键上传步骤掩盖。
```

### PAY-001：记录首个商业候选基线

**目标**

对选定 SHA 运行 PAY-000，建立原始 baseline run；如有失败，按风险域开独立修复 PR，最后再做 closure run。

**输出**

- `docs/product/validation/runs/YYYY-MM-DD-listingkit-commercial-baseline.md`

**禁止**

- 在 baseline 记录 PR 中顺带修改业务代码。
- 把失败描述为“环境问题”但不提供证据。

## 6. M1：身份、租户、店铺和资源隔离

### PAY-010：保护 Product Sourcing 外部路由

**目标**

修复 `product-sourcing` 模块未明确进入 ZITADEL 认证范围的问题。

**建议文件**

- `internal/listingkit/httpapi/zitadel_auth_route_authorization.go`
- `internal/productenrich/httpapi/sourcea1688/*`
- `internal/product/sourcehandoff/a1688/httpapi/*`
- 路由 descriptor 和权限测试

**实现要求**

- `product-sourcing` 路由必须要求 ZITADEL 认证。
- 创建任务必须要求明确的 write permission。
- 请求体中的 `tenant_id`、`user_id` 不得成为身份来源。
- 未认证、无权限或身份上下文缺失时 fail closed。
- 记录 actor、tenant、route、source ID 和拒绝原因的安全审计事件；不得记录 Token。

**验收标准**

```text
[ ] 无 Token 请求返回 401。
[ ] 无权限身份返回 403。
[ ] 请求体伪造 tenant_id/user_id 不改变实际身份上下文。
[ ] 认证后的租户和用户被写入任务。
[ ] 安全审计日志不包含 Token、Cookie 或 Secret。
```

### PAY-011：统一客户 API 的 authoritative identity context

**目标**

禁止外部调用者通过 body、query 或伪造 Header 覆盖已验证的 tenant/user。

**建议文件**

- `internal/listingkit/api/tenant_context.go`
- `internal/listingkit/api/handler_tasks.go`
- `internal/listingkit/api/*_handler.go`
- `internal/app/httpapi/server_auth.go`
- 前端 proxy-auth 相关文件

**实现要求**

- 定义唯一的 `AuthenticatedIdentity` / authoritative request context。
- tenant 和 user 只能来自认证中间件写入的可信 context。
- 普通租户 API 忽略或拒绝 body/query tenant override。
- platform-admin 跨租户操作使用单独 route、permission 和显式目标 tenant 参数。
- 兼容旧调用时，只能采用短期、可观测、可关闭的 feature flag，并记录退休条件。

**验收标准**

```text
[ ] tenant A 无法通过 body/query/header 访问 tenant B。
[ ] 普通用户不能调用 platform-admin 跨租户 API。
[ ] platform-admin 操作产生 actor、target tenant 和 reason 审计。
[ ] 所有任务创建、列表、详情、提交和设置接口使用同一身份规则。
```

### PAY-012：店铺和来源店铺归属校验

**目标**

在任何任务创建、草稿、发布、同步或 source handoff 前验证目标店铺属于当前租户。

**实现要求**

- `SheinStoreID`、`SourceStoreID`、store profile 和 token 解析必须 tenant-scoped。
- 请求中的 store ID 不能只因数据库存在就被接受。
- 任务中的已解析 store snapshot 与当前租户不一致时拒绝执行。
- 店铺被禁用、过期或权限不足时返回稳定错误码和用户下一步动作。

**验收标准**

```text
[ ] tenant A 不能使用 tenant B 的 SHEIN store。
[ ] tenant A 不能使用 tenant B 的 source store。
[ ] 已禁用店铺不能进入远端提交。
[ ] 店铺切换不会复用旧租户的价格、Cookie 或 resolution cache。
```

### PAY-013：上传素材和对象存储资源隔离

**目标**

确保图片上传、读取、删除和 imgproxy 访问均有租户边界。

**实现要求**

- 对象 key 必须包含不可伪造的 tenant scope。
- GET/DELETE 不能只依赖用户提供的 key。
- 校验 MIME、文件签名、大小、图片解码和允许格式。
- S3 私有 bucket 或签名访问优先；公开 URL 必须评估客户数据暴露。
- 删除和容量退款必须幂等。

**验收标准**

```text
[ ] tenant A 不能读取或删除 tenant B 的图片。
[ ] 伪造 Content-Type 不绕过文件校验。
[ ] 重复删除不造成重复额度退款。
[ ] 上传失败不产生孤儿用量或孤儿对象。
```

### PAY-014：跨租户安全回归套件

**目标**

建立独立的 commercial security suite，覆盖任务、店铺、素材、订阅、设置、提交和管理员 API。

**建议位置**

- `tests/commercial_security_test.go`
- 各模块 focused tests

**必须覆盖**

- 未认证访问。
- 伪造 tenant/user Header。
- body/query tenant override。
- 跨租户 task ID。
- 跨租户 store ID。
- 跨租户 object key。
- 普通用户调用 platform-admin API。
- legacy tenant fallback 不得扩权。

## 7. M2：来源追溯和 1688 商业闭环

### PAY-020：持久化平台中立的 SourceReference

**目标**

任务重新加载后仍可追溯来源，而不是只在临时 HTTP handoff 响应中看见 identity/warnings。

**建议模型**

```text
SourceReference
  source_type
  source_platform
  source_id
  source_key
  source_url
  source_version
  source_fingerprint
  snapshot_id
  source_run_id
  request_id
  warnings
```

**实现要求**

- SourceReference 是平台中立 DTO，不能让 root ListingKit 依赖 crawler DTO 或完整 `SourceEnvelope`。
- 可先放入 `GenerateRequest` JSON 或独立 task metadata；必须可持久化、查询、导出。
- 旧任务兼容读取。
- warnings 使用稳定 code，不把任意远端响应原文直接暴露给客户。

**验收标准**

```text
[ ] 创建任务后重新从数据库读取仍能获得 source key 和 warnings。
[ ] task list/detail 可以展示来源类型和可理解 warning。
[ ] retry/requeue 不丢失来源追溯信息。
[ ] 不新增 source-specific 分支到 root ListingKit。
```

### PAY-021：1688 contract closeout

**目标**

用确定性 fixture 完成：

```text
Product1688 fixture
  -> SourceEnvelope
  -> catalog/asset facts
  -> persisted ListingKit task
  -> preview/readiness
```

**验收矩阵**

- 正常商品。
- 缺失 source ID 但可 fingerprint。
- 缺标题。
- 缺图片。
- 缺成本/价格。
- 重复图片 URL。
- 部分变体。
- source adapter error。

**验收标准**

```text
[ ] 正常 fixture 创建真实持久化 task。
[ ] lineage 和 warnings 在重新读取后可见。
[ ] preview/readiness 使用现有 SHEIN 所有权，不创建新提交路径。
[ ] 缺失事实是 warning/error，不静默制造商品真相。
```

### PAY-022：1688 operational smoke

**目标**

在 staging 执行：

```text
1688 URL
  -> integration crawler adapter
  -> Product1688
  -> SourceEnvelope
  -> task
  -> preview/readiness
```

**要求**

- 使用授权、可公开测试或业务确认可抓取的商品。
- 不默认自动发布。
- 记录 crawl snapshot、task ID、warning、耗时和失败阶段。
- 验收报告不得包含登录凭证或原始 Cookie。

## 8. M3：SHEIN 提交安全、定价和远端对账

### PAY-030：审计并强化提交幂等约束

**目标**

复用现有 submission attempt，不新增第二个状态机；在数据库层保证相同业务动作不会重复远端执行。

**幂等范围**

```text
tenant_id + store_id + task_id + action + confirmed_payload_revision
  -> one stable submission_intent_id
  -> one active or successful remote execution
```

**实现要求**

- 用户确认时由服务端创建并持久化稳定的 `submission_intent_id`；同一确认意图的超时重试、页面刷新、服务重启和恢复任务必须复用该 ID。
- 客户端 `idempotency_key` 只能用于定位同一确认意图，不能仅因 key 不同就获得新的远端执行资格。
- 同一 tenant、store、task、action 和 confirmed payload revision 只能存在一个 running、unknown 或 succeeded 的远端执行 owner。
- 只有内容 revision 变化并再次确认，或原意图明确终止且按策略允许重试时，才能创建新的 submission intent。
- `save_draft` 和 `publish` 使用不同 action scope。
- 数据库唯一约束或原子 claim。
- 并发请求只有一个 owner 可进入远端调用。
- 成功 attempt 重放返回已有结果。
- running/unknown attempt 不直接重复调用远端。

**验收标准**

```text
[ ] 100 个并发相同请求只产生一次远端调用。
[ ] 同一确认意图使用两个不同客户端 idempotency key 并发请求仍只产生一次远端调用。
[ ] 网络超时后重试不重复创建草稿或发布。
[ ] 服务重启后仍能识别已有 attempt。
[ ] 重放返回相同 remote record 或进入 reconciliation。
[ ] 内容 revision 变化后必须重新确认，才允许创建新的远端执行意图。
```

### PAY-031：冻结提交时的店铺、定价、币种和库存快照

**目标**

防止用户确认后，后台配置变化导致错店铺、错价格或错库存。

**实现要求**

- attempt 创建时保存 store resolution、pricing rule version、currency、supply price、SKU price、cost、stock 和 action。
- UI 在确认前展示关键快照。
- 正式发布必须二次确认。
- 快照过期或基础事实发生关键变化时要求重新确认。
- 首发默认关闭自动促销和自动核价，逐租户 feature flag 开启。

**验收标准**

```text
[ ] 提交中途修改规则不会改变已创建 attempt 的 payload。
[ ] 错币种、零价格、缺 SKU 价格、成本倒挂被阻止。
[ ] 用户确认的 store/action 与远端请求一致。
```

### PAY-032：远端未知状态 reconciliation

**目标**

解决“远端可能成功、本地显示失败”的商业高风险场景。

**实现要求**

- 将 timeout、连接中断和 persist failure 区分为 `unknown_remote_state`。
- unknown 状态先查询远端记录，再决定成功回填、失败或人工处理。
- 对账任务必须幂等、可限速、可审计。
- UI 显示“正在核对远端状态”，不得诱导用户再次发布。

**验收标准**

```text
[ ] 模拟远端成功、本地持久化失败后能安全回填。
[ ] 模拟远端请求超时后不会立即重复提交。
[ ] 无法自动判断时进入明确人工队列。
[ ] 所有 reconciliation 结果有审计记录。
```

### PAY-033：商业发布策略和租户 feature flags

**目标**

将“草稿优先、白名单发布”落实为后端策略，而不只是 UI 文案。

**建议 flag**

```text
commercial_pilot_enabled
shein_save_draft_enabled
shein_publish_enabled
shein_auto_pricing_enabled
shein_auto_promotion_enabled
product_sourcing_1688_enabled
```

**要求**

- flag tenant-scoped，并由 platform-admin 管理。
- 后端强制执行，前端隐藏不等于授权。
- 改 flag 有 actor、reason 和时间审计。
- 关闭 publish 后已有任务仍可查看，但不能新发起发布。

## 9. M4：套餐、计量和人工商业台账

### PAY-040：定义商业产品目录和套餐语义

**目标**

把内部模块名映射为客户能理解的完整能力，修复 Basic/Professional 等套餐与实际入口不一致的问题。

**必须决策**

- 哪个套餐可创建 ListingKit 任务。
- 哪个套餐可使用 SDS、1688、设计生成、商品图和保存草稿。
- 正式发布是否为独立 entitlement。
- 失败、取消、平台拒绝和工程重放是否扣费。
- storage 采用上传量、当前占用还是 byte-hours。

**建议首发计费维度**

```text
listing_tasks_created
studio_design_jobs_succeeded
product_image_jobs_succeeded
shein_drafts_succeeded
shein_publishes_succeeded
storage_bytes_current
```

首发不建议按每次 AI 内部调用直接向客户计费，但必须内部记录 AI 成本。

### PAY-041：实现幂等 usage event ledger

**目标**

从可变累计数升级为可审计的业务事件，再由事件汇总得到 usage counter。

**建议模型**

```text
UsageEvent
  event_id
  tenant_id
  module_code
  metric
  quantity
  source_type
  source_id
  status: reserved / committed / released / reversed
  occurred_at
  idempotency_key
  metadata
```

**实现要求**

- `event_id/idempotency_key` 唯一。
- authorize + reserve 原子化。
- 成功 commit；明确失败策略后 release/reverse。
- 重试不会重复扣费。
- 原有 counter 可由 ledger 重建或每日校验。

**验收标准**

```text
[ ] 高并发不能突破硬配额。
[ ] 相同业务事件重放不会重复扣费。
[ ] 失败任务按已定义政策退款或保留费用。
[ ] counter 与 ledger 汇总一致。
```

### PAY-042：统一所有收费入口的 entitlement 和 usage

**覆盖入口**

- `GenerateListingKit`
- Studio async design jobs
- Studio product image jobs
- 图片上传和删除
- 1688 task creation
- SHEIN save draft
- SHEIN publish
- 批量入口和内部 retry/recovery

**验收标准**

```text
[ ] 任何公开入口都不能绕过 entitlement。
[ ] internal retry 不重复收费。
[ ] batch partial success 只对成功或已定义的事件计费。
[ ] UI 显示的剩余额度与后端授权一致。
```

### PAY-043：建立人工商业订阅台账

**目标**

在没有自助支付前，支持人工合同、付款、套餐和到期管理。

**建议字段**

```text
customer_id
legal_entity
tenant_id
contract_or_order_id
plan_code
price
currency
billing_period
starts_at
expires_at
payment_status
invoice_status
sales_owner
support_tier
notes
```

**要求**

- 与现有 tenant subscription 建立稳定关联。
- 应用套餐必须记录业务订单/合同引用和 actor。
- 到期前通知，过期后按明确宽限期降级。
- 价格和合同信息只对 platform-admin 可见。

### PAY-044：用量对账、导出和补记

**目标**

每日自动核对 usage ledger、counter、任务和远端成功结果。

**要求**

- 生成 tenant/metric/day 汇总。
- 发现少记、多记和孤儿事件。
- 计量写入失败进入补记队列，禁止 `_ = RecordUsage(...)` 静默丢失。
- 人工调整必须有 before/after、actor、reason 和关联工单。
- 支持 CSV 导出供人工账单核对。

## 10. M5：数据保护、生产配置和供应链硬化

### PAY-050：生产配置 fail-closed

**目标**

移除开发配置对商业生产的隐式影响。

**要求**

- production 配置使用 INFO/WARN 和结构化日志。
- 示例密码、示例域名、空关键 Secret 和不安全默认值启动失败。
- ZITADEL auth、数据库、S3、Temporal、SHEIN、AI 的关键配置缺失时 fail-fast。
- Secret 不再 `optional: true`，除非该能力明确禁用。
- 浏览器路径、headless 策略和运行目录与 Linux 容器一致。
- 敏感字段统一脱敏。

### PAY-051：容器与 Kubernetes 硬化

**要求**

- 基础镜像固定版本或 digest，不使用可变 `latest` 作为商业 release 基线。
- 容器使用非 root 用户。
- 尽可能只读 root filesystem。
- 移除不需要的 capabilities。
- 配置 requests/limits、PDB、合理副本和反亲和性。
- 增加 NetworkPolicy，只开放必需的 UI、API、数据库、S3、Temporal、ZITADEL 和外部平台流量。
- 镜像扫描、SBOM、依赖许可证清单进入 release artifact。

### PAY-052：数据库、对象存储和关键状态备份

**目标**

建立可验证恢复能力，而不是只生成备份文件。

**最低要求**

- PostgreSQL 自动备份、保留策略和加密。
- 对象存储版本控制或生命周期/复制策略。
- Temporal、RabbitMQ、Redis 的恢复责任和可接受数据丢失范围明确。
- 每月恢复到隔离环境并验证关键租户任务、subscription、usage 和 source reference。
- 记录 RPO/RTO。

**试点建议内部目标**

```text
RPO <= 24h
RTO <= 4h
```

这些是内部目标，不自动构成对客户 SLA。

### PAY-053：schema migration release gate

**要求**

```text
backup
  -> migration dry run / staging migration
  -> compatibility tests
  -> production migration
  -> application rollout
  -> verification
  -> documented roll-forward/rollback
```

- 禁止依赖不可逆 AutoMigrate 而没有备份和兼容评估。
- 大表回填分批执行并可暂停。
- 应用新旧版本在滚动升级窗口内兼容。

### PAY-054：数据保留、导出和删除流程

**要求**

- 定义任务、图片、AI 输入输出、远端响应摘要、审计和备份保留周期。
- 提供租户数据导出流程。
- 提供停用、软删除、宽限期和最终物理删除流程。
- 删除覆盖数据库、S3、cache、Temporal、日志和后续备份到期处理。
- 每次删除有审批、范围预览和不可逆确认。

## 11. M6：可观测性、支持和真实验收

### PAY-060：完整付费链路监控

**在现有 SHEIN worker 监控基础上增加**

- UI/API 请求率、延迟、4xx/5xx。
- ZITADEL discovery/introspection 失败。
- PostgreSQL 连接、慢查询、锁和容量。
- RabbitMQ/Temporal/Redis backlog 和失败。
- S3 错误率和容量。
- AI 请求失败率、耗时和内部成本。
- 每租户任务量、失败率和异常成本。
- save draft/publish 成功率。
- unknown remote state 和 reconciliation backlog。
- subscription/quota 拒绝。
- usage event 写入或对账失败。
- backup 失败和恢复演练过期。

### PAY-061：告警、值班和事故管理

**要求**

- 定义 P0/P1/P2 严重等级。
- 每个告警有 owner、runbook、客户影响判断和降级动作。
- P0 至少包括：跨租户风险、重复发布、错店铺/错价格、数据不可恢复、认证全面故障。
- 定义工作时间支持和非工作时间升级联系人。
- 建立事故通知和复盘模板。

### PAY-062：配置健康检查和付费 onboarding preflight

**检查项**

- ZITADEL 身份和租户映射。
- SHEIN store token、权限、站点和类目 API。
- SDS 登录态。
- 1688 source 能力是否启用。
- AI client。
- 图片生成与 S3。
- Temporal worker。
- 数据库、Redis、RabbitMQ。
- 套餐 entitlement、剩余额度和 feature flags。

**验收标准**

```text
[ ] 新客户正式使用前可生成一份 preflight 报告。
[ ] 关键依赖失败时不能开始高成本或远端提交动作。
[ ] 报告不泄露 Secret。
```

### PAY-063：客户可理解的失败和支持入口

**要求**

- 所有已知 blocker 有稳定 key、说明和修复区域。
- unknown blocker 有兜底，不出现无下一步动作。
- subscription/quota 错误显示套餐、metric、used/limit 和联系支持入口。
- 远端 unknown 状态显示“正在核对”，不提示重复发布。
- UI 可复制 task ID、attempt ID、store ID、error code 和脱敏摘要。

### PAY-064：真实商业验收集

每个商业候选 release 至少完成：

```text
[ ] 真实环境 preflight。
[ ] SDS -> SHEIN 保存草稿成功。
[ ] SDS -> SHEIN 发布成功。
[ ] 1688 -> task -> preview/readiness 成功。
[ ] 可恢复失败路径。
[ ] readiness blocker 修复路径。
[ ] Token 过期恢复路径。
[ ] 远端成功、本地持久化失败的 reconciliation 路径。
[ ] 并发重复点击幂等路径。
[ ] 两个不同租户的隔离验证。
[ ] 配额耗尽和续费/增额路径。
```

输出放在 `docs/product/validation/runs/`，包含精确 SHA、task/attempt ID、脱敏输入、状态流转、远端 ID、最终结论和关闭标准。

## 12. M7：邀请制付费试点发布

### PAY-070：建立 pilot tenant allowlist

**要求**

- 只有 allowlist 中的 tenant 可见 commercial pilot 功能。
- 每个 tenant 独立配置来源、草稿、发布、自动定价和配额。
- 默认：草稿开、发布关、自动促销关、自动核价关。
- 开通和关闭均有 actor、reason 和合同引用。

### PAY-071：客户 onboarding runbook

**必须包含**

1. 建立 tenant 和用户。
2. 绑定合同/订单和套餐。
3. 配置店铺并验证归属。
4. 运行 preflight。
5. 创建第一条测试任务。
6. 保存第一条草稿。
7. 培训 readiness、重试和支持流程。
8. 仅在验收后启用正式发布。
9. 明确数据保留、服务时间和不支持范围。

### PAY-072：发布和回滚流程

```text
固定 release SHA
  -> commercial-readiness workflow 全绿
  -> 镜像扫描/SBOM
  -> 数据备份
  -> schema migration gate
  -> staging deploy
  -> preflight + save-draft smoke
  -> 人工批准
  -> production deploy
  -> 登录/任务/素材/订阅/草稿 smoke
  -> 观察窗口
  -> 扩大 allowlist 或回滚
```

**要求**

- 使用不可变 SHA image tag。
- 禁止商业 release 使用 `latest`。
- deployment workflow 必须依赖已验证 artifact 或相同 SHA 的 CI 结果。
- post-deploy smoke 失败立即停止扩容和新增租户。
- 数据库不支持直接回滚时，必须有可执行 roll-forward。

### PAY-073：Paid Pilot A 观察门禁

在增加第 6 个租户前，至少满足：

```text
[ ] 无跨租户安全事件。
[ ] 无重复远端草稿或发布事件。
[ ] 无错店铺、错币种或错价格事件。
[ ] 所有远端 unknown 状态有最终结论。
[ ] 用量 ledger 与 counter 每日对账一致。
[ ] 至少完成一次数据库恢复演练。
[ ] P0/P1 告警能送达并被处理。
[ ] 客户问题可以按 SOP 在工作时间内完成分级。
[ ] 商业毛利和 AI/S3/支持成本有实际数据。
```

## 13. 人工责任项

以下事项可以由 AI 起草、生成模板和检查清单，但不能仅凭 AI 完成后视为商业门禁通过。

| 事项 | AI 可以做 | 必须人工确认 |
| --- | --- | --- |
| 首发套餐和价格 | 建模、成本测算模板、后台实现 | 产品、财务和销售批准 |
| 服务条款和隐私政策 | 起草初稿、字段清单 | 合格律师审核 |
| 数据处理协议和子处理方清单 | 生成模板和系统事实清单 | 法务、供应商和经营主体确认 |
| 支付、发票和税务 | 设计接口和台账 | 财务、税务和支付主体确认 |
| SHEIN/1688/SDS 使用授权 | 记录技术依赖 | 业务和法务确认平台条款 |
| 生产 Secret 和账号 | 验证配置存在性 | 运维安全地注入和轮换 |
| 客户发布权限 | 实现 feature flag | 业务 owner 逐租户批准 |
| P0 事故响应 | 生成 runbook | 明确真人负责人和联系方式 |
| Release Go/No-Go | 汇总自动证据 | 产品、工程、运维共同签字 |

## 14. 推荐 PR 队列

严格按依赖顺序推进；本队列覆盖并取代章节出现顺序：

| 顺序 | Task | PR 建议标题 | Gate |
| ---: | --- | --- | --- |
| 1 | PAY-000 | `ci: add commercial readiness validation workflow` | M0 |
| 2 | PAY-001 | `test: record first commercial baseline validation` | M0 |
| 3 | PAY-040 | `docs: define paid pilot product catalog and usage policy` | Commercial policy |
| 4 | PAY-010 | `security: protect product sourcing routes` | M1 |
| 5 | PAY-011 | `security: enforce authenticated tenant identity` | M1 |
| 6 | PAY-012 | `security: enforce tenant store ownership` | M1 |
| 7 | PAY-013 | `security: isolate uploaded tenant assets` | M1 |
| 8 | PAY-014 | `test: add commercial cross tenant security suite` | M1 |
| 9 | PAY-030 | `fix: enforce shein submission idempotency` | M3 |
| 10 | PAY-031 | `fix: freeze shein commercial submission snapshot` | M3 |
| 11 | PAY-032 | `feat: reconcile unknown shein remote submission state` | M3 |
| 12 | PAY-033 | `feat: add tenant commercial release flags` | M3 |
| 13 | PAY-020 | `feat: persist neutral product source references` | M2 |
| 14 | PAY-021 | `test: close 1688 contract flow through persisted task` | M2 |
| 15 | PAY-022 | `test: record staging 1688 operational smoke` | M2 |
| 16 | PAY-041 | `feat: add idempotent subscription usage ledger` | M4 |
| 17 | PAY-042 | `fix: enforce usage across paid feature entrypoints` | M4 |
| 18 | PAY-043 | `feat: add manual commercial subscription registry` | M4 |
| 19 | PAY-044 | `feat: reconcile and export subscription usage` | M4 |
| 20 | PAY-050 | `security: harden production configuration` | M5 |
| 21 | PAY-051 | `security: harden listingkit containers and manifests` | M5 |
| 22 | PAY-052 | `ops: add verified backup and restore workflow` | M5 |
| 23 | PAY-053 | `ops: gate listingkit schema migrations` | M5 |
| 24 | PAY-054 | `feat: add tenant export and deletion operations` | M5 |
| 25 | PAY-060 | `ops: add paid pilot service observability` | M6 |
| 26 | PAY-061 | `docs: define commercial incident response` | M6 |
| 27 | PAY-062 | `feat: add tenant onboarding preflight` | M6 |
| 28 | PAY-063 | `feat: improve paid workflow failure guidance` | M6 |
| 29 | PAY-064 | `test: record commercial release acceptance set` | M6 |
| 30 | PAY-070 | `feat: add paid pilot tenant allowlist` | M7 |
| 31 | PAY-071 | `docs: add paid pilot onboarding runbook` | M7 |
| 32 | PAY-072 | `ci: gate paid pilot production releases` | M7 |
| 33 | PAY-073 | `docs: record paid pilot A review` | M7 |

PAY-030 到 PAY-032 若仓库已有相应机制，AI 应先做 inventory，补约束和测试，不重新实现平行状态机。PAY-020 到 PAY-022 不得在 PAY-030 到 PAY-033 完成前进入可销售范围或连接付费租户真实店铺。

## 15. 通用验证命令

后端：

```powershell
go test ./... -count=1

go test -race ./internal/app/runtime/listingcontrol `
  -run TestControlPlaneService -count=1

go test -race ./internal/listingadmin `
  -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" `
  -count=1

make build-all
```

重点安全和商业测试：

```powershell
go test ./internal/listingkit/api/... -count=1
go test ./internal/listingkit/httpapi/... -count=1
go test ./internal/listingkit/store/... -count=1
go test ./internal/listingsubscription/... -count=1
go test ./internal/product/sourcehandoff/... -count=1
go test ./internal/productenrich/httpapi/sourcea1688/... -count=1
go test ./tests/... -count=1
```

前端：

```powershell
Set-Location web/listingkit-ui
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```

部署与镜像：

```powershell
docker build -f deployments/docker/Dockerfile.product-listing-api .
docker build -f deployments/docker/Dockerfile.listingkit-ui .
kustomize build deployments/kubernetes/listingkit-workbench/overlays/prod
```

任何真实发布验证必须另行记录 task ID、attempt ID、tenant、store、action、远端 ID、最终状态和恢复动作。

## 16. 停止条件

出现以下任一情况时，停止扩大试点，先修复或回滚：

- 发现跨租户读写或身份覆盖。
- 发现重复远端草稿、重复发布或幂等失效。
- 发现错店铺、错币种、错价格或用户未确认即发布。
- 本地和远端状态无法对账且没有安全人工队列。
- 用量重复扣费、配额可被并发绕过或账单无法解释。
- 备份无法恢复。
- P0 告警无人接收。
- 生产 Secret、Cookie 或客户数据泄露到日志、监控或 Git。
- release workflow 使用未验证 SHA、`latest` 镜像或跳过关键门禁。
- 客户遇到失败但 UI 和 SOP 都无法给出下一步动作。

## 17. 当前立即动作

从 PAY-000 开始：

```text
ci: add commercial readiness validation workflow
```

然后执行 PAY-001 记录当前不可变 SHA 的原始状态，再执行 PAY-040 冻结套餐和用量政策。按第 14 节先完成 M1 和 M3，之后才推进 1688 商业闭环；不要在核心身份和远端提交安全门禁完成前扩大来源或新目标平台工作台。
