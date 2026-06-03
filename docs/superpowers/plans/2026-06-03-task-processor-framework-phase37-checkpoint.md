## Task Processor Framework Phase 37 Checkpoint

### Status

`Phase 37` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action target clone aggregate ownership` 这条切片
- 它没有回头重开 `Phase 36` shared clone helper home move
- 它没有扩大成 generic cloning framework
- 它没有顺手重构 navigation dispatch 执行流

对应计划文档：

- [2026-06-03-task-processor-framework-phase37-action-target-clone-aggregate-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase37-action-target-clone-aggregate-ownership.md:1)

### What Landed

#### 1. Aggregate clone outward behavior 保持稳定

这一轮没有新增 behavior fixture，因为现有 clone 行为测试已经直接覆盖：

- `cloneAssetGenerationActionTarget(...)`
- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

并且本轮验证也重新证明了这些 outward clone semantics 没变。

#### 2. Action-target aggregate clone owner 已把 nested clone shaping 显式委托出去

新增本地 aggregate-shape seam：

- [task_generation_action_target_clone_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone_shape.go:1)

对应提交：

- `d00ed48c` `refactor: clarify listingkit action target clone aggregate ownership`

当前 split 已经很清楚：

- [task_generation_action_target_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
  - 只保留 nil check
  - 只保留 top-level shallow copy
  - 委托给 local aggregate-shape seam

- [task_generation_action_target_clone_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone_shape.go:1)
  - 负责 filters clone delegation
  - 负责 queue/retry clone delegation
  - 负责 expected impact clone delegation
  - 负责 navigation target clone delegation

也就是说，aggregate clone owner 不再把 nested clone shaping 直接内联在一个函数体里。

#### 3. Shared clone helper home 被完整保留

这一轮没有动：

- [task_generation_shared_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_shared_clone.go:1)

这让 `Phase 36` 刚刚收下来的 shared helper home 继续稳定存在，没有为了继续拆 aggregate clone 又回退。

#### 4. Aggregate clone guardrail 已补齐

新增 / 对齐的边界测试：

- [phase37_action_target_clone_aggregate_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase37_action_target_clone_aggregate_boundary_test.go:1)
- [phase22_action_target_clone_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase22_action_target_clone_boundary_test.go:1)

对应提交：

- `91a23d75` `test: lock listingkit action target clone aggregate boundaries`

当前 guardrail 锁住了 4 件事：

- aggregate clone owner 继续只拥有 top-level copy + local shape seam dispatch
- nested clone shaping 继续留在 local aggregate-shape seam
- shared clone helper 继续留在 shared helper home
- outward clone behavior 保持稳定

### Acceptance Check

`Phase 37` 需要证明的核心点有四个：

1. aggregate clone outward behavior 保持稳定
2. action-target aggregate clone owner 不再直接内联 nested clone shaping
3. shared clone helper home 没有被重新搅乱
4. aggregate clone guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 37` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续细拆 navigation target clone aggregate

当前：

- [service_generation_navigation_dispatch_helpers.go](/D:/code-task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)

里的 `cloneGenerationReviewNavigationTarget(...)` 仍然同时知道：

- conditional clone
- descriptor clone
- queue/session/preview query clone
- nested action target clone

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 2. 它没有扩大成 review navigation builder redesign

本阶段只停在 action target aggregate clone，没有去动 broader review navigation construction flow。

### Residual Responsibilities Still Present

`Phase 37` 收完之后，clone 邻域里最显眼的 residual hotspot 已经从 action-target aggregate clone，转移到 review-navigation aggregate clone：

- [service_generation_navigation_dispatch_helpers.go](/D:/code-task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)

当前 `cloneGenerationReviewNavigationTarget(...)` 仍然聚合了多类 nested clone shaping，并且还串着 action-target clone。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit review navigation target clone aggregate ownership

重点锚点：

- [service_generation_navigation_dispatch_helpers.go](/D:/code-task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)
- [generation_review_navigation_target.go](/D:/code-task-processor/internal/listingkit/generation_review_navigation_target.go:1)

原因很直接：

- `Phase 37` 已经把 action-target aggregate clone 收干净
- 当前 clone 邻域里剩下最明显的 aggregate hotspot，就是 review-navigation target clone 这一层
- 这比回头再抠 action-target 或 shared helper home，更像下一块 bounded、低风险、收益清晰的小切片

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- aggregate clone outward behavior 保持稳定
- aggregate clone seam 已按预期落地
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
