# ListingKit 重构进展审查（2026-06-24）

## 文档状态

- 状态：Active snapshot
- 审查日期：2026-06-24
- 审查基线：`master` commit `4829df08677a8b21960bfef59c702c3dc5027a2e`
- 最新提交：`Decouple listing runtime health check from management client`
- 适用对象：技术负责人、后端、前端、QA、运维和代码审查者

本文件记录当前重构执行状态，不替代以下长期架构文档：

- `docs/architecture/project-boundaries.md`
- `docs/architecture/architecture-review-checklist.md`
- `docs/architecture/platform-boundary-strategy.md`
- `docs/refactoring/listingkit-refactoring-roadmap.md`

## 1. 结论

当前重构已经从“大范围拆分期”进入**稳定收口期**。

主要判断如下：

1. SDS-to-SHEIN 批量生产闭环已经完成代码和真实环境验证。
2. SHEIN publishing / workspace 规则已经大规模迁出 `internal/listingkit` 根包。
3. 通用 Studio 和 submission 机制已经形成独立领域模块。
4. Go Listing Control Plane 已经替代 Java 调度路径进入生产部署形态。
5. Listing runtime 正在快速停止对 Management HTTP fallback 的依赖。
6. 项目尚未完成重构，剩余工作主要集中在 Control Plane 生产安全、Management Client 退休、ListingKit 根包所有权收尾和 CI 全量门禁。

当前不适合继续进行无业务牵引的大规模目录移动，也不适合立即启动新的大型多平台工作台建设。

下一阶段应以以下目标为主：

```text
先把 Go Listing Control Plane 做成可证明的唯一调度事实源；
继续退休 listing runtime 中的 management compatibility shell；
只迁移仍然拥有真实业务状态或规则的 ListingKit root seam；
把完整测试、并发测试和生产验证变成强制门禁。
```

## 2. 分阶段进度判断

| 阶段 | 当前状态 | 判断 |
| --- | --- | --- |
| Phase 0：基线和守门规则 | 基本完成 | 项目目标、边界、review checklist、平台策略、兼容层退休和外部客户端台账已经形成。前端 CI 完整，后端 CI 仍不是 `go test ./...`。 |
| Phase 1：SDS 批量生产闭环 | 完成 | fan-out、durable task ownership、幂等、最终门禁、strict baseline、兼容分组、状态投影和真实 SHEIN `save_draft` 已验证。 |
| Phase 2：收缩 ListingKit 根包 | 高度推进，未完成 | 大量 SHEIN publishing/workspace 规则已迁出；根包仍持有 facade、repository adapter、任务持久化顺序、部分 Studio 运行行为和旧兼容入口。 |
| Phase 3：平台边界固化 | SHEIN 已形成模板 | SHEIN publishing/workspace 边界比较清晰；Amazon、TEMU、Walmart 仍以历史包和迁移盘点为主。 |
| Phase 4：外部集成和运行时清理 | 正在加速 | Go Control Plane 和 local provider 路径正在淘汰 Java/Management HTTP fallback，但 runtime 仍保留 `management.ClientManager` 壳和若干直接依赖。 |
| Phase 5：多平台复制 | 暂不启动大规模建设 | 应等待 Control Plane、Management retirement 和 SHEIN runtime 稳定门槛关闭。 |

## 3. 已完成的关键成果

## 3.1 项目目标和重构路线已正式落库

当前仓库已经存在：

- `docs/product/listingkit-project-goals.md`
- `docs/refactoring/listingkit-refactoring-roadmap.md`

这两份文档已经把项目从“通用 task processor”重新定位为跨境商品 Listing 自动化平台，并明确采用“能力牵引、小步迁移、兼容收缩”的策略。

## 3.2 SDS 批量生产闭环已通过真实验证

相关证据：

- `docs/product/validation/runs/2026-06-21-shein-sds-batch-production-closure-regression.md`
- `docs/product/validation/runs/2026-06-21-shein-sds-batch-production-closure.md`

真实环境已经验证：

```text
SDS baseline warmup
compatibility-aware batch generation
2 designs x 2 selections fan-out
重复请求幂等
受控 store rejection
workspace 属性修复
readiness 清零
真实 SHEIN save_draft 成功
```

店铺 `870` 的复测记录中，任务最终进入：

```text
task status = completed
needs_review = false
submit readiness = ready
submission action = save_draft
submission status = success
SHEIN response code = 0
```

因此，`listingkit-refactoring-roadmap.md` 中 Phase 1 的真实发布门槛已经关闭。

## 3.3 ListingKit 根包中的 SHEIN 规则已大量迁出

根据 `docs/refactoring/listingkit-refactoring-roadmap.md` 的 Phase 2 执行记录，已迁出的主要规则包括：

### Workspace / readiness

- repair center 组装
- readiness taxonomy
- readiness reason / guidance / repair hints
- category / attribute / sale-attribute readiness checks
- source facts readiness
- final review readiness
- freshness readiness
- revision patch application
- repair revision seed 和 validation clone

主要目标包：

```text
internal/marketplace/shein/workspace
```

### Publishing / submission

- save_draft / publish 动作分发
- translation decision
- supplier / publish payload policy
- image upload、cache、payload normalization
- site / SKU / quantity / dimension normalization
- SKU pricing、style、variant、supplier SKU normalization
- sensitive-word retry
- final draft submit / image application
- submission state transition
- remote refresh selection和状态策略

主要目标包：

```text
internal/publishing/shein
internal/marketplace/shein/publishing
internal/listing/submission
```

这说明 `internal/listingkit` 已经逐步回到：

```text
产品 facade
跨领域 orchestration
任务和结果持久化顺序
旧 API / 旧数据兼容
runtime adapter
```

## 3.4 通用 Listing 领域已经形成

当前已经形成或持续扩展的领域包包括：

```text
internal/listing/studio
internal/listing/submission
internal/listing/preview
internal/listing/sourcefacts
```

其中：

- Studio 包持有通用 session、batch、run 和状态聚合机制。
- Submission 包持有锁、重试、lease、phase、event、failure 和 recovery 等通用策略。
- Preview 包持有平台无关的预览元数据、能力和 fallback 规则。
- Source facts 包持有来源事实 readiness 规则。

`docs/refactoring/listingkit-boundary-checkpoint.md` 已明确停止线：

> 不再为了“继续变薄”而迁移 helper；只有真实减少 root ownership 的 seam 才继续迁移。

这是正确的收口方向。

## 3.5 架构治理已经从文档转成守门测试

当前稳定入口包括：

- `docs/architecture/README.md`
- `docs/architecture/project-boundaries.md`
- `docs/architecture/architecture-review-checklist.md`
- `docs/architecture/platform-boundary-strategy.md`
- `docs/architecture/external-client-boundary-inventory.md`
- `docs/architecture/compatibility-retirement.md`
- `docs/architecture/temporal-boundaries.md`

当前已经有较完整的 guard baseline，覆盖：

```text
业务域不得反向依赖 app/httpapi
marketplace/publishing 不得依赖 ListingKit facade
platform registration 必须保持薄层
旧 compatibility path 不得重新成为正式入口
外部 client import 必须保持 allowlist
Temporal SDK 只能位于 runtime/orchestration adapter
```

这比继续做大规模文件移动更有长期价值。

## 3.6 Go Listing Control Plane 已经落地

2026-06-23 至 2026-06-24 的主要提交包括：

- `afb4d3e` — Merge Go listing control plane
- `fc0aa0c` — Enable listing control plane dispatch by default
- `f97e0bf` — Stop applying legacy SHEIN listing daemonsets
- `c01378e` — Make listing consumers reload tasks from DB
- `572cc70` — Ack listing tasks after durable claim
- `89e5956` — Keep task RPC local in listing runtime
- `10a9177` — Stop remaining listing clients from management fallback
- `4829df0` — Decouple listing runtime health check from management client

当前实现已经包含：

```text
cmd/listing-control-plane
internal/app/runtime/listingcontrol
internal/listingcontrol/scheduler.go
internal/listingcontrol/store_runtime.go
internal/listingcontrol/quota.go
internal/listingcontrol/recovery.go
internal/app/runtime/listingcontrol/status.go
```

主要能力包括：

- Postgres 公平扫描和原子 claim
- 按店铺 owner / queue / capacity 路由
- RabbitMQ 持久消息发布
- publish 失败回滚
- stale queued / processing timeout 恢复
- structured quota 与 legacy quota 隔离
- `/health`、`/ready`、`/status` 状态接口
- 独立 Deployment 和 worker/runtime 分离

这是本轮重构中最重要的运行时所有权迁移。

## 3.7 Listing runtime 已开始停止 Management fallback

近期代码已经明确执行以下规则：

```text
本地 provider 存在时，本地 miss 不再回退 Management HTTP；
Task RPC 由本地 repository/provider 处理；
import task 和 strategy 由本地 runtime 处理；
SHEIN listing 启动必须验证 local runtime 能力；
health check 通过窄接口而不是直接绑定具体 Management Client 类型。
```

这说明 Management Client 已经从“事实源”退回到“迁移期兼容壳”。

但它尚未完全退休。

## 4. 当前仍然存在的核心缺口

## 4.1 Control Plane 多实例唯一执行保护已通过 rollout 验证

`ListingControlPlaneConfig` 已包含：

```text
LeaderLockKey
LeaderLockTTL
```

`internal/app/runtime/listingcontrol` 已将 Redis leader lease 接入 scheduler cycle：每轮 recovery/dispatch 前先获取或续租 leader lock，未获取到 lease 的实例进入 standby，不执行 recovery/dispatch。

2026-06-24 已完成一次临时双实例 rollout 验证，记录见：

```text
docs/product/validation/runs/2026-06-24-listing-control-plane-leader-rollout.md
```

验证结果：

```text
两个 control-plane pod 同时启动时，只有 leader 执行 scan/dispatch/recovery
standby pod 的 /status 暴露 leader owner
standby pod 保持 Kubernetes ready，但 dispatch/recovery 为 0
双实例 rollout 成功完成，验证后生产恢复为 1 副本
```

剩余验证重点是：

```text
主动删除 leader pod 后，standby 能在下轮获得 lease
旧 worker watchdog 保持关闭，recovery owner 不重复
```

## 4.2 Skip / delay reason 已完成代码层持久化

Control Plane 现在仍会在 `DispatchSummary.Decisions` 和 `/status` 中展示 skip reason，同时 scheduler 对以下情况会把 delay reason 写回 import task：

```text
store disabled
no live owner
queue mode invalid
quota exhausted
no capacity
queue depth unavailable
```

写入字段为：

```text
stage = dispatch
reason_code = <skip reason>
error_message = Dispatch delayed: <skip reason>
remark = Dispatch delayed: <skip reason>
```

任务状态保持 pending / pending_retry / crawled 等原 dispatchable 状态，不会因为“暂不可调度”被改成失败。dry-run 模式仍保持只读。

剩余验证重点是：

```text
任务列表和运营接口能直接显示最近一次 dispatch delay reason
长期 Pending 任务重启后仍保留最后一次 delay reason
必要时再补 last_dispatch_checked_at / next_dispatch_after
```

## 4.3 Daily limit 已进入 Store Runtime 容量计算

Control Plane 现在的 Store Runtime 容量计算已经同时考虑：

```text
daily completed count
in-flight count
store daily limit
external remaining quota
queue depth
```

当前语义：

```text
runtime_capacity = min(owner_browser_capacity, maxQueuedPerStore)
daily_remaining = daily_limit - completed_today - processing - queued
capacity = min(runtime_capacity, daily_remaining)
```

当 `daily_remaining <= 0` 时，Store Runtime 返回：

```text
reason = daily_limit_exhausted
capacity = 0
```

调度决策同时追加到 `listing_dispatch_event`，记录：

```text
action
reason_code
capacity
queued
processing
completed_today
daily_limit
owner_node
```

剩余验证重点是生产观察：确认 `draft/published` 作为 `completed_today` 的口径符合运营预期，以及任务列表能读到最后一次 delay reason。

## 4.4 部分配置已经定义但尚未接通

以下字段目前需要逐一确认是否真正生效：

```text
LeaderLockKey
LeaderLockTTL
QuotaKeyTTLGrace
```

未接通的配置不能长期保留为“看起来支持”的能力。

处理原则：

```text
要么接通并测试；
要么删除或标记 reserved / unsupported；
不要让运维误以为配置已经生效。
```

## 4.5 Management Client 仍然是 runtime 聚合壳

虽然本地数据已经停止 fallback，但以下结构仍以 Management Client 为聚合对象：

```text
PlatformProcessorRegistry.managementClient
PlatformRuntimeContext.ManagementClient
SharedResources.ManagementClient
runDebugTask 中的 ManagementClient task lookup
```

这意味着：

- 业务数据已经本地化；
- 类型和装配所有权还没有本地化。

下一步应把能力拆成 runtime-owned ports，例如：

```text
ListingRuntimeHealthValidator
ImportTaskLoader
StoreProvider
PricingRuleProvider
ProductDataProvider
TaskStatusWriter
```

然后逐步删除：

```text
GetManagementClient()
PlatformRuntimeContext.ManagementClient
debug runtime 对 Management Client 的直接读取
```

## 4.6 `internal/listingkit` 仍然很大，但不能继续机械拆分

根据 boundary checkpoint，根包仍合理持有：

- API shell DTO
- repository implementation / adapter
- expected-updated-at conflict
- mixed session field assignment adapter
- concrete batch run executor loop
- generation resume 和 task creation orchestration
- logging 和 legacy error translation
- task / result persistence ordering

这些不应因为文件还多就全部迁走。

下一轮只应迁移满足以下条件的 seam：

```text
有明确目标 owner；
迁移后 root 不再持有核心状态或规则；
目标包可以独立测试；
不会把 Temporal determinism 或平台副作用错误抽象成通用层；
不是单行 wrapper 搬家。
```

## 4.7 后端 CI 仍未执行全仓测试

当前 `.github/workflows/ci.yml` 的后端只运行指定 package：

```text
go test ./cmd/product-listing-api ... ./internal/listingkit ...
go test ./tests/...
```

前端已经执行：

```text
npm run lint
npm run typecheck
npm test
npm run build
```

后端应该升级为：

```bash
go test ./... -count=1
```

高风险模块再增加：

```bash
go test -race ./internal/listingcontrol ./internal/listingkit ./internal/listing/studio ./internal/listing/submission
```

至少需要保证新增 `internal/listingcontrol`、runtime、platform、integration 等包不会因为 CI package allowlist 漏跑。

## 4.8 Go Control Plane 实施计划与代码状态已经漂移

`docs/superpowers/plans/2026-06-23-go-listing-control-plane.md` 中任务仍以未完成 checkbox 为主，但大量代码已经合并并进入部署配置。

需要补一份正式 closeout / validation 记录，明确：

```text
哪些任务已完成；
哪些只完成代码未完成生产验证；
哪些设计项被调整；
Java scheduler 是否已全部停用；
worker watchdog 是否只有一个 owner；
store 976 / 1030 等真实店铺的验证结果；
回滚路径是否演练。
```

## 5. 当前推荐优先级

## P0：Control Plane 生产加固

按顺序完成：

```text
1. leader lock 已实现并通过临时双实例 rollout 验证
2. 持久化 dispatch skip/delay reason 已完成代码层落地
3. daily limit / in-flight / quota capacity 已完成代码层统一
4. recovery owner 去重和回滚验证
5. status endpoint 增加 leader、last success、配置生效状态
6. 真实店铺 dispatch / consume / recovery 验收报告
```

退出条件：

```text
Java scheduler 关闭后，Go 是唯一调度事实源；
重启后运营仍能解释任务为什么未调度；
多个 scheduler 实例不会重复发布；
配额、pause、owner、queue、daily limit 都有稳定 reason code；
publish 失败和 stale task 能安全恢复。
```

## P0：CI 和并发门禁

```text
1. 后端 CI 改为 go test ./... -count=1
2. listingcontrol 增加 race test
3. claim / rollback / recovery 增加并发集成测试
4. cmd/listing-control-plane 和 cmd/shein-listing 都执行 build
5. architecture guard tests 固定进入 CI
```

## P1：Management Client 退休第二阶段

```text
1. 把 local runtime capabilities 从 Management Client 中抽为本地 ports
2. PlatformProcessorRegistry 不再暴露 GetManagementClient
3. debug task loader 改为本地 repository
4. Store / Product / Pricing / Task RPC 的 compatibility client 只保留外部兼容入口
5. 更新 external-client-boundary-inventory.md
```

## P1：ListingKit 根包收尾

```text
1. 列出 root 中仍然拥有状态机的文件，而不是按文件数继续拆
2. 优先迁移真正的平台规则和通用 policy
3. facade / orchestration / persistence ordering 保留在 root
4. 删除零调用 wrapper 和 test-only compatibility seam
5. 每次迁移同步更新 boundary checkpoint
```

## P1：文档状态收口

```text
1. 更新 listingkit-refactoring-roadmap.md 的更新日期
2. 增加 2026-06-23/24 Control Plane 和 Management retirement 记录
3. closeout Go Control Plane implementation plan
4. 记录真实 rollout 和 rollback 验证
5. 避免继续创建重复的长期架构政策文档
```

## P2：多平台复制准备

在以下门槛完成前，不建议建设新的大型 TEMU / Amazon / Walmart 工作台：

```text
Control Plane 唯一所有权通过；
Management HTTP fallback 从 SHEIN runtime 退出；
后端全量 CI 通过；
SHEIN runtime 连续稳定；
ListingKit root ownership checkpoint 更新。
```

可以先做的工作仅限：

- 平台能力边界盘点；
- API / readiness 契约设计；
- 迁移成本评估；
- 不影响生产路径的 package guard。

## 6. 建议的两周执行计划

## 第 1 周：Control Plane 可靠性

```text
Day 1-2
- leader lock 已实现
- 状态接口已暴露 leader identity 和 lease
- 双实例 rollout 已验证 leader/standby，不再重复 dispatch/recovery

Day 3
- 持久化 skip/delay reason 已完成
- 任务列表可查询最近调度原因仍需生产观察确认

Day 4
- daily limit / in-flight capacity 已接入 Store Runtime
- dispatch event 审计表已新增
- 清理未生效配置仍待处理

Day 5
- 跑 claim、publish rollback、双实例和 recovery 并发测试
- 输出第一份 Control Plane validation report
```

## 第 2 周：Runtime retirement 和质量门禁

```text
Day 1-2
- 抽 ListingRuntimeHealthValidator / ImportTaskLoader 等本地 ports
- 移除 runtime 对 GetManagementClient 的依赖

Day 3
- debug task 改用本地 repository
- 更新 management retirement inventory

Day 4
- CI 切换 go test ./...
- 增加 race/build job

Day 5
- 更新 roadmap/checkpoint/control-plane closeout
- 评审下一轮只剩哪些真实 ownership seam
```

## 7. 不建议继续做的工作

当前暂停以下行为：

```text
为了目录一致性进行大规模 rename / move；
继续把单行 wrapper 从 ListingKit 搬到其他包；
在 Control Plane 稳定前继续增加第二套调度或 watchdog；
在 Management retirement 完成前新增 Management HTTP 业务调用；
在 SHEIN runtime 稳定前复制完整多平台工作台；
没有测试和 ownership 收益的抽象化重构。
```

## 8. 重构完成定义的当前差距

长期完成定义是：

```text
每个核心领域有一个明确事实源；
ListingKit 根包主要承担 facade 和 orchestration；
平台规则位于平台边界；
通用 submission 和 Studio 机制可独立测试；
外部客户端可替换；
真实失败能够安全恢复；
新增平台不需要复制已有状态机和技术债。
```

当前已满足或接近满足：

```text
SDS Batch Graph 成为核心事实源；
SHEIN publishing/workspace 规则大部分已有 owner；
通用 Studio / submission 模块形成；
真实 SDS -> SHEIN save_draft 闭环通过；
架构 review 和 import guards 已形成。
```

当前未满足：

```text
Management Client 从 runtime 类型边界退休；
后端全仓 CI 和关键 race 门禁；
ListingKit root 最终 ownership closeout；
其他平台复制稳定模板。
```

因此本轮重构应定义为：

> **已经进入后半程，但还未到“结构治理完成”的阶段。下一步的价值不在继续搬文件，而在把新事实源和新运行时做成唯一、持久、可验证的生产所有权。**

## 9. 相关文档

- `docs/product/listingkit-project-goals.md`
- `docs/refactoring/listingkit-refactoring-roadmap.md`
- `docs/refactoring/listingkit-boundary-checkpoint.md`
- `docs/refactoring/project-wide-refactoring-plan.md`
- `docs/architecture/project-boundaries.md`
- `docs/architecture/architecture-review-checklist.md`
- `docs/architecture/next-steps.md`
- `docs/architecture/platform-boundary-strategy.md`
- `docs/architecture/external-client-boundary-inventory.md`
- `docs/architecture/compatibility-retirement.md`
- `docs/superpowers/specs/2026-06-23-go-listing-control-plane-design.md`
- `docs/superpowers/plans/2026-06-23-go-listing-control-plane.md`
- `docs/product/validation/runs/2026-06-21-shein-sds-batch-production-closure.md`
- `docs/product/validation/runs/2026-06-21-shein-sds-batch-production-closure-regression.md`

### 2026-06-24 production validation: active deletion takeover

- Deployed xuwei190/task-processor-listing-control-plane:f1f8a06a and verified rollout succeeded.
- Deleted the active control-plane pod to validate leader failover behavior.
- Replacement pod shein-listing-control-plane-6446d7f79f-xj2pc acquired the Redis leader lock after the 30s TTL window.
- /ready confirmed eady=true, isLeader=true, consecutiveErrors=0.
- Dispatch resumed with a healthy observed cycle: dispatchCandidates=10, dispatched=1, skipped=9, ailed=0.

Status: leader active deletion takeover is production-validated. Remaining production validation focus: operator task-list reason visibility and rollback rehearsal.

### 2026-06-24 operator task-list reason visibility

- Added the admin import-task list "dispatch reason" column so operators can see persisted dispatch delay causes from the task row.
- The UI displays easonCode, stage, and the persisted rrorMessage/emark message for delayed tasks.
- The frontend API schema now accepts both camelCase and snake_case variants for delay fields (easonCode/eason_code, rrorMessage/rror_message) to keep the operator view robust across backend serializers.
- Focused frontend test passed: 
pm test -- import-task-admin-page.test.tsx.

Status: task-list operator reason visibility is code-validated. Production UI deployment remains the next integration step if this app is served from a separately deployed frontend artifact.

### 2026-06-24 production validation: dispatch event and task reason persistence

- Read-only production DB observation confirmed listing_dispatch_event is receiving control-plane decisions.
- Last 15 minutes contained 1380 dispatch events, including 37 dispatched events plus skipped events for store_paused, 
o_capacity, and store_unknown.
- Last 60 minutes contained 2738 task rows with non-empty eason_code, confirming task-list-visible delay/error reason fields are being persisted.
- Daily-limit audit fields were present for stores 322/976, 246/1041, and 246/1025, including configured daily_limit and observed queue depth.

Status: backend production persistence for event audit and operator-visible task reasons is validated. Frontend publication for the new task-list column is intentionally deferred. Remaining high-value production exercise: rollback rehearsal.

### 2026-06-24 production validation: rollback rehearsal

- Rolled shein-listing-control-plane from current image 1f8a06a back to previous known-good image 3fd80e1.
- Rollback pod became ready, acquired the Redis leader lock after the expected TTL window, and resumed dispatch with consecutiveErrors=0.
- Rolled forward again to 1f8a06a.
- Final pod became ready, acquired leader ownership, and resumed dispatch with dispatchCandidates=10, dispatched=1, skipped=9, ailed=0.
- Final production image is back on xuwei190/task-processor-listing-control-plane:f1f8a06a.

Status: rollback rehearsal is production-validated. Remaining deferred item: publish the frontend admin task-list reason column when the ListingKit UI deployment window is available.
