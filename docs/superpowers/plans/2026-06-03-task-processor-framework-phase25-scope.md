## Task Processor Framework Phase 25 Scope Recommendation

### Candidate Directions

`Phase 24` 收口之后，ListingKit action/helper 邻域里至少还有两个可继续推进的方向：

#### 方向一：ListingKit action execute request handoff ownership

当前最集中的 residual hotspot 在：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)

也就是 execute phase 仍然同时持有：

- retry request clone handoff
- queue request clone handoff
- retry / queue 分支选择
- persistence-session input shaping

#### 方向二：继续深挖 shared `queue/retry` clone helper ownership

例如继续围绕：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)

去讨论它们是否应该继续离开 shared helper home。

这两个方向都合理，但优先级并不相同。

### Recommendation

`Phase 25` 应先聚焦 **ListingKit action execute request handoff ownership**。

也就是优先处理：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)

而不是现在马上把 shared `queue/retry` clone helper 的定义位置重新全面讨论一遍。

### Why This Is The Right Next Slice

#### 1. `Phase 24` 已把 review-navigation builder 的 queue clone duplication 收掉

现在已经明确落地了：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [phase24_review_navigation_queue_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase24_review_navigation_queue_clone_boundary_test.go:1)

这意味着：

- review-navigation 邻域里的 queue clone reuse 已经收清楚
- 继续在这个 builder 周围做对称性打磨，收益已经明显下降

#### 2. 下一块真实压力已经转移到 execute phase 自己的 request/persistence handoff

当前 execute phase：

- 先在 `retryable` 分支里 clone retry request
- 再在默认分支里 clone queue request
- 然后基于不同 page 结果组 `persistenceSession`

这说明根因不再是“某个 clone helper 定义放哪儿”，而是：

- execute phase 自己还在同时持有 request handoff 与 persistence-session 输入塑形

这比直接重开 shared helper owner 更像下一块真正的 bounded ownership hotspot。

#### 3. 这条线比 shared helper relocation 更适合做小切片

shared `queue/retry` clone helper 仍然被很多路径使用：

- action execute
- navigation dispatch
- temporal result
- descriptor / conditional shaping

如果现在直接重开它们的定义位置，会很容易变成一个多 consumer 的大问题。

相比之下，execute phase 本地 request handoff：

- 影响范围更集中
- 行为语义更清晰
- 可以先在 execute-phase 局部收一刀，而不碰别的 consumer

#### 4. 这条线还能顺着已有 action phase model 继续往前走

当前 action 路径已经有明确的 phase 切分：

- entry
- execute
- refresh
- projection
- finalize

所以 execute phase 自己的 request handoff / persistence-session shaping，再切一小刀，会和前面几轮的 ownership 收口风格非常一致，不需要引入新的抽象风格。

### Why Not Reopen Shared Queue / Retry Clone Helpers First

不建议 `Phase 25` 继续优先追 shared `queue/retry` clone helper ownership，原因有三个：

#### 1. 共享性证据仍然很强

当前这些 helper 仍在多个路径里被直接使用，说明它们暂时仍然更像 shared utility，而不是已经明显该继续搬家的本地 seam。

#### 2. 现在去动 helper 定义位置，会把 slice 变大

一旦下刀，很容易把：

- action execute
- navigation dispatch
- temporal result
- descriptor / conditional

这些路径都卷进来。

#### 3. execute phase 自己的本地混合职责更像眼前的根因

既然 execute phase 已经明确同时持有 request clone handoff 和 persistence-session shaping，就没必要跳过这个更近、更具体的 residual hotspot，直接去碰更宽的 shared helper 问题。

### Proposed Phase Shape

一个合适的 `Phase 25` 形状应该是：

1. 保持 `Phase 24` 的 review-navigation queue clone reuse 不动
2. 不重开 shared `queue/retry` helper 的 broader multi-consumer owner
3. 只围绕 action execute phase 的 request clone / persistence handoff 下刀
4. 先明确哪些 request handoff 可以封装成更明确的 local seam
5. 把 execute-phase truly local branch shaping 留在 execute home
6. 用 focused tests 和 ownership guardrail 锁住“harness 更清晰、outward behavior 不变”的方向

### Guardrails

`Phase 25` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 24` 已稳定的 review-navigation queue clone split
2. 不要把范围扩成 shared `queue/retry` helper 的全面大改
3. 不要改变 action execute outward behavior
4. 不要引入新的 generic request-handoff framework
5. 优先把 execute phase 自己的 local request/persistence shaping 收清楚

### Recommended Next Step

下一步建议直接写一个 `Phase 25` implementation plan，主题明确限定为：

**ListingKit action execute request handoff ownership**

计划锚点应放在：

- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)

结论需要明确：

- `Phase 25` 应先聚焦 execute phase 本地 request handoff / persistence-session shaping
- 不建议现在就全面重开 shared `queue/retry` clone helper ownership
- 不建议把这一刀扩大成 generic request-handoff abstraction
