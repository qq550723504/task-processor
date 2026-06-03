## Task Processor Framework Phase 26 Checkpoint

### Status

`Phase 26` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute handoff result-adaptation ownership` 这条切片
- 它没有回头重开 `Phase 25` 已稳定的 execute top-level / handoff split
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic result-adaptation framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase26-action-execute-handoff-result-adaptation.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase26-action-execute-handoff-result-adaptation.md:1)

### What Landed

#### 1. Handoff result-adaptation behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecuteRequestHandoffPhase.run(...)` 的行为覆盖：

- `retryable` 分支返回 `retryPage`
- `persistenceQueue` 继续来自 `generationWorkQueueFromRetryPage(retryPage)`
- `default` 分支返回 `queuePage`
- `persistenceQueue` 继续来自 `generationWorkQueueFromPage(queuePage)`
- 两个分支都锁住了 `persistenceQueue` 与各自 page-derived queue 的对齐语义
- queue 分支下，`persistenceQueue.Items` 不复用 `queuePage.Items` 底层存储

对应提交：

- `ea25b679` `test: lock listingkit action execute handoff adaptation behavior`

这一步先把 handoff seam 当前最关键的 outward adaptation contract 钉住了。

#### 2. Result-adaptation 已从 request-handoff seam 里分出来

新增本地 seam：

- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)

对应提交：

- `2ee7c95c` `refactor: split listingkit action execute handoff result adaptation`

当前 split 已经很清楚：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
  - 负责 `retryable / default` branching
  - 负责 `RetryTaskGenerationTasks(...)` / `GetTaskGenerationQueue(...)`
  - 继续通过 shared clone helpers 完成 request handoff

- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1)
  - 负责 `retryPage -> persistenceQueue`
  - 负责 `queuePage -> persistenceQueue`

也就是说，handoff seam 不再直接内联 page/result adaptation。

#### 3. `Task 3` 在这一轮是合理的 no-op

这轮没有再额外引入第二次生产代码搬运。

原因是 `Task 2` 落完之后，ownership 已经足够清楚：

- invocation 留在 handoff seam
- adaptation 留在新的 adaptation seam
- execute 顶层 orchestration 不需要再改

因此 `Task 3` 在这轮对生产代码来说是一个合理的 no-op，而不是遗漏。

#### 4. Handoff ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1)
- [phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)

对应提交：

- `2ee7c95c` `refactor: split listingkit action execute handoff result adaptation`

当前 guardrail 锁住了 3 件事：

- handoff seam 顶层继续拥有 branch-local invocation 和 shared clone helper handoff
- result adaptation 继续留在单独的本地 adaptation home
- shared clone helpers 继续留在 [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1) 这个 shared home，不回流进 adaptation 文件

### Acceptance Check

`Phase 26` 需要证明的核心点有四个：

1. handoff result-adaptation 行为先被测试锁住
2. page/result adaptation 不再直接内联在 handoff seam 里
3. execute top-level orchestration 没有被重新放大
4. handoff ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward adaptation behavior
- [task_generation_action_execute_request_handoff_result_adaptation.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff_result_adaptation.go:1) 已成为 adaptation owner
- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1) 已缩成 branch invocation + shared clone handoff shell
- [phase26_action_execute_handoff_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase26_action_execute_handoff_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 26` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 handoff seam 内部的 branch invocation ownership

当前：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

仍然同时承载：

- `retryable / default` branching
- service invocation
- shared clone helper handoff

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 execute / refresh / projection 的新一轮清理

这一轮严格停在 handoff seam 本地，没有去重开：

- execute top-level shell
- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 26` 收完之后，最显眼的 residual hotspot 已经从 handoff seam 的 result adaptation，转移到 handoff seam 自己内部的 branch invocation ownership：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

当前这条 seam 仍然同时持有：

- `retryable / default` branching
- downstream service invocation
- shared clone helper handoff

这说明下一块更真实的 ownership 压力，已经不再是 result adaptation，而是 handoff seam 是否还该进一步拆成更明确的 branch-local invocation home。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared helper home，而是先聚焦：

#### 1. ListingKit action execute handoff branch-invocation ownership

重点锚点：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

原因很直接：

- `Phase 26` 已经把 result adaptation 从 handoff seam 里收出去
- 当前 handoff seam 里剩下最明显的混合职责，就是 branch selection、service invocation、shared clone handoff 的并置
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 handoff seam 内部的小步收口

下一步更适合只围绕：

- branch invocation
- shared clone handoff
- outward page/result behavior stability

下刀，而不是一次性把 action execute 邻域扩成更大的抽象工程。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecuteRequestHandoff.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- handoff result-adaptation behavior 保持稳定
- handoff seam 与 adaptation seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
