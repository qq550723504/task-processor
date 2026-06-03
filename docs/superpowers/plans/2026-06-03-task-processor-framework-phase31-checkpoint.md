## Task Processor Framework Phase 31 Checkpoint

### Status

`Phase 31` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff mode-routing pairing ownership` 这条切片
- 它没有回头重开 `Phase 30` 已稳定的 result-shape / adaptation split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic pairing framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase31-action-execute-handoff-mode-routing-pairing.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase31-action-execute-handoff-mode-routing-pairing.md:1)

### What Landed

#### 1. Mode-routing pairing behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecuteRequestHandoffModeRoutingPhase.run(...)` 的行为覆盖：

- `retryable` mode 继续配对 retry invocation seam 和 retry result seam
- default mode 继续配对 queue invocation seam 和 queue result seam
- outward `retryPage / queuePage / persistenceQueue` 行为保持不变
- `Phase 30` 的 result-shape / adaptation 行为保持不变

对应提交：

- `9a3d8eed` `test: lock listingkit action execute handoff mode pairing behavior`

这一步先把 mode-routing seam 当前最关键的 outward pairing contract 钉住了。

#### 2. Mode-routing pairing 已从 broad mode-routing body 里分出来

新增本地 pairing seam：

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)

对应提交：

- `247adf5d` `refactor: split listingkit action execute handoff mode pairing`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1)
  - 负责 `retryable / default` mode 选择
  - 负责委托给 local pairing seam

- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1)
  - 负责 retry branch invocation/result pairing
  - 负责 queue branch invocation/result pairing

也就是说，mode-routing seam 不再直接内联 branch invocation/result 的配对逻辑。

#### 3. Phase 27 / 28 / 30 的本地 seams 被完整保留

这一轮没有把前面几轮已经收干净的本地 owner 又揉回去。

当前结构保持为：

- top-level handoff entry seam
- mode-routing seam
- mode-pairing seam
- branch-local invocation seams
- branch-local result seams
- result-shape seam
- adaptation seam

这意味着 `Phase 27`、`Phase 28` 和 `Phase 30` 已明确下来的局部边界都继续稳定存在，没有为了“再抽一层”而回退。

#### 4. Mode-pairing ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase31_action_execute_handoff_mode_pairing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go:1)
- [phase29_action_execute_handoff_mode_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase29_action_execute_handoff_mode_boundary_test.go:1)
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)

对应提交：

- `8a1569f0` `test: lock listingkit action execute handoff mode pairing boundaries`

当前 guardrail 锁住了 4 件事：

- mode-routing seam 继续只拥有 mode selection 和 pairing seam dispatch
- mode-pairing 继续留在本地 pairing home
- branch-local invocation 和 result seams 继续留在各自本地 home
- outward action execute behavior 保持稳定

### Acceptance Check

`Phase 31` 需要证明的核心点有四个：

1. mode-routing pairing behavior 先被测试锁住
2. branch invocation/result pairing 不再直接内联在 mode-routing seam 里
3. `Phase 27/28/30` 的本地 seams 没有被重新放大
4. handoff mode-pairing ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward pairing behavior
- [task_generation_action_execute_request_handoff_mode_pairing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_pairing.go:1) 已成为 pairing owner
- [task_generation_action_execute_request_handoff_mode_routing.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_mode_routing.go:1) 已缩成更明确的 mode-selection shell
- [phase31_action_execute_handoff_mode_pairing_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase31_action_execute_handoff_mode_pairing_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 31` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 unified handoff result-shape 的镜像双轨

当前：

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

仍然同时承载：

- retry/queue 两条镜像的 result normalization
- retry/queue 两条镜像的 page-to-persistence 映射

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 mode-routing layer 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 31` 收完之后，最显眼的 residual hotspot 已经从 mode-routing pairing，转移到 unified handoff result normalization 这一层：

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

当前这块仍然保留：

- `fromRetryPage / fromQueuePage`
- `persistenceQueueFromRetryPage / persistenceQueueFromQueuePage`

这说明下一块更真实的 ownership 压力，已经不再是“谁做 mode pairing”，而是“为什么统一 handoff result owner 仍按 retry/queue 双轨镜像演进”。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff result normalization ownership

重点锚点：

- [task_generation_action_execute_request_handoff_result_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_shape.go:1)
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

原因很直接：

- `Phase 31` 已经把 mode-routing pairing 收干净
- 当前 handoff 邻域里剩下最明显的混合职责，是 unified result owner 仍然保留 retry/queue 成对镜像的 normalization / persistence 映射知识
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 result layer 的小步收口

下一步更适合只围绕：

- unified handoff result normalization
- retry/queue 双轨镜像职责
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

- handoff mode-pairing behavior 保持稳定
- mode-routing seam 与 pairing seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
