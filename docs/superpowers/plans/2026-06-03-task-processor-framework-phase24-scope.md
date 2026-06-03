## Task Processor Framework Phase 24 Scope Recommendation

### Candidate Directions

`Phase 23` 收口之后，ListingKit review-navigation / clone-helper 邻域里至少还有两个可继续推进的方向：

#### 方向一：ListingKit review-navigation queue clone shaping ownership

当前最接近的 residual duplication 在：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

也就是 review-navigation builder 仍然本地做 `QueueQuery` shallow clone，而 shared queue clone helper 已经存在。

#### 方向二：继续深挖 shared queue / retry clone helper ownership

例如继续围绕：

- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [cloneRetryGenerationTasksRequest(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)
- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)
- [task_generation_navigation_dispatch_primary.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_primary.go:1)

去判断 shared helper home 是否还应该继续收窄。

这两个方向都合理，但优先级并不相同。

### Recommendation

`Phase 24` 应先聚焦 **ListingKit review-navigation queue clone shaping ownership**。

也就是优先处理：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

而不是现在就把 shared `queue/retry` clone helper 的多 consumer owner 全面重开。

### Why This Is The Right Next Slice

#### 1. `Phase 23` 已把 action-target clone duplication 收干净

现在已经明确落地了：

- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)
- [phase23_navigation_action_target_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase23_navigation_action_target_clone_boundary_test.go:1)

这意味着：

- action-target clone reuse 已经收好
- navigation local home 也已经缩到真正的 navigation-specific shaping

所以 review-navigation 邻域里下一块最突出的 residual pressure，已经不再是 action-target clone，而是 builder 本地 queue clone。

#### 2. Builder 本地 queue clone 与 shared queue clone helper 已形成明显重复

当前：

- `buildGenerationReviewActionNavigationTarget(...)` 本地做 `cloned := *target.QueueQuery`
- `cloneGenerationQueueQuery(...)` 已存在并被大量 consumer 使用

这和 `Phase 23` 之前 action-target clone duplication 的形态很相似：一个局部 builder 还保留着与 shared/local helper 相邻的重复 copy semantics。

#### 3. 这条线比重开 shared queue / retry helper 更适合做 bounded refactor

`cloneGenerationQueueQuery(...)` 和 `cloneRetryGenerationTasksRequest(...)` 目前都还是多路径共享：

- action execute
- navigation dispatch
- descriptor / conditional shaping
- temporal result

如果现在直接重开 shared `queue/retry` helper owner，会更容易把切片变宽。

相比之下，review-navigation builder 里的 queue clone shaping：

- 影响范围更集中
- 与当前刚完成的 navigation clone ownership 高度相邻
- 更适合在不触碰广泛 consumer 的前提下，再完成一刀清晰的 ownership 收口

#### 4. 这条线仍然可以保持 feature-local，不会提前抽象过度

下一步如果围绕 review-navigation queue clone shaping 下刀，仍然可以把范围控制在：

- queue clone reuse
- builder-local shaping
- review-navigation outward identity stability

而不需要提前发明新的 shared queue-clone policy layer。

### Why Not Reopen Shared Queue / Retry Clone Helpers First

不建议 `Phase 24` 继续优先追 shared `queue/retry` clone helper ownership，原因有三个：

#### 1. 共享性证据现在仍然很强

从当前调用面看：

- action execute
- navigation dispatch
- descriptor / conditional shaping
- temporal result

都在使用 `cloneGenerationQueueQuery(...)`，这说明它现在仍然更像 shared helper，而不是一眼就该继续收窄的 local seam。

#### 2. 现在去动 shared `queue/retry` helper，很容易把切片扩大到多 consumer rewrite

一旦下刀，很容易连带牵出：

- queue read / dispatch shaping
- retry request shaping
- temporal result query shaping

这会明显放大范围。

#### 3. 当前已有一个更近、更小、更清楚的 residual duplication

既然 builder local queue clone 已经是更近的重复点，就没有必要跳过它，直接去处理更宽的 shared helper owner 问题。

### Proposed Phase Shape

一个合适的 `Phase 24` 形状应该是：

1. 保持 `Phase 23` 的 action-target clone reuse 不动
2. 不重开 shared `queue/retry` clone helper 的多 consumer owner
3. 只围绕 review-navigation builder 的 queue clone shaping 下刀
4. 先明确哪些 queue clone 语义可以直接复用 shared queue clone home
5. 把 builder-local truly navigation-owned shaping 留在 review-navigation file
6. 用 focused tests 和 ownership guardrail 锁住“复用更多、重复更少、outward behavior 不变”的方向

### Guardrails

`Phase 24` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 23` 已稳定的 action-target clone split
2. 不要把范围扩成 shared `queue/retry` helper 的全面大改
3. 不要改变 review-navigation outward identity / descriptor behavior
4. 不要引入新的 shared clone policy abstraction
5. 优先让 builder-local queue clone 只保留 truly local shaping

### Recommended Next Step

下一步建议直接写一个 `Phase 24` implementation plan，主题明确限定为：

**ListingKit review-navigation queue clone shaping ownership**

计划锚点应放在：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [buildGenerationReviewActionNavigationTarget(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:40)
- [cloneGenerationQueueQuery(...)](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)

结论需要明确：

- `Phase 24` 应先聚焦 review-navigation builder 的 queue clone reuse
- 不建议现在就全面重开 shared `queue/retry` clone helper ownership
- 不建议把这一刀扩大成 queue-clone framework
