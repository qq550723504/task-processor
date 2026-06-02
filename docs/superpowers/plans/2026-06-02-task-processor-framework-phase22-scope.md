# Task Processor Framework Phase 22 Scope Recommendation

## Candidate Directions

`Phase 21` 收口之后，ListingKit action helper 邻域里至少还有两个可继续推进的方向：

### 方向一：ListingKit action target clone helper ownership

当前 broad helper file 里剩下最重的一簇是：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

也就是围绕：

- `cloneAssetGenerationActionTarget(...)`
- `cloneAssetGenerationActionImpact(...)`
- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

这一簇共享 clone helper 的 ownership 收敛。

### 方向二：继续深挖 target-resolution seam 的对称性清理

例如继续围绕：

- [task_generation_action_target_resolution.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_resolution.go:1)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:9)

做进一步的命名/alias/test-home 对称化。

这两个方向都合理，但优先级并不相同。

## Recommendation

`Phase 22` 应先聚焦 **ListingKit action target clone helper ownership**。

也就是优先处理：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

而不是继续围绕 `Phase 21` 已经稳定下来的 target-resolution seam 做对称性清理。

## Why This Is The Right Next Slice

### 1. Phase 21 已经把 target-resolution 行为收清楚

现在已经明确落地了：

- [task_generation_action_target_resolution.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_resolution.go:1)
- [task_generation_action_entry.go](/D:/code/task-processor/internal/listingkit/task_generation_action_entry.go:1)
- [phase21_action_target_resolution_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase21_action_target_resolution_boundary_test.go:1)

这意味着：

- resolution behavior 已有明确本地 home
- entry consumer 已切到新的 local seam
- broad helper file 里的 resolution implementation 已被挪走

所以 target-resolution 已不再是“最突出的 residual hotspot”。

### 2. 剩下最明显的压力已经转移到共享 clone helper cluster

当前 `service_generation_actions.go` 里剩下的 helper 几乎都在做 clone/copy semantics：

- target clone
- impact clone
- queue query clone
- retry request clone

这说明根因已经从：

- `target resolution mixed into broad helper file`

转移成：

- `shared clone helper cluster still lacks a clearer ownership home`

比起继续追 target-resolution alias 对称性，这一簇更像下一刀应该先切的地方。

### 3. 这条线和已完成的 resolution slice 关联紧，但不需要重开行为逻辑

clone helper 的问题本质上更接近：

- 共享 copy semantics
- 谁拥有 nested target clone
- 哪些 clone helper 仍然应该作为 shared utility 留在 broad file

而不是重新设计 target-resolution outward behavior。

这让它很适合作为下一阶段的 bounded refactor：

- 可以复用 `Phase 21` 已锁住的 resolution behavior tests
- 不必重开 entry / execute / persist / projection 行为
- 也不必碰 temporal seams

### 4. 继续深挖 target-resolution 对称性收益已经下降

如果现在继续围着：

- `requestedAssetGenerationActionKey(...)` 的命名或位置
- target-resolution phase 的轻微 test-home/alias 调整

去做对称性清理，会更像“结构更漂亮”，而不是“真实 ownership 根因还没解决”。

相比之下，clone helper cluster 仍然是 broad helper file 里真实残留的一组共享职责。

## Why Not Continue With More Target-Resolution Cleanup First

不建议 `Phase 22` 继续优先追 target-resolution seam 的对称性清理，原因有三个：

### 1. 当前残留更多是命名/归属细节，不是行为 ownership 根因

`Phase 21` 已经把真正的 resolution behavior 收出去，剩下更多是：

- clone helper 仍在 broad file
- target-resolution 周边的共享 utility 还没完全有更清晰的 home

### 2. 再往下拆容易进入“为了对称而重构”

如果现在继续围着 target-resolution 打磨，很容易变成：

- 文件更整齐
- alias 更少
- helper 名更一致

这些都不坏，但没有 clone helper cluster 那么直接地对应真实职责压力。

### 3. 当前 guardrail 已足够支撑暂时停在这里

现在已有：

- [phase21_action_target_resolution_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase21_action_target_resolution_boundary_test.go:1)
- [phase18_action_service_entry_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase18_action_service_entry_boundary_test.go:1)

因此 target-resolution 这条线已经有足够的边界保护，可以先把注意力转向更高收益的 residual hotspot。

## Why Not Jump Straight To A Generic Clone Framework

现在也不建议一步跳到“通用 clone/copy framework”，原因同样明确：

1. 当前被证明的热点只是 ListingKit feature-local 的 clone helper cluster
2. 还没有足够证据表明 repo 里多个 feature 都需要同一种 clone abstraction
3. 现在先抽通用层，很容易为假想共性引入额外框架，而不是解决当前 broad helper file 的真实根因

因此，`Phase 22` 仍应保持 feature-local。

## Proposed Phase Shape

一个合适的 `Phase 22` 形状应该是：

1. 保持 `Phase 19` / `Phase 20` / `Phase 21` 的行为 seams 不动
2. 只围绕共享 clone helper cluster 下刀
3. 先明确哪些 clone helpers 仍应作为 shared utility 保留，哪些应由 action target 本地 seam 持有
4. 保持当前 clone semantics、nested target copy behavior、retry/query copy behavior 不变
5. 用 focused tests 和 ownership guardrail 锁住“clone helper 归属更清晰，但 outward clone behavior 不变”的方向

## Guardrails

`Phase 22` 建议保持以下 guardrails：

1. 不要回头重开 `Phase 19` / `Phase 20` / `Phase 21` 已稳定的行为 seams
2. 不要把范围扩成所有 helper 的一次性大拆
3. 不要改变当前 clone/copy outward behavior
4. 不要引入 repo-wide generic clone framework
5. 优先让 feature-local seam 承接 action-target-specific clone semantics，再决定哪些通用 clone helper 仍留 shared home

## Recommended Next Step

下一步建议直接写一个 `Phase 22` implementation plan，主题明确限定为：

**ListingKit action target clone helper ownership**

计划锚点应放在：

- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:13)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:25)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:36)
- [service_generation_actions.go](/D:/code/task-processor/internal/listingkit/service_generation_actions.go:44)

结论需要明确：

- `Phase 22` 应先聚焦 clone helper ownership
- 不建议继续优先做 target-resolution seam 的对称性清理
- 不建议现在就抽通用 clone framework
