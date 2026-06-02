## Task Processor Framework Phase 22 Checkpoint

### Status

`Phase 22` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action target clone helper ownership` 这条切片
- 它没有回头重开 `Phase 19` / `Phase 20` / `Phase 21` 已稳定的行为 seams
- 它没有把范围扩成整个 helper 邻域的大拆分
- 它没有引入通用 clone framework

对应计划文档：

- [2026-06-02-task-processor-framework-phase22-clone-helper-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-02-task-processor-framework-phase22-clone-helper-ownership.md:1)

### What Landed

#### 1. Clone behavior 已先被锁住

在 [service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1) 里补齐了这 4 组 helper 的当前 clone 语义：

- `cloneAssetGenerationActionTarget(...)`
- `cloneAssetGenerationActionImpact(...)`
- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

对应提交：

- `d1faccea` `test: lock listingkit action target clone behavior`

这一步先把真实契约钉住了：

- `nil` 输入返回 `nil`
- 返回值是新的指针
- 应当 deep-clone 的 nested pointers / slices 继续 defensive clone
- 修改 clone 不会污染 original
- `GenerationQueueQuery` 继续保持当前 shallow struct copy 语义

#### 2. Action-target-local clone semantics 已有更窄的 home

新增本地 clone home：

- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

对应提交：

- `004a6f11` `refactor: clarify listingkit action target clone ownership`

这一步把：

- `cloneAssetGenerationActionTarget(...)`
- `cloneAssetGenerationActionImpact(...)`

从 broad helper file：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)

里收了出来。

现在 `action target` 本地 clone 语义不再混在更宽的 shared helper cluster 里。

#### 3. 真正共享的 clone helpers 被明确保留下来

仍然留在 shared helper home 的只有：

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

也就是 [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1) 现在只继续承载多处路径确实仍在共享的 queue / retry clone。

这意味着 `Phase 22` 不是简单地“把函数搬个家”，而是把 broad helper cluster 明确拆成了：

- action-target-local clone semantics
- still-shared queue / retry clone semantics

#### 4. Consumer 路径没有被迫重写

这轮没有引入真实的 `Task 3` consumer 改写。

原因是所有调用方仍在同一个 `listingkit` package 内，包级函数解析保持不变，所以：

- 不需要改 import
- 不需要改调用签名
- 不需要做行为适配

这也说明这次 ownership split 保持住了最小写面。

#### 5. Clone ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase22_action_target_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase22_action_target_clone_boundary_test.go:1)
- [phase21_action_target_resolution_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase21_action_target_resolution_boundary_test.go:1)

对应提交：

- `cb503efc` `test: lock listingkit action target clone boundaries`

当前 guardrail 锁住了 3 件事：

- `cloneAssetGenerationActionTarget(...)` 和 `cloneAssetGenerationActionImpact(...)` 必须继续留在 feature-local clone home
- `cloneGenerationQueueQuery(...)` 和 `cloneRetryGenerationTasksRequest(...)` 必须继续留在 shared helper home
- `Phase 21` 的 target-resolution ownership 边界不因为这次 clone split 被打散

### Acceptance Check

`Phase 22` 需要证明的核心点有四个：

1. 当前 clone 行为先被测试锁住
2. action-target-local clone semantics 不再留在 broad helper file
3. truly shared queue / retry clone semantics 继续留在 shared home
4. 这次 ownership split 不要求 consumer 路径发生行为改写

这四件事现在都成立。

更具体地说：

- [service_generation_actions_test.go](/D:/code/task-processor/internal/listingkit/service_generation_actions_test.go:1) 已锁住 outward clone behavior
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1) 已成为 action-target-local clone semantics 的 owner
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1) 只保留 shared queue / retry clone helpers
- 没有任何 consumer 被迫改调用来适应这次 ownership move

因此，`Phase 22` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续深挖 navigation-specific action target clone duplication

本阶段没有顺手处理：

- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)

这条 navigation 专用 clone 语义。

这是下一阶段更合适的热点，而不是这阶段漏做。

#### 2. 它没有重开 shared queue / retry clone helper 的更大归属问题

虽然：

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

被明确保留在 [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)，但本阶段没有进一步回答：

- 它们将来是否还需要更窄的 shared home
- 哪些 consumer 真正构成下一轮 pressure

这也是刻意保留的下一阶段判断题，而不是遗漏。

#### 3. 它没有引入通用 clone abstraction

这轮所有改动都保持在 ListingKit feature-local 范围内，没有尝试为整个 repo 建一个统一 clone 层。

### Residual Responsibilities Still Present

`Phase 22` 收完之后，clone/helper 邻域里最显眼的 residual hotspot 已经不再是 broad helper cluster 本身，而是 navigation-specific action target clone duplication：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)

它当前仍然自己做一套：

- filters clone
- queue query clone
- retry request clone
- expected impact clone
- `NavigationTarget = nil` 的 navigation-specific shaping

而这些语义现在和新的：

- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

形成了明显的相邻重复。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是马上重开 shared queue / retry clone home，而是先聚焦：

#### 1. ListingKit navigation action-target clone shaping ownership

重点锚点：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

原因很直接：

- `Phase 22` 已经把 broad helper cluster 收成了 clearer split
- 现在最真实的下一波 pressure 是 navigation-specific clone semantics 与 local clone home 的重复邻接
- 这比直接重开 shared queue / retry clone 更像一个 bounded、可验证、不会扩散的大头问题

#### 2. 继续保持小切片

下一步更适合先围绕：

- action target clone reuse
- navigation-specific shaping
- `NavigationTarget = nil` 这种 navigation-only contract

分出一个更明确的 seam，而不是一次性重写整个 review navigation target file。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestResolveAssetGenerationActionTarget.*" -count=1
go test ./internal/listingkit -run "TestCloneAssetGeneration.*|TestCloneGeneration.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- outward clone behavior 保持稳定
- ownership split 保持稳定
- boundary guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
