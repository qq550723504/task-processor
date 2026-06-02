## Task Processor Framework Phase 23 Scope Recommendation

### Candidate Directions

`Phase 22` 收口之后，ListingKit clone/helper 邻域里至少还有两个可继续推进的方向：

#### 方向一：ListingKit navigation action-target clone shaping ownership

当前最明显的重复邻接在：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

也就是 navigation 专用 action-target clone 语义，与 `Phase 22` 刚收出来的 local clone home 之间的 ownership 再收敛。

#### 方向二：继续深挖 shared queue / retry clone helper ownership

例如继续围绕：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:15)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:23)
- [task_generation_action_execute.go](/D:/code/task-processor/internal/listingkit/task_generation_action_execute.go:19)
- [task_generation_navigation_dispatch_primary.go](/D:/code/task-processor/internal/listingkit/task_generation_navigation_dispatch_primary.go:1)

去判断 `cloneGenerationQueueQuery(...)` / `cloneRetryGenerationTasksRequest(...)` 是否也该继续收窄 home。

这两个方向都合理，但优先级并不相同。

### Recommendation

`Phase 23` 应先聚焦 **ListingKit navigation action-target clone shaping ownership**。

也就是优先处理：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

而不是现在马上重开 shared queue / retry clone helper 的更大归属问题。

### Why This Is The Right Next Slice

#### 1. Phase 22 之后，最显眼的 residual pressure 已经转移到 navigation-specific clone duplication

现在已经明确落地了：

- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1)
- [phase22_action_target_clone_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase22_action_target_clone_boundary_test.go:1)

这意味着：

- broad helper cluster 的大块 ownership 已经收清楚
- shared queue / retry clone 也已经被明确标记为“本阶段先不动”

所以当前最突出的 residual hotspot，已经变成 navigation-specific clone semantics 与 local clone home 的邻接重复。

#### 2. `cloneAssetGenerationActionTargetForNavigation(...)` 的职责和 local clone seam 高度重叠

当前这条 navigation helper 仍然自己承载：

- filters clone
- queue query clone
- retry request clone
- expected impact clone
- navigation-only shaping

前四件事和 [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1) 已经高度重复，真正只属于 navigation 的差异只剩：

- `NavigationTarget = nil`
- 以及 surrounding navigation target assembly

这非常像下一刀应该先切的真实 ownership seam。

#### 3. 这条线比 shared queue / retry clone 更适合做 bounded refactor

`cloneGenerationQueueQuery(...)` 和 `cloneRetryGenerationTasksRequest(...)` 当前有很多 consumer：

- action execute
- navigation dispatch
- navigation descriptor / conditional
- temporal result

这说明如果现在就去重开 shared queue / retry clone 的 owner，会更容易扩散。

相比之下，navigation-specific action-target clone duplication：

- 影响范围更集中
- 语义更贴近 `Phase 22` 刚完成的 local clone home
- 更容易在不触碰广泛 consumer 的前提下做出有价值的 ownership 收敛

#### 4. 这条线还能继续保持 feature-local，不会提前抽象过度

下一步如果围绕 navigation-specific action-target clone 收权，仍然可以把范围控制在：

- review navigation target construction
- action-target-local clone reuse
- navigation-only shaping

而不需要提前发明通用 clone strategy / policy abstraction。

### Why Not Reopen Shared Queue / Retry Clone Helpers First

不建议 `Phase 23` 继续优先追 shared queue / retry clone helper ownership，原因有三个：

#### 1. 这两条 helper 目前仍然真的被多路径共享

从当前调用面看：

- action execute
- navigation dispatch
- descriptor / conditional shaping
- temporal result

都在使用 `cloneGenerationQueueQuery(...)`，`cloneRetryGenerationTasksRequest(...)` 的共享性证据比 `cloneAssetGenerationActionTargetForNavigation(...)` 强得多。

#### 2. 现在就动 shared queue / retry clone，更容易把切片变宽

因为一旦收这两条 helper，就很容易连带牵出：

- queue read / dispatch plan
- retry request shaping
- temporal result query shaping

这会让下一阶段从一个局部 ownership slice，变成一个多 consumer 的 helper rewrite。

#### 3. 当前没有足够证据表明 shared home 已经错了

`Phase 22` 只是把这两条 helper 明确留在 shared home，还没有证据证明它们现在就必须继续移动。

而 navigation-specific action-target clone duplication 则已经有非常直接的结构性信号。

### Proposed Phase Shape

一个合适的 `Phase 23` 形状应该是：

1. 保持 `Phase 22` 的 clone split 不动
2. 不重开 `queue/retry` shared helpers 的更大 consumer 面
3. 只围绕 navigation-specific action-target clone duplication 下刀
4. 先明确哪些 clone 语义可以直接复用 local clone home
5. 把 navigation-only shaping 留在 review navigation target 附近
6. 用 focused tests 和 ownership guardrail 锁住“复用更多、重复更少、navigation contract 仍清晰”的方向

### Guardrails

`Phase 23` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 22` 已稳定的 clone split
2. 不要把范围扩成所有 shared clone helper 的大重构
3. 不要改变 navigation target 的 outward shaping behavior
4. 不要引入通用 clone strategy / framework
5. 优先让 navigation-specific action-target shaping 只保留 navigation truly owns 的那部分差异

### Recommended Next Step

下一步建议直接写一个 `Phase 23` implementation plan，主题明确限定为：

**ListingKit navigation action-target clone shaping ownership**

计划锚点应放在：

- [generation_review_navigation_target.go](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:1)
- [cloneAssetGenerationActionTargetForNavigation(...)](/D:/code/task-processor/internal/listingkit/generation_review_navigation_target.go:55)
- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

结论需要明确：

- `Phase 23` 应先聚焦 navigation-specific action-target clone duplication
- 不建议现在就重开 shared queue / retry clone helper ownership
- 不建议把这一刀扩大成通用 clone abstraction
