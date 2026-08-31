# Phase 2A 商业工具（Commerce Tool）基础设计

## 1. 状态与结论

状态：已于 2026-08-31 确认设计方向。本文档通过审阅后，才进入实施计划阶段。

本设计决定：在 Product Agent 实现前，先建立一套小而稳定、与 Agent 框架无关的
Commerce Tool 合同，再通过一个真实的只读纵向切片证明该合同可用。Tool 边界复用
现有权限、可信身份、AI 能力治理、链路追踪、工作流和 Schema 组件，不建立第二套
权限系统、AI 模型路由、重试引擎、商品模型或工作流运行时。

首个真实 Tool 为 canonical product 检查工具。它通过窄应用/领域服务读取数据，将
source lineage 与 canonical facts 分开返回，并且只计算确定性的证据诊断结果。它不
修改商品状态、不调用付费模型、不生成平台发布载荷，也不执行发布。

实现必须排在当前进行中的目标目录架构 Phase 2 合并之后。该架构任务正在修改
`go.mod`、配置、`internal/app/httpapi`、Feature Flag 装配和平台所有权。如果在旧
基线上并行修改 Tool Runtime，会产生不必要的合并冲突，并诱导新代码继续依赖即将
退休的包。

## 2. 背景

Phase 2A 的权威要求记录在以下位置：

- `docs/product/ai-commerce-agent-platform-strategy.md`；
- `docs/refactoring/current-refactoring-status.md`；
- GitHub Issues #128、#133、#134；
- `docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`；
- `docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md`。

这些文档要求建立唯一的 Tool 合同，至少包含：

- 稳定的 Tool 标识和版本；
- 输入和输出 Schema；
- `read`、`compute`、`propose`、`write`、`publish` 风险声明；
- tenant/user 权限要求；
- timeout、retry、idempotency 和 side effect 所有权；
- 成本和用量元数据；
- 确定性错误分类；
- 审计和 trace 元数据；
- 与 Agent 精确 allowlist 的绑定。

Phase 2A 期间只允许执行 `read`、`compute`、`propose` Tool，`write` 和 `publish`
保持禁用。Agent 代码不得访问 GORM Repository、Provider SDK、Marketplace Client
或某个 Agent 框架专用的 DTO。

## 3. 仓库现状与复用结论

### 3.1 已有 AI Capability 治理能力

`internal/aicapability` 已经拥有：

- Provider-neutral AI capability 和 operation；
- Model Catalog 和 Tenant Model Policy；
- 路由、fallback、routing mode 和 execution plan；
- tenant、user、business task、trace 和 idempotency 元数据；
- 模型调用记录、用量、成本、延迟、哈希和结果状态；
- AI 专用标准错误，包括 budget、Agent step limit、Tool denial 等分类。

Commerce Tool 不得复制这些能力。AI-backed Tool Executor 必须把模型选择、凭据、
Provider 重试、成本计算和模型调用记录委托给 `internal/aicapability`。确定性 Tool
不能伪装成模型调用写入 AI Invocation Ledger。

### 3.2 已有可信身份与 Casbin 权限体系

`internal/authidentity` 在 context 中保存已经验证的 tenant、user 和 roles，并将
tenant/user scope 同步到 `internal/shared/aiidentity`。`internal/authz` 已使用 Casbin
管理 ListingKit 权限。ListingKit Repository 的读取也已经按 tenant 和 owner 执行
fail-closed 可见性检查。

因此，Commerce Tool 只定义可注入的 Principal 和 Authorizer Port。生产适配器只从
可信 context 解析身份，并把策略判断委托给现有 Casbin Authorizer。Tool JSON 输入
永远不能把 tenant ID、user ID 或 roles 当成权限依据。

### 3.3 已有 Schema 与可观测组件

仓库已经锁定以下依赖：

- 通过 `kin-openapi` 间接使用的 `github.com/santhosh-tekuri/jsonschema/v6`；
- OpenTelemetry API 和 HTTP instrumentation；
- 包含 `semver` 能力的 `golang.org/x/mod`。

只有在生产代码实际引用时，才把对应依赖提升为 direct dependency。不得新增第二个
JSON Schema Validator、自制 SemVer Parser 或新的 Trace 框架。

### 3.4 已有 Workflow 与 Feature Flag 所有者

Temporal 继续拥有跨进程、可恢复的业务执行和 workflow-level retry。目标目录架构
Phase 2 分支正在引入隔离的 OpenFeature Runtime。Phase 2B 的 Product Agent 开关和
tenant allowlist 复用该 Runtime。Tool Registry 不成为工作流引擎，也不实现第二套
Feature Flag。

### 3.5 已有商品事实来源

仓库已经存在：

- `canonical.Product` 和字段 trace；
- 中立的 `product/sourcing.SourceEnvelope`；
- `catalog.ProductFacts` 和 `asset.Facts`；
- 独立于 canonical product facts 保存的 ListingKit source lineage；
- ProductEnrich 和 ProductImage 面向服务的接口。

Tool Adapter 只把这些现有值投影为有版本的输出，不建立 Agent 自有的 canonical
product、source envelope、asset catalog 或 marketplace rule store。

## 4. 任务与实施顺序约束

#134 不能在第一个变更中被诚实地一次性完成：

- source evidence 依赖仍未完成的 #30 受控 1688 闭环；
- marketplace readiness 依赖仍未完成的 #34 contract；
- ProductEnrich 和 ProductImage propose Tool 在语义上依赖 #130 的 AI Capability
  发布级验证，但 #134 当前没有明确写出该依赖；
- 目标架构迁移禁止新增生产代码继续导入 `internal/listingkit` 等退休根包。

正确顺序是：

1. 合并目标目录架构 Phase 2；
2. 实现与框架无关的 #133 contract 和 registry；
3. 通过目标方向的服务 Port 接入一个真实 canonical product inspection 切片；
4. 在 #30 和相关 product 所有权迁移后接入 source/facts Tool；
5. 在 #130 验证通过后接入 ProductEnrich/ProductImage propose Tool；
6. 在 #34 和 marketplace 规则所有权稳定后接入 readiness/rule Tool。

在声明 AI-backed propose Tool 完成前，#134 应补充对 #130 的依赖。本文档不直接修改
GitHub Issue 状态。

## 5. 目标

- 让 fake Agent Runtime 和未来真实 Agent Runtime 使用同一套稳定 Tool Contract。
- 明确每个 Tool 的 owner、schema、risk、permission、side effect、timeout、retry
  owner、idempotency 和 usage owner。
- 保证模型只能提供业务参数，不能提供执行权限。
- 在服务端同时校验 Tool input 和 output。
- 将 Agent Definition 绑定到精确 Tool ID 和版本。
- Phase 2A 全程禁止 `write` 和 `publish`。
- 复用领域服务和现有治理能力，不暴露内部存储或客户端。
- 让首个真实 Tool 在不调用付费模型的情况下独立测试。
- 在目录迁移期间保持目标依赖方向。

## 6. 非目标

- 实现 Product Agent 推理或引入 Agent Framework。
- 在 Phase 2A 引入 Eino、Google ADK、LangGraph 或其他 Agent Runtime。
- 替换固定 ProductEnrich、ProductImage、RabbitMQ 或 Temporal 流程。
- 构建动态 Plugin Loader 或运行时 Tool 安装机制。
- 暴露通用的公网 HTTP Tool Execution API。
- 增加 write、save-draft、publish 或任意网络访问 Tool。
- 持久化 Agent chain-of-thought 或原始敏感 Tool 载荷。
- 在 Registry 变更中顺便完成 #30、#34、#130 或整个 #134。
- 把 product/listing 包移动混入 Tool 功能实现。

## 7. 所有权与依赖方向

永久合同属于独立包：

```text
internal/commercetool
```

它不进入 `internal/kernel/module`。Kernel Registry 目前只负责启动期装配：routes、
worker pools、Temporal workers、task handlers 和 workflow names。Tool lookup、权限、
Schema、风险策略和执行审计属于运行时安全行为。把两者合并会使 Kernel Registry 演变
为跨领域 god registry。

依赖方向如下：

```text
Agent Runtime（Phase 2B）
  -> commercetool Registry / BoundToolSet
      -> 领域所有者提供的 Tool Adapter
          -> 窄领域/应用服务 Port
              -> 现有领域实现

app composition
  -> 构造领域 Adapter
  -> 构造 authorization / identity / audit Adapter
  -> 创建不可变 Registry

AI-backed 领域 Adapter
  -> 现有 aicapability 治理
  -> 现有 ProductEnrich 或 ProductImage 服务接口
```

`internal/commercetool` 禁止导入：

- Agent Framework；
- Gin、Temporal、RabbitMQ 或 GORM；
- Provider SDK；
- Marketplace Client；
- ListingKit 实现包；
- Product、Asset 或 Marketplace DTO 包。

领域 Adapter 放在其目标业务所有者中，例如未来的 `internal/product/.../tools`、
`internal/marketplace/.../tools` 或 `internal/listing/.../tools`。App Composition 可以
同时导入 Contract 和各领域所有者，但不能包含 Tool 翻译或业务规则。

不得在根 `internal/listingkit` 下新增 Adapter，也不得让新 Tool 包导入该退休根包。
如果目标方向的窄读服务尚不存在，应等待或提取该服务，而不是制造永久兼容依赖。

## 8. 合同模型

具体 Go 文件布局可在实施中微调，但以下公开语义由本设计固定。

### 8.1 工具标识

Tool Definition 包含：

```go
type ToolRef struct {
    ID      string
    Version string
}

type Definition struct {
    Ref          ToolRef
    Capability   string
    Owner        string
    Description  string
    InputSchema  json.RawMessage
    OutputSchema json.RawMessage
    Risk         RiskLevel
    Permission   PermissionRequirement
    SideEffects  SideEffectPolicy
    Idempotency  IdempotencyPolicy
    Timeout      TimeoutPolicy
    Retry        RetryPolicy
    Usage        UsagePolicy
}
```

`ID` 是稳定语义名称，例如 `product.canonical.inspect`。`Version` 是带 `v` 前缀的
精确语义版本，通过 `golang.org/x/mod/semver` 校验。`Capability` 是
`product.canonical` 之类的 Commerce Domain 分组，不是 AI Provider Capability 的
重复定义。

AI-backed Adapter 内部继续使用真正的 `aicapability.Capability` 和
`aicapability.Operation`。Commerce Tool 不重新定义 Provider、Model、Routing、
Pricing 或 Prompt 类型。

### 8.2 风险等级

Definition 识别全部长期风险值：

```text
read
compute
propose
write
publish
```

Phase 2A Execution Policy 将风险上限硬编码为 `propose`。完整 Definition 可以被识别，
但被注册不代表可执行。绑定或调用 `write`、`publish` 必须返回确定性策略错误。未来
开放这些风险需要新的设计评审，不能由 Tool 参数或 Agent 输出开启。

风险语义如下：

- `read`：通过已授权领域服务读取现有事实；
- `compute`：对输入或读取到的事实执行纯计算或确定性计算；
- `propose`：可以调用受治理 AI 并返回候选，但不能应用候选；
- `write`：修改本地业务状态；
- `publish`：创建或改变外部 Marketplace 可见状态。

### 8.3 副作用与幂等性

每个 Definition 必须同时声明这两个字段，缺失时 Registry 构造失败。

Phase 2A 可执行 Tool 不得修改业务状态。`propose` Tool 可能消耗受治理 AI 资源，因此
必须声明 AI Capability Ledger 是 usage owner，并在底层受治理操作需要时要求
idempotency key。

最小 Idempotency Mode：

```text
not_applicable
deterministic
required_key
```

Tool 边界不实现第二个 Idempotency Store。它只校验声明和 key，把 key 传给所有者
服务，并依赖该服务现有的 effect/ledger 语义。

### 8.4 超时与重试所有权

每个 Tool 都有由 Tool Invoker 强制执行的端到端硬超时。内部所有者可以使用更短的
timeout，但不能延长 Tool Deadline。

每个 Definition 只能声明一个 Retry Owner：

```text
none
caller
ai_capability
domain_workflow
```

Registry 绝不自动重试 Executor。确定性计算通常使用 `none`；安全读取可以使用
`caller`；AI Provider Attempt 由 `ai_capability` 负责；持久业务重试继续归 Temporal
或领域 Workflow 所有。同一失败范围不能有两个 Retry Owner。

### 8.5 成本与用量所有权

Usage Metadata 声明 Tool 属于：

- 不计量的确定性/读取工作；
- 由现有 AI Capability Ledger 计量；
- 由另一个明确存在的领域 Ledger 计量。

它不包含 Provider Pricing 逻辑。AI-backed Executor 将 child model invocation 关联到
Tool Call ID 和 Agent Run ID，并由 `internal/aicapability` 记录 model、policy、prompt、
token、image、latency 和 cost。

## 9. 模型可见参数与可信调用元数据

模型可见的 Tool Schema 只包含业务参数。首个 Tool 的输入只有：

```json
{"task_id":"..."}
```

Runtime 单独构造可信 Call Metadata：

```go
type CallMetadata struct {
    CallID          string
    AgentID         string
    AgentVersion    string
    AgentRunID      string
    BusinessTaskID  string
    TraceID         string
    IdempotencyKey  string
}
```

tenant、user 和 roles 由 `PrincipalResolver` 从可信 context 解析，不能出现在模型参数
或输出中。未来持久化 Agent Runtime 必须先从服务端拥有的 Agent Run Record 恢复可信
Principal，再调用 Tool，不能信任模型生成的身份值。

API 和测试必须强制这种分离。Tool 参数 JSON 默认拒绝未声明属性，防止夹带的
`tenant_id`、`user_id`、`roles`、`permission` 或 `tool_version` 成为权限依据。

## 10. 权限模型

Core Contract 只定义小型 Port，不直接导入 Casbin 或 HTTP Identity Package：

```go
type PrincipalResolver interface {
    ResolvePrincipal(context.Context) (Principal, error)
}

type Authorizer interface {
    Authorize(context.Context, Principal, PermissionRequirement) error
}
```

生产 Adapter 必须：

- 从 `authidentity` 或可信恢复 context 解析 tenant、user 和 roles；
- 把权限判断委托给现有 Casbin Authorizer；
- 永远不使用 Tool 参数中的身份 fallback；
- 对缺失或不完整身份执行 fail closed。

不同职责下执行两次权限保护：

1. Tool Invoker 校验 Agent allowlist、risk、identity 和声明的 permission。
2. 领域 Service/Repository 保留自己的 tenant/owner 可见性检查。

第二次检查不是重复劳动，它可以防止过宽 Tool Permission 绕过具体记录的 owner scope。

Canonical Inspection Tool 初期复用现有 ListingKit Read Permission，不新建平行 Agent
Role Model。只有出现具体的非 ListingKit Product Read Path 且权限语义确实不同，才
考虑新增 Commerce-wide Permission。

## 11. JSON Schema 校验

Input Schema 和 Output Schema 都是必填项，并在 Registry 构造时编译。Schema 无效或
使用不支持的能力时，Registry 不得创建成功。

实现使用 `github.com/santhosh-tekuri/jsonschema/v6`，规则如下：

- 所有 Tool 使用同一个受支持的 JSON Schema Draft；
- Object Input/Output 默认 `additionalProperties: false`；
- Executor 解码前校验 Input；
- 返回 Agent Runtime 前校验 Output；
- Schema 错误必须标准化，不能泄漏 Go 类型名或敏感原始载荷；
- 使用 focused fixtures 证明接受和拒绝的结构；
- Typed Adapter Test 检测 Go DTO 序列化与 Schema 的漂移。

首个切片不引入 Schema Generator。对于一个 Tool，手写最小 Schema 更清晰，也避免
增加第二个生成器依赖。如果将来出现可量化的重复，再单独评估生成方案，并使用
Contract Diff Test 保护兼容性。

## 12. 注册表与 Agent 绑定

Registry 创建后不可变。它由有限个完整 `Tool` 值构造，每个 Tool 都包含 Definition
和 Executor。

以下情况必须构造失败：

- ID/version/capability/owner 为空或格式错误；
- 精确 Tool Ref 重复；
- Schema 缺失或无效；
- permission、risk、side effect、idempotency、timeout、retry、usage 声明缺失；
- Executor 为 nil；
- 声明互相矛盾，例如明确由 AI 治理的 propose Tool 却声明为 unmetered。

Agent Definition 包含精确 Allowlist：

```go
type AgentDefinition struct {
    ID           string
    Version      string
    AllowedTools []ToolRef
}
```

绑定过程校验每个 Tool Ref，并返回 `BoundToolSet`。Agent Runtime 只能拿到该集合，不能
从 Global Registry 枚举或直接调用 Executor。Definition 查询可以公开安全元数据，但
任何公开 API 都不能返回不受保护的 Executor。

Product Agent Allowlist 固定精确版本。新增 Tool Version 不能静默替换既有 Agent
Definition 使用的版本。

## 13. 调用顺序

`BoundToolSet` 的调用流程：

1. 校验 Call Metadata 和精确 Agent Identity；
2. 只能从 Bound Allowlist 解析 Tool；
3. 强制执行 Phase 2A Risk Ceiling；
4. 解析可信 Principal；
5. 校验声明的 Permission；
6. 校验 Idempotency Requirement；
7. 校验 JSON Input Schema；
8. 创建端到端 Timeout Context；
9. 启动 OpenTelemetry Span 并记录安全 Call Metadata；
10. 精确执行一次领域 Adapter；
11. 标准化领域错误；
12. 校验 JSON Output Schema；
13. 记录终态 Tool Audit Metadata，返回结构化结果。

Invoker 绝不重试第 10 步。Executor 已执行后，即使 Recorder/Exporter 失败，也不得
自动重放 Executor。该失败通过 Observability 和安全的 Audit Status 暴露，同时保留
已经产生的 Result。未来开放 write/publish 前，必须为其单独设计持久化 begin/commit
审计协议。

## 14. 确定性工具错误

Tool Error 的范围比 AI Provider Error 更广。如果强制所有确定性领域读取使用
`aicapability.ErrorCategory`，会错误地让 AI Control Plane 拥有非 AI 失败。因此
Commerce Tool 定义一个小型边界错误分类：

```text
invalid_input
identity_integrity
permission_denied
tool_not_allowed
not_found
failed_precondition
conflict
deadline_exceeded
dependency_unavailable
output_invalid
budget_exceeded
unknown_execution_state
internal
```

每个 Error Code 有固定 Retryability 规则，Adapter 不能随意把 Internal Error 或
Permission Error 标记为 retryable。返回模型的错误只包含安全 code 和 message，不能
包含 Provider Payload、Credential、SQL Error 或 Marketplace Raw Response。

AI-backed Adapter 将现有 `aicapability.ErrorCategory` 确定性映射到该边界分类，同时
在 AI Invocation Ledger 中保留详细 AI Category。Domain Adapter 只映射已记录的
Sentinel/Typed Error，未知原因统一降级为 `internal`。

## 15. 审计、Trace 与敏感数据处理

Tool Call Audit Metadata 包含：

- Tool Call ID；
- Parent Agent ID/version/run ID；
- 精确 Tool ID/version/capability/owner；
- 从可信 context 获取的 tenant/user/business-task/trace 标识；
- risk、permission、retry owner 和 usage owner；
- 开始/结束时间与 latency；
- input/output hash；
- outcome 和标准化 Error Code；
- 关联的 AI Invocation ID（如适用）。

Generic Tool Audit Record 不存储 Raw Tool Input/Output、Source Snapshot、Prompt、
Provider Response、Credential、Cookie 或 Marketplace Token。需要人工审核的候选内容
继续由其领域业务存储作为权威来源。

OpenTelemetry 负责 Span 与传播。小型 Tool Audit Recorder Port 可以输出结构化、非
敏感事件，但它不替代 AI Invocation Ledger，也不建立第二个 Billing Ledger。

## 16. 首个真实纵向切片

### 16.1 工具标识

```text
tool_id:       product.canonical.inspect
version:       v1.0.0
capability:    product.canonical
risk:          read
side_effects:  none
idempotency:   deterministic
retry_owner:   caller
usage_owner:   unmetered
permission:    existing ListingKit read permission
```

### 16.2 输入

模型可见输入只有非空 `task_id`。tenant、user、roles、Agent Identity、Tool Version
和 Trace 均不能出现在 JSON 中。

### 16.3 所有者服务

Adapter 调用 Consumer-owned Narrow Read Port。生产 Port 必须保留现有 tenant/owner
过滤，并且只返回投影需要的数据：

- canonical product snapshot；
- 位于 canonical product 之外的 source reference/lineage；
- 响应关联所需的 task identity。

Tool 不直接调用 GORM Repository 或 Canonical Cache，也不依赖 Broad ListingKit
Service Interface。如果实施开始时目标方向的 Read Port 尚未提取，则本纵向切片负责
提取该窄 Port，并使用现有读取行为测试证明兼容性。

### 16.4 输出

输出包含：

- task ID；
- 现有 canonical product 的只读投影；
- 独立字段中的 source lineage；
- 根据 field trace 和现有 canonical review helper 计算的确定性 diagnostics；
- 总体 `needs_review` 值。

该投影不是第二套 Product Fact Model。它不拥有持久化，也不能作为 canonical write
payload 提交。Contract Test 必须证明它只是权威领域值的只读投影。

明确排除 Marketplace Readiness。#34 和各 Marketplace Rule Owner 尚未完成时，
`product.canonical.inspect` 不得猜测 SHEIN/TEMU/Amazon 的 Category、Attribute、Image
或 Publishing Rule。

### 16.5 错误映射

- task ID 为空或格式错误 -> `invalid_input`；
- 缺失可信身份 -> `identity_integrity`；
- Casbin 拒绝 -> `permission_denied`；
- tenant/owner 不可见 -> `not_found`；
- task 存在但没有 canonical product -> `failed_precondition`；
- 读取超时 -> `deadline_exceeded`；
- 非预期服务错误 -> `internal`。

## 17. 后续工具适配器

后续 Adapter 继续复用同一 Contract，并且只有其 Owner 与 Dependency Gate 准备完成后
才能接入：

| Tool 家族 | 必要 Owner/Gate | Phase 2A 行为 |
| --- | --- | --- |
| source evidence | #30 和中立 sourcing read port | 读取 evidence 和 warning |
| catalog/asset facts | product catalog/asset 所有权 | 读取 normalized facts |
| ProductEnrich | #130 governed execution evidence | 只 propose text/product patch |
| ProductImage | #130 governed vision/image execution | 只 analyze/propose action |
| marketplace rules | marketplace-owned rule service | 读取 category/attribute rule |
| readiness validator | #34 deterministic contract | 计算 blocker/warning |

AI-backed Adapter 返回 proposal、evidence、confidence、unresolved issue 和 review
guidance，绝不应用 proposal。未来任何人工批准的变更仍必须由领域服务重新校验。

## 18. 应用装配与运行时的关系

App Composition 构造具体领域 Tool 和不可变 Registry。它可以通过显式 Builder 收集
Tool 列表，但不能把 Tool Runtime 行为加入 `kernel/module.Registry`。

Phase 2A 不暴露用户可访问的 Tool Execution Endpoint。测试和小型 Conformance
Harness 直接使用 Registry。Phase 2B Agent Runtime 获得 Bound Tool Set，并将其适配
到选定框架。Framework Adapter 负责翻译：

- Tool Definition -> Framework Tool Declaration；
- Framework Arguments -> 经过 JSON 校验的 Invocation；
- Tool Result/Error -> Framework-safe Structured Result。

任何 Framework Type 都不能进入 `internal/commercetool` 或领域服务。

Product Agent Feature Flag 和 Tenant Allowlist 属于 Phase 2B，并复用 OpenFeature。
它们不能使 write/publish 变得可执行；Tool Risk Ceiling 仍是独立服务端 Gate。

## 19. 开源组件策略

Phase 2A 复用或提升现有组件：

| 关注点 | 现有组件/所有者 | 决策 |
| --- | --- | --- |
| 权限策略 | Casbin | 通过 Authorizer Adapter 复用 |
| 可信身份 | authidentity + aiidentity | 复用；Tool JSON 不携带身份 |
| JSON Schema | santhosh-tekuri/jsonschema/v6 | 提升为 direct dependency |
| 语义版本校验 | golang.org/x/mod/semver | 复用 |
| Trace | OpenTelemetry | 复用 |
| Durable Workflow | Temporal | 保持在 Tool Invocation Loop 外部 |
| Feature Flag | 架构 Phase 2 引入的 OpenFeature Runtime | Phase 2B 复用 |
| AI Routing/Cost/Ledger | internal/aicapability | 复用；不建第二套 AI Control Plane |

Phase 2A 不引入 Agent Framework。进入 Phase 2B 后，再把 Eino 或其他候选作为可替换
Runtime Adapter 对照本合同评估，而不是先选择框架再反向定义合同。

## 20. 交付切片

### 切片 A：#133 合同与不可变注册表

- 增加 Contract Type 和校验；
- 增加 JSON Schema 编译及 Input/Output 校验；
- 增加不可变 Registry 和精确 Agent Allowlist 绑定；
- 增加 Principal/Authorizer/Audit Port；
- 增加 Risk、Timeout、Retry、Idempotency、Usage、Error Policy；
- 使用测试证明 write/publish 被拒绝及 Framework Import Boundary；
- 不增加公网 Endpoint 或 Agent Framework。

### 切片 B：Canonical Inspection 纵向切片

- 增加或提取目标方向的 Canonical Read Port；
- 增加领域所有者 Tool Adapter；
- 接入现有 Casbin/Identity 和 Trace Adapter；
- 增加 Typed Projection/Schema Parity Test；
- 增加 Tenant/Owner Isolation 与禁止 Repository 直连的 Guard；
- 通过 Fake Agent Test 使用的同一个 Registry Contract 执行。

### 切片 C 及以后：按依赖门禁接入适配器

- #30/product ownership 后接入 source/fact Tool；
- #130 后接入 AI proposal Tool；
- #34 和 marketplace ownership 后接入 readiness/marketplace Tool。

切片 A 可以关闭 #133。切片 B 记录 #134 的部分证据。只有 Product Agent 所需的最小
Tool Set 和所有声明的 Dependency Gate 真实完成后，#134 才能关闭。

## 21. 测试与架构护栏

### 21.1 合同测试

- 拒绝不完整 Definition；
- 拒绝无效 SemVer 和重复 Tool Ref；
- 拒绝无效 Schema 和 Schema Mismatch；
- 拒绝未声明 Input Property；
- 拒绝缺失可信身份和 Permission Failure；
- 拒绝非 Allowlist Tool 和 Version Mismatch；
- 在 Phase 2A Policy 下拒绝 write/publish；
- 强制 Timeout 和 Exactly-once Executor Invocation；
- 证明 Registry 不会重试 Executor；
- 确定性标准化领域错误与 AI Capability Error；
- 证明 Recorder Failure 不会重放已完成 Executor。

### 21.2 首个工具测试

- 成功读取 canonical product，并单独返回 source lineage；
- Cross-tenant 和非 owner 读取返回 not found；
- Tenant Admin/Platform Admin 行为与现有 Access Semantics 一致；
- 缺失 canonical product 返回 failed precondition；
- Diagnostics 完全确定性，不依赖网络或模型；
- Input/Output Fixture 与已编译 Schema 一致；
- Output 不能被当作 Write Command。

### 21.3 导入边界护栏

- `internal/commercetool` 禁止导入 Agent Framework、HTTP、Workflow、Persistence、
  Provider、Marketplace 或领域实现包；
- 领域 Tool Adapter 禁止导入 GORM Store 或 Provider SDK；
- AI-backed Adapter 必须调用 Governed Capability/Service Boundary；
- 本任务不得新增生产代码导入根 `internal/listingkit`；
- App Package 只负责构造。

### 21.4 验证命令

每个切片执行：

- Focused Package Test；
- 被修改领域服务的 Reverse-dependency Test；
- Repository Import Boundary/Depguard Test；
- 环境无关时执行 `go test ./... -count=1`；
- `go vet`、配置的 lint 和 code-health verification；
- `git diff --check`。

必须记录 Exact Commit 和任何环境依赖的测试排除项。

## 22. 失败与回滚行为

- Registry 构造失败时 Tool Runtime 不可用，但不能影响现有固定 Product/Listing Flow。
- Tool 失败返回结构化错误，canonical product 状态保持不变。
- Phase 1 和 Phase 2A Exit Evidence 齐备前，Product Agent 保持 Feature Disabled。
- Agent Runtime 被关闭或失败时，现有固定流程继续可用。
- 新 Tool Version 必须通过新的精确 Agent Allowlist Entry 显式启用。
- Phase 2A 不修改业务状态，因此 Tool 实现不需要数据库回滚。

## 23. 被否决的方案

### 扩展 `kernel/module.Registry` 注册表

否决原因：启动期 Contribution Collection 与受控运行时 Tool Execution 的所有权和安全
语义不同。合并后会产生 God Registry，并让无关模块耦合 Agent Policy。

### 用一个变更完成全部 #134 适配器

否决原因：#30、#34、#130 尚未完成，当前 Package Owner 也正在迁移。强行实施只能
产生虚假能力或新增 Legacy Import。

### 使用 Eino/ADK 工具类型作为领域合同

否决原因：会提高 Framework Replacement 成本，并让 Runtime Library 决定领域 DTO、
Error、Identity 和 Retry 行为。

### 让 Agent 运行时调用 Repository 或 Provider Client

否决原因：会绕过 Tenant/Owner Check、复制 Service Logic、泄漏 Credential，并产生
独立的 Retry/Effect Owner。

### 所有工具调用都复用 AI Invocation Record

否决原因：确定性读取/计算不是模型调用。这样会污染 Model、Cost 和 Provider 语义。
只有 AI-backed Executor 运行时，Tool Audit 才关联真实 Child AI Invocation Record。

### 新增通用 RBAC、Trace、Schema、Workflow 或 Feature Flag Library

否决原因：仓库已经具备 Casbin、OpenTelemetry、JSON Schema、Temporal 和正在落地的
OpenFeature Runtime。

## 24. 验收条件映射

### 任务 #133

切片 A 的证据满足以下条件时，本设计完成 #133：

- 所有必要元数据均为必填且经过校验；
- Agent Runtime 只能获得精确 Allowlist 中的 Tool Version；
- 缺失 Schema/Permission/Side-effect Declaration 时构造失败；
- write/publish 不可执行；
- Contract 不导入 Agent Framework。

### 任务 #134

切片 B 证明同一合同能连接真实领域服务，并为 #134 提供部分证据。完整 #134
还要求 Product Agent 最小集合中声明的 source、facts、proposal、marketplace、validator
能力分别通过其 Owner/Dependency Gate。

### Phase 2A 退出门禁

只有 Fake Agent 和 Product Agent Integration Test 使用同一 Registry Contract 完成
read、compute、propose，并且不存在第二套 Compatibility Interface，Agent 也没有任何
Repository、Provider SDK 或 Marketplace Client 直连能力时，Phase 2A 才能退出。
