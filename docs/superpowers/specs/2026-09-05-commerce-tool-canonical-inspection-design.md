# B0 Canonical Product Inspection Tool 设计

## 1. 状态与结论

状态：`IMPLEMENTATION_READY`

独立评审：PR #295。Round 1 基于 commit
`1086e5c176ea0bd98b0ac69e47b70b0083a4f6a5` 完成；Round 2 与最终实现评审发现的
Blocker/Implementation Test 已按 15.2、15.3 节收敛根因并重新冻结。没有未处理
Blocker，也没有进行第三轮正常 Architecture Review。

本设计是
`docs/superpowers/specs/2026-08-31-commerce-tool-foundation-design.md`
中“切片 B：Canonical Inspection 纵向切片”的实施增补。它不重新设计 Commerce Tool
Registry，也不扩大 Phase 2A 的产品范围。

B0 交付一个真实、只读、无模型调用的 Commerce Tool：

```text
tool_id: product.canonical.inspect
version: v1.0.0
```

模型输入保持为 `task_id`。工具使用可信 `Principal` 解析与该任务关联的产品身份，
通过 `internal/product/catalog.SnapshotReader` 读取权威 `ProductSnapshot`，返回一个
不持久化、不可回写的只读投影和确定性诊断。

核心架构决策是：

- `internal/listing/task` 拥有 task-scoped canonical subject 窄查询合同；
- 现有 ListingKit task repository 暂时实现该目标 Port，但不会向 Tool 泄漏
  `listingkit.Task`、GORM 或旧 Service Interface；
- `internal/product/catalog` 继续拥有唯一 canonical product facts 和版本；
- `internal/product/catalog/tools/canonicalinspect` 拥有 Tool Definition、Schema、
  Executor、只读投影和错误归一化；
- `internal/commercetool` 继续只拥有框架无关的合同与调用治理，不导入任何领域实现；
- 不新增 Agent Runtime、HTTP Tool endpoint、数据库表、缓存或业务写操作。

## 2. 绑定边界

### 2.1 Product Decisions

以下已经确认的产品决策高于本设计和后续 Reviewer 的理想化扩展：

1. Phase 2A 先完成 Commerce Tool Foundation，再启动 Product Agent PoC。
2. 第一批 Tool 只允许 `read / compute / propose`；B0 仅为 `read`。
3. Tool 是现有领域能力的窄适配器，不拥有业务事实。
4. Agent Runtime 不得直接访问 GORM repository、Provider SDK 或 Marketplace Client。
5. tenant、user、roles、permission 等权威字段不能来自模型 JSON。
6. B0 使用现有 Casbin、可信身份、OpenTelemetry 和 Commerce Tool Registry，不建设
   第二套 IAM、Schema、Trace、Workflow 或 Agent Framework。
7. `internal/product/catalog.ProductSnapshot` 是 canonical product facts 的唯一权威
   所有者；ListingKit 只是消费者。
8. Phase 2A 不开放 write、save draft 或 publish。

### 2.2 Must

- 输入 JSON 只有 `task_id`，且必须与 `CallMetadata.BusinessTaskID` 精确一致。
- 原始 `Call.Arguments` 在任何 clone、JSON decode、Schema validation、身份解析或
  Executor 调用前执行 64 KiB 全局上限检查；超限审计不能再次扫描完整载荷。
- 每次调用都重新解析可信 Principal、执行 Tool permission 检查，并在 task 查询边界
  再执行 tenant/owner 过滤。
- tenant admin 和 platform admin 只能绕过当前 tenant 内的 owner 过滤，不能跨 tenant。
- permission preflight、task query 和 Executor 二次校验必须共享同一个配置化 Casbin
  `ListingKitAuthorizer`；配置的 platform-admin user/role 与内置 `platform_admin`、历史
  `admin` alias 语义一致。
- 不可见任务与不存在任务统一返回 `not_found`，不得暴露资源是否存在。
- 新任务使用持久化的 `SourceSnapshotVersion` 读取不可变版本；旧任务版本为零时读取
  当时最新的完整已提交版本，并在输出中返回实际版本。
- task 已存在但缺少 canonical product identity 或 snapshot 未就绪时返回
  `failed_precondition`。
- Tool 输出必须通过严格 JSON Schema，且不能包含 Commerce Tool 保留权威字段。
- 输出只投影权威 Snapshot 和任务 source lineage，不复制持久化所有权或创建第二套
  Product Fact Model。
- 所有 context cancellation/deadline 必须向两个读取边界传播；Executor 不重试。
- 成功或失败均不得修改 task、snapshot、asset、audit 以外的业务状态。Registry 现有
  audit 行为保持不变。

### 2.3 Should

- 同一个测试矩阵覆盖 owner、tenant admin、platform admin、cross-owner、cross-tenant
  和 legacy owner fallback。
- Tool Projection 使用 Catalog 类型作为内存事实来源；只在序列化边界复制和清理，
  不重新解释 marketplace readiness。
- 新的生产 Port 使用明确的 actor 参数，不依赖“context 缺身份时放宽查询”的旧 worker
  行为。
- 真实 Registry conformance test 使用 SQLite task/catalog repository、现有 Casbin
  Authorizer Adapter 和 OpenTelemetry tracer，且不需要真实模型或外部服务。

### 2.4 Out of Scope

- #30 source evidence reader 和完整 1688 闭环；
- #34 marketplace readiness / rule lookup；
- #130 ProductEnrich/ProductImage proposal；
- Product Agent Runtime、Agent Framework 或 Agent-facing HTTP API；
- 把全部 ListingKit task lifecycle 迁入 `internal/listing/task`；
- 修改 canonical snapshot、批准资产、Listing draft 或发布状态；
- 新建 audit 数据库表或业务事务；
- 将 `product.canonical.inspect` 输出作为 write command 接受。

### 2.5 Accepted Risks

- 旧任务的 `SourceSnapshotVersion == 0` 沿用现有行为，读取调用时最新的已提交
  snapshot。不同时间的两次调用可能看到不同版本；输出携带实际版本以保证结果可解释。
  本切片不通过写 task 来补 pin，因为 B0 必须无副作用。
- ListingKit task table 和 GORM model 尚未整体迁入 `internal/listing/task`。B0 只从现有
  repository 提取一个目标方向 Port，不借此扩大 task lifecycle 重构。
- Phase 2A 尚无 Product Agent runtime，因此 B0 不挂载公网 endpoint。真实 invocation
  通过 conformance/integration test 证明；正式运行时装配属于 Phase 2B。

## 3. Threat Model

### 3.1 In Scope

- 模型或调用方尝试在 JSON 中伪造 tenant、user、role、permission 或 Tool metadata；
- 使用自己可见的调用上下文读取其他 tenant 或同 tenant 其他 owner 的任务；
- `task_id` 与审计 `BusinessTaskID` 不一致造成 confused deputy 或审计错位；
- Repository 错误返回错误 tenant/owner subject 后 Tool 未二次校验；
- 超长原始 invocation 或超大持久化 Snapshot 消耗不受控内存；
- repository timeout、cancellation 或临时不可用被错误映射为可重试/不可重试状态；
- source metadata 中未来出现 Commerce Tool 保留字段，导致权威信息进入模型输出；
- Tool 直接导入 ListingKit DTO、GORM、Provider SDK 或 Agent Framework，形成永久耦合。

### 3.2 Out of Scope

- 已获得合法任务读取权限的用户看到其任务中本来可见的商品事实；
- canonical product 内容本身的商业敏感性分级；沿用现有任务读取产品决策；
- 对经过授权的 Product Agent 做内容水印、DLP 或模型侧提示注入防护；B0 不调用模型；
- 对旧任务做历史 Snapshot 回填；
- 跨请求 side-channel、响应时间均一化和隐藏是否为 legacy task。

### 3.3 Security Outcome

本设计不扩大现有 task read 权限。Tool 的权限检查是第一层，task repository 的显式
tenant/owner 查询是第二层，Executor 对返回 subject 的 tenant/owner 再校验是第三层。
任何一层失败都不能退化为无 scope 查询。

## 4. 权威所有者与依赖方向

```text
BoundToolSet.Invoke
  -> product/catalog/tools/canonicalinspect.Executor
      -> listing/task.CanonicalSubjectReader
          -> existing ListingKit task repository implementation
      -> product/catalog CompleteSnapshotReader
          -> integration/persistence/product/catalog bounded read-only adapter
      -> read-only projection + deterministic diagnostics
```

所有权表：

| 事实/行为 | 权威所有者 | B0 行为 |
| --- | --- | --- |
| Tool contract、allowlist、schema validation、timeout、audit | `internal/commercetool` | 直接复用 |
| Task identity、tenant、owner、product key、pinned version | `internal/listing/task` Port，现有 ListingKit repository 实现 | 窄读取 |
| Canonical product facts 与 snapshot version | `internal/product/catalog` | 窄读取 |
| 持久化实现 | `internal/listingkit/store` 与 `internal/integration/persistence/product/catalog` | Tool 不直接导入 |
| Casbin role/permission policy | `internal/authz` | 通过 Adapter 复用 |
| 可信用户身份 | `internal/authidentity` | 通过 Adapter 复用 |
| Trace | OpenTelemetry | 通过 Registry 复用 |

禁止依赖：

- `internal/commercetool` 继续禁止任何领域、应用、持久化或框架依赖；
- `internal/product/catalog/tools/canonicalinspect` 禁止导入根 `internal/listingkit`、
  `internal/listingkit/store`、GORM、HTTP、Temporal、Provider SDK、Marketplace Client 或
  Agent Framework；
- `internal/listing/task` 禁止导入 `internal/commercetool`、根 `internal/listingkit`、
  GORM、Provider SDK 或 Agent Framework；
- App/Integration 只能组装依赖，不能复制产品诊断规则。

## 5. Task-scoped Narrow Read Port

`internal/listing/task` 新增纯合同，概念结构如下：

```go
type Actor struct {
    TenantID string
    UserID   string
    Roles    []string
}

type SourceLineage struct {
    Key      string
    Type     string
    Platform string
    ID       string
    URL      string
}

type CanonicalSubject struct {
    TaskID          string
    TenantID        string
    OwnerUserID     string
    ProductKey      string
    SnapshotVersion uint64
    Source          *SourceLineage
}

type CanonicalSubjectReader interface {
    ReadCanonicalSubject(context.Context, Actor, string) (CanonicalSubject, error)
}
```

合同规则：

- Actor 的 tenant/user 必须非空、无首尾空白且受长度限制；roles 必须非空并复制；
- task ID 必须非空、无首尾空白且不超过 128 bytes；
- 查询始终显式包含 tenant；非 tenant admin/platform admin 时同时包含 owner；
- `internal/listing/task` 只声明 `TenantAdminChecker` 窄 Port，不依赖具体 IAM；composition
  root 将执行 permission preflight 的同一个配置化 `ListingKitAuthorizer` 注入 task
  repository 和 Executor；
- 不存在、cross-tenant、cross-owner 均返回 `listingtask.ErrCanonicalSubjectNotFound`；
- task 数据损坏或缺少 ProductKey 返回 `listingtask.ErrCanonicalSubjectNotReady`；
- task persistence 临时故障返回 `listingtask.ErrCanonicalSubjectUnavailable`，保留
  cancellation/deadline；
- 返回值必须复制 SourceLineage，不能暴露 `listingkit.GenerateRequest` 指针。

现有 task repository 在已有生产文件中实现此 Port，复用现有 task table、owner fallback
和 PostgreSQL/SQLite scope helper。不得增加第二个 task repository 或复制完整 task
lifecycle 查询。

## 6. Tool Contract

### 6.1 Definition

```text
tool_id:       product.canonical.inspect
version:       v1.0.0
capability:    product.canonical
owner:         product.catalog
risk:          read
side_effects:  none
idempotency:   deterministic
retry_owner:   caller
usage_owner:   unmetered
permission:    listingkit.admin.read
timeout:       3s
```

`listingkit.admin.read` 是现有 Casbin policy；它已授予 operator、tenant admin 和
platform admin。B0 不新增权限体系。owner scope 仍独立执行，不能因通过该 permission
而自动获得 tenant-wide 访问。

### 6.2 Input

```json
{
  "task_id": "task-uuid"
}
```

Schema 要求：

- object；
- `additionalProperties: false`；
- `task_id` required；
- UTF-8 string，1..128 bytes；
- Go 入口再校验 canonical whitespace 和 `BusinessTaskID` 精确一致。

### 6.3 Output

概念输出：

```json
{
  "task_id": "task-uuid",
  "product_key": "crawler:1688:123",
  "snapshot_version": 7,
  "snapshot": {},
  "source_lineage": {},
  "diagnostics": {
    "needs_review": true,
    "review_reasons": [],
    "warnings": []
  }
}
```

规则：

- 不输出 `tenant_id`、`user_id`、roles、permission 或任何 Tool authority metadata；
- `snapshot` 是 `catalog.ProductSnapshot` 的深复制只读投影；
- Diagnostics 只投影 Catalog 已持久化的 `Review` 和 `Warnings`，不创建第二套
  marketplace readiness 规则；
- task source lineage 与 snapshot field/source traces 分开返回；
- `SourceRecord.Metadata` 不直接暴露。B0 输出只保留类型化 source 字段，避免未来任意
  metadata key 绕过保留字段策略；
- 输出序列化上限为 1 MiB，与仓库现有 Agent snapshot 边界一致。超过上限返回
  `failed_precondition`，不得截断后伪装为完整商品；
- 8 MiB encoded snapshot 上限只属于 B0 的 bounded read-only persistence adapter；SQL
  先按字节长度做条件投影，超限时不把 payload 扫描进 Go，再执行 hash、JSON unmarshal
  和 clone；共享 Catalog publication/repository 不增加全局上限，以兼容已经持久化或被
  现有流程接受的大快照；Tool 自己仍执行更严格的 1 MiB 输出上限；
- Output Schema 必须严格声明全部返回字段并禁用 additional properties。

### 6.4 Business Task Binding

Executor 必须验证：

```text
input.task_id == envelope.Metadata().BusinessTaskID
```

不一致返回 `invalid_input`，且不调用任何 Reader。这避免调用记录指向任务 A、实际读取
任务 B 的审计错位。

## 7. 执行与错误映射

执行顺序：

1. Registry 校验 allowlist、可信 Principal、permission、input schema 和 timeout；
2. Executor 解码 input，并绑定 BusinessTaskID；
3. Executor 把可信 Principal 映射为 `listing/task.Actor`；
4. Task Reader 使用显式 tenant/owner scope 读取 CanonicalSubject；
5. Executor 再校验 subject task/tenant/owner/product identity；
6. pinned version 大于零时调用 `GetSnapshot`，否则调用 `GetCurrentSnapshot`；
7. 深复制、清理任意 metadata、构建 Catalog-owned diagnostics；
8. 序列化并检查 1 MiB 上限；
9. Registry 校验 output schema、保留字段、trace 和 audit。

错误映射：

| 来源 | Tool Error |
| --- | --- |
| 非法 task ID、BusinessTaskID 不一致 | `invalid_input` |
| 非 canonical Principal/Actor | `identity_integrity` |
| Casbin 拒绝 | `permission_denied` |
| task 不存在、cross-tenant、cross-owner、subject scope mismatch | `not_found` |
| task 缺 ProductKey、snapshot 未就绪、输出超过上限 | `failed_precondition` |
| task/catalog repository 明确不可用 | `dependency_unavailable` |
| context deadline/cancellation 导致超时 | `deadline_exceeded` |
| 持久化状态损坏、克隆/编码失败、未分类错误 | `internal` |
| Registry output schema 或保留字段失败 | `output_invalid` |

只有 `deadline_exceeded` 和 `dependency_unavailable` 沿用 Commerce Tool 的 retryable
分类。Executor 自身不重试。

## 8. 一致性、幂等与失败语义

B0 跨越 task store 与 catalog store 两个读取边界，但没有多个持久化步骤，因此不需要
共享事务、Unit of Work、outbox、Saga 或 Temporal。

| 场景 | 结果 |
| --- | --- |
| task read 失败 | 不读取 Catalog；无业务状态变化 |
| task read 成功、Catalog read 失败 | 返回归一化错误；无业务状态变化 |
| 响应丢失后重试 | pinned task 读取同一版本；legacy task 读取当时 current version |
| 进程重启 | 无本地状态需要恢复 |
| context 取消 | 两个 Reader 收到取消；不得继续调用下一边界 |
| 并发调用 | 各自只读；结果由各自读到的完整 committed snapshot 决定 |
| Catalog 正在发布新版本 | pinned task 不受影响；legacy task 读取原子提交前或后的完整版本 |
| 权限被撤销 | 下一次调用重新解析 Principal 和 Casbin policy，不复用 Tool 结果缓存 |
| tenant/owner 数据漂移 | task reader 的显式 scope 和 Executor 二次校验使调用安全失败 |

`deterministic` 表示对一个确定的 committed snapshot 不调用随机数、网络模型或隐式
fallback，并且没有副作用；它不承诺 legacy current-read 永久返回相同版本。

## 9. 资源边界

- 输入 task ID：最多 128 bytes；
- tenant ID：沿用 task storage 上限 64 bytes；
- user ID：沿用 task storage 上限 128 bytes；
- roles：最多 32 项，每项最多 128 bytes；
- 原始 `Call.Arguments`：最多 64 KiB，且必须在 clone/decode 前拒绝；超限路径不得保留或
  哈希完整载荷，审计使用固定 oversized marker hash；
- Tool execution timeout：3 秒，从 input schema/permission preflight 完成后开始，覆盖
  task read、catalog read、projection 和序列化；Executor 在每个 CPU 阶段后重新检查
  context；
- input schema validation 位于 execution timeout 之前，output schema validation 位于
  execution timeout 之后。两者分别被 64 KiB raw input 与 1 MiB output 上限约束；不得
  把 Definition 的 3 秒描述为整个 `BoundToolSet.Invoke` 的严格 wall-clock 上限；
- audit 使用 Registry 已有独立 `AuditTimeout`，不占用 3 秒 execution timeout；
- B0 Catalog reader encoded snapshot：最多 8 MiB；SQL 使用 byte-length 条件投影，超限
  payload 不进入 Go，随后才执行 hash/unmarshal/clone；共享 Catalog reader/writer 不受
  此限制；
- 输出：最多 1 MiB，超限不截断；
- Reader 调用次数：每次最多一次 task read 和一次 catalog read；
- 不调用网络、模型、消息队列、Temporal、文件系统或 cache；
- 不增加 Executor 内部并发。

## 10. 开源与已有能力复用

B0 不新增第三方依赖：

- JSON Schema：复用 `santhosh-tekuri/jsonschema/v6`；
- 权限：复用 Casbin 与 `internal/authz`；
- 身份：复用 `internal/authidentity`；
- Trace：复用 OpenTelemetry；
- Snapshot clone/version：复用 `internal/product/catalog`；
- Persistence：复用现有 ListingKit task repository 与 product catalog GORM adapter；
- Tool invocation/audit/error：复用 `internal/commercetool`。

不存在需要引入 Agent Framework、通用 Plugin SDK、第二套 RBAC 或新的 workflow engine
才能满足的 Must requirement。

## 11. 验证证据映射

| 不变量 | 必须提供的证据 |
| --- | --- |
| JSON 不能携带 authority | input schema + reserved field tests |
| 审计任务与读取任务一致 | BusinessTaskID mismatch test，Reader 调用次数为零 |
| cross-tenant 不可见 | SQLite real repository integration test |
| cross-owner 不可见 | operator fixture integration test |
| tenant/platform/configured/legacy alias admin 仅 tenant-wide | 同一配置化 authorizer 的 preflight、repository、Executor matrix tests |
| Tool 不直连 persistence/legacy DTO | AST/import guard + depguard |
| Snapshot 是权威读取 | real catalog repository current/versioned tests |
| 不存在隐式 current fallback | pinned reader missing/version unsupported tests |
| 无业务副作用 | 调用前后 task/catalog row count 与 version/head 断言 |
| diagnostics 不创建 marketplace rules | projection tests只比较 Review/Warnings |
| output 无保留字段和任意 metadata | nested reserved metadata fixture |
| cancellation/deadline 有界 | blocking Reader + Registry timeout test |
| raw invocation 有界 | 64 KiB exact/over-limit test，断言不保留/哈希原始载荷且 PrincipalResolver 未调用 |
| B0 Catalog 物化有界且不破坏兼容 | database-side conditional projection assertion + bounded exact/over tests + shared large snapshot compatibility test |
| output size 有界 | exact limit / over limit tests |
| 无模型或外部依赖 | import guard + conformance test |

## 12. Rollout 与回滚

B0 没有用户可访问 endpoint，也不修改已有流程。合并只增加一个可构造 Tool 和窄查询
Port；Phase 2B 显式把其 ToolRef 加入 AgentDefinition allowlist 后才会运行。

回滚只需撤销 Tool/Port 代码，不涉及数据迁移、表回滚、任务恢复或外部补偿。若 Tool
构造失败，现有 ListingKit、Product Sourcing、ImageAgent、SDS 和发布流程不受影响。

## 13. #134 边界

B0 完成后只为 #134 提供 canonical product reader 的部分证据，不关闭 #134：

- source evidence 仍等待 #30；
- ProductEnrich/ProductImage proposal 仍等待 #130；
- readiness/marketplace rule 仍等待 #34 和 Marketplace owner 稳定。

#134 当前正文只显式列出 #30、#34、#133，遗漏 #130。实现 PR 中应补充依赖说明，但
不能把该跟踪修正误认为 #130 已完成。

## 14. 被否决方案

### 输入改为 product_key

否决。Catalog 当前只有 tenant-qualified identity，没有与现有任务一致的 owner access
语义。直接按 product key 读取会改变产品授权边界。

### Tool Adapter 直接调用 ListingKit Repository 或 GORM

否决。会把 legacy DTO 和持久化实现固化进 Product Agent 边界，并绕过目标 Port。

### 新建第二个 Task Repository

否决。会复制 tenant/owner 查询和 legacy owner fallback，产生安全语义漂移。

### 通过兼容包返回 listingkit.Task

否决。会形成第二套 Compatibility Interface，并使后续 Listing Task 迁移受 B0 约束。

### B0 同时完成 #30、#34、#130

否决。依赖门禁未完成，范围超过一个可验证、可回滚的纵向切片。

### 为两次读取增加事务、Saga 或 Temporal

否决。B0 没有持久化副作用；pinned snapshot 已提供跨读取边界所需的一致性。新增恢复
机制不会解决真实 Must requirement。

## 15. Architecture Review Exit

本设计最多进行两轮正常 Architecture Review。Finding 必须按仓库分类规则处理。只有
命中明确 Blocker 后果的 finding 才重新打开架构；其他问题进入 Implementation Test 或
Backlog。

设计在以下条件满足后标记为 `IMPLEMENTATION_READY`：

- 独立评审没有未处理的 Blocker；
- task-scoped Port、owner/tenant 行为和 legacy version 语义明确；
- 权威 owner、错误映射、资源边界和验证证据完整；
- 没有引入新的通用框架或第二套权限/持久化实现。

### 15.1 Round 1 Finding 分类

```text
Finding: 原始 Call.Arguments 在 task_id 校验前可能被完整 clone/decode。
Product requirement affected: 请求大小与资源耗尽边界。
Classification: IMPLEMENTATION_TEST
Reason: 问题真实，但不改变 B0 的 owner、权限或状态模型；可由 Registry 入口的全局
        64 KiB 前置检查和调用次数测试收敛。
Action: Task 3 先写 exact/over-limit 测试，再在任何 clone/decode 前拒绝。

Finding: Tool 的 1 MiB 输出检查发生在 Catalog 已 unmarshal/clone 持久化 JSON 之后。
Product requirement affected: 持久化读取的资源耗尽边界。
Classification: IMPLEMENTATION_TEST
Reason: 问题真实，但不要求新增事务或恢复机制；Catalog publication/read 的 encoded
        payload guard 可以在实现阶段验证。
Action: Task 4 增加 B0 专用 8 MiB bounded read guard，并保留 Tool 1 MiB 输出 guard；
        Round 2 证明全局写前/读前 guard 会破坏兼容，因此不采纳全局限制。

Finding: Definition 的 3 秒 timeout 当前不覆盖 input/output schema validation。
Product requirement affected: timeout/deadline 的准确合同。
Classification: IMPLEMENTATION_TEST
Reason: 当前 Registry 明确在 preflight 后建立 execution deadline；输入输出已有独立
        字节上限，没必要为 B0 改写全局调用状态机。
Action: 将合同明确为 3 秒 Executor timeout；测试 schema 阶段的独立字节边界和 audit
        timeout，不宣称严格端到端 3 秒。
```

Security Review 没有 finding。Round 1 的三项均不命中 AGENTS.md 的 Blocker 后果。

### 15.2 Round 2 Finding 分类与重新冻结

```text
Finding: 配置化 platform-admin user/role 只获得 listingkit.platform_admin，未获得
         listingkit.admin.read；task scope 与 Executor 又调用未配置的默认 authorizer。
Product requirement affected: 配置管理员的正确授权、同 tenant owner bypass 和核心 happy path。
Classification: BLOCKER
Reason: 命中“错误授权或绕过明确的访问控制”以及“核心 happy path 按当前设计无法完成”。
Action: 配置管理员同时获得 listingkit.admin.read；ListingKitAuthorizer 实现窄
        TenantAdminChecker；composition root 将同一个配置化实例注入 preflight adapter、
        task repository 与 Executor，并用 configured user/role 及 cross-tenant 测试证明。

Finding: 8 MiB 被实现为共享 Catalog publication/read 全局限制，会拒绝现有 durable
         snapshot 的合法读取。
Product requirement affected: 兼容 rollout、已有 canonical product 的持续可读性。
Classification: BLOCKER
Reason: 命中“核心 happy path 按当前设计无法完成”和“rollout / migration 按当前方案无法
        安全完成”。
Action: 删除共享 Catalog 上限；新增只暴露 CompleteSnapshotReader 的 B0 bounded adapter，
        在 materialization 前执行 8 MiB 限制；测试共享 repository 仍能发布/读取大快照。

Finding: Task Reader 的数据库故障作为未知错误进入 Tool internal。
Product requirement affected: 稳定错误合同和可重试依赖故障。
Classification: IMPLEMENTATION_TEST
Reason: 不改变所有权、事务或状态机；可在现有 Port 和 adapter 中稳定映射。
Action: 增加 ErrCanonicalSubjectUnavailable，保留 context 错误，Executor 映射为
        dependency_unavailable，并用关闭数据库的真实 repository 测试证明。

Finding: 超过 64 KiB 的原始参数虽提前拒绝，但失败审计仍哈希完整攻击载荷。
Product requirement affected: 请求大小与资源耗尽边界。
Classification: IMPLEMENTATION_TEST
Reason: 不改变架构或业务状态；可在 Registry 入口丢弃原始字节并使用固定审计标记。
Action: oversized invocation state 不 clone、不保留、不哈希原始 Arguments，只记录固定
        SHA-256 标记；测试断言 state 中没有原始载荷且审计仍完整记录失败。
```

两项 Blocker 已按上述最小边界修复，且未新增 IAM、repository owner 或全局数据约束。
设计重新达到 `IMPLEMENTATION_READY`；按“两轮正常评审”上限冻结此基线。

### 15.3 Final Implementation Review 分类

```text
Finding: 历史 admin alias 拥有 listingkit.platform_admin，却没有 B0 所需的
         listingkit.admin.read。
Product requirement affected: 已承认平台管理员的核心 happy path。
Classification: BLOCKER
Reason: 命中“核心 happy path 按当前设计无法完成”；这是同一授权根因的 sibling path。
Action: 为 admin alias 补齐 read permission，并加入 auth adapter 与真实 Registry
        conformance matrix；tenant 边界仍由共享 checker 强制。

Finding: bounded reader 在 Go 中检查长度前，GORM 已扫描完整 SnapshotJSON。
Product requirement affected: 持久化读取的资源耗尽边界。
Classification: IMPLEMENTATION_TEST
Reason: 不改变所有权或错误合同，可由 persistence query projection 收敛。
Action: bounded reader 使用数据库方 byte-length 条件投影，超限时返回 NULL payload；测试
        同时断言 SQL 包含 CASE guard、exact 成功、over-limit 失败和共享 reader 兼容。

Finding: dynamic map 的 propertyNames.not 只检查 enum 内容，没有验证 negated schema 的
         完整语义；附加不匹配字符串键的约束时会重新允许保留 authority 字段。
Product requirement affected: 严格 Schema 必须在注册时排除全部保留 authority 字段。
Classification: IMPLEMENTATION_TEST
Reason: runtime scan 仍会拒绝攻击载荷，不产生错误授权或跨租户访问；问题可在 schema
        注册校验与回归测试内收敛，不改变冻结架构。
Action: 只承认精确的 not:{enum:[...]} 证明形状，拒绝带 type、ref 或组合关键字的
        negated schema；回归测试证明无效组合在 Registry 编译阶段失败。
```

以上问题按实现证据修复，不引入第三轮正常 Architecture Review。冻结状态继续为
`IMPLEMENTATION_READY`。
