# Task Processor Framework Phase 18 Checkpoint

## Status

`Phase 18` 已按目标切片完成，可以把这一阶段视为功能上收口的 checkpoint。

这一阶段没有去重开 `Phase 17` 已稳定的 projection seams，也没有提前把范围扩成通用 temporal action framework。目标更聚焦：

1. 从 `ExecuteTaskGenerationAction(...)` service entry 中抽出 entry/bootstrap seam
2. 抽出 persisted-review durable handoff seam
3. 抽出 post-projection finalization seam
4. 用 service-entry guardrail 把新的编排顺序和所有权边界锁住

这个目标现在已经达成。

## What Landed

### 1. Action entry/bootstrap 现在有了明确的 feature-local seam

新的 entry seam 位于：

- [internal/listingkit/task_generation_action_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_entry.go:1)

这部分落在以下提交里：

- `35714784` `refactor: extract listingkit action entry seam`
- `fa96e1e6` `test: cover listingkit action entry seam`

`ExecuteTaskGenerationAction(...)` 现在通过 `buildTaskGenerationActionEntryPhase(s).run(...)` 进入本地 action path：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

这个 seam 现在负责：

- queue / base result bootstrap
- target resolution
- `ExpectedImpact` 缺省回填
- `previousReviewSession` 构造
- outward `GenerationActionExecutionResult` 基础壳和 audit 构造

这一步的意义不是单纯把代码搬进新文件，而是把 service entry 里原本混在一起的“前置状态准备 + target/audit shaping”收进了 ListingKit 本地可见的边界。

### 2. Persisted-review handoff 现在通过独立 seam 做 durable handoff

新的 persistence seam 位于：

- [internal/listingkit/task_generation_action_persist.go](/D:/code/task-processor/internal/listingkit/task_generation_action_persist.go:1)

这部分落在以下提交里：

- `b68ad97e` `refactor: extract listingkit action persistence seam`

`persisted review decision` 现在通过 `buildTaskGenerationActionPersistPhase(s).run(...)` 完成 durable handoff，并且仍然保持在：

1. execution 之后
2. refresh 之前

这个 seam 现在负责：

- persisted-review action eligibility check
- `execution.persistenceSession` handoff
- `persistGenerationReviewDecision(...)` 的 nil-safe durable 调用

关键点在于，service entry 不再自己持有 persisted-review 写入判断和 handoff 细节，但时序没有变化。

### 3. Post-projection finalization 现在通过独立 seam 收尾

新的 finalization seam 位于：

- [internal/listingkit/task_generation_action_finalize.go](/D:/code/task-processor/internal/listingkit/task_generation_action_finalize.go:1)

这部分落在以下提交里：

- `0fa1f8c0` `refactor: extract listingkit action finalization seam`

`ExecuteTaskGenerationAction(...)` 现在通过 `buildTaskGenerationActionFinalizePhase().run(...)` 完成最终收尾：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

这个 seam 现在负责：

- projection 字段 copy-back 到 outward result
- `applyGenerationConditionalStateToActionResult(result)` 最终应用

也就是说，projection 完成后，service entry 不再自己拼接 outward result 的最后一段状态，而是交给一个明确的 finalization boundary。

### 4. Service-entry guardrail 已和新顺序对齐

边界保护现在位于：

- [internal/listingkit/phase18_action_service_entry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase18_action_service_entry_boundary_test.go:1)
- [internal/listingkit/phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)

这部分落在以下提交里：

- `88fa74cb` `test: lock listingkit action service-entry boundaries`

这些 guardrail 现在锁住了本地 action path 的顺序：

1. entry
2. execute
3. persist
4. refresh
5. projection
6. finalize

并且继续要求 `executeLayerTemporalAction(...)` 保持 service entry 的最前置短路边界：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

这一步很重要，因为它把 `Phase 18` 的核心成果从“代码被拆了”提升成“拆分顺序和边界被测试锁住了”。

## Acceptance Check

`Phase 18` 原本需要证明四件事：

1. action bootstrap 可以经由显式的 ListingKit-owned seam 进入本地 action path
2. persisted-review durable handoff 可以经由显式 seam 保持原有时序
3. post-projection finalization 可以经由显式 seam 保持 outward result shaping 顺序
4. service-entry orchestration split 可以被稳定 guardrail 锁住

现在这四件事都成立。

更具体地说：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164) 不再直接持有完整的 bootstrap / persist / finalize 内联块
- entry、persist、finalize 都有了各自的本地 home
- persisted-review handoff 仍然发生在 execution 之后、refresh 之前
- projection copy-back 之后仍然会再应用 conditional state finalization
- service entry 的本地路径顺序已经被边界测试锁成 `entry -> execute -> persist -> refresh -> projection -> finalize`

这已经足够把 `Phase 18` 定义为按预期完成的切片。

## What This Phase Did Not Try To Solve

### 1. 它没有回头重开已经稳定的 lower seams

这一阶段没有重开：

- [internal/listingkit/task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
- [internal/listingkit/task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)
- [internal/listingkit/task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)
- [internal/listingkit/task_generation_action_projection_session.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection_session.go:1)
- [internal/listingkit/task_generation_action_projection_finalize.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection_finalize.go:1)

这是刻意的边界控制。`Phase 18` 的目标不是重新拆一次 execute/refresh/projection，而是把 service entry 上下游剩余的 orchestration 压力拿出来。

### 2. 它没有重做 layer-temporal branching ownership

这一阶段也没有重开：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

`executeLayerTemporalAction(...)` 仍然同时持有 action-key branching、workflow client enablement/config checks、platform resolution、start-input assembly、outward result assembly、audit shaping。

这仍然是后续热点，但它不是 `Phase 18` 要解决的问题。

### 3. 它没有把范围扩成通用 temporal action framework

这一阶段没有尝试抽象出 repo-wide 或 cross-feature 的 temporal action framework。所有新增 seam 都保持在 ListingKit feature-local 边界里：

- [internal/listingkit/task_generation_action_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_entry.go:1)
- [internal/listingkit/task_generation_action_persist.go](/D:/code/task-processor/internal/listingkit/task_generation_action_persist.go:1)
- [internal/listingkit/task_generation_action_finalize.go](/D:/code/task-processor/internal/listingkit/task_generation_action_finalize.go:1)

这也是对的，因为当前根因是 ListingKit action service entry 的局部所有权聚集，不是缺一个通用框架。

## Residual Responsibilities Still Present

### Layer-temporal bypass 现在是这个邻域里最明显的剩余混合块

`Phase 18` 收完以后，`ExecuteTaskGenerationAction(...)` 的 service-entry orchestration 已经明显变薄，剩下仍然显著混合多种职责的热点是：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

`executeLayerTemporalAction(...)` 目前仍同时持有：

1. action-key branching ownership
2. workflow client enablement / config checks
3. platform resolution
4. temporal start-input assembly
5. outward `GenerationActionExecutionResult` assembly
6. audit shaping

这说明问题根源已经从“service entry 什么都管”进一步收敛成“layer-temporal 旁路自身仍然在一个 helper 里聚集多种职责”。

## What Should Move To The Next Phase

如果继续往下拆，下一阶段最值得处理的就是：

### 1. ListingKit layer-temporal action branching ownership

也就是围绕：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

继续做 feature-local seam 收敛，而不是回头重开已经稳定的：

- entry seam
- persist seam
- refresh seam
- projection seams
- finalize seam

### 2. 保持切片只在 ListingKit action 邻域内前进

下一步应该继续停留在 ListingKit action temporal bypass 本身，不要跳到 HTTP/bootstrap/runtime 之外，也不要直接扩成“所有 temporal action 的统一框架”。

## Verification Summary

本阶段 fresh 通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationAction.*|TestTaskGenerationActionProjection.*|TestTaskGenerationActionRefresh.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以覆盖当前切片，因为它们同时覆盖了：

- action entry / persist / finalize seams 对现有 action pipeline 的集成影响
- projection / refresh / broader action surfaces 的回归风险
- ListingKit HTTP 与 temporal 下游编译和测试面
