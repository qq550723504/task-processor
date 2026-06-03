## Task Processor Framework Phase 25 Checkpoint

### Status

`Phase 25` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action execute request handoff ownership` 这条切片
- 它没有回头重开 `Phase 24` 已稳定的 review-navigation queue clone reuse
- 它没有把范围扩大成 shared `queue/retry` clone helper 的多 consumer 重构
- 它没有引入新的 generic request-handoff framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase25-action-execute-request-handoff.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase25-action-execute-request-handoff.md:1)

### What Landed

#### 1. Action execute request handoff behavior 已先被锁住

在 [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 里补齐了 `taskGenerationActionExecutePhase.run(...)` 的行为覆盖：

- `retryable` 分支会先 clone `RetryRequest`
- downstream 对收到的 `RetryRequest` 的变异不会污染原始 `target.RetryRequest`
- `default` 分支仍然通过 `QueueQuery` 路径执行，且不污染原始 `target.QueueQuery`
- `execution.retryPage / execution.queuePage` 的结果映射保持不变
- `persistenceSession` 继续维持 page-derived queue + original target query 的 handoff 语义
- retry 路径下 `persistenceSession.Queue` 不直接复用 `retryPage.ExecutedQueue` 指针

对应提交：

- `e334c72d` `test: lock listingkit action execute request handoff behavior`

这一步先把 execute-phase 当前最关键的 outward contract 钉住了。

#### 2. Execute-local request handoff seam 已抽出来

新增本地 seam：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

对应提交：

- `a7232ec6` `refactor: split listingkit action execute request handoff`

当前 split 已经很清楚：

- `taskGenerationActionExecuteRequestHandoffPhase.run(...)`
  - 负责 retry / queue 分支选择
  - 负责调用 `RetryTaskGenerationTasks(...)` / `GetTaskGenerationQueue(...)`
  - 继续通过 shared clone helpers 完成 request handoff
  - 把 page 结果适配为 execute-local 的 `persistenceQueue`

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1)
  - 现在只负责调用 handoff seam
  - 组装 `taskGenerationActionExecution`
  - 通过 `handoff.persistenceQueue + target.QueueQuery` 构造 `persistenceSession`

也就是说，execute phase 本体不再直接内联 request clone handoff 和 page/result adaptation。

#### 3. Persistence-session shaping 被有意保留在 execute phase 顶层

这轮没有强行再拆一个 `Task 3` 的生产 seam。

原因是 `Phase 25 Task 2` 落完之后，顶层 execute phase 里只剩：

- `buildGenerationReviewSession(baseResult, handoff.persistenceQueue, target.QueueQuery)`

这一行 persistence-session 组装。

这个残余已经足够小，不值得为了对称性硬拆出第二个 production seam。于是 `Task 3` 在这一轮是一个合理的 no-op，而不是遗漏。

#### 4. Execute-phase ownership guardrail 已补齐

新增 / 对齐的边界测试：

- [phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1)
- [phase10_task_generation_action_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase10_task_generation_action_boundary_test.go:1)
- [phase18_action_service_entry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase18_action_service_entry_boundary_test.go:1)

对应提交：

- `683eaf28` `test: lock listingkit action execute boundaries`

当前 guardrail 锁住了 3 件事：

- execute phase 顶层必须通过 `buildTaskGenerationActionExecuteRequestHandoffPhase(...).run(...)` 路由 request handoff
- request-handoff seam 自己拥有 branching 和 shared clone helper 的消费
- shared clone helpers 继续留在 [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1) 这个 shared home，不回流到 execute-local handoff 文件

同时，`Phase 10` / `Phase 18` 的旧边界也已经对齐到新的 ownership 现实，不再错误要求 execute phase 自己直接持有 `RetryTaskGenerationTasks / GetTaskGenerationQueue / switch target.InteractionMode`。

### Acceptance Check

`Phase 25` 需要证明的核心点有四个：

1. execute-phase request handoff 行为先被测试锁住
2. request clone handoff 和 page/result adaptation 不再直接内联在 execute body 里
3. persistence-session shaping 没有被误拆成无价值的小 seam
4. execute ownership guardrails 已把新 split 钉住

这四件事现在都成立。

更具体地说：

- [service_generation_retry_test.go](/D:/code/task-processor/internal/listingkit/service_generation_retry_test.go:1) 已锁住 outward behavior
- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1) 已成为 request handoff 的本地 owner
- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:1) 已缩成更明确的 orchestration shell
- [phase25_action_execute_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase25_action_execute_boundary_test.go:1) 已把这个 split 钉住

因此，`Phase 25` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有重开 shared `queue/retry` clone helper 的定义位置

本阶段没有去移动：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

这两条 helper 仍然继续留在 shared home。

#### 2. 它没有继续深挖 request-handoff seam 内部的 page/result adaptation

当前：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

仍然同时承载：

- retry / queue service invocation
- page -> `persistenceQueue` adaptation

这不是本阶段漏掉，而是下一阶段更合适的 residual hotspot。

#### 3. 它没有扩大成 refresh / projection / finalize 的新一轮清理

这一轮严格停在 execute phase 本地，没有去重开：

- refresh
- projection
- finalize

这样保持了 slice 足够窄。

### Residual Responsibilities Still Present

`Phase 25` 收完之后，最显眼的 residual hotspot 已经从 execute-phase 顶层 request handoff，转移到 request-handoff seam 自己内部的 branch result adaptation：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

当前这条本地 seam 仍然同时持有：

- retry / queue service invocation
- shared clone helper handoff
- `generationWorkQueueFromRetryPage(...)` / `generationWorkQueueFromPage(...)` 的 page-derived queue adaptation

这说明下一块更真实的 ownership 压力，已经不再是 execute body 本身，而是 handoff seam 里的 branch invocation 与 result adaptation 是否还该再分。

### What Should Move To The Next Phase

下一阶段最值得推进的，不是回头去动 shared clone helper home，而是先聚焦：

#### 1. ListingKit action execute handoff result-adaptation ownership

重点锚点：

- [task_generation_action_execute_request_handoff.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
- [generationWorkQueueFromRetryPage(...)](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
- [generationWorkQueueFromPage(...)](/D:/code/task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

原因很直接：

- `Phase 25` 已经把 request clone handoff 从 execute body 里收出去
- 当前 handoff seam 里剩下最明显的混合职责，就是 service invocation 与 page/result adaptation 的并置
- 这比直接重开 shared helper 定义位置，更像下一块 bounded、低风险、收益清晰的小切片

#### 2. 继续保持 handoff seam 内部的小步收口

下一步更适合只围绕：

- branch invocation
- result adaptation
- `persistenceQueue` handoff

下刀，而不是一次性把 action execute 邻域扩成更大的抽象工程。

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestCloneGenerationQueueQuery.*|TestCloneRetryGenerationTasksRequest.*" -count=1
go test ./internal/listingkit -run "TestTaskGenerationActionExecute.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- execute request handoff behavior 保持稳定
- execute-local handoff seam 的 split 保持稳定
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
