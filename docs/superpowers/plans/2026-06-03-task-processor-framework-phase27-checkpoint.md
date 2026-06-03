## Task Processor Framework Phase 27 Checkpoint

### Status

`Phase 27` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff branch-invocation ownership` 这条切片
- 它没有回头重开 `Phase 26` 已稳定的 handoff / adaptation split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic invocation framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase27-action-execute-handoff-branch-invocation.md](/D:/code-task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase27-action-execute-handoff-branch-invocation.md:1)

### What Landed

#### 1. Handoff branch behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecuteRequestHandoffPhase.run(...)` 的行为覆盖：

- `retryable` 分支会先 clone `RetryRequest`
- `default` 分支会先 clone `QueueQuery`
- downstream 对收到请求的变异不会污染原始 target
- `retryable` 分支继续返回 `retryPage`
- `default` 分支继续返回 `queuePage`
- `Phase 26` 已建立的 `persistenceQueue` outward adaptation 行为保持不变

对应提交：

- `b2609a6f` `test: lock listingkit action execute handoff branch behavior`

这一步先把 handoff seam 当前最关键的 outward branch contract 钉住了。

#### 2. Branch-local invocation 已从 handoff seam 里分出来

新增本地 branch seam：

- [task_generation_action_execute_request_handoff_retry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
- [task_generation_action_execute_request_handoff_queue.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)

对应提交：

- `d408ebaa` `refactor: split listingkit action execute handoff branch invocation`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - 负责 `retryable / default` branch selection
  - 负责路由到 branch-local invocation seam
  - 负责继续调用 `Phase 26` 的 result-adaptation seam

- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
  - 负责 retry branch 的 clone handoff
  - 负责 `RetryTaskGenerationTasks(...)`

- [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)
  - 负责 queue branch 的 clone handoff
  - 负责 `GetTaskGenerationQueue(...)`

也就是说，handoff seam 不再直接内联 branch-local downstream invocation。

#### 3. 顶层 handoff seam 已压成更明确的 orchestration shell

这一轮没有再引入额外的生产层次，而是直接把现有 `run(...)` 收窄成：

- mode switch
- branch-local phase 调用
- result-adaptation seam 路由

这样保住了 slice 足够窄，同时让 `Phase 26` 的 adaptation home 不被回流。

#### 4. Handoff branch ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase27_action_execute_handoff_branch_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go:1)
- [phase25_action_execute_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)

对应提交：

- `d408ebaa` `refactor: split listingkit action execute handoff branch invocation`

当前 guardrail 锁住了 4 件事：

- handoff seam 顶层继续只拥有 branch selection 和 seam routing
- retry / queue branch-local invocation 继续留在各自本地 branch home
- result adaptation 继续留在 `Phase 26` 的 adaptation home
- shared clone helpers 继续留在 [service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1) 这个 shared home

### Acceptance Check

`Phase 27` 需要证明的核心点有四个：

1. handoff branch behavior 先被测试锁住
2. branch-local invocation 不再直接内联在 handoff seam 里
3. `Phase 26` 的 adaptation home 没有被重新放大
4. handoff branch ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code-task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward branch behavior
- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1) / [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1) 已成为 branch-local invocation owner
- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1) 已缩成更明确的 orchestration shell
- [phase27_action_execute_handoff_branch_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 27` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 handoff seam 顶层的 branch result routing ownership

当前：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

仍然同时承载：

- `retryable / default` branch selection
- branch-local phase 调用
- `fromRetryPage / fromQueuePage` 结果路由

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 handoff seam 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 27` 收完之后，最显眼的 residual hotspot 已经从 handoff seam 的 branch invocation，转移到 handoff seam 自己内部的 branch result routing：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

当前这条 seam 仍然同时持有：

- `retryable / default` branch selection
- branch-local phase 调用
- `fromRetryPage / fromQueuePage` 结果路由

这说明下一块更真实的 ownership 压力，已经不再是 branch-local invocation，而是 handoff seam 是否还该进一步拆成更明确的 branch-result routing home。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff branch-result routing ownership

重点锚点：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

原因很直接：

- `Phase 27` 已经把 branch-local invocation 从 handoff seam 里收出去
- 当前 handoff seam 里剩下最明显的混合职责，就是 branch selection、branch-local phase 调用、result routing 的并置
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 handoff seam 内部的小步收口

下一步更适合只围绕：

- branch result routing
- `Phase 26` adaptation seam 的消费位置
- outward page/result behavior stability

下刀，而不是一次性把 action execute 邻域扩成更大的抽象工程。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- handoff branch behavior 保持稳定
- handoff seam 与 branch-local invocation seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
