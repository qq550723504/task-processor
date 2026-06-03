## Task Processor Framework Phase 29 Checkpoint

### Status

`Phase 29` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff interaction-mode routing ownership` 这条切片
- 它没有回头重开 `Phase 28` 已稳定的 handoff / branch-result routing split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic mode-routing framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase29-action-execute-handoff-interaction-mode-routing.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase29-action-execute-handoff-interaction-mode-routing.md:1)

### What Landed

#### 1. Handoff mode behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecuteRequestHandoffPhase.run(...)` 的行为覆盖：

- `retryable` mode 继续通过 retry 本地 seams 路由
- default mode 继续通过 queue 本地 seams 路由
- outward `retryPage / queuePage` surfacing 保持不变
- `Phase 27` 的 invocation 行为保持不变
- `Phase 28` 的 result-routing 行为保持不变

对应提交：

- `067832af` `test: lock listingkit action execute handoff mode behavior`

这一步先把 handoff seam 当前最关键的 outward mode-routing contract 钉住了。

#### 2. Interaction-mode routing 已从 handoff seam 里分出来

新增本地 mode-routing seam：

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)

对应提交：

- `fc067595` `refactor: split listingkit action execute handoff mode routing`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - 现在只做一件事：把 request handoff 顶层入口委托给 mode-routing seam

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)
  - 负责 `retryable / default` mode 选择
  - 负责路由到 `Phase 27` 的 invocation seams
  - 负责路由到 `Phase 28` 的 result-routing seams

也就是说，handoff 顶层已经不再直接内联任何 mode-specific 分支逻辑。

#### 3. Phase 27 / 28 的本地 seams 被完整保留

这一轮没有把前两轮刚建立的 ownership 又混回去。

当前结构保持为：

- top-level handoff entry
- local mode-routing seam
- local invocation seams
- local result-routing seams
- shared adaptation home

这意味着 `Phase 27` 和 `Phase 28` 已明确下来的局部边界都继续稳定存在，没有为了“再抽一层”而回退。

#### 4. Handoff mode-routing ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase29_action_execute_handoff_mode_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go:1)
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
- [phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)

对应提交：

- `fc067595` `refactor: split listingkit action execute handoff mode routing`

当前 guardrail 锁住了 4 件事：

- top-level handoff seam 继续只拥有入口委托职责
- mode-routing 继续留在本地 routing home
- invocation 和 result-routing 继续留在各自本地 home
- outward action execute behavior 保持稳定

### Acceptance Check

`Phase 29` 需要证明的核心点有四个：

1. handoff interaction-mode behavior 先被测试锁住
2. local retry / queue seam dispatch 不再直接内联在 top-level handoff seam 里
3. `Phase 27/28` 的本地 seams 没有被重新放大
4. handoff mode-routing ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward mode-routing behavior
- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1) 已成为 interaction-mode routing owner
- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1) 已缩成更明确的 entry shell
- [phase29_action_execute_handoff_mode_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 29` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 unified handoff result shape / adaptation ownership

当前：

- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:9)

仍然共同承载：

- `retryPage / queuePage` 两种结果形状
- `persistenceQueue` 这一条 shared durability shape

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 handoff seam 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 29` 收完之后，最显眼的 residual hotspot 已经从 handoff seam 顶层的 interaction-mode routing，转移到 unified handoff result shape / adaptation 这一层：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:9)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

当前这块仍然同时知道：

- `retryPage`
- `queuePage`
- `persistenceQueue`

这说明下一块更真实的 ownership 压力，已经不再是“怎么做 mode routing”，而是“为什么一个 handoff result DTO 还同时承载两种页面形状和一条 shared persistence shape”。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff result-shape / adaptation ownership

重点锚点：

- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:9)

原因很直接：

- `Phase 29` 已经把 top-level handoff entry 收成了纯委托
- 当前 handoff 邻域里剩下最明显的混合职责，是 unified result DTO 与 adaptation 共同知道两种页面形状和一条 shared persistence queue
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 handoff result 层的小步收口

下一步更适合只围绕：

- handoff result DTO ownership
- adaptation seam 的 shape knowledge
- outward retry/queue result behavior stability

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

- handoff mode behavior 保持稳定
- top-level handoff seam 与 mode-routing seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
