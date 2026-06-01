# Task Processor Framework Phase 19 Scope Recommendation

## Candidate Directions

在 `Phase 18` 收完以后，ListingKit action 邻域里还剩下两个看起来可能继续推进的方向：

1. 回头继续细抠 `ExecuteTaskGenerationAction(...)` 已稳定的 service-entry seams
2. 转向 `layer-temporal action branching ownership`

这两个方向都还在同一个 action 邻域里，但优先级已经不一样了。

## Recommendation

`Phase 19` 建议聚焦 **ListingKit layer-temporal action branching ownership**。

也就是优先处理：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

而不是回头重开：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:164)

## Why This Is The Right Next Slice

### 1. Service-entry orchestration 已在 Phase 18 收干净

`Phase 18` 已经把 `ExecuteTaskGenerationAction(...)` 的本地 action path 收敛成明确顺序：

1. entry
2. execute
3. persist
4. refresh
5. projection
6. finalize

也就是说，service entry 现在主要承担 orchestration，而不再继续内联持有 bootstrap、persisted-review handoff、projection copy-back、conditional-state finalization 的具体细节。

因此，下一步再回去抠这些 seam，不会带来同等级别的 ownership 收益。

### 2. Layer-temporal bypass 仍然把多种职责压在同一个 helper 里

当前真正还显著混合职责的块是：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

`executeLayerTemporalAction(...)` 仍然同时持有：

1. action-key branching
2. workflow client enablement / config checks
3. platform resolution
4. start-input assembly
5. outward `GenerationActionExecutionResult` assembly
6. audit shaping

这已经不是单一条件分支的问题，而是一个旁路 helper 仍在同时做“是否可执行”“如何启动 workflow”“如何构造 outward result”三类事。

### 3. 它已经是独立 helper，所以适合作为下一刀

这一块的另一个优势是：它已经是单独 helper，而不是散落在 service entry 里的一大段匿名逻辑。

这意味着下一阶段可以直接围绕 `executeLayerTemporalAction(...)` 下刀，继续做 feature-local ownership split，而不需要回头重开已经稳定的：

- entry / persist / refresh / projection / finalize seams

这正符合“延续现有切面边界，而不是重新洗牌整个 action flow”的推进方式。

## Why Not Start With Stable Service-Entry Seams

不建议把 `Phase 19` 的第一刀重新放回 service entry，原因有三个：

1. `Phase 18` 已经把 service-entry orchestration 的主要根因压力收走了
2. 再次拆 entry / persist / finalize 很容易变成对已稳定 seam 的局部重排，而不是解决新的根因热点
3. 这样会分散注意力，让真正还混合职责的 layer-temporal bypass 继续原地不动

换句话说，现在再去重开 `ExecuteTaskGenerationAction(...)` 已稳定的本地路径，收益会明显低于直接处理 temporal bypass 本身。

## Why Not Start With A Generic Temporal Action Framework

也不建议现在直接扩成通用 temporal action framework，原因同样明确：

1. 当前压力只在 ListingKit action 邻域内被证明为真实热点
2. 现有 layer-temporal 分支数量仍然很小，先做 feature-local split 成本更低
3. 如果现在先抽象，很容易在还没有第二个强证据使用点之前引入过度泛化

这一阶段应该继续遵守“先解决局部根因，再看是否真的出现跨特性共性”的节奏。

## Proposed Phase Shape

`Phase 19` 的切片建议围绕 `executeLayerTemporalAction(...)` 形成明确边界，优先考虑把以下职责拆开：

1. action-key branching ownership
2. workflow client/config enablement checks
3. per-action start-input assembly
4. outward result / audit shaping

一个合理的阶段形状应该是：

- 保留 `executeLayerTemporalAction(...)` 作为最外层 temporal bypass entry
- 把具体分支逻辑继续下放到 ListingKit 本地 helper / phase
- 保持已有 queue-only outward semantics 不变
- 用新的 boundary tests 锁住 branch ordering 和责任归属

重点不是做大重构，而是把 temporal bypass 从“一个 helper 里塞完所有事”继续收敛成“一个 helper 负责调度，具体职责各有 home”。

## Guardrails

写 `Phase 19` 计划时，建议保持这些 guardrails：

1. 不要回头重开 `Phase 18` 已稳定的 entry / persist / refresh / projection / finalize seams
2. 不要把范围扩到 HTTP、bootstrap、runtime 或其它 ListingKit 邻域之外
3. 不要现在就抽成 repo-wide generic temporal action framework
4. 保持所有新增 helper 继续是 ListingKit feature-local
5. 保持现有 layer-temporal short-circuit、`queue_only` outward behavior、audit 语义不变

## Recommended Next Step

下一步建议直接写一个 `Phase 19` implementation plan，主题就是 **ListingKit layer-temporal action branching ownership**，根位置明确锚定在：

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:220)

计划应当把目标限定为：继续收敛 temporal bypass 的局部 ownership，而不是回头重开已稳定 seams，也不是提前抽象出通用 temporal action framework。
