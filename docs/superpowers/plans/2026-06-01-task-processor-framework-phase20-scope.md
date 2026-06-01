# Task Processor Framework Phase 20 Scope Recommendation

## Candidate Directions

`Phase 19` 收口之后，ListingKit action temporal 邻域里至少有两个可继续推进的方向：

### 方向一：ListingKit layer-temporal request-shape parsing ownership

核心热点：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

也就是 `resolveLayerTemporalPlatform(...)` 这一类 temporal request-shape parsing 逻辑的归属问题。

### 方向二：更宽的 generation action target-resolution helper ownership

同一文件里还存在更广义的 action target helpers：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:56)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:130)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:140)

也就是围绕：

- `resolveAssetGenerationActionTarget(...)`
- `requestedAssetGenerationActionKey(...)`
- `cloneAssetGenerationActionTarget(...)`

这一带的 helper ownership 整理。

这两个方向都合理，但优先级并不相同。

## Recommendation

`Phase 20` 应先聚焦 **ListingKit layer-temporal request-shape parsing ownership**。

也就是优先处理：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

而不是现在就启动更宽的 `service_generation_actions.go` 大拆分，也不是抽象通用 request-parsing framework。

## Why This Is The Right Next Slice

### 1. Phase 19 已把 temporal execution seams 收清楚

`Phase 19` 已经把 temporal execution 侧的三个关键 seam 明确落地：

- [task_generation_action_temporal_result.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_result.go:1)
- [task_generation_action_temporal_standard.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_standard.go:1)
- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)

这意味着：

- standard temporal branch ownership 已经有 home
- platform temporal branch ownership 已经有 home
- shared outward result / audit shaping 已经有 home

所以 temporal 邻域里的下一块真实热点，已经不再是 execution branching，而是 execution 之前的 request-shape parsing ownership。

### 2. 剩下最明显的 feature-specific logic 就是 request-shape traversal

`resolveLayerTemporalPlatform(...)` 现在仍沿着多个 request-shape 层级提取 platform：

- `QueueQuery`
- `NavigationTarget`
- `SessionQuery`
- `PreviewQuery`
- `FollowUpReads`
- nested `ActionTarget`

这一段逻辑非常明显地仍然是 temporal-specific 的 request parsing，而不是普适的 action target resolution。

### 3. 这个 helper 还混在更泛的 actions helper 文件里

当前 temporal request parsing 还和这些更泛的 helpers 同处一地：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:56)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:130)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:140)

这恰好说明 ownership 的根问题还没完全清掉：

- temporal execution 已经局部落地
- temporal request parsing 还没真正落到 temporal 本地

因此，下一步最自然的切片就是先把 temporal-specific request parsing 收回本地。

## Why Not Start With Broader Generation Action Target-Resolution Helper Ownership

不建议 `Phase 20` 一上来就先做更宽的 action target resolver ownership，原因有三个：

### 1. 它的范围更宽，容易混入非 temporal concerns

`resolveAssetGenerationActionTarget(...)` 一带涉及：

- action key 解析
- overview / request target resolution
- target clone
- interaction mode 缺省处理

这些职责并不只服务 temporal path，本质上是更泛的 generation action target-resolution 邻域。

### 2. 当前最清晰的热点仍是 temporal-specific request parsing

相比之下，`resolveLayerTemporalPlatform(...)` 已经被证明直接服务 `layer-temporal` 路径，而且问题形态也更聚焦：

- 它就是 request-shape traversal ownership 还未本地化

这比现在就对更宽 resolver helper 做整体重排更适合作为下一刀。

### 3. 过早做大拆分会模糊根因

如果现在直接把整个 [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:1) 一次性大拆，很容易把：

- temporal request parsing ownership
- generic action target resolution ownership
- cloning / key resolution helper ownership

混成同一个工程动作，反而削弱“按根因切片”的清晰度。

## Why Not Start With A Generic Request-Parsing Framework

现在也不建议抽象通用 request-parsing framework，原因同样明确：

1. 当前被证明为热点的，是 ListingKit temporal 邻域内的局部 request-shape traversal
2. 还没有足够证据表明 repo 里已有多个 feature 在复用同一种 request-parsing abstraction
3. 现在先抽象，风险是为了假想共性引入额外框架层，而不是解决现有 ownership 根因

因此，`Phase 20` 应继续保持 feature-local。

## Proposed Phase Shape

一个合适的 `Phase 20` 形状应该是：

1. 保留现有 temporal execution seams 不动
2. 只围绕 temporal-specific request-shape parsing ownership 下刀
3. 给 `resolveLayerTemporalPlatform(...)` 一类逻辑找一个更贴近 temporal 本地语义的 home
4. 保持现有 platform defaulting / normalization 行为不变
5. 用边界测试锁住“temporal request parsing 不再散落在更泛 actions helper 文件里”的方向

这个阶段的重点不是让 `service_generation_actions.go` 立刻变小，而是把 temporal-specific parsing 从更泛 helper ownership 中分离出来。

## Guardrails

`Phase 20` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 19` 已稳定的 standard / platform / result temporal execution seams
2. 不要把范围扩成整个 `service_generation_actions.go` 的一次性大拆
3. 不要顺手把 `resolveAssetGenerationActionTarget(...)` 一带全部重排
4. 不要抽象 repo-wide generic request-parsing framework
5. 保持现有 temporal outward behavior、workflow start behavior、platform defaulting 与 normalization 不变
6. 优先用 feature-local seam 承接 temporal request parsing ownership

## Recommended Next Step

下一步建议直接写一个 `Phase 20` implementation plan，主题明确限定为：

**ListingKit layer-temporal request-shape parsing ownership**

计划锚点应放在：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

结论需要明确：

- `Phase 20` 应先聚焦 temporal request-shape parsing ownership
- 不建议现在就把整个 `service_generation_actions.go` 一次性大拆
- 不建议现在就抽通用 request-parsing framework
