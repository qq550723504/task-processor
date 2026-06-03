## Task Processor Framework Phase 32 Checkpoint

### Status

`Phase 32` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff result normalization ownership` 这条切片
- 它没有回头重开 `Phase 31` 已稳定的 mode-routing / pairing split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic normalization framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase32-action-execute-handoff-result-normalization.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase32-action-execute-handoff-result-normalization.md:1)

### What Landed

#### 1. Unified result normalization behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 unified handoff result normalization 的行为覆盖：

- retry path 继续归一到同样的 outward handoff 结构
- queue path 继续归一到同样的 outward handoff 结构
- retry/queue `persistenceQueue` derivation 保持不变
- `Phase 31` 的 routing / pairing 行为保持不变

对应提交：

- `c86cd83c` `test: lock listingkit action execute handoff result normalization behavior`

这一步先把 unified result layer 当前最关键的 outward normalization contract 钉住了。

#### 2. Result normalization 已从 broad result-shape owner 里分出来

新增本地 normalization seam：

- [task_generation_action_execute_request_handoff_result_normalization.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go:1)

对应提交：

- `68141dec` `refactor: split listingkit action execute handoff result normalization`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff_result_normalization.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go:1)
  - 负责 retry/queue normalization
  - 负责用 adaptation seam 派生 `persistenceQueue`
  - 产出统一的 normalization object

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
  - 现在只负责 outward handoff DTO shape
  - 不再直接拥有 retry/queue mirror normalization

也就是说，unified result-shape owner 不再隐式拥有比它需要更多的 retry/queue normalization 逻辑。

#### 3. PersistenceQueue mapping home 被完整保留

这一轮没有把 page-to-persistence 映射再混回 normalization seam。

当前结构保持为：

- mode-routing seam
- mode-pairing seam
- branch-local result seams
- result-normalization seam
- result-shape seam
- adaptation seam

这意味着 `Phase 30` 已明确下来的 adaptation owner 继续稳定存在，没有为了“再抽一层”而回退。

#### 4. Result-normalization ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase32_action_execute_handoff_result_normalization_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go:1)
- [phase30_action_execute_handoff_result_shape_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase30_action_execute_handoff_result_shape_boundary_test.go:1)
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)
- [phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)

对应提交：

- `9ac2e650` `test: lock listingkit action execute handoff result normalization boundaries`

当前 guardrail 锁住了 4 件事：

- unified result normalization ownership 继续留在本地 normalization home
- persistenceQueue mapping 继续留在 adaptation home
- outward handoff DTO shape 继续留在 result-shape home
- outward action execute behavior 保持稳定

### Acceptance Check

`Phase 32` 需要证明的核心点有四个：

1. unified result normalization behavior 先被测试锁住
2. retry/queue mirror normalization 不再直接内联在 broad result-shape owner 里
3. persistenceQueue mapping 没有被重新放大
4. handoff result-normalization ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward normalization behavior
- [task_generation_action_execute_request_handoff_result_normalization.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_normalization.go:1) 已成为 normalization owner
- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1) 已缩成更明确的 outward shape home
- [phase32_action_execute_handoff_result_normalization_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase32_action_execute_handoff_result_normalization_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 32` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 mode-pairing seam 的镜像双轨

当前：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

仍然同时承载：

- `runRetryable`
- `runQueue`

这两条几乎镜像的 branch orchestration 路径。

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 unified result layer 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 32` 收完之后，最显眼的 residual hotspot 已经从 unified result normalization，转移到 mode-pairing seam 自己的镜像双轨：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

当前这条 seam 仍然保留：

- `runRetryable`
- `runQueue`

两条几乎同形的 orchestrated pairing 路径。

这说明下一块更真实的 ownership 压力，已经不再是“谁拥有 unified result normalization”，而是“为什么一个 mode-pairing seam 还要沿 retry/queue 两条镜像路径同步演进”。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff mode-pairing normalization ownership

重点锚点：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

原因很直接：

- `Phase 32` 已经把 unified result normalization 从 result-shape owner 里收出去
- 当前 handoff 邻域里剩下最明显的混合职责，是 mode-pairing 仍然保留 retry/queue 两条镜像 orchestration
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 pairing layer 的小步收口

下一步更适合只围绕：

- `runRetryable / runQueue` 的镜像双轨
- invocation/result seam pairing orchestration
- outward retry/queue behavior stability

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

- handoff result-normalization behavior 保持稳定
- normalization seam 与 result-shape/adaptation seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
