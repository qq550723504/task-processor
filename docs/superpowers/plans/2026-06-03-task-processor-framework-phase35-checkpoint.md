## Task Processor Framework Phase 35 Checkpoint

### Status

`Phase 35` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff branch request shaping ownership` 这条切片
- 它没有回头重开 `Phase 32` / `Phase 33` / `Phase 34` 已稳定的 result-side split
- 它没有移动 shared `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)` 的定义位置
- 它没有扩大成 generic request-routing framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase35-action-execute-handoff-branch-request-shaping.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase35-action-execute-handoff-branch-request-shaping.md:1)

### What Landed

#### 1. Branch request shaping behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecuteRequestHandoffRetryPhase` 和 `taskGenerationActionExecuteRequestHandoffQueuePhase` 的行为覆盖：

- retry branch 继续在调用前 clone `RetryRequest`
- queue branch 继续在调用前 clone `QueueQuery`
- downstream mutation 不会污染原始 target
- phase 对外 page 行为保持不变

对应提交：

- `5d93c57a` `test: lock listingkit action execute handoff branch request shaping behavior`

#### 2. Branch-local request shaping 已从 invocation seam 里分出来

新增本地 request-shaping seams：

- [task_generation_action_execute_request_handoff_retry_request.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_request.go:1)
- [task_generation_action_execute_request_handoff_queue_request.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_request.go:1)

对应提交：

- `2e1eea1f` `refactor: split listingkit action execute handoff branch request shaping`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff_retry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
  - 只保留 retry service invocation
  - 从 local retry request seam 获取已整形请求

- [task_generation_action_execute_request_handoff_queue.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)
  - 只保留 queue service invocation
  - 从 local queue request seam 获取已整形请求

- [task_generation_action_execute_request_handoff_retry_request.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_request.go:1)
  - 只负责 `cloneRetryGenerationTasksRequest(target.RetryRequest)`

- [task_generation_action_execute_request_handoff_queue_request.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_request.go:1)
  - 只负责 `cloneGenerationQueueQuery(target.QueueQuery)`

也就是说，retry/queue invocation owners 不再各自内联 request clone handoff。

#### 3. Shared clone helper home 被完整保留

这一轮没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

这让切片保持足够窄，也避免把当前 phase 扩成 cross-consumer helper relocation。

#### 4. Branch request shaping guardrail 已补齐

新增 / 对齐的边界测试：

- [phase35_action_execute_handoff_branch_request_shaping_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase35_action_execute_handoff_branch_request_shaping_boundary_test.go:1)
- [phase27_action_execute_handoff_branch_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase27_action_execute_handoff_branch_boundary_test.go:1)

对应提交：

- `a2e5cca6` `test: lock listingkit action execute handoff branch request shaping boundaries`

当前 guardrail 锁住了 4 件事：

- retry/queue invocation owners 继续只拥有 service invocation
- request shaping 继续留在各自 branch-local request home
- shared clone helper 定义继续留在 service helper home
- outward action execute behavior 保持稳定

### Acceptance Check

`Phase 35` 需要证明的核心点有四个：

1. branch request shaping behavior 先被测试锁住
2. retry/queue invocation seams 不再各自内联 clone handoff
3. shared clone helper 定义位置没有被误扩大
4. branch-request-shaping guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 35` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有移动 shared clone helper 的定义位置

当前：

- [service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1)

仍然同时持有：

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 2. 它没有重开 navigation / action-target clone consumers

这些 shared clone helpers 仍然被多处 navigation / action-target 相关逻辑复用。本阶段没有扩大到这些 consumer 的 ownership 调整。

### Residual Responsibilities Still Present

`Phase 35` 收完之后，最显眼的 residual hotspot 已经从 branch-local request shaping，转移到 shared clone helper home 本身：

- [service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1)

当前它仍然承载两类跨 consumer 的 clone helper，而这些 helper 已经被：

- action execute handoff request seams
- action target clone
- review navigation / navigation dispatch

等多条路径共同依赖。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit shared queue/retry clone helper ownership

重点锚点：

- [service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1)

原因很直接：

- `Phase 35` 已经把 action execute handoff 本地 request shaping 收干净
- 当前 handoff 邻域里剩下最明显的 ownership 压力，是 shared clone helper 还挂在 broad service helper home 上
- 这比回头再抠 request/result side 更像下一块真实的 multi-consumer hotspot

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- branch request shaping behavior 保持稳定
- request-shaping seams 已按预期落地
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
