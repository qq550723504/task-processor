# Task Processor Framework Phase 19 Checkpoint

## Status

`Phase 19` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `layer-temporal action branching ownership` 这条切片
- 它没有回头重开 `Phase 18` 已稳定的 service-entry seams
- 它没有把范围扩成通用 temporal framework

对应计划文档：

- [2026-06-01-task-processor-framework-phase19-layer-temporal-action-branching.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-01-task-processor-framework-phase19-layer-temporal-action-branching.md:1)

## What Landed

### 1. Layer-temporal result shaping 已有共享本地 seam

新增共享 temporal result seam：

- [task_generation_action_temporal_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_result.go:1)

对应提交：

- `fd4cd9ef` `refactor: extract listingkit temporal action result seam`

这一层现在负责：

- shared `queue_only` outward result shaping
- shared `layer_temporal` audit shaping
- temporal outward `ResolvedTarget.QueueQuery` shaping

这意味着 temporal 分支不再各自在 router 内直接拼装 outward result / audit。

### 2. Standard/product temporal start-input assembly 已下放到标准分支 seam

新增 standard temporal seam：

- [task_generation_action_temporal_standard.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_standard.go:1)

对应提交：

- `f09b9c7c` `refactor: extract listingkit standard temporal branch seam`

这一层现在负责：

- standard temporal workflow enablement / client checks
- standard/product temporal start-input assembly
- 调用 shared temporal result seam 返回 outward queue-only 结果

`executeLayerTemporalAction(...)` 不再直接内联标准 temporal workflow 的启动输入组装。

### 3. Platform temporal platform-resolution 与 start-input assembly 已下放到平台分支 seam

新增 platform temporal seam：

- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)

对应提交：

- `99cf9380` `refactor: extract listingkit platform temporal branch seam`

这一层现在负责：

- platform temporal workflow enablement / client checks
- platform resolution
- platform temporal start-input assembly
- 调用 shared temporal result seam 返回 outward queue-only 结果

也就是说，platform temporal 分支的“先解平台，再组装 workflow start input”现在有了明确的 ListingKit 本地 home。

### 4. Temporal branch router 已被压薄并由边界测试锁住

关键边界测试：

- [phase19_action_layer_temporal_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase19_action_layer_temporal_boundary_test.go:1)

对应提交：

- `b500661f` `test: lock listingkit layer temporal action boundaries`

当前结果是：

- `executeLayerTemporalAction(...)` 现在是 router-only 入口
- 它主要只做 action-key routing、standard/platform seam delegation、以及 non-temporal `handled=false` fallback

也就是：

- [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

不再承担 standard/platform temporal start-input assembly，也不再承担 outward queue-only result/audit shaping。

## Acceptance Check

`Phase 19` 需要证明的核心点有四个：

1. `executeLayerTemporalAction(...)` 仍保留 outer temporal short-circuit entry，但已经变成 router-only 入口
2. standard/product temporal start-input assembly 已有独立 standard seam
3. platform temporal start-input assembly 与 platform resolution 已有独立 platform seam
4. outward queue-only result/audit shaping 已有 shared temporal result seam

这四件事现在都成立。

更具体地说：

- [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220) 仍是 temporal bypass entry，但不再内联完整 temporal branch implementation
- [task_generation_action_temporal_standard.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_standard.go:1) 已承接 standard temporal workflow start-input assembly
- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1) 已承接 platform resolution 与 platform temporal workflow start-input assembly
- [task_generation_action_temporal_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_result.go:1) 已成为 shared outward queue-only result / audit shaping 的唯一 home
- non-temporal fallback 保持不变

因此，`Phase 19` 可以明确认定为已完成。

## What This Phase Did Not Try To Solve

### 1. 它没有去重开 Phase 18 的 service-entry seams

本阶段没有回头重开这些已经稳定的边界：

- [task_generation_action_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_entry.go:1)
- [task_generation_action_persist.go](/D:/code/task-processor/internal/listingkit/task_generation_action_persist.go:1)
- [task_generation_action_finalize.go](/D:/code/task-processor/internal/listingkit/task_generation_action_finalize.go:1)
- [task_generation_action_refresh.go](/D:/code/task-processor/internal/listingkit/task_generation_action_refresh.go:1)
- [task_generation_action_projection.go](/D:/code/task-processor/internal/listingkit/task_generation_action_projection.go:1)

这是刻意保持切片边界稳定，而不是重新洗牌已收敛的 service-entry orchestration。

### 2. 它没有扩成通用 temporal framework

本阶段所有新增 seam 都保持在 ListingKit feature-local 范围内：

- [task_generation_action_temporal_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_result.go:1)
- [task_generation_action_temporal_standard.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_standard.go:1)
- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)

它没有尝试抽 repo-wide 的 temporal action abstraction，也没有引入 generic request-parsing framework。

### 3. 它没有试图一次性重构更宽的 action target resolver ownership

虽然 temporal 分支继续依赖：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

但本阶段没有把整个 `service_generation_actions.go` 做宽范围拆分，也没有顺手重构 `resolveAssetGenerationActionTarget(...)` 一带的通用 helper ownership。

## Residual Responsibilities Still Present

`Phase 19` 收完以后，剩余热点已经不再是 temporal branch router 本身，而是更靠近 request-shape / helper ownership。

当前最明显的 residual hotspot 是：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

`resolveLayerTemporalPlatform(...)` 仍然负责沿着多个 request-shape 层级做 platform 提取，包括：

- `QueueQuery`
- `NavigationTarget`
- `SessionQuery`
- `PreviewQuery`
- `FollowUpReads`
- nested `ActionTarget`

这说明 temporal execution seams 已经收清楚，但 temporal request-shape parsing ownership 还没有真正落到更本地的 home。

## What Should Move To The Next Phase

下一阶段最值得推进的，不再是继续拆 temporal branch router，而是：

### 1. ListingKit layer-temporal request-shape parsing ownership

重点锚点：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

原因很直接：

- `Phase 19` 已把 standard / platform / result 三个 temporal execution seams 收清楚
- temporal 邻域里剩下最明显的 feature-specific logic 已经变成 request-shape traversal
- 这个 helper 仍和更泛的 action helpers 混在一起，ownership 还不够本地

### 2. 继续保持小切片，而不是扩成大拆分

下一步更适合先收 temporal-specific request parsing，而不是立刻回到整个 action target resolver 的广义整理。

## Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationLayerTemporal.*|TestExecuteTaskGenerationActionStarts(StandardProductTemporalWorkflow|PlatformAdaptTemporalWorkflow)|TestTaskGenerationActionPhaseOwnershipBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- layer-temporal router / branch seam / result seam 的边界已经按预期落地
- standard 与 platform temporal workflow 启动路径保持稳定
- HTTP 与 temporal 下游测试面没有被这次切片回归破坏
