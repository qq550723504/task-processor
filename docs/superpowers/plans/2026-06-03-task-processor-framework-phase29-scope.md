## Task Processor Framework Phase 29 Scope Recommendation

### Candidate Directions

`Phase 28` 收口之后，ListingKit execute/action 邻域里至少还有两个可继续推进的方向：

#### 方向一：ListingKit action execute handoff interaction-mode routing ownership

当前最集中的 residual hotspot 在：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

也就是 handoff seam 仍然同时持有：

- `retryable / default` mode 选择
- retry / queue local seam routing

#### 方向二：继续深挖 shared `queue/retry` clone helper ownership

例如继续围绕：

- [cloneGenerationQueueQuery(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:23)

去讨论它们是否应该继续离开 shared helper home。

这两个方向都合理，但优先级并不相同。

### Recommendation

`Phase 29` 应先聚焦 **ListingKit action execute handoff interaction-mode routing ownership**。

也就是优先处理：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

而不是现在就把 shared `queue/retry` clone helper 的更宽归属问题重新全面讨论一遍。

### Why This Is The Right Next Slice

#### 1. `Phase 28` 已把 handoff seam 里的 branch-result routing 收干净

现在已经明确落地了：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)
- [task_generation_action_execute_request_handoff_retry_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_retry_result.go:1)
- [task_generation_action_execute_request_handoff_queue_result.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff_queue_result.go:1)
- [phase28_action_execute_handoff_routing_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase28_action_execute_handoff_routing_boundary_test.go:1)

这意味着：

- result-routing 责任已经从 handoff seam 中被剥离
- 再围着 routing 本身做对称性打磨，收益已经下降

#### 2. 下一块真实压力已经转移到 handoff seam 自己的 interaction-mode routing 并置

当前 handoff seam 里最明显的 mixed concern 是：

- `retryable / default` mode selection
- retry / queue local seam routing

这说明根因不再是“谁拥有 branch-result routing”，而是：

- handoff seam 自己还在同时持有 mode orchestration 与 local seam dispatch

这比直接重开 shared helper owner 更像下一块真正的 bounded ownership hotspot。

#### 3. 这条线比 shared helper relocation 更适合做小切片

shared `queue/retry` clone helper 仍然被很多路径使用：

- action execute handoff
- navigation dispatch
- temporal result
- descriptor / conditional shaping

如果现在直接重开它们的定义位置，会很容易变成一个多 consumer 的大问题。

相比之下，handoff seam 本地 interaction-mode routing：

- 影响范围更集中
- 行为语义更清晰
- 可以先在 handoff seam 局部收一刀，而不碰别的 consumer

#### 4. 这条线还能顺着 `Phase 27/28` 刚建立的 seam 继续推进

当前 execute 邻域已经形成了清晰的局部结构：

- top-level execute orchestration
- request handoff seam
- branch-local invocation seams
- branch-local result seams
- shared adaptation home

所以 handoff seam 自己的 interaction-mode routing 再切一小刀，会和前面几轮的 ownership 收口风格非常一致，不需要引入新的抽象风格。

### Why Not Reopen Shared Queue / Retry Clone Helpers First

不建议 `Phase 29` 继续优先追 shared `queue/retry` clone helper ownership，原因有三个：

#### 1. 共享性证据仍然很强

当前这些 helper 仍在多个路径里被直接使用，说明它们暂时仍然更像 shared utility，而不是已经明显该继续搬家的本地 seam。

#### 2. 现在去动 helper 定义位置，会把 slice 变大

一旦下刀，很容易把：

- action execute handoff
- navigation dispatch
- temporal result
- descriptor / conditional

这些路径都卷进来。

#### 3. handoff seam 自己的 interaction-mode routing 混合职责更像眼前的根因

既然 handoff seam 已经明确同时持有 mode selection 和 local seam dispatch，就没必要跳过这个更近、更具体的 residual hotspot，直接去碰更宽的 shared helper 问题。

### Proposed Phase Shape

一个合适的 `Phase 29` 形状应该是：

1. 保持 `Phase 28` 的 handoff / branch-result routing split 不动
2. 不重开 shared `queue/retry` helper 的 broader multi-consumer owner
3. 只围绕 handoff seam 的 interaction-mode routing 下刀
4. 先明确哪些 routing 可以封装成更明确的 local routing home
5. 把真正 top-level 的 handoff orchestration 留在 handoff home
6. 用 focused tests 和 ownership guardrail 锁住“harness 更清晰、outward behavior 不变”的方向

### Guardrails

`Phase 29` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 28` 已稳定的 handoff / branch-result routing split
2. 不要把范围扩成 shared `queue/retry` helper 的全面大改
3. 不要改变 action execute outward behavior
4. 不要引入新的 generic mode-routing framework
5. 优先把 handoff seam 自己的 local interaction-mode routing 收清楚

### Recommended Next Step

下一步建议直接写一个 `Phase 29` implementation plan，主题明确限定为：

**ListingKit action execute handoff interaction-mode routing ownership**

计划锚点应放在：

- [task_generation_action_execute_request_handoff.go](/D:/code-task-processor/internal/listingkit/task_generation_action_execute_request_handoff.go:1)

结论需要明确：

- `Phase 29` 应先聚焦 handoff seam 本地的 interaction-mode routing ownership
- 不建议现在就全面重开 shared `queue/retry` clone helper ownership
- 不建议把这一刀扩大成 generic routing abstraction
