# Task Processor Framework Phase 21 Scope Recommendation

## Candidate Directions

`Phase 20` 收口之后，ListingKit generation action 邻域里至少还有两个可继续推进的方向：

### 方向一：ListingKit generation action target-resolution helper ownership

当前 broad helper file 里剩下最重的一簇是：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

也就是围绕：

- `resolveAssetGenerationActionTarget(...)`
- `collectAssetGenerationActionTargets(...)`
- `cloneAssetGenerationActionTarget(...)`
- `requestedAssetGenerationActionKey(...)`

这一簇 helper 的 ownership 收敛。

### 方向二：继续深挖 temporal request-parsing alias / test home 的对称性清理

例如继续追这些点：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [phase20_action_temporal_request_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase20_action_temporal_request_boundary_test.go:1)

也就是继续围绕：

- `resolveLayerTemporalPlatform(...)` 这个 legacy alias
- parsing behavior / boundary test home 的进一步对称化

这两个方向都说得通，但优先级并不相同。

## Recommendation

`Phase 21` 应先聚焦 **ListingKit generation action target-resolution helper ownership**。

也就是优先处理：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

而不是继续围绕 `Phase 20` 已经稳定下来的 temporal parsing slice 做对称性清理。

## Why This Is The Right Next Slice

### 1. Phase 20 已经把 temporal parsing 这个 feature-specific hotspot 收清楚

现在已经明确落地了：

- [task_generation_action_temporal_request_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_request_platform.go:1)
- [task_generation_action_temporal_platform.go](/D:/code/task-processor/internal/listingkit/task_generation_action_temporal_platform.go:1)
- [phase20_action_temporal_request_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase20_action_temporal_request_boundary_test.go:1)

这意味着：

- traversal implementation 已有明确本地 home
- platform temporal consumer 已切到新的 local parsing home
- broad helper file 里的 temporal parsing 已缩成 seam alias

所以 temporal parsing 已不再是“最突出的 residual hotspot”。

### 2. 剩下最明显的压力已经转移到 action target resolution / cloning helper cluster

当前 `service_generation_actions.go` 里剩下的 helper 簇同时承载：

- action key normalization / fallback
- overview / request target lookup
- request-target clone 与 interaction-mode defaulting
- nested target copy semantics

这说明根因已经从：

- `temporal request parsing mixed into broad helper file`

转移成：

- `generic generation action target resolution and clone semantics still mixed into one broad helper cluster`

比起继续追 temporal alias 对称性，这一簇更像下一刀应该先切的地方。

### 3. 这条线与已完成的 temporal slices 依赖更松，更适合继续 bounded refactor

`resolveAssetGenerationActionTarget(...)` 一带主要服务的是：

- action target lookup
- request/overview resolution source shaping
- clone/default semantics

而不是 temporal workflow start 路径本身。

这让它更适合成为下一阶段的 bounded refactor：

- 不需要重开 `Phase 19` / `Phase 20` 的 temporal seams
- 也不必碰 HTTP / bootstrap / runtime 层

### 4. 继续深挖 Phase 20 的对称性收益已经明显下降

继续围绕：

- `resolveLayerTemporalPlatform(...)` alias
- parsing behavior test home
- guardrail 形式进一步对称化

当然还能再做一点，但这些更多是“结构更漂亮”，而不是“ownership 根因还没解决”。

相比之下，action target resolution helper cluster 仍然是一个真实的职责聚合点，收益更高。

## Why Not Continue With More Temporal Parsing Cleanup First

不建议 `Phase 21` 继续优先追 temporal parsing alias / test home 的对称性清理，原因有三个：

### 1. 当前残留更多是命名债务，不是 ownership 根因

`resolveLayerTemporalPlatform(...)` 现在只是 seam alias：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)

它当然还能继续对称化，但这已经不是实现 ownership 的主要风险点了。

### 2. 再往下拆容易进入“为了对称而重构”

如果现在继续围着 temporal parsing 打磨，很容易变成：

- test home 更纯
- alias 更少
- 命名更一致

这些都不坏，但它们没有下一簇 action target helper 那么直接地对应真实职责压力。

### 3. 当前 guardrail 已足够支撑暂时停在这里

现在已有：

- [phase20_action_temporal_request_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase20_action_temporal_request_boundary_test.go:1)
- [phase19_action_layer_temporal_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase19_action_layer_temporal_boundary_test.go:1)

因此 temporal parsing 这条线已经有了足够的边界保护，可以先把注意力转向更高收益的 residual hotspot。

## Why Not Jump Straight To A Generic Action Helper Framework

现在也不建议一步跳到“通用 generation action helper framework”，原因同样明确：

1. 当前被证明的热点只是 ListingKit feature-local 的 action target helper cluster
2. 还没有足够证据表明 repo 里多个 feature 都在复用同一种 target-resolution abstraction
3. 现在先抽通用层，很容易为假想共性引入额外框架，而不是解决当前 broad helper file 的真实根因

因此，`Phase 21` 仍应保持 feature-local。

## Proposed Phase Shape

一个合适的 `Phase 21` 形状应该是：

1. 保持 `Phase 19` / `Phase 20` temporal seams 不动
2. 只围绕 `action key -> target lookup -> target clone/defaulting` 这条链下刀
3. 给 `resolveAssetGenerationActionTarget(...)` 一带找一个更明确的 feature-local home
4. 保持当前 target-resolution outward behavior、error surface、clone semantics 不变
5. 用 focused tests 和 ownership guardrail 锁住“action target resolution 不再混在同一个 broad helper cluster 里”的方向

这个阶段的重点不是让 `service_generation_actions.go` 立刻变成空文件，而是把下一块真实的职责聚合点收出去。

## Guardrails

`Phase 21` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 19` / `Phase 20` 已稳定的 temporal seams
2. 不要把范围扩成整个 generation action helper file 的一次性大拆
3. 不要顺手重构 unrelated clone helpers 或 request DTOs
4. 不要引入 repo-wide generic action helper framework
5. 保持当前 action target resolution behavior、error semantics、interaction-mode defaulting、clone semantics 不变
6. 优先让 feature-local seam 承接 `action key -> target lookup -> target clone/defaulting` 这条链

## Recommended Next Step

下一步建议直接写一个 `Phase 21` implementation plan，主题明确限定为：

**ListingKit generation action target-resolution helper ownership**

计划锚点应放在：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:17)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:45)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:57)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:99)

结论需要明确：

- `Phase 21` 应先聚焦 generation action target-resolution helper ownership
- 不建议继续优先做 temporal parsing slice 的对称性清理
- 不建议现在就抽通用 generation action helper framework
