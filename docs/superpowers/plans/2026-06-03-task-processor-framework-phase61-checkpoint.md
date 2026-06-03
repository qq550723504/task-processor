## Task Processor Framework Phase 61 Checkpoint

### Status

`Phase 61` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action target filter mutation ownership` 这条切片
- 它没有回头重开 action target filters clone layering
- 它没有回头重开 action target impact clone layering
- 它没有回头重开 shared retry request clone layering
- 它没有回头重开 queue query clone ownership
- 它没有扩大成 broader action execute orchestration redesign
- 它没有引入 generic mutation framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase61-action-target-filter-mutation-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase61-action-target-filter-mutation-ownership.md:1)

### What Landed

#### 1. Action target filter mutation outward behavior 已有直接夹具

这一轮补了两条直接行为测试：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

它们直接锁住了：

- preview-capability action 会清空 `ExecutionQuality`
- preview-capability action 会关闭 `RetryableOnly`
- preview-capability action 会填入 `PreviewCapability`
- preview-capability action 在 quality grade 为空时会回填 `ideal`
- review-only section action 会保留已有 quality grade，同时清空 execution quality
- mutation 结果不会污染传入的 base filters

这让后续继续拆 mutation rule 时，不再只能依赖间接 action-target 回归。

#### 2. Action target filter mutation home 已压成 clone/init + dispatch

当前 split 已经更清楚：

- [generation_overview.go](/D:/code/task-processor/internal/listingkit/generation_overview.go:291)
  - `actionFiltersForKey(...)` 现在只保留：
  - base filters defensive clone
  - nil-init fallback
  - local mutation home dispatch

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
  - 现在负责：
  - preview capability specialization
  - action-key specific grade rewriting
  - retryability toggles
  - execution-quality resets
  - ideal review defaulting

这意味着 action target filter mutation 不再继续直接混在 aggregate home 里。

对应提交：

- `refactor: clarify listingkit action target filter mutation ownership`

#### 3. 既有 clone layering 被完整保留

这一轮没有动：

- [generation_overview.go](/D:/code/task-processor/internal/listingkit/generation_overview.go:282)
- [generation_filters_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_filters_clone_shape.go:1)
- [generation_filters_platforms_clone.go](/D:/code/task-processor/internal/listingkit/generation_filters_platforms_clone.go:1)

也没有回流污染：

- action target clone aggregate home
- impact clone layering
- queue / retry shared clone homes

这让 `Phase 59` 到 `Phase 60` 刚收下来的 clone 边界保持稳定，没有因为 mutation ownership 收口又被重新搅乱。

#### 4. Action target filter mutation guardrail 已补齐

新增边界测试：

- [phase61_action_target_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase61_action_target_filter_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- `actionFiltersForKey(...)` 继续只保留 clone/init + dispatch
- preview capability 和 action-key mutation rules 继续留在新的 local mutation home
- mutation home 不回流持有 clone helper
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 61` 需要证明的核心点有四个：

1. action target filter mutation outward behavior 保持稳定
2. `actionFiltersForKey(...)` 不再直接持有整坨 mutation rules
3. 既有 clone layering 没有被重新搅乱
4. action target filter mutation guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 61` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续深挖 preview capability 这条特例链

当前：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

虽然 ownership 已经比之前清楚，但 preview capability specialization 仍然和普通 action-key switch 共处一个 local home。

#### 2. 它没有把 action-key switch 继续拆成更细的 pairing/final owners

本阶段只先把 mutation rules 从 aggregate home 里拿出来，没有为了对称性继续把所有 case 再切成更多层。

### Residual Responsibilities Still Present

`Phase 61` 收完之后，最明显的 residual hotspot 已经不在 clone 线，而在 mutation home 内部：

- preview capability specialization 仍然是单独一类语义
- 普通 action-key mutation switch 仍然是另一类语义
- 两者目前还共享同一个 local owner

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit action target preview capability filter mutation ownership

重点锚点：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
- `applyAssetGenerationActionFiltersMutation(...)`
- `applyAssetGenerationPreviewCapabilityFilters(...)`

原因很直接：

- 这条 preview-capability specialization 已经形成独立语义块
- 它比继续拆 action-key switch 更像一个 bounded、收益清晰的小切片
- 先把 preview 特例单独收清楚，再考虑是否还值得继续抠其余 switch rules，会更稳

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestActionTargetFilterMutationBoundary|TestCloneAssetGenerationActionTarget|TestActionTargetFiltersCloneBoundary|TestActionTargetFiltersPlatformSliceBoundary|TestTaskGenerationActionTargetCloneAggregateBoundary" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestTaskGenerationAction.*Boundary|TestActionTargetFiltersCloneBoundary|TestActionTargetFiltersPlatformSliceBoundary|TestActionTargetFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- action target filter mutation outward behavior 保持稳定
- mutation ownership split 已按预期落地
- ownership guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
