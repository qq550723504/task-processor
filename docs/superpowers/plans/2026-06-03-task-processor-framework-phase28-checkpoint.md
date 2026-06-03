## Task Processor Framework Phase 28 Checkpoint

### Status

`Phase 28` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff branch-result routing ownership` 这条切片
- 它没有回头重开 `Phase 27` 已稳定的 handoff / branch-local invocation split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic routing framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase28-action-execute-handoff-branch-result-routing.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase28-action-execute-handoff-branch-result-routing.md:1)

### What Landed

#### 1. Handoff routing behavior 继续被保持并验证

这轮没有新增专门的行为测试文件，而是沿用了 `Phase 27` 已建立的 focused 行为测试面，并通过 fresh 验证确认 branch-result routing 仍然保持既有 outward 行为：

- `retryable` 分支继续返回 `retryPage`
- `default` 分支继续返回 `queuePage`
- 两个分支继续通过 `Phase 26` 的 adaptation home 产出 `persistenceQueue`
- `Phase 27` 的 branch-local invocation 行为保持不变

这组行为在以下 focused 测试命令中继续被覆盖并通过：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
```

#### 2. Branch-result routing 已从 handoff seam 里分出来

新增本地 result-routing seam：

- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
- [task_generation_action_execute_request_handoff_queue_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)

对应提交：

- `5c04bd17` `refactor: split listingkit action execute handoff routing`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - 负责 `retryable / default` branch selection
  - 负责路由到 branch-local invocation seam
  - 负责路由到 branch-result routing seam

- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
  - 负责 retry result 路由
  - 通过 `Phase 26` adaptation home 返回最终 handoff object

- [task_generation_action_execute_request_handoff_queue_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
  - 负责 queue result 路由
  - 通过 `Phase 26` adaptation home 返回最终 handoff object

也就是说，handoff seam 不再直接内联 branch-result routing。

#### 3. `Phase 26` adaptation home 被完整保留

这一轮没有把 routing 和 adaptation 混回去。

当前结构保持为：

- branch-local invocation
- branch-local result routing
- shared adaptation home

这意味着 `Phase 26` 已明确下来的 `fromRetryPage(...)` / `fromQueuePage(...)` 语义继续稳定地留在：

- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

#### 4. Handoff routing ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
- [phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)
- [phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)

对应提交：

- `ca2a451f` `test: lock listingkit action execute handoff routing boundaries`

当前 guardrail 锁住了 4 件事：

- handoff seam 顶层继续只拥有 branch selection 和 seam routing
- retry / queue branch-local invocation 继续留在各自本地 invocation home
- retry / queue result routing 继续留在各自本地 result home
- `Phase 26` adaptation 继续留在 shared adaptation home

### Acceptance Check

`Phase 28` 需要证明的核心点有四个：

1. handoff routing outward behavior 保持稳定
2. branch-result routing 不再直接内联在 handoff seam 里
3. `Phase 26` adaptation home 没有被重新放大
4. handoff routing ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1) / [task_generation_action_execute_request_handoff_queue_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1) 已成为 branch-result routing owner
- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1) 已缩成更明确的 orchestration shell
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 28` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 handoff seam 顶层的 interaction-mode branch selection ownership

当前：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

仍然同时承载：

- `retryable / default` mode 选择
- retry / queue local seam 的 routing

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 handoff seam 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 28` 收完之后，最显眼的 residual hotspot 已经从 handoff seam 的 branch-result routing，转移到 handoff seam 自己顶层的 interaction-mode branch selection：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

当前这条 seam 仍然同时持有：

- `retryable / default` branch selection
- retry / queue local seam routing

这说明下一块更真实的 ownership 压力，已经不再是 branch-result routing，而是 handoff seam 是否还该进一步拆成更明确的 interaction-mode routing home。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff interaction-mode routing ownership

重点锚点：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

原因很直接：

- `Phase 28` 已经把 branch-result routing 从 handoff seam 里收出去
- 当前 handoff seam 里剩下最明显的混合职责，就是 mode 选择和 local seam routing 的并置
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 handoff seam 内部的小步收口

下一步更适合只围绕：

- interaction-mode routing
- outward page/result behavior stability
- local seam 调度位置

下刀，而不是一次性把 action execute 邻域扩成更大的抽象工程。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- handoff routing behavior 保持稳定
- handoff seam 与 branch-result routing seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
