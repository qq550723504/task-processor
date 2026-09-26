# Repository Instructions

## 基本原则

- 如果指令有问题，先反馈，不直接执行。
- 解决问题时从根因入手，不只修复表面现象。
- 发现现有架构设计问题时及时说明，不要忽略。
- 优先复用仓库已有能力和成熟开源实现，避免重复建设。

## 产品先行与工程投入

**先交付真实用户能使用的核心产品，工程化做到当前阶段够用，再由实际使用、规模、风险和维护成本触发完善。** 本原则适用于规划者、项目经理、实现者和 Reviewer，不是取消安全底线或绕过现有准入与合并规则。

- 派工前在现有 Issue 简要说明：使用者、当前操作场景、可见交付结果、本次不做。底层工作须指出正在阻塞的具体用户操作。普通有界任务不要求重型 PRD 或额外治理平台，但正式开发仍必须满足下方 **Architecture First / Design Basis** 开工准入。
- 优先完成一条可用业务路径，再根据实际反馈迭代。没有明确当前使用价值的功能、通用抽象、容量优化、专项验收平台和工具扩展进入 Backlog；不得以“以后可能需要”、菜单完整、Issue 已存在或已经投入为由自动开工或合并。
- 工程增强由实测瓶颈、重复故障、实际运维成本或明确的当前安全/业务风险触发；不按假设的海量用户预建，也不必等到某个统一用户数才处理真实问题。先复用当前能力和成熟组件，再考虑最小局部补齐。
- 登录与授权、租户隔离、凭据保护、必要事务和幂等、关键外部副作用控制，以及当前数据保存/备份/恢复底线不能以 MVP 为由取消；实现深度与本次开放范围和风险匹配，不因此要求先建设完整平台。
- 未开放的功能不伪造可用入口、数字或成功状态，也不作为无关核心路径的前置。内部试用、客户交付和生产上线分别确定适用范围；未经相应验收不得宣称已上线或普遍可用。
- Agent、PM 和 Reviewer 不得自行追加产品需求、验收矩阵、人工操作流程或验收工具作为交付前置。确需扩范围，先说明具体阻碍、最小改动和成本，取得用户明确决定；不能从一个真实缺陷自动推导出建设通用系统的授权。
- 用户新的明确阶段决定可替代旧阶段要求，由协调方更新当前 Issue 范围及直接冲突引用；历史证据保留，延期/取消不写成 PASS，旧文档和评审不能恢复已撤回要求。没有用户决定不得自行降低当前 Must、接受新风险或改动 CI/保护规则。

## Delivery Batch 与里程碑评审

默认采用 **单实现者连续交付 + 独立里程碑检查 + 一个用户结果一个主要 PR**。目标是减少机械切片、重复审核和因 SHA/main 变化导致的无价值返工，同时保留必要的安全、权限、数据和合并底线。

- 一个 `Delivery Batch` 对应一个明确的用户可见结果，例如“账户中心 v1 可实际使用”“输入 1688 链接后可查看已保存商品”“生成一个可检查的平台草稿”。默认维护一个主要开发分支和一个主要 PR。
- 默认只有一个写入实现者连续完成范围内实现、必要开发自检、提交、推送和 PR 维护；除非用户明确要求，不为同一 Delivery Batch 常驻增加 PM、架构师、多 Writer 或多 Reviewer 线程。
- 不为了文件数、提交数、测试证据、内部模块边界或让每个小步骤单独过审而机械拆 PR。只有子结果能够独立产生用户价值、需要独立生命周期/风险处置，或主线被一个真正独立的 main Blocker 阻塞时，才拆出辅助 PR；辅助 PR 合入后立即返回主 Delivery Batch。
- 开工时同步一次最新 main；开发期间不为了“始终最新”持续 merge main。形成最终候选前再同步一次实际 main，并只处理真实组合问题。
- Astra/独立 Reviewer 默认只做两个检查点：
  1. **高风险边界检查**：仅当本轮实际新增/修改授权或租户边界、新状态机、新事务/恢复协议、不可安全重复的外部副作用、资金/计费、新持久化事实 owner 或明显新的跨系统安全边界时执行；
  2. **最终交付检查**：完整用户路径形成后，一次集中检查用户目标是否可完成、是否存在越权/数据损坏/错误副作用，以及本轮实际 diff 和运行交付是否一致。
- 普通 UI、Compose 接线、已有 owner/合同的调用和既有安全边界内的局部实现，不默认触发一次新的全局架构审核。
- Reviewer 提出具体 findings 后，修复完成只复核对应增量；已经确认且未变化的代码和证据不重复完整审核。只有修复实际改变原授权、状态机、事务、持久化或副作用边界时，才扩大复核范围。
- 已绑定明确代码且未变化的有效证据允许复用。merge-main 只带来无关模块、依赖、文档或机械组合变化时，不把旧 PASS 冒充新 SHA 的直接执行结果，但可以明确引用为“未变化代码已有证据”，只验证新增组合风险。
- 开发期间只运行与当前修改和风险直接相关的必要测试；完整 PR CI 默认留给稳定阶段候选和最终合并候选。不要人工重复 CI 已覆盖的相同检查。
- 活跃 Delivery Batch 临近合并时，不主动插入无关依赖升级、格式整理或治理型 PR；此类维护集中到独立维护窗口，除非它正在阻塞当前交付或修复明确安全问题。
- 用户可以授予**有边界的合并窗口授权**：在产品逻辑 diff、权限、安全、持久化和副作用边界不变的前提下，后续若仅发生 merge-main、依赖同步、文档或元数据变化，独立 Reviewer 只需确认组合无新问题且必需 CI 通过，无需再次请求同一产品决定。实际 GitHub merge 仍必须使用最终准确 HEAD/expected-head 语义，不得管理员绕过、强推或修改保护规则。任何产品逻辑、权限、安全、持久化或副作用变化都会使该窗口授权失效。
- 汇报以“用户现在能完成什么、入口在哪里、具体阻碍是什么、需要什么产品决定”为主。测试数量、代码行数、PR 数量和报告数量只作为辅助证据，不能替代产品进展。

## Legacy Hard-Cut

仓库对旧代码采用 `Hard-Cut + Selective Extraction`，详细规则见：

- `docs/refactoring/legacy-hard-cut-policy.md`
- `docs/refactoring/legacy-register.md`

当前 Legacy 只有两种合法处理结果：

- `EXTRACT`：仍然正确、可复用的行为抽取到当前架构的正确 owner；新代码不得继续依赖旧 owner。
- `RETIRE`：属于旧设计的代码/测试/DTO/Workflow/Task/Workspace/状态机不兼容、不扩展，完成切换后删除或停止使用。

**当前没有 Legacy Compatibility 类别。**

遇到旧代码时：

1. 先判断它是否仍是当前 Product/Architecture requirement；不是则 `RETIRE`。
2. 若行为仍有价值，优先把行为抽取到当前 Domain/Capability owner，而不是 Wrap 旧 Service。
3. 不得为了“保持旧代码可用”在新架构内部增加 fallback、双读、双写、双向同步或第二套事实源。
4. 旧测试不是 Architecture Authority；只保留仍然有效的业务、安全、权限、幂等、平台或确定性行为，旧实现细节测试应删除/改写。
5. ProductEnrich/ProductImage 旧 task/queue/worker/API、Task-first Product 模型、旧 Listing Workspace/Task Dashboard、平台独立 Workbench 等已 hard-cut 的设计不得通过兼容层重新进入新代码。
6. 新的 Product/Agent/Tool/Marketplace/Console 代码不得直接依赖已登记为 `RETIRE` 的 legacy abstraction。
7. 如果未来发现具体、当前的外部可观察契约或持久化运行态似乎要求兼容，必须停下来报告用户；当前没有可执行的 legacy migration/compatibility Exception，只有用户新的明确产品决定可以改变本禁令，Reviewer 或 Agent 无权自行放行。不得自行增加“临时”兼容路径。

PR 触碰 Legacy 时必须说明：

```text
Legacy decision: EXTRACT | RETIRE
Reusable behavior:
Current owner:
Cutover/deletion condition:
```

“继续兼容旧内部设计”不是当前允许的默认决策。

## 全新系统产品基线

[PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08](docs/product/greenfield-no-legacy-migration.md) 适用于后续全部业务任务：按全新安装、空业务数据、当前模型和当前产品需求设计。不得新增旧数据迁移、旧 ID 映射、旧 Service 包装、compatibility adapter、fallback、双读/双写/同步、tenantbridge consumer 或第二事实源。历史 A/B1/B2、#364/PR #366 与旧 C/D 仅是历史证据，不构成实现前置或迁移授权。

现有合格能力和成熟开源组件仍可按当前 owner 复用；发现旧代码只作 `EXTRACT | RETIRE`。若主张存在外部可观察兼容义务，先提供具体当前证据并报告用户；只有用户新的明确产品决定可以改变本禁令，Reviewer 或 Agent 无权自行放行。不得以旧 Issue、PR、设计、测试或代码默认推定。该产品决定不授权真实环境的数据访问、迁移、删除或清理。

## Architecture First 开工准入

**任何改变产品能力、业务运行语义或领域合同的正式开发，在 Writer 修改生产代码前，必须存在可追溯的 Design Basis。** 设计依据必须先于实现；不能先写正式代码，再用实现倒推架构。

### Design Basis 分级

每个执行 Issue 必须在以下三类中选择一类，并写入当前正文：

1. **Independent Architecture**
   - 必须有独立架构文档；
   - 文档必须引用适用的产品决定；涉及 UI / IA / 页面命名 / 交互时必须引用当前 Figma Product Authority；
   - 必须完成适用的独立 Architecture Review；
   - 只有显式达到 **`IMPLEMENTATION_READY`** 后，Writer 才能修改正式生产业务路径。

   满足以下任一条件时，默认属于 Independent Architecture：
   - 新产品能力或新的端到端业务流程；
   - 新增或改变领域边界、canonical fact owner 或公共领域合同；
   - 新表、新持久化事实、数据库所有权变化，或新的跨数据库一致性问题；
   - 新增或修改状态机、事务边界、补偿、对账、恢复协议；
   - 登录、授权、租户、身份所有权或跨企业边界变化；
   - 资金、余额、资源、额度、订阅、计费、结算或支付语义变化；
   - 新增 AI / Agent / provider 调用，或改变其准入、预算、计量、结算、副作用语义；
   - 新增不可安全重复的外部副作用，或改变 retry / UNKNOWN owner；
   - 新的跨系统安全边界；
   - 下方架构敏感阈值表命中且现有批准架构不足以直接覆盖。

   独立架构文档至少明确：Product Outcome / Scope / Out of Scope、Product/Figma Authority、domain/fact owner、contract → implementation → injection → consumer、状态与持久化边界、权限/租户边界、幂等与 retry/UNKNOWN/recovery、外部副作用、Legacy decision、验证与验收方法。

2. **Reuse Existing Architecture**
   - 适用于单域、有界、完全复用当前已批准架构/合同的功能；
   - 不强制新建独立 Markdown，但 Issue 必须给出轻量 Design Basis；
   - 至少写明：用户结果、引用的 Architecture / Contract、当前 owner、主要调用路径、不变量、本次明确不改变的状态机/权限/事实 owner/恢复协议、验收方法；
   - 若实现过程中发现必须改变上述边界，立即停止正式实现并升级为 Independent Architecture，不得在实现 PR 中临时发明架构。

3. **N/A**
   - 仅限纯文档、文案、纯 UI 样式、测试修复、机械重构，或不改变产品/领域/状态/权限/持久化/外部副作用语义的局部 bugfix；
   - Issue 必须写 `Design Basis: N/A` 并说明理由；
   - “改动很小”“只有一个文件”本身不是 N/A 理由。

### Ready 与开工

执行 Issue 只有在以下条件满足后才能标为 `Ready` 并派 Writer 修改生产代码：

- 用户结果、Scope / Out of Scope 已明确；
- 涉及产品 UI 时，适用 Figma / Product Authority 已明确；
- Design Basis 分类和引用已完整；
- Independent Architecture 已完成适用 Architecture Review，并明确为 `IMPLEMENTATION_READY`；
- 依赖、owner 和正式开工条件已满足。

缺少设计准入时保持 `Backlog` 或 `Blocked`，不能进入 `In Progress`。

在 `IMPLEMENTATION_READY` 之前允许只读代码调查、现状映射、接口/运行证据收集以及明确标为非正式实现的有界 Spike / POC；**不得修改正式生产业务路径、建立正式 schema / state machine，或提交会被当作正式实现消费的业务代码。**

Architecture Review 的目的不是生成更多文档。复用已有架构时不重复全局评审；简单 N/A 任务不为形式创建设计文档、Reviewer 或治理系统。

## 开发准入与停止条件

Architecture First 分类完成后，再判断任务是否命中额外的架构敏感风险提示线。满足以下任一条件且现有批准架构不能直接覆盖时，必须使用 Independent Architecture：

- 跨越 3 个及以上独立子系统；
- 跨越多个一致性边界，例如数据库、外部服务、消息队列、缓存、文件系统或浏览器状态中的两个及以上；
- 新增或修改状态机、补偿、对账恢复、授权边界、租户边界或破坏性操作；
- 预计超过 30 个范围相关文件；
- 预计新增超过 1,500 行生产代码，或生产代码变更超过 2,500 行；
- 同时包含基础设施重构和面向用户的功能交付。

上述条件是风险提示线，不是机械拆 PR 的目标。命中后先识别本次真正新增的高风险边界；一个完整用户结果仍可保持一个主要 PR。若已有架构确实完整覆盖，则在 Issue 的 Reuse Existing Architecture 中引用并说明；若覆盖不足，必须先补独立架构文档和评审，达到 `IMPLEMENTATION_READY` 后再编码。

准入只覆盖本次实际改变或消费的边界；不为普通有界修改重复设计全局架构。切片应有独立交付价值，不为满足行数而形式拆分、隐藏依赖或搬动文件规避统计；上述阈值和既有审批要求保持有效。

## 架构敏感修改

针对本次交付涉及的边界，复用已有合同与有效证据，实施前明确：

- 业务、安全和租户不变量；
- 权威数据所有者以及事务边界；
- 状态、事件、前置条件、转换和持久化效果；
- 幂等身份，以及相同键不同载荷的处理；
- 每个持久化边界在失败、响应丢失、重试、重启、取消和并发时的结果；
- 自动恢复的触发条件、入口和唯一责任方；
- 授权撤销、跨企业上下文和缓存漂移行为；
- 请求大小、超时、deadline 和资源耗尽边界；
- 每条不变量对应的验证证据。

涉及多个持久化步骤时，先评估共享事务或 Unit of Work；跨越持久化边界时，优先评估仓库已有能力或成熟方案，例如 transactional outbox、Saga 或项目已有的 Temporal。

## 产品边界与 Threat Model

架构设计或安全评审前，必须先读取当前任务已经确认的 Product Decisions、Threat Model、Must / Should / Out of Scope 和 Accepted Risks；这些边界高于 Reviewer 自行扩展的理想安全目标。

- Agent 不得自行扩大 Threat Model，也不得把明确的 Out of Scope / Accepted Risk 重新升级成阻塞项。
- 技术上成立但只影响已明确接受风险的 finding，分类为 `ACCEPTED_RISK` 或 `NOT_APPLICABLE`，不得为其新增架构机制。
- 如果 Product Decision 允许某种行为，例如“允许提示手机号是否已注册”，则仅用于隐藏该行为的 account-enumeration、branch-neutral capacity、cross-window side-channel 等增强不得阻塞 Phase1。
- 只有当前明确的 Must requirement 无法由现有仓库能力满足，且最小局部补齐或成熟方案也不足时，才可提出新的通用框架、Saga、权限体系、Admission Control、Scheduler/Reconciler 或 IAM 能力；须经用户明确授权及适用独立准入，不能由 Agent 自行引入。

## 评审 Finding 分类

收到 Code Review / Security Review finding 后，**先分类，再修改代码或设计**。Reviewer 的 P0/P1/P2 严重度不是项目阻塞等级的自动映射。

每条 finding 必须归入以下一种：

- `BLOCKER`：必须在继续实施前处理；
- `IMPLEMENTATION_TEST`：问题真实，但适合通过实现、并发测试、fault injection 或 staging 验证收敛；
- `BACKLOG`：有价值但不影响当前阶段正确性；
- `ACCEPTED_RISK`：属于已明确接受的风险；
- `NOT_APPLICABLE`：与当前 Product Decision、Threat Model 或 Scope 不一致。

只有以下后果之一明确成立时，finding 才可分类为 `BLOCKER`：

- 跨租户访问或租户数据混淆；
- 权限提升、错误授权或绕过明确的访问控制；
- 资金、额度、资源或计费重复增加、重复扣减或错误归属；
- 数据损坏、数据丢失或不可逆错误迁移；
- 重复且不可安全恢复的外部副作用；
- 错误 Consent、错误身份所有权或认证核心被本地系统替代；
- 覆盖或破坏已有 paid plan / entitlement；
- 删除仍拥有 durable business assets 的身份或组织；
- Provider mutation 无法安全判断结果且会导致重复身份、授权或永久不一致；
- 核心 happy path 按当前设计无法完成；
- rollout / migration 按当前方案无法安全完成。

把 finding 判为 `BLOCKER` 时，必须指出命中了上面的哪一类；无法指出则不得以 Blocker 修改架构。

每条 finding 同时指出受影响的当前 Must、具体用户操作/安全后果和阻塞层级（实施、单片合并、试用或生产）。未覆盖某个理论场景不自动等于当前阻塞；`IMPLEMENTATION_TEST` 也不自动授权新建整套验收工具。必要修复与非阻塞完善分开，后者不拖住已满足范围的交付。

## 架构评审停止规则

架构评审的目标是达到 `IMPLEMENTATION_READY`，不是达到“自动评审 0 finding”。

- 架构敏感修改默认最多进行两轮正常 Architecture Review。
- 第二轮之后出现的非 Blocker P1/P2，默认转为 `IMPLEMENTATION_TEST` 或 `BACKLOG`，不得继续创建 V8/V9/V10 等设计文档链。
- 已达到 `IMPLEMENTATION_READY` 的设计视为冻结基线；只有新的 `BLOCKER` 可以重新打开架构设计。
- 冻结后，理论 hardening、极端运维优化、纯 side-channel 增强和明确 Accepted Risk 不得阻止编码。
- 如果实现阶段的真实测试证明现有状态机、事务边界、恢复协议或公共契约存在 Blocker，再退回设计阶段；不要用推测性的无限枚举替代实现验证。

处理 review comment 时优先采用以下格式：

```text
Finding:
Product requirement affected:
Classification: BLOCKER | IMPLEMENTATION_TEST | BACKLOG | ACCEPTED_RISK | NOT_APPLICABLE
Reason:
Action:
```

## 实施与验证

- 业务规则与缺陷修复使用 TDD：先写能捕获目标缺陷的测试并确认失败，再写最小实现；纯文档、文案和不涉及业务行为的有界修改只做适用检查，不为形式制造 RED 或测试框架。
- 架构敏感工作在同一 Delivery Batch 内拆成可独立验证、可回滚的小切片；小切片默认是实现/提交边界，不要求每片独立 PR 或重复完整审核。
- 收到评审问题后先检查同根因的 sibling paths，再批量修复。
- 修复改变状态机、事务边界、恢复协议或公共契约时，只有命中上述 `BLOCKER` 条件才退回设计阶段重新评审；非 Blocker 优先转为实现测试或 backlog。
- 开发者完成修改后只执行任务约定、与改动风险匹配的必要开发自检和既有必需检查，如编译、类型检查、相关测试及 CI；如实报告结果，不自行扩大到完整故障矩阵或专项环境验收。
- **实现者不是最终验收者。** 开发自检、CI 绿色或另一 AI 的代码评审均不等于用户验收；不得自行签发“产品验收通过/可上线”。完成实现后交出运行入口、操作步骤和已知限制，由用户或其指定独立验证者确认实际使用效果。
- 独立复核按现有规则检查真实差异、受影响路径和有效证据，不只采信作者报告；默认按 Delivery Batch 的高风险检查点和最终交付检查点执行。修复后只复核相关增量；与风险无关的全量重跑、同 SHA 重复评审、未变化代码的重复审核或默认新增 AI 互验线程不作为交付流程。
- 未经单独授权，不开发/扩展 runner、人工验收 session、故障注入平台或验证工具的验证工具。已有有效测试保留并按需复用；有独立验收任务时，按其明确范围执行，缺少真实执行就保留 NOT_RUN。

## Issue 驱动派工

执行 Issue 时遵循 [Issue 驱动派工规则 V1](docs/engineering/issue-driven-development.md)，使用其中引用的 Issue / PR 模板。

- 开工先读 Issue 当前正文、引用依据和适用规则，检查已有接单会话与关联 PR，并在 Issue 记录接单。
- 一个 Delivery Batch 同一时间默认只有一个写入负责人，使用独立分支 / worktree；范围内实现、review finding 修复与 CI 修复留在同一主要 PR。除非子结果可独立交付或存在真正独立的 main Blocker，不为每个实现小片另开 PR。
- 授权范围内实现、必要开发自检、提交、推送及维护 PR 可连续执行，不逐步重复请示；独立验收按明确派工执行。默认不授权合并、部署、Issue 关闭或真实数据操作。
- 进度写 Issue，验收映射、最终 HEAD、CI、独立评审与批准证据写 PR；本文件不保存滚动状态。
- `IMPLEMENTATION_TEST` 在实现和验证层收敛、不必重开设计；当前切片 Must 未满足时仍阻止合并。

## 交付与停止点

- 面向用户的任务以可运行、可操作的结果交接：实际访问入口或正常启动命令、登录/使用步骤、可用与未开放功能、数据保存方式和已知限制。纯底层任务交付当前消费者可使用的最小接口与说明，不用代码行数、PR 数量或报告数量替代结果。
- 实现完成后按既有状态进入 `In Review`，明确“实现完成，待独立验证/用户试用”；不是擅自将 Issue 标为 Done 或授权合并。具体问题由原 owner 在批准范围内修复，不默认派生下一轮工具建设。
- 内部试用实例仅在获准范围内准备和保留；不能把结束即销毁的验收环境当作用户交付。stop/restart 与 destroy/数据删除分开，凭据私密交接，不因交付要求擅自操作共享或生产环境。
- 汇报优先说明现在能用什么、入口在哪里、哪个具体问题阻碍使用、需要用户决定什么；测试/CI 为辅助证据。若连续推进的只是工具、报告或验证流程，应暂停非必要扩展并重新核对当前用户价值，不停止必要安全修复。
- 本文件与现有 Issue/PR 已足够承载这些约束；不为执行本规则另建治理系统、检查器、状态机或模板工程。
