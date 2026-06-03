## Task Processor Framework Phase 24 Checkpoint

### Status

`Phase 24` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit review-navigation queue clone shaping ownership` 这条切片
- 它没有回头重开 `Phase 23` 已稳定的 action-target clone reuse
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 queue-clone policy abstraction

对应计划文档：

- [2026-06-03-task-processor-framework-phase24-review-navigation-queue-clone-shaping.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase24-review-navigation-queue-clone-shaping.md:1)

### What Landed

#### 1. Review-navigation queue clone behavior 已先被锁住

在 [generation_review_navigation_target_test.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_test.go:1) 里补齐了 `buildGenerationReviewActionNavigationTarget(...)` 的 queue clone 行为测试：

- 返回的 `QueueQuery` 是新的 clone 指针
- 字段值与源 `QueueQuery` 保持一致
- 修改返回的 `QueueQuery` 不会污染 original
- outward navigation identity / descriptor 行为继续保持不变

对应提交：

- `b34576f7` `test: lock listingkit review navigation queue clone behavior`

这一步先把 review-navigation builder 的真实 outward contract 钉住了。

#### 2. Review-navigation builder 已复用 shared queue clone home

对应提交：

- `a8c9c0b5` `refactor: reuse listingkit queue clone for review navigation`

变更点在：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)

当前：

- `buildGenerationReviewActionNavigationTarget(...)`

不再自己做：

- `cloned := *target.QueueQuery`

而是直接复用：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

这说明 review-navigation builder 里的 common queue clone semantics 已不再由本地 builder 重复持有。

#### 3. Review-navigation local home 只保留真正的本地 shaping

在 `Phase 24` 之前：

- review-navigation builder 同时承载了 local builder 组装
- 和 queue clone 的共享实现细节

`Phase 24` 之后，本地 builder 继续只保留：

- `DispatchKind: "action"`
- `ActionTarget` 通过 `cloneAssetGenerationActionTargetForNavigation(...)`
- `QueueQuery` 通过 shared clone home 取值
- `applyIdentityToNavigationTarget(...)` 的本地组装

这让 review-navigation file 的 ownership 又收紧了一层：它不再是 queue clone 的另一个实现 home。

#### 4. Review-navigation queue clone ownership guardrail 已补齐

新增边界测试：

- [phase24_review_navigation_queue_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go:1)

对应提交：

- `6a07ccd3` `test: lock listingkit review navigation queue clone boundaries`

当前 guardrail 锁住了两组关键边界：

- `buildGenerationReviewActionNavigationTarget(...)` 必须通过 `cloneGenerationQueueQuery(target.QueueQuery)` 复用 shared queue clone home
- `generation_review_navigation_target.go` 只保留 review-navigation-local queue shaping，不再内联 queue shallow clone 实现

同时也明确保持：

- `Phase 23` 的 action-target clone reuse 不被打散
- `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)` 继续留在 shared helper home

### Acceptance Check

`Phase 24` 需要证明的核心点有四个：

1. review-navigation builder 的 queue clone behavior 先被测试锁住
2. common queue clone work 不再在 builder 本地重复实现
3. review-navigation local home 只保留 truly local shaping
4. `Phase 23` 的 action-target clone split 继续保持稳定

这四件事现在都成立。

更具体地说：

- [generation_review_navigation_target_test.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_test.go:1) 已锁住 outward queue clone behavior
- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1) 已通过 `cloneGenerationQueueQuery(...)` 复用 shared queue clone home
- [phase24_review_navigation_queue_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go:1) 已把这个 split 钉住
- `Phase 23` 的 navigation action-target clone boundary 没有被这轮改坏

因此，`Phase 24` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有全面重开 shared `queue/retry` clone helper ownership

本阶段没有继续下钻：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

在 action execute / navigation dispatch / temporal result 这些多 consumer 路径里的更大 owner 问题。

#### 2. 它没有处理 action execute 里的 request handoff shaping

当前：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)

仍然同时持有：

- `RetryTaskGenerationTasks(... cloneRetryGenerationTasksRequest(...))`
- `GetTaskGenerationQueue(... cloneGenerationQueueQuery(...))`
- `buildGenerationReviewSession(...)` 的 persistence-session 输入塑形

这不是本阶段漏掉，而是下一阶段更适合的 residual hotspot。

#### 3. 它没有扩大成所有 queue clone consumer 的统一大清理

本阶段只围绕 review-navigation builder 下刀，没有顺手去改：

- navigation dispatch
- action execute
- temporal result
- descriptor / conditional shaping

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 24` 收完之后，最显眼的 residual hotspot 已经从 review-navigation builder 的 queue clone duplication，转移到 action execute request handoff：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)

当前这条 execute phase 仍然同时承载：

- retry request clone handoff
- queue request clone handoff
- retry / queue 分支选择
- persistence-session input shaping

也就是说，queue clone reuse 已经在 review-navigation 这里收干净了，下一块更真实的 ownership 压力已经转移到了 action execute branch 自己的 request/persistence handoff 邻域。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是继续围着 review-navigation builder 做对称性清理，而是先聚焦：

#### 1. ListingKit action execute request handoff ownership

重点锚点：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

原因很直接：

- `Phase 24` 已把 review-navigation builder 里的 queue clone duplication 收掉
- 当前最接近的下一波 pressure，不是 helper 定义位置，而是 execute phase 自己如何持有 request clone / persistence-session handoff
- 这比直接重开 shared helper home 的多 consumer 大题，更像一个 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 action execute 邻域内的小步收口

下一步更适合只围绕：

- queue vs retry request clone handoff
- persistence-session input shaping
- execute-phase outward behavior stability

下刀，而不是一次性去改 service-level shared clone helper owner。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- review-navigation queue clone behavior 保持稳定
- builder 对 shared queue clone home 的复用保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
