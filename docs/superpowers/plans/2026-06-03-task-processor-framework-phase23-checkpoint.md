## Task Processor Framework Phase 23 Checkpoint

### Status

`Phase 23` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit navigation action-target clone shaping ownership` 这条切片
- 它没有回头重开 `Phase 22` 已稳定的 broader clone split
- 它没有重开 shared `queue/retry` clone helper 的更大归属问题
- 它没有引入通用 clone strategy / framework

对应计划文档：

- [2026-06-02-task-processor-framework-phase23-navigation-action-target-clone-shaping.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-02-task-processor-framework-phase23-navigation-action-target-clone-shaping.md:1)

### What Landed

#### 1. Navigation-specific clone behavior 已先被锁住

在 [generation_review_navigation_target_test.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_test.go:1) 里补齐了两组关键行为测试：

- `cloneAssetGenerationActionTargetForNavigation(...)`
- `buildGenerationReviewActionNavigationTarget(...)`

这一步锁住了当前真实契约：

- `nil` 输入返回 `nil`
- common nested fields 仍然 defensive clone
- navigation action target 会清空 `NavigationTarget`
- 修改返回的 navigation target / action target 不会污染 original
- outward identity / descriptor 继续与当前实现保持一致

这组测试现在已经在主工作区里，并已做 fresh 验证通过。

#### 2. Navigation clone 已复用 `Phase 22` 的 local clone home

对应提交：

- `c81d05a1` `refactor: reuse listingkit action target clone for navigation shaping`

变更点在：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)

当前 `cloneAssetGenerationActionTargetForNavigation(...)` 不再重复持有：

- filters clone
- queue query clone
- retry request clone
- expected impact clone

它现在先复用：

- [cloneAssetGenerationActionTarget(...)](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

然后只保留 navigation-specific shaping：

- `NavigationTarget = nil`

这说明 common action-target clone semantics 已继续收敛到 `Phase 22` 刚建立的 local clone home。

#### 3. Navigation local home 现在只保留真正属于 navigation 的那部分差异

在 `Phase 23` 之前：

- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)

既承担 common clone work，也承担 navigation-specific shaping。

`Phase 23` 之后，这条 helper 的职责已经收窄成：

- 先复用 common clone seam
- 再应用 navigation-only delta

因此 review-navigation file 的 ownership 现在更清楚了：它不再是“另一套 action-target clone 实现”，而是“navigation-specific shaping home”。

#### 4. Navigation clone ownership guardrail 已补齐

新增边界测试：

- [phase23_navigation_action_target_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go:1)

对应提交：

- `204be649` `test: lock listingkit navigation action target clone boundaries`

当前 guardrail 锁住了 3 件事：

- `cloneAssetGenerationActionTargetForNavigation(...)` 必须先走 `cloneAssetGenerationActionTarget(target)`
- navigation local home 只能再额外做 `NavigationTarget = nil` 这类 navigation-specific shaping
- common clone home 继续持有 shared action-target clone semantics，不允许 navigation-specific shaping 回流进去

### Acceptance Check

`Phase 23` 需要证明的核心点有四个：

1. navigation-specific action-target clone behavior 先被测试锁住
2. common action-target clone work 不再在 navigation local home 里重复实现
3. navigation local home 只保留 review-navigation-specific shaping
4. `Phase 22` 的 local clone split 不被打散

这四件事现在都成立。

更具体地说：

- [generation_review_navigation_target_test.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_test.go:1) 已锁住 outward behavior
- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1) 已不再重复 common clone work
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1) 继续持有 shared action-target clone semantics
- [phase23_navigation_action_target_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 23` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared queue / retry clone helper ownership

本阶段没有继续下钻：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

这些 helper 的多 consumer shared ownership 问题仍然刻意留给后续阶段判断。

#### 2. 它没有处理 review-navigation builder 里残留的 queue clone shaping

当前：

- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)

仍然在本地做：

- `cloned := *target.QueueQuery`

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 generation review navigation file 的全面重写

本阶段只收 `action-target clone shaping` 这一块，没有顺手去改：

- session query builder
- preview query builder
- identity application flow

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 23` 收完之后，review-navigation 邻域里最显眼的 residual hotspot 已经从 action-target clone duplication，转移到 queue clone shaping：

- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

当前 builder 仍然本地做：

- `cloned := *target.QueueQuery`

而 repo 内已经存在一个明确的 shared queue clone helper：

- `cloneGenerationQueueQuery(...)`

这说明下一块更真实的压力不是 action-target clone，而是 review-navigation builder 与 shared queue clone home 之间的重复 queue-shaping ownership。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是马上全面重开 shared `queue/retry` helper，而是先聚焦：

#### 1. ListingKit review-navigation queue clone shaping ownership

重点锚点：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

原因很直接：

- `Phase 23` 已经把 action-target clone duplication 收掉
- 当前 review-navigation 邻域里剩下最接近的重复，就是 builder 自己做 queue shallow clone
- 这比直接重开 shared `queue/retry` helper 的整体 owner，更像一个 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 review-navigation 邻域内的小步收口

下一步更适合只围绕：

- queue clone reuse
- builder-local shaping
- outward identity stability

下刀，而不是一次性把 action execute / dispatch / temporal result 全部卷进来。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTargetForNavigation.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTargetForNavigation.*|TestCloneAssetGeneration.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestGenerationReviewActionNavigationTarget.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- navigation-specific clone behavior 保持稳定
- common clone reuse 与 navigation-specific shaping split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
