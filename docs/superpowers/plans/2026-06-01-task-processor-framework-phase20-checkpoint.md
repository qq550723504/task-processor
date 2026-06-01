# Task Processor Framework Phase 20 Checkpoint

## Status

`Phase 20` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `layer-temporal request-shape parsing ownership` 这条切片
- 它没有回头重开 `Phase 19` 已稳定的 temporal execution seams
- 它没有把范围扩成整个 `service_generation_actions.go` 的大拆分
- 它没有引入通用 request-parsing framework

对应计划文档：

- [2026-06-01-task-processor-framework-phase20-layer-temporal-request-shape-parsing.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-01-task-processor-framework-phase20-layer-temporal-request-shape-parsing.md:1)

## What Landed

### 1. Layer-temporal request parsing 已有明确的本地 home

新增本地 parsing seam：

- [task_generation_action_temporal_request_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_request_platform.go:1)

对应提交：

- `1704ffc4` `refactor: extract listingkit temporal platform request seam`

这一层现在负责：

- temporal request-shape traversal
- platform trimming / lowercasing
- nested action target recursion
- `shein` 默认值

也就是说，`layer-temporal` 的 request parsing 不再由更泛的 action helper file 直接承载实现本体。

### 2. Legacy helper 已被压成 seam alias

当前 broad helper file：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)

里保留下来的：

- [resolveLayerTemporalPlatform(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

现在只是一个 alias：

- 委托给 `resolveTemporalRequestPlatform(req)`

这意味着 `service_generation_actions.go` 不再拥有 traversal implementation，本阶段的 ownership 已经从“helper file 内联实现”收敛成“legacy alias + 本地 owner”。

### 3. Platform temporal seam 已直接消费新的 parsing home

平台 temporal seam：

- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)

对应提交：

- `09671620` `refactor: route listingkit platform temporal seam through local request parsing`

现在直接依赖：

- `resolveTemporalRequestPlatform(req)`

并保持原有职责不变：

- workflow enablement / client checks
- platform temporal workflow start-input assembly
- shared temporal result seam handoff

这说明 `Phase 20` 并不只是“把实现搬了个家”，而是把真实的 platform temporal consumer 也切到了新的 local parsing home。

### 4. Request-parsing ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase20_action_temporal_request_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase20_action_temporal_request_boundary_test.go:1)
- [phase19_action_layer_temporal_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase19_action_layer_temporal_boundary_test.go:1)

对应提交：

- `0eac1e9a` `test: lock listingkit temporal request parsing boundaries`

当前 guardrail 锁住了 3 件事：

- traversal implementation 必须继续留在 `task_generation_action_temporal_request_platform.go`
- platform temporal phase 必须消费 `resolveTemporalRequestPlatform`
- `service_generation_actions.go` 里的 legacy helper 只能保留 seam alias，不再长回 traversal 细节

同时，`Phase 19` 的 execution seam 职责没有被这轮 request-parsing slice 重新打散。

### 5. 行为测试 home 也收紧了一层

对应提交：

- `cab52dde` `test: lock layer temporal platform parsing behavior`

以及在 `Task 4` 里完成的测试 home 调整。

当前结果是：

- temporal parsing 行为测试已经不再混在更泛的 `service_generation_actions_test.go` 里
- `service_generation_actions_test.go` 继续保留 service / runtime 行为验证
- parsing behavior 与 ownership guardrail 现在有了更贴近 temporal seam 的本地 home

## Acceptance Check

`Phase 20` 需要证明的核心点有四个：

1. temporal request-shape traversal implementation 已有明确本地 home
2. platform temporal seam 已通过这个本地 home 取平台，而不是继续依赖 broad helper implementation
3. `service_generation_actions.go` 不再拥有 traversal 细节，只保留 legacy seam alias
4. `Phase 19` execution seams 的职责边界保持稳定

这四件事现在都成立。

更具体地说：

- [task_generation_action_temporal_request_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_request_platform.go:1) 已成为 traversal/defaulting/normalization 的唯一实现 home
- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1) 已直接调用 `resolveTemporalRequestPlatform(req)`
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13) 里的 `resolveLayerTemporalPlatform(...)` 已缩成 alias
- [phase19_action_layer_temporal_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase19_action_layer_temporal_boundary_test.go:1) 继续锁住 router / standard / platform / result phase 的 execution responsibilities

因此，`Phase 20` 可以明确认定为已完成。

## What This Phase Did Not Try To Solve

### 1. 它没有启动整个 action helper cluster 的宽拆分

本阶段没有顺手重构这些更宽的 helpers：

- [resolveAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [collectAssetGenerationActionTargets(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [requestedAssetGenerationActionKey(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

这是一条刻意保留下来的下一阶段热点，而不是本阶段漏做。

### 2. 它没有抽通用 request-parsing framework

本阶段所有改动都保持在 ListingKit feature-local 范围内：

- [task_generation_action_temporal_request_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_request_platform.go:1)
- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)

没有试图为 repo 里所有 request traversal 提前引入抽象层。

### 3. 它没有重开 Phase 19 的 execution seam 设计

这轮虽然更新了：

- [phase19_action_layer_temporal_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase19_action_layer_temporal_boundary_test.go:1)

但更新只是在对齐新的 parsing home，并没有重新设计 `router / standard / platform / result` 这四层 execution seam 的职责。

## Residual Responsibilities Still Present

`Phase 20` 收完之后，`service_generation_actions.go` 里最明显的 residual hotspot 已经不再是 temporal parsing，而是更宽的 action target helper cluster：

- [resolveAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [collectAssetGenerationActionTargets(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [requestedAssetGenerationActionKey(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

这一簇现在同时承载：

- action key normalization / lookup
- overview / request target resolution
- interaction-mode defaulting
- target clone / copy semantics

也就是说，temporal-specific parsing 已经被从 broad helper file 里剥离，下一块更真实的 ownership 压力已经转移到了更宽的 generation action target-resolution 邻域。

## What Should Move To The Next Phase

下一阶段最值得推进的，不再是继续围着 temporal parsing 做对称性清理，而是：

### 1. ListingKit generation action target-resolution helper ownership

重点锚点：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

原因很直接：

- temporal request parsing ownership 已经收清楚
- broad helper file 里剩下最重的一块就是 action target resolution / clone / key helper cluster
- 它比继续深挖 temporal parsing 更像下一块真正的 residual hotspot

### 2. 继续保持小切片，而不是一次性拆空整个 helper file

下一步更适合先围绕：

- `action key -> target lookup -> target clone/defaulting`

这条链分出一个更明确的 ownership seam，而不是一次性把所有 clone helper 都打包重构。

## Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*" -count=1
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*|TestTaskGenerationLayerTemporalPlatform.*|TestExecuteTaskGenerationActionStartsPlatformAdaptTemporalWorkflow" -count=1
go test ./internal/listingkit -run "TestResolveLayerTemporalPlatform.*|TestTaskGenerationLayerTemporal.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- temporal parsing 行为保持稳定
- platform temporal seam 与新 local parsing home 的 handoff 保持稳定
- request-parsing ownership guardrail 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
