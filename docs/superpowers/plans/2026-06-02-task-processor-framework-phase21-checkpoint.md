# Task Processor Framework Phase 21 Checkpoint

## Status

`Phase 21` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `generation action target-resolution helper ownership` 这条切片
- 它没有回头重开 `Phase 19` / `Phase 20` 已稳定的 temporal seams
- 它没有把范围扩成整个 `service_generation_actions.go` 的大拆分
- 它没有引入通用 action helper framework

对应计划文档：

- [2026-06-01-task-processor-framework-phase21-action-target-resolution-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-01-task-processor-framework-phase21-action-target-resolution-ownership.md:1)

## What Landed

### 1. Action target-resolution 已有明确的本地 home

新增本地 target-resolution seam：

- [task_generation_action_target_resolution.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_resolution.go:1)

对应提交：

- `be21a666` `refactor: extract listingkit action target resolution seam`

这一层现在负责：

- action key fallback
- overview `primary + secondary` target traversal
- request-target fallback
- request-target `InteractionMode` defaulting
- returned resolution source (`overview` / `request_target`)

也就是说，`resolveAssetGenerationActionTarget(...)` 这条行为链已经不再混在 broad helper file 里。

### 2. Entry phase 已直接消费新的 local target-resolution home

Entry seam：

- [task_generation_action_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_entry.go:1)

对应提交：

- `d5d083fc` `refactor: route listingkit action execution through local target resolution`

现在通过：

- `buildTaskGenerationActionTargetResolutionPhase().run(queue, req)`

拿到 `resolved target/source`，而不是继续在 entry 内自己做：

- `buildAssetGenerationOverview(queue)`
- `resolveAssetGenerationActionTarget(...)`

与此同时，以下职责仍明确保留在 entry phase：

- impact hydration
- previous review session building
- result / audit shaping

这说明 `Phase 21` 并不只是“把 helper 搬了个文件”，而是真正把 execution consumer 切到了新的 local resolution home。

### 3. Action target-resolution 行为已被 focused tests 锁住

行为测试对应提交：

- `dd7ce43b` `test: lock listingkit action target resolution behavior`

当前锁住的行为包括：

- top-level `ActionKey` 与 `req.Target.ActionKey` 的 fallback
- invalid / missing action key 的当前错误面
- overview `PrimaryActionTarget` 与 `SecondaryActionTargets` 的匹配优先级
- request-target clone/defaulting semantics
- blank defaulting 与 non-blank preserve
- defensive clone 语义

相关测试位于：

- [service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1905)

### 4. Ownership guardrail 已补齐并和既有 boundary suite 对齐

新增 / 对齐的边界测试：

- [phase21_action_target_resolution_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase21_action_target_resolution_boundary_test.go:1)
- [phase18_action_service_entry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase18_action_service_entry_boundary_test.go:1)

对应提交：

- `b4198fba` `test: lock listingkit action target resolution boundaries`

当前 guardrail 锁住了 3 件事：

- target-resolution implementation 必须继续留在 `task_generation_action_target_resolution.go`
- entry phase 必须通过 local resolution seam 消费 target/source
- `service_generation_actions.go` 不再拥有完整的 target-resolution implementation

同时，`Phase 18` 的 service-entry boundary 也已和新的 ownership 分布完成闭环。

## Acceptance Check

`Phase 21` 需要证明的核心点有四个：

1. action target-resolution 行为已有明确本地 home
2. entry seam 已通过这个本地 home 获取 `resolved target/source`
3. `service_generation_actions.go` 不再拥有完整 target-resolution implementation
4. 当前 action-key fallback、overview/request precedence、clone/defaulting outward behavior 保持不变

这四件事现在都成立。

更具体地说：

- [task_generation_action_target_resolution.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_resolution.go:1) 已成为 resolution behavior 的本地 owner
- [task_generation_action_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_entry.go:1) 已通过 resolution phase 拿 target/source
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1) 只剩 service facade、temporal alias 和共享 clone helpers
- [phase21_action_target_resolution_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase21_action_target_resolution_boundary_test.go:1) 与 [phase18_action_service_entry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase18_action_service_entry_boundary_test.go:1) 已共同锁住新的 ownership 分布

因此，`Phase 21` 可以明确认定为已完成。

## What This Phase Did Not Try To Solve

### 1. 它没有继续收 temporal parsing alias

本阶段没有再去对称性清理：

- [resolveLayerTemporalPlatform(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:9)

这是刻意保留下来的小命名债务，而不是本阶段漏做。

### 2. 它没有把共享 clone helper 一起整包搬走

当前仍保留在：

- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [cloneAssetGenerationActionImpact(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

这是一条刻意保留给下一阶段的 residual hotspot，因为这些 clone helpers 当前还有多处非本 slice 路径在复用。

### 3. 它没有重开 execute / persist / refresh / projection seams

本阶段没有重新设计：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
- [task_generation_action_persist.go](/D:/code/task-processor/internal/listingkit/task_generation_action_persist.go:1)
- [task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)
- [task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)

这轮只把 entry seam 的 target-resolution ownership 收干净，没有回头重洗其他 phase。

## Residual Responsibilities Still Present

`Phase 21` 收完之后，`service_generation_actions.go` 里最明显的 residual hotspot 已经不再是 target-resolution，而是剩下的共享 clone helper cluster：

- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [cloneAssetGenerationActionImpact(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

这一簇当前同时服务：

- generation resolved action summary
- generation navigation dispatch
- action target resolution
- platform card / summary attach 等路径

也就是说，target-resolution 行为已经被从 broad helper file 里剥离，下一块更真实的 ownership 压力已经转移到了“共享 clone helper 还没有明确 home”这一层。

## What Should Move To The Next Phase

下一阶段最值得推进的，不再是继续围着 target-resolution 行为做对称性清理，而是：

### 1. ListingKit action target clone helper ownership

重点锚点：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

原因很直接：

- target-resolution 行为已经收清楚
- broad helper file 里剩下最重的一块就是 clone helper cluster
- 它比继续深挖 target-resolution alias 更像下一块真正的 residual hotspot

### 2. 继续保持小切片，而不是一次性掏空所有 helper

下一步更适合先围绕：

- `cloneAssetGenerationActionTarget(...)`
- 以及它牵出的几条 clone/copy semantics

分出一个更明确的 ownership seam，而不是一次性把所有 helper 都打包重构。

## Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*" -count=1
go test ./internal/listingkit -run "Test(TaskGenerationActionServiceEntryBoundary|TaskGenerationActionPhaseOwnershipServiceEntryBoundary|ResolveAssetGenerationActionTarget.*|TaskGenerationActionExecutePhase.*|ExecuteTaskGenerationAction.*)" -count=1
go test ./internal/listingkit -run "TestResolveAssetGenerationActionTarget.*|TestTaskGenerationAction.*Boundary|TestTaskGenerationActionPhaseOwnershipBoundary|TestTaskGenerationActionServiceEntryBoundary|TestTaskGenerationActionPhaseOwnershipServiceEntryBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- target-resolution 行为保持稳定
- entry seam 与新的 local resolution home 的 handoff 保持稳定
- ownership guardrail 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
