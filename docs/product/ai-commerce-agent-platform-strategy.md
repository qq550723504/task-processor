# AI Commerce Agent Platform 产品战略

> 状态：Active strategic direction；产品 UI / IA 以最终 Figma Authority 为准  
> 原战略日期：2026-08-12；ListingKit 退休与产品投影校准：2026-09-26  
> 历史成熟度基线：`master@beb6e81638e1400e7cb1266c825e65aee23713c5`（不是当前代码/验收状态）  
> 适用对象：产品、研发、运营、QA、商业化与架构负责人

## 1. 战略决策

`task-processor` 的长期产品目标调整为：

> **构建 AI 驱动的新一代跨境电商智能体平台，让用户从“操作软件功能”逐步转向“给 AI 一个电商目标，由 Agent 调用受控电商能力完成工作”。**

面向用户的产品为硕米智能引擎。其导航、页面命名、交互语义和能力投影以 [最终 Figma UI / IA Authority](final-ui-ia-authority.md) 中当前可见、非归档终稿为准；本文的技术能力分解不能成为另一套产品菜单。

这不是一次推倒重建，也不是把现有确定性工作流改写成大模型工作流。当前有效的 Product Sourcing、Product Enrichment、Product Image、Marketplace、Temporal/RabbitMQ、ZITADEL 与 AI Capability 能力继续按各自 owner 复用；旧 ListingKit / Task-first 的混合职责和产品投影必须按 EXTRACT → RETIRE 收口，不能整体保留为子产品或长期执行引擎。有效行为不等于旧 owner，详见 §9。

支撑 Figma 产品的能力关系是（不是导航或已开放清单）：

```text
硕米智能引擎 / Figma 产品页面
  ├─ BusinessTask / 业务任务投影
  ├─ Commerce Agents / Commerce Tools
  ├─ Product Sourcing / Catalog / Asset
  ├─ Product Enrichment / Image / Content
  ├─ Listing / Marketplace / Store
  ├─ Identity / Organization / Commercial / Resource
  └─ AI Control Plane / 受控执行基础设施
```

其中：

- **AI Commerce Agent Platform** 描述长期产品方向；最终 UI / IA 由 Figma 决定；
- **ListingKit** 是待退休旧架构，原有独立有效行为转到当前领域，不成为新功能默认落点；
- **AI Capability & Agent Platform** 是模型治理和 Agent Runtime 的底层技术边界；
- 工程先后、具体范围和可用状态由 #137、当前执行 Issue 及其准确代码/验收证据确定，不从历史路线图自动派工。

## 2. 为什么调整方向

### 2.1 当前项目已经超出“任务处理器”范畴

原战略基线记录了以下能力资产；这是方向依据，不是今天的全量验收清单：

- 多来源商品接入与中立化；
- canonical product / catalog / asset facts；
- AI 商品理解、文案、图片生成与编辑；
- 平台类目、属性、价格、图片和发布规则；
- SHEIN 生产主链路及 TEMU/Amazon 能力资产；
- Listing Control Plane；
- RabbitMQ 与 Temporal；
- 多租户、ZITADEL 身份与授权；
- Prompt 管理；
- AI Capability Router、Invocation Ledger 等模型治理基础。

继续把产品定义成“上架工具”或“任务处理器”，会低估这些能力之间的组合价值。

### 2.2 真正有复用价值的是电商能力，而不是单个平台页面

平台工作台会变化，第三方接口也会变化；但下面这些能力可以长期复用：

```text
商品发现
商品读取
商品理解
商品标准化
类目判断
属性推断
内容生成
图片处理
成本与价格计算
平台校验
草稿生成
人工审核
平台提交
失败恢复
```

这些能力天然适合作为受控 `Commerce Tools`，由 Agent 在明确边界内组合使用。

### 2.3 用户最终需要的是“完成业务目标”

传统工作台要求用户理解系统模块并逐步操作：

```text
导入 -> 生成 -> 改图 -> 补属性 -> 定价 -> 校验 -> 保存 -> 发布
```

Agent 化以后，目标交互可以变为：

```text
“把这个 1688 商品准备成适合 SHEIN 美国站销售的商品，
利润率至少 25%，图片做差异化处理，先生成草稿，不要直接发布。”
```

系统负责解释目标、制定有限计划、调用工具、校验结果、修复可修复问题，并在高风险动作前请求人工确认。

产品价值由“提供功能”升级为“交付工作结果”。

## 3. 产品使命

平台使命：

> **让跨境电商团队能够把选品、商品理解、内容与图片生产、平台适配、校验和上架等重复知识工作交给可审计、可约束、可恢复的 AI Agent，同时保留业务事实、平台规则和高风险操作的确定性控制。**

平台不追求“完全无人值守”。

目标是逐步建立：

```text
AI-first execution
+ deterministic guardrails
+ human approval
+ durable workflow
+ measurable business outcome
```

## 4. 核心产品原则

### 4.1 Agent 负责不确定性，领域服务负责确定性

适合 Agent 的工作：

- 商品语义理解；
- 类目候选判断；
- 缺失资料诊断；
- 属性候选推断；
- 文案与图片策略选择；
- 根据 validator 结果选择修复动作；
- 根据目标选择下一项只读或受控工具。

不应交给 Agent 自由决定的工作：

- 利润公式；
- 权限判断；
- 租户隔离；
- SKU 唯一性；
- 幂等控制；
- 平台确定性 validation；
- 业务状态机；
- 远端提交状态确认；
- 账务、配额和审计规则。

### 4.2 Tool-first，不复制业务能力

Agent 不重新实现商品、图片、平台和发布逻辑。

正确关系：

```text
Agent
  -> Tool Contract
    -> Existing Domain Service
      -> Provider / Marketplace / Database
```

Agent 只是受控调用方。

### 4.3 单一事实源

- 商品事实：商品领域拥有；
- 平台 Listing 草稿：平台/Listing 领域拥有；
- 用户业务任务：当前 BusinessTask 产品合同；内部执行/恢复由相应领域与 Temporal/RabbitMQ 路径负责，不由旧 ListingKit Task 充当产品事实；
- Agent 单次推理状态：Agent Runtime 拥有；
- 模型调用与成本：AI Invocation Ledger 拥有；
- 身份、租户与授权：ZITADEL 和现有授权边界拥有。

不得因为引入 Agent 建立第二份商品数据库、第二套发布状态机或第二套权限模型。

### 4.4 高风险动作必须显式授权

默认将工具分为：

```text
Read Tool
Compute Tool
Propose Tool
Write Tool
External Side-effect Tool
```

初期 Agent 只允许前三类。

保存草稿、修改价格、上传远端素材、创建平台商品、发布商品等操作必须经过明确策略与人工确认，后续才逐项开放。

### 4.5 每次 Agent 运行必须有界

至少限制：

- 最大 step；
- 最大模型调用次数；
- 最大工具调用次数；
- 最大运行时长；
- 最大估算成本；
- 最大修复循环次数；
- 工具 allowlist；
- provider / model policy。

Agent 不允许无限自循环。

### 4.6 受控开放，不以旧架构兜底

新 Agent 能力通过 feature flag、tenant allowlist 和评测门禁逐步启用；不可用时停止相应能力，不自动回接旧 ListingKit / Task-first 路径。

当前领域中仍有效的确定性流程按合同保留。这不是保留旧 Service、旧工作台或双写/fallback 的许可；Legacy 只有 EXTRACT / RETIRE。

## 5. 目标用户

### 5.1 跨境电商运营

希望用自然语言或目标式任务完成商品处理，而不是在多个页面和工具之间重复录入。

典型目标：

- “把这批商品准备成 SHEIN 可发布状态”；
- “找出这 50 个商品缺失的关键资料并补全候选”；
- “把这款商品做成适合美国站的差异化 Listing”；
- “只生成草稿，我审核后再发布”。

### 5.2 运营负责人

希望获得：

- Agent 工作量与成功率；
- 人工接管率；
- 自动修复率；
- 单商品处理成本；
- 平台 validator 通过率；
- 发布成功率；
- 失败原因分布。

### 5.3 商家或业务团队负责人

希望把一部分重复运营岗位工作转化为可配置的 AI Worker，同时保留权限、预算和审核权。

## 6. 用户交互模型

长期交互不应只存在 Chat 页面；下列三种交互方式必须投影到 Figma 已批准的页面，不另建平行菜单或恢复旧工作台。

### 6.1 Goal-based Agent Workspace

在 Figma「AI工作台」及其业务任务页面中，用户直接表达目标：

```text
“分析这款 1688 商品，生成 SHEIN 美国站资料，
售价保证毛利率至少 25%，先不要发布。”
```

Agent 返回：

- 对目标的结构化理解；
- 执行计划；
- 当前步骤；
- 使用过的证据和工具；
- blocker；
- 需要人工决策的问题；
- 最终候选结果。

### 6.2 Embedded Copilot

Agent 能力嵌入 Figma 对应的供应、店铺商品或其他已批准业务页面，复用当前 Console 与领域 owner；不以已有 ListingKit 页面为产品模板，不保留旧 Listing Workspace。

例如在属性区：

```text
诊断缺失属性
生成候选
解释依据
重新验证
```

在图片区：

```text
诊断图片问题
推荐处理方式
生成候选场景图
按当前产品合同进行程序校验与用户确认
```

具体图像步骤以当前批准流程为准；能力示例不要求每次额外调用模型质量评分，也不恢复已取消的 Extract / Review 步骤。

### 6.3 Batch Agent Jobs

用于批量商品：

```text
100 个来源商品
  -> Agent 批量诊断
  -> 自动处理低风险项
  -> 聚合阻断项
  -> 人工只审核例外
```

这是平台真正形成运营杠杆的入口。

## 7. 目标产品架构

下图为能力分层，不是导航；产品入口仍由 Figma Authority 决定。

```text
┌─────────────────────────────────────────────┐
│              Commerce Experience            │
│ Figma AI工作台 / 供应与店铺业务页面 / 账户等   │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│                 Agent Runtime               │
│ Plan / Tool Call / Repair / Checkpoint      │
│ Budget / Trace / Human Interrupt            │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│                Commerce Agents              │
│ Product / Listing / Sourcing / Content ...  │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│                 Commerce Tools              │
│ Source / Catalog / Image / Validator /      │
│ Pricing / Marketplace / Draft / Publish     │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│                Domain Capabilities          │
│ Product / Asset / Listing / Marketplace /  │
│ Store / Sourcing / Enrichment / Image       │
└──────────────────────┬──────────────────────┘
                       │
┌──────────────────────▼──────────────────────┐
│               Platform Foundation           │
│ AI Control Plane / Temporal / RabbitMQ /    │
│ ZITADEL / Storage / Observability           │
└─────────────────────────────────────────────┘
```

### 7.1 Commerce Experience

负责用户目标、任务进度、Agent trace 的产品化呈现，以及人工确认和接管。

### 7.2 Agent Runtime

只拥有单次 Agent 运行内部状态：

- Agent definition/version；
- step；
- tool invocation；
- structured state；
- checkpoint；
- interrupt/resume；
- budget；
- stop reason。

不接管跨业务生命周期的 durable orchestration。

### 7.3 Commerce Agents

Agent 是业务角色和能力组合，不是新的事实源。

原战略列出的三个能力方向为：

1. Product Agent；
2. Listing Agent；
3. Sourcing Agent。

这不是同时开工三个 Agent 的授权；先交付哪个有界用户结果，由当前 Issue 和 Roadmap 决定。

### 7.4 Commerce Tools

Tool 必须：

- 有明确 owner；
- 输入输出结构化；
- 有 tenant/user context；
- 可观测；
- 可超时；
- 有权限等级；
- 写操作具备 idempotency；
- 不允许 Agent 直接访问底层数据库绕过领域规则。

### 7.5 AI Control Plane

继续沿用现有 `internal/aicapability` 方向，集中管理：

- model catalog；
- tenant policy；
- routing；
- fallback；
- provider health；
- budget；
- invocation ledger；
- cost；
- prompt metadata；
- eval evidence。

## 8. 第一批 Commerce Agents

## 8.1 Product Agent

### 目标

把来源商品或当前 canonical product 转化成“事实更完整、证据清晰、可进入目标平台适配”的标准商品候选。

### 输入

- canonical product；
- source envelope / source evidence；
- asset references；
- validation report；
- target marketplace context。

### 工具

- 读取商品事实；
- 读取来源证据；
- 视觉分析；
- 文本分析；
- 属性候选生成；
- deterministic validator；
- 生成 `ProposedProductPatch`。

### 输出

```text
ProposedProductPatch
Evidence
Confidence
ValidationResult
UnresolvedQuestions
HumanReviewRequired
```

### 首个 PoC

继续使用现有设计确定的“商品资料诊断与补全 Agent”。

首版不允许 Agent 直接修改商品事实。

## 8.2 Listing Agent

### 目标

把一个已具备可靠事实的商品准备成目标平台可审核、可提交的 Listing。

### 典型执行

```text
load product
 -> determine category candidate
 -> map required attributes
 -> generate platform content
 -> prepare image requirements
 -> calculate pricing candidate
 -> deterministic readiness
 -> repair allowed blockers
 -> produce listing draft proposal
 -> human review
```

### 第一目标平台

**SHEIN。**

原战略基线以其已有平台能力验证 Agent 收益；这不是保留旧 ListingKit 工作台的依据，也不是所有当前路径均已通过生产验收的声明。

TEMU、Amazon 后续复制已经验证的 Agent + Tool 契约，而不是各自再建一套 Agent Runtime。

## 8.3 Sourcing Agent

### 目标

根据业务目标从多个商品来源寻找、过滤、分析和排序候选商品。

长期能力：

```text
Goal
 -> search/query source
 -> collect product facts
 -> normalize
 -> evaluate cost / category / image / competition signals
 -> deduplicate
 -> score
 -> explain recommendation
```

Sourcing Agent 不直接承担平台发布。

## 9. ListingKit 退休与有效能力承接

**ListingKit 需要退休，不作为硕米产品中的长期子产品、默认工作台或核心执行引擎。** 本节替代旧战略中保留 ListingKit 执行面的决定，和最终 Figma Authority、Legacy Policy、#29 的 EXTRACT → RETIRE 保持一致。

| 原有有效行为 | 当前承接方向 |
| --- | --- |
| 商品事实、来源证据、资产、内容/图片候选 | `internal/product/*` 及现有 AI capability，保持各自事实 owner。 |
| 平台规则、字段映射、定价与平台约束 | `internal/marketplace/*`，不由 root ListingKit 再持有。 |
| Listing 草稿、readiness、提交与恢复 | `internal/listing/*` 及既有执行 owner，保持唯一状态机。 |
| 外部客户端、持久化和运行依赖 | `internal/integration/*` 与 `internal/app/*`；应用层只装配。 |
| 用户界面与业务任务 | Figma「AI工作台 / 供应市场 / 店铺中心」等当前产品页面及 BusinessTask 合同，不恢复独立 Listing Workspace。 |

旧 root `internal/listingkit`、`internal/compatibility/listingkit`、Task-first 产品投影不是新代码落点。先抽取独立有效行为，切换当前调用方，再删除或停用原实现；不保留永久 facade、旧 Task 双模型、fallback 或双写。具体实现以当前 Legacy Register 和责任 Issue 为准，不在本战略中宣称退休已完成。

Agent 通过受控 Commerce Tools 调用当前领域 owner，不借旧 ListingKit facade 间接调用，也不直接拼远端 payload 或写提交状态。`web/listingkit-ui` 内合格 Console / BFF / 组件按实际职责复用；历史路径名本身不要求保留旧产品，也不是删除全部前端的依据。

## 10. 与现有运行时的职责关系

### 10.1 普通 Go 服务

用于短时、确定性转换、校验和领域规则。

### 10.2 RabbitMQ

继续承载简单后台任务、事件分发和已有稳定 worker。

### 10.3 Temporal

继续拥有：

- 长业务流程；
- durable waiting；
- 人工审核等待；
- 外部异步任务等待；
- 跨进程恢复；
- 业务级 retry / compensation。

Temporal 可以把 Agent Run 当成一个明确输入输出的 Activity/能力调用。

### 10.4 Agent Runtime

只负责单次推理中的：

- 动态工具选择；
- 有限修复；
- 推理 checkpoint；
- human interrupt；
- Agent trace。

不得让 Temporal 和 Agent Runtime 同时拥有同一层 retry 语义。

## 11. 产品级安全模型

Agent 平台的发布门禁必须比传统工作台更严格。

### 11.1 工具权限等级

建议统一元数据：

```text
risk_level: read | compute | propose | write | external_side_effect
requires_human_approval: bool
idempotency_required: bool
allowed_roles: []
budget_class: low | medium | high
```

### 11.2 人工确认

第一阶段以下动作默认必须人工确认：

- 应用商品字段补丁；
- 修改销售价格；
- 删除/替换关键资产；
- 保存平台草稿；
- 创建远端商品；
- 正式发布；
- 批量写操作。

### 11.3 Agent 永远不能绕过

- tenant scope；
- role authorization；
- deterministic validator；
- publish readiness；
- idempotency；
- budget policy；
- data policy。

## 12. 评测与成功指标

Agent 是否上线不能只看“看起来更聪明”。

必须建立对照评测。

### 12.1 Product Agent 指标

- 必填事实补全率；
- 无证据推断率；
- validator 通过率；
- 人工修改字段数；
- 人工审核耗时；
- P95 latency；
- 单任务 AI cost。

### 12.2 Listing Agent 指标

- readiness 首次通过率；
- blocker 自动解决率；
- 人工接管率；
- draft 成功率；
- publish 成功率；
- 重复提交率；
- 单 Listing 平均人工操作次数。

### 12.3 Sourcing Agent 指标

- 推荐商品被接受率；
- 重复/无效候选率；
- source evidence 完整率；
- 每个可用候选获取成本；
- 人工筛选时间下降比例。

## 13. 路线图

下面保留原战略的阶段关系，不是第二份实时执行队列。当前顺序、具体开工和完成状态只看 #137 与执行 Issue；Figma 页面归属和 §9 退休决定不因历史 Phase 编号后置。

### Phase 0：战略切换与边界固化

目标：让仓库后续需求和架构决策以 AI Commerce Agent Platform 为长期方向。

- 建立本战略文档；
- README 明确 Figma 定义的硕米产品定位；
- 有效能力归当前领域 owner，旧 ListingKit 产品与混合架构按 EXTRACT → RETIRE 退出；
- 新功能评审增加“是否应作为 Commerce Tool / Agent capability”判断；
- 禁止因为战略调整立即做全仓重命名或微服务拆分。

### Phase 1：AI Capability Platform 稳定化

继续完成已有方向：

- model catalog；
- tenant policy；
- capability router；
- invocation ledger；
- cost / usage；
- provider-neutral adapter；
- prompt metadata；
- eval 基础。

每次只迁移一个 service-facing AI capability。

### Phase 2A：最小 Commerce Tool Foundation

在 Product Agent 之前建立一套最小、稳定、框架无关的 Tool 合同：

- ToolDefinition / version / capability；
- input/output schema；
- read / compute / propose 风险等级；
- tenant/user permission；
- timeout / retry owner；
- deterministic error taxonomy；
- audit / trace metadata；
- AgentDefinition allowlist 绑定。

第一批只接入 Product Agent 所需的 source evidence、canonical product、
catalog/asset facts、当前 Product Enrichment proposal、Product Image analysis proposal、
marketplace rule lookup 和 deterministic validator。不得开放 write 或 publish，
不得让 Agent 直接访问 repository、provider SDK 或 marketplace client。

### Phase 2B：Product Agent PoC

在 Phase 1 与 Phase 2A 达到退出门禁后，实现“商品资料诊断与补全 Agent”。

门禁：

- 只通过 Phase 2A Tool Registry 获取 read / compute / propose tools；
- 最多有限修复循环；
- 所有输出进入人工审核；
- feature flag 与 tenant allowlist；
- step、model call、token、runtime 和 cost hard budget；
- 与固定流程做离线 A/B eval。

只有证明质量或人工效率有可量化提升且风险指标不恶化后，才继续扩大 Agent 使用面。

### Phase 3：Commerce Tool 扩展与生产级治理

Phase 3 不再创建第一套 Tool 合同，而是在 Product Agent PoC 证明价值后扩展
Phase 2A 的同一合同，包括更多领域工具、版本迁移策略、运行 SLO 和平台适配器。
write / publish tools 仍需独立的审批、幂等、权限与审计门禁。

### Phase 4：SHEIN Listing Agent

让 Agent 在不直接发布的前提下，把商品准备到 `human-review-ready`。

成功标准：

```text
Agent 可以把一个合格 canonical product
稳定推进到 SHEIN 可审核 Listing Draft；
blocker 有解释；
可修复 blocker 能有限自动修复；
关键变更全部可审计。
```

之后再逐步开放保存草稿等有副作用工具。

### Phase 5：Sourcing Agent

在 Product Sourcing 当前 source loop 被完整验证后启动。

先支持一个明确业务目标和有限来源，不同时扩大量数据源。

### Phase 6：Agent Workspace

形成 Figma「AI工作台」中的统一交互，不新建名为 Agent Workspace 的平行一级产品入口：

- 对话/目标任务；
- Plan；
- Step；
- Tool trace；
- Agent result；
- approval；
- resume；
- exception handling；
- batch execution。

### Phase 7：多 Agent 协作

只有当单 Agent 已经产生稳定价值，并且存在清晰的职责拆分收益时再引入。

可能的协作：

```text
Sourcing Agent
  -> Product Agent
    -> Listing Agent
```

首版不建设通用 multi-agent swarm，不让 Agent 之间自由聊天形成业务状态。

## 14. 原战略 Now / Next / Later（历史基线，不作当前派工依据）

本节保留原战略基线时的排序，不能因为标题中的 Now / Next 或 Issue 仍开放而重新启动已完成、暂停或撤回的工作。今天的状态从 #137 与具体 Issue / PR 获取。

### Now

原战略时点的代码和交付重点是：

1. 保持 SHEIN 生产主链路稳定；
2. 完成当前基线验证；
3. 收口 Product Sourcing MVP 与受控 1688 链路；
4. 稳定 AI Capability Phase 1/2 已落地能力；
5. 不启动大规模目录重构；
6. 不因为新定位同时扩多个平台 Agent。

### Next

完成上述门禁后：

1. 最小 Commerce Tool Foundation；
2. Product Agent 有界运行时；
3. Product Agent PoC 与 fixed pipeline 对照评测；
4. Commerce Tool 扩展与生产级治理；
5. SHEIN Listing Agent read/propose-only 版本；
6. Agent Workspace 最小入口。

### Later

- Sourcing Agent；
- 受控 write tools；
- TEMU/Amazon Listing Agent；
- 多 Agent handoff；
- AI Runtime 独立扩缩容；
- Agent marketplace / custom agent definitions；
- 更完整的电商运营 Agent。

## 15. 明确不做的事情

在当前阶段不要：

- 把所有业务流程改成 Agent；
- 用 LangGraph/ADK/Eino 替换 Temporal；
- 让 LLM 直接写数据库；
- 让 Agent 绕过平台 validator；
- 让 Agent 默认拥有 publish 权限；
- 建一个全仓共享的“万能 AI Client”；
- 为 Agent 复制 canonical product；
- 创建第二套 submission state machine；
- 一次性 Tool 化整个仓库；
- 为了“Agent 架构”把模块化单体提前拆成微服务；
- 同时建设大量 Agent；
- 先做 Multi-Agent 再证明单 Agent 的商业价值。

## 16. 新需求评审准则

以后新增需求时，依次回答：

1. 这是用户业务目标，还是底层能力？
2. 是否已有领域 owner？
3. 能否用确定性代码解决？
4. 如果需要 AI，是固定 AI capability 还是动态 Agent？
5. 如果是 Agent，需要哪些 Tools？
6. Tool 是否可以只读或 proposal-only？
7. 是否涉及外部副作用？
8. 谁拥有最终业务状态？
9. 如何 eval？
10. 如何安全停用未获准或不可用的能力，而不回接退休路径？

只有确实需要动态工具选择、有限自修复或人工 interrupt 的场景才进入 Agent Runtime。

## 17. 文档权威关系

按职责读取，而不是由历史战略覆盖当前 Figma 或退休决定：

1. **[最终 Figma UI / IA Authority](final-ui-ia-authority.md)**：硕米产品导航、页面命名、交互语义与领域能力的产品投影；ListingKit 不属于目标子产品。
2. **本文与当前领域 Architecture Specs**：长期方向、事实 owner、确定性业务、安全、权限与 Tool/Agent 合同；技术能力图不能反向创造菜单。
3. **[Legacy Policy](../refactoring/legacy-hard-cut-policy.md)、[Register](../refactoring/legacy-register.md) 与 [全新系统基线](greenfield-no-legacy-migration.md)**：EXTRACT / RETIRE、新代码落点与无旧数据迁移/兼容前置。
4. **GitHub Issue #137 与具体执行 Issue / PR**：当前排序、范围、准确 HEAD、实际证据和操作权限；历史状态文档按注明的基线阅读。
5. **`docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`**：AI Control Plane 与 Agent Runtime 技术设计，受以上边界约束。

`docs/product/listingkit-project-goals.md`、`docs/product/listingkit-next-execution-plan.md` 是旧产品材料，**不再是当前子产品使命或近期执行权威**。其中有效规则只能由当前领域合同明确承接，不能恢复旧 Workspace、Task-first、永久 facade 或“ListingKit 不退休”的结论。

已经完成的 implementation plan 只保留为历史执行证据；Product Sourcing、Marketplace 等专项合同按当前有效决定使用，不能因为文档写着 Active 就覆盖新的批准。

最终 Figma 目标不等于当前 release capability；战略方向不能绕过真实事实、安全、验证和发布门禁，也不授权一次性开发 Figma 全部功能。

## 18. 一句话产品定义

对外产品定位：

> **硕米智能引擎 — 以 AI 工作台和专业智能体为核心的 AI 电商经营平台。给 AI 一个业务目标，让它调用受控商品、内容、图片和平台能力完成工作。**

对内工程定义：

> **Agent 是当前可靠电商领域能力之上的动态决策与交互层，不是新的业务事实源；ListingKit 的有效行为归当前 owner，旧产品与混合架构退出。**
