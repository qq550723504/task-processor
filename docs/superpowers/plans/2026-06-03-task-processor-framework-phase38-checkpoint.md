## Task Processor Framework Phase 38 Checkpoint

### Status

`Phase 38` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit review navigation target clone aggregate ownership` 这条切片
- 它没有回头重开 `Phase 36` shared clone helper home move
- 它没有回头重开 `Phase 37` action-target aggregate clone split
- 它没有扩大成 broader review navigation dispatch redesign

对应计划文档：

- [2026-06-03-task-processor-framework-phase38-review-navigation-target-clone-aggregate-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase38-review-navigation-target-clone-aggregate-ownership.md:1)

### What Landed

#### 1. Review-navigation aggregate clone outward behavior 已被显式锁住

新增直接行为夹具：

- [generation_review_navigation_target_test.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_test.go:1)

这轮直接锁住了：

- `cloneGenerationReviewNavigationTarget(nil)` 继续返回 `nil`
- top-level `DispatchKind` 和 identity semantics 继续保持不变
- `Conditional / Descriptor / QueueQuery / SessionQuery / PreviewQuery / ActionTarget` 都继续做 defensive clone
- clone 后对返回值的修改不会回写污染原始 target

对应提交：

- `ae76ce17` `refactor: clarify listingkit review navigation clone aggregate ownership`

#### 2. Review-navigation aggregate clone owner 已把 nested clone shaping 显式委托出去

新增本地 aggregate-shape seam：

- [generation_review_navigation_target_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_clone_shape.go:1)

当前 split 已经很清楚：

- [service_generation_navigation_dispatch_helpers.go](/D:/code/task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)
  - `cloneGenerationReviewNavigationTarget(...)` 只保留 nil check
  - 只保留 top-level shallow copy
  - 委托给 local aggregate-shape seam
  - 最后继续走 `applyIdentityToNavigationTarget(...)`

- [generation_review_navigation_target_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target_clone_shape.go:1)
  - 负责 conditional clone delegation
  - 负责 descriptor clone delegation
  - 负责 queue / session / preview query clone delegation
  - 负责 nested action-target clone delegation

也就是说，review-navigation aggregate clone owner 不再把 nested clone shaping 直接内联在一个函数体里。

#### 3. 既有 clone homes 被完整保留

这一轮没有动：

- [task_generation_shared_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_shared_clone.go:1)
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

这让 `Phase 36` 和 `Phase 37` 刚刚收下来的 clone homes 继续稳定存在，没有为了继续拆 review-navigation aggregate clone 又回退。

#### 4. Review-navigation aggregate clone guardrail 已补齐

新增边界测试：

- [phase38_review_navigation_clone_aggregate_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase38_review_navigation_clone_aggregate_boundary_test.go:1)

对应提交：

- `7c05191a` `test: lock listingkit review navigation clone aggregate boundaries`

当前 guardrail 锁住了 4 件事：

- aggregate review-navigation clone owner 继续只拥有 top-level copy + local shape seam dispatch
- nested clone shaping 继续留在 local aggregate-shape seam
- shared / action-target clone homes 继续留在各自既有 home
- outward clone behavior 继续保持稳定

### Acceptance Check

`Phase 38` 需要证明的核心点有四个：

1. aggregate review-navigation clone outward behavior 保持稳定
2. review-navigation aggregate clone owner 不再直接内联 nested clone shaping
3. shared / action-target clone homes 没有被重新搅乱
4. aggregate clone guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 38` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续细拆 navigation descriptor clone aggregate

当前：

- [generation_navigation_target_conditional.go](/D:/code/task-processor/internal/listingkit/generation_navigation_target_conditional.go:1)

里的 `cloneGenerationNavigationDescriptor(...)` 仍然同时知道：

- conditional clone
- dispatch plan clone
- invalidates slice clone
- follow-up reads clone

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 2. 它没有扩大成 review navigation builder redesign

本阶段只停在 review-navigation target aggregate clone，没有去动 broader navigation construction / dispatch flow。

### Residual Responsibilities Still Present

`Phase 38` 收完之后，clone 邻域里最显眼的 residual hotspot 已经从 review-navigation target aggregate clone，转移到 navigation descriptor aggregate clone：

- [generation_navigation_target_conditional.go](/D:/code/task-processor/internal/listingkit/generation_navigation_target_conditional.go:1)

当前 `cloneGenerationNavigationDescriptor(...)` 仍然聚合了多类 nested clone shaping，并且还串着 `cloneGenerationNavigationDispatchPlan(...)`。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit navigation descriptor clone aggregate ownership

重点锚点：

- [generation_navigation_target_conditional.go](/D:/code/task-processor/internal/listingkit/generation_navigation_target_conditional.go:1)
- [generation_navigation_descriptor.go](/D:/code/task-processor/internal/listingkit/generation_navigation_descriptor.go:1)

原因很直接：

- `Phase 38` 已经把 review-navigation target aggregate clone 收干净
- 当前 clone 邻域里剩下最明显的 aggregate hotspot，就是 descriptor clone 这一层
- 这比回头再抠 review-navigation target 或 shared helper home，更像下一块 bounded、低风险、收益清晰的小切片

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary|TestGenerationReviewNavigationCloneAggregateBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- aggregate review-navigation clone outward behavior 保持稳定
- aggregate review-navigation clone seam 已按预期落地
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
