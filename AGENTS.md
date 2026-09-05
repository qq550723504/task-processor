# Repository Instructions

## 基本原则

- 如果指令有问题，先反馈，不直接执行。
- 解决问题时从根因入手，不只修复表面现象。
- 发现现有架构设计问题时及时说明，不要忽略。
- 优先复用仓库已有能力和成熟开源实现，避免重复建设。

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
7. 如果未来确实发现外部可观察契约或持久化运行态必须临时兼容，必须先停下来建立显式、可评审的 Exception；不得自行增加“临时”兼容路径。

PR 触碰 Legacy 时必须说明：

```text
Legacy decision: EXTRACT | RETIRE
Reusable behavior:
Current owner:
Cutover/deletion condition:
```

“继续兼容旧内部设计”不是当前允许的默认决策。

## 开发准入与停止条件

开始编码前，先判断任务是有界修改还是架构敏感修改。满足以下任一条件即属于架构敏感修改：

- 跨越 3 个及以上独立子系统；
- 跨越多个一致性边界，例如数据库、外部服务、消息队列、缓存、文件系统或浏览器状态中的两个及以上；
- 新增或修改状态机、补偿、对账恢复、授权边界、租户边界或破坏性操作；
- 预计超过 30 个范围相关文件；
- 预计新增超过 1,500 行生产代码，或生产代码变更超过 2,500 行；
- 同时包含基础设施重构和面向用户的功能交付。

命中停止条件时，先拆分任务；不能安全拆分时，先完成架构设计和独立评审，再继续实施。

## 架构敏感修改

实施前明确：

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
- 只有明确的 Must requirement 无法由现有仓库能力满足时，才允许引入新的通用框架、Saga、权限体系、Admission Control、Scheduler/Reconciler 或第二套 IAM 能力。

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

- 使用 TDD：先写能捕获目标缺陷的测试并确认失败，再写最小实现。
- 架构敏感工作拆成可独立验证、可回滚的小切片。
- 收到评审问题后先检查同根因的 sibling paths，再批量修复。
- 修复改变状态机、事务边界、恢复协议或公共契约时，只有命中上述 `BLOCKER` 条件才退回设计阶段重新评审；非 Blocker 优先转为实现测试或 backlog。
- 完成修改后运行与风险匹配的验证，在声明完成前确认测试结果。
