## Task Processor Framework Phase 36 Checkpoint

### Status

`Phase 36` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit shared queue/retry clone helper ownership` 这条切片
- 它没有回头重开 `Phase 32` / `Phase 33` / `Phase 34` / `Phase 35` 已稳定的 handoff local seams
- 它没有扩大成 generic cloning framework
- 它没有顺手移动 non-clone helper

对应计划文档：

- [2026-06-03-task-processor-framework-phase36-shared-clone-helper-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase36-shared-clone-helper-ownership.md:1)

### What Landed

#### 1. Shared clone helper outward behavior 保持稳定

这一轮没有新增 behavior fixture，因为现有 clone helper 测试已经直接覆盖：

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

并且本轮验证也重新证明了这些 outward clone semantics 没变。

#### 2. Shared clone helper home 已从 broad service helper 文件中分离出来

新增明确的 shared clone home：

- [task_generation_shared_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_shared_clone.go:1)

对应提交：

- `51629f53` `refactor: clarify listingkit shared clone helper ownership`

当前 split 已经很清楚：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
  - 只保留 `ExecuteTaskGenerationAction(...)`
  - 只保留 `resolveLayerTemporalPlatform(...)`
  - 不再顺带持有 shared clone helper

- [task_generation_shared_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_shared_clone.go:1)
  - 只保留 `cloneGenerationQueueQuery(...)`
  - 只保留 `cloneRetryGenerationTasksRequest(...)`

这让 shared clone helper 不再显得像“顺手遗留在 service 文件里的工具”，而是一个明确的 feature-local shared seam。

#### 3. Direct consumers 继续通过 shared clone seam，而不是重定义 clone 逻辑

当前直接 consumer 继续保持不变：

- [task_generation_action_target_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
- [generation_review_navigation_target.go](/D:/code-task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [task_generation_action_execute_request_handoff_retry_request.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_request.go:1)
- [task_generation_action_execute_request_handoff_queue_request.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_request.go:1)

它们都还是调用 shared seam，没有把 clone logic 又各自长回本地 home。

#### 4. Shared clone helper guardrail 已补齐

新增 / 对齐的边界测试：

- [phase36_shared_clone_helper_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase36_shared_clone_helper_boundary_test.go:1)
- [phase21_action_target_resolution_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase21_action_target_resolution_boundary_test.go:1)
- [phase22_action_target_clone_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase22_action_target_clone_boundary_test.go:1)
- [phase24_review_navigation_queue_clone_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go:1)
- [phase25_action_execute_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)
- [phase27_action_execute_handoff_branch_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go:1)

对应提交：

- `120ce2e3` `test: lock listingkit shared clone helper boundaries`

当前 guardrail 锁住了 4 件事：

- shared clone helper 继续留在 final shared home
- direct consumers 继续调用 shared seam
- direct consumers 不重新定义 clone helper
- outward behavior 保持稳定

### Acceptance Check

`Phase 36` 需要证明的核心点有四个：

1. shared clone helper outward behavior 保持稳定
2. shared clone helper home 不再挂在 broad service helper 文件里
3. direct consumers 没有各自长回 clone implementation
4. shared clone helper guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 36` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续细拆 clone consumer 自身的 shape ownership

当前：

- [task_generation_action_target_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
- [generation_review_navigation_target.go](/D:/code-task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [service_generation_navigation_dispatch_helpers.go](/D:/code-task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)

仍然分别持有自己的 clone-shaping 语义。

本阶段没有去决定这些 consumer 之间还能不能再收一层，因为那已经超出“shared clone helper home ownership”的范围。

#### 2. 它没有扩大成 navigation / action target clone redesign

这一步只收 shared helper home，没有去重开 navigation dispatch 或 action target clone 的 broader shape cleanup。

### Residual Responsibilities Still Present

`Phase 36` 收完之后，最显眼的 residual hotspot 已经从 shared helper home，转移到 shared clone helper 的主要 consumer 之一：

- [task_generation_action_target_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

当前这条 seam 仍然同时拥有：

- action target top-level clone shape
- queue/retry clone helper consumption
- navigation target clone chaining
- impact / filters clone chaining

这说明下一块更真实的 ownership 压力，已经不再是 helper 放哪，而是 “谁拥有 action-target clone aggregate shape”。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit action target clone aggregate ownership

重点锚点：

- [task_generation_action_target_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

原因很直接：

- `Phase 36` 已经把 shared helper home 收干净
- 当前 clone 邻域里剩下最明显的 ownership hotspot，是 action-target clone 这一层仍然聚合了多类 nested clone shaping
- 这比回头再抠 shared helper home，更像下一块 bounded、低风险、收益清晰的小切片

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationQueueQuery.*|TestCloneGenerationRetryGenerationTasksRequest.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- shared clone helper outward behavior 保持稳定
- shared clone helper home 的迁移没有破坏 direct consumers
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
