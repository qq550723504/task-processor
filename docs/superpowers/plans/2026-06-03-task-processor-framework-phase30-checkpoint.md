## Task Processor Framework Phase 30 Checkpoint

### Status

`Phase 30` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff result-shape / adaptation ownership` 这条切片
- 它没有回头重开 `Phase 29` 已稳定的 handoff entry / mode-routing split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic result-shape framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase30-action-execute-handoff-result-shape-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase30-action-execute-handoff-result-shape-ownership.md:1)

### What Landed

#### 1. Handoff result-shape behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 unified handoff result shape 的行为覆盖：

- retry 路径继续返回 `retryPage`
- queue 路径继续返回 `queuePage`
- 两条路径继续返回派生后的 `persistenceQueue`
- outward handoff object shape 保持不变
- `Phase 27/28/29` 的 routing / seam ownership 都保持不变

对应提交：

- `e6df7421` `test: lock listingkit action execute handoff result shape behavior`

这一步先把 handoff result layer 当前最关键的 outward contract 钉住了。

#### 2. Unified result-shape ownership 已从 adaptation seam 里分出来

新增本地 result-shape seam：

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)

对应提交：

- `9b23d9e6` `refactor: clarify listingkit action execute handoff result shape ownership`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
  - 负责 outward handoff DTO shape
  - 负责 `retryPage / queuePage` 和 `persistenceQueue` 的 unified result construction
  - 通过 adaptation seam 获取派生后的 `persistenceQueue`

- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
  - 现在只负责 `page -> persistenceQueue`
  - 不再直接构造 unified handoff DTO

也就是说，adaptation seam 不再隐式拥有比它需要更多的 outward DTO shape。

#### 3. Branch-specific result seams 被完整保留

这一轮没有把 retry/queue result seams 又重新混成一个大块。

当前结构保持为：

- mode-routing seam
- retry/queue local result seams
- local result-shape seam
- adaptation seam

这意味着 `Phase 28` 和 `Phase 29` 已经明确下来的局部边界都继续稳定存在，没有为了“再抽一层”而回退。

#### 4. Handoff result-shape ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase30_action_execute_handoff_result_shape_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go:1)
- [phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
- [phase29_action_execute_handoff_mode_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go:1)
- [phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)

对应提交：

- `1d4b1dcb` `test: lock listingkit action execute handoff result shape boundaries`

当前 guardrail 锁住了 4 件事：

- unified handoff result-shape ownership 继续留在本地 result-shape home
- adaptation 继续只拥有 `page -> persistenceQueue` mapping
- branch-specific result seams 继续留在本地 branch result home
- outward action execute behavior 保持稳定

### Acceptance Check

`Phase 30` 需要证明的核心点有四个：

1. unified handoff result-shape behavior 先被测试锁住
2. adaptation 不再直接拥有更宽的 outward DTO shape
3. branch-specific result seams 没有被重新放大
4. handoff result-shape ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward result-shape behavior
- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1) 已成为 unified result-shape owner
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1) 已缩成纯 persistenceQueue mapping
- [phase30_action_execute_handoff_result_shape_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 30` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 mode-routing seam 自己的 pairing / routing concentration

当前：

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)

仍然同时承载：

- interaction mode
- branch executor selection
- branch-to-result seam pairing

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 handoff result layer 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 30` 收完之后，最显眼的 residual hotspot 已经从 unified handoff result-shape / adaptation，转移到 mode-routing seam 本身的 pairing concentration：

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)

当前这条 seam 仍然同时知道：

- interaction mode
- branch invocation seam
- branch result seam

这说明下一块更真实的 ownership 压力，已经不再是“谁拥有 unified result shape”，而是“为什么一个 mode-routing seam 还同时知道 branch executor selection 和 branch-to-result seam pairing”。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff mode-routing pairing ownership

重点锚点：

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)

原因很直接：

- `Phase 30` 已经把 handoff result-shape / adaptation 收干净
- 当前 handoff 邻域里剩下最明显的混合职责，就是一个 seam 同时知道 mode、branch invocation 和 branch result pairing
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 mode-routing 层的小步收口

下一步更适合只围绕：

- mode-routing pairing
- branch executor / result seam 配对关系
- outward retry/queue behavior stability

下刀，而不是一次性把 action execute 邻域扩成更大的抽象工程。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationRetryGenerationTasksRequest" -count=1
go test ./internal/listingkit -run "TestTaskGenerationAction.*Boundary|TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- handoff result-shape behavior 保持稳定
- result-shape seam 与 adaptation seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
