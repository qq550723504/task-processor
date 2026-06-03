## Task Processor Framework Phase 34 Checkpoint

### Status

`Phase 34` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff branch-specific result routing ownership` 这条切片
- 它没有回头重开 `Phase 31` / `Phase 32` / `Phase 33` 已稳定的 mode-pairing / normalization / result-shape split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的位置重构
- 它没有引入 generic branch-result-routing framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase34-action-execute-handoff-branch-result-routing.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase34-action-execute-handoff-branch-result-routing.md:1)

### What Landed

#### 1. Branch-specific result routing behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecuteRequestHandoffResultDispatchPhase` 的行为覆盖：

- retry result seam 继续把 `retryPage` 路由成同样的 outward handoff
- queue result seam 继续把 `queuePage` 路由成同样的 outward handoff
- `Phase 32` 的 unified result normalization / result shape 行为保持不变
- outward `retryPage / queuePage / persistenceQueue` 行为保持不变

对应提交：

- `ea61f20f` `test: lock listingkit action execute handoff branch result routing behavior`

#### 2. Branch-specific result routing 已收进更窄的 local dispatch home

新增本地 dispatch seam：

- [task_generation_action_execute_request_handoff_result_dispatch.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_dispatch.go:1)

对应提交：

- `36795a27` `refactor: split listingkit action execute handoff branch result routing`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - 只保留 retry branch page 入口
  - 委托到 local result-dispatch seam

- [task_generation_action_execute_request_handoff_queue_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - 只保留 queue branch page 入口
  - 委托到 local result-dispatch seam

- [task_generation_action_execute_request_handoff_result_dispatch.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_dispatch.go:1)
  - 负责 branch-specific page 输入
  - 负责接 unified result-normalization seam
  - 负责接 unified result-shape seam

也就是说，retry/queue result owners 不再各自内联同一段 `normalization -> shape` 串接。

#### 3. Unified normalization / result-shape layers 被完整保留

这一轮没有把已经收干净的 unified layers 又重新放大。

当前结构保持为：

- mode-routing seam
- mode-pairing seam
- mode-pairing-normalization seam
- branch-local invocation seams
- branch-local result seams
- local result-dispatch seam
- result-normalization seam
- result-shape seam
- adaptation seam

这意味着 `Phase 32` / `Phase 33` 已经明确下来的 unified ownership 仍然稳定存在。

#### 4. Branch-result-routing guardrail 已补齐

新增 / 对齐的边界测试：

- [phase34_action_execute_handoff_branch_result_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase34_action_execute_handoff_branch_result_routing_boundary_test.go:1)
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
- [phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)

对应提交：

- `8a745d44` `test: lock listingkit action execute handoff branch result routing boundaries`

当前 guardrail 锁住了 4 件事：

- retry/queue result owners 继续只拥有 branch-specific page 输入
- result-dispatch seam 继续拥有 `normalization -> shape` pairing
- unified normalization / result-shape 继续留在各自既有 home
- outward action execute behavior 保持稳定

### Acceptance Check

`Phase 34` 需要证明的核心点有四个：

1. branch-specific result routing behavior 先被测试锁住
2. retry/queue mirrored shells 不再各自内联 unified dispatch
3. unified normalization / result-shape layers 没有被重新放大
4. branch-result-routing guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 34` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 branch invocation request shaping

当前：

- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
- [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)

仍然各自同时知道：

- branch service call
- request clone helper

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 2. 它没有移动 shared clone helper home

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 handoff branch result routing 本地，没有去重开更大的 action lifecycle 清理。

### Residual Responsibilities Still Present

`Phase 34` 收完之后，最显眼的 residual hotspot 已经从 branch-specific result routing，转移到 branch invocation 自己的 request shaping：

- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
- [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)

当前这两条 seam 仍然同时知道：

- branch service invocation
- clone helper handoff

这说明下一块更真实的 ownership 压力，已经不再是 result routing，而是 branch request shaping。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit action execute handoff branch request shaping ownership

重点锚点：

- [task_generation_action_execute_request_handoff_retry.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry.go:1)
- [task_generation_action_execute_request_handoff_queue.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue.go:1)

原因很直接：

- `Phase 34` 已经把 branch-specific result routing 收进 local dispatch home
- 当前 handoff 邻域里剩下最明显的 mixed responsibility，是 branch invocation seam 仍然同时知道请求 clone 和 service call
- 这比回头再抠 result-normalization / result-shape 层，更像下一块 bounded、低风险、收益清晰的小切片

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- branch-specific result routing behavior 保持稳定
- local result-dispatch seam 已按预期落地
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
