## Task Processor Framework Phase 62 Checkpoint

### Status

`Phase 62` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit action target preview capability filter mutation ownership` 这条切片
- 它没有回头重开 action target filter clone layering
- 它没有回头重开 action target impact clone layering
- 它没有回头重开 shared queue/retry clone owners
- 它没有重构整个 action-key switch
- 它没有扩大成 generic filter mutation framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase62-action-target-preview-capability-filter-mutation.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase62-action-target-preview-capability-filter-mutation.md:1)

### What Landed

#### 1. Preview-capability mutation outward behavior 继续保持稳定

这一轮没有再扩大行为面，而是沿用并 fresh 验证了 `Phase 61` 刚补上的直接夹具：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

它继续直接锁住：

- preview capability assignment
- render-preview toggle
- retryability reset
- execution-quality reset
- ideal-grade fallback
- defensive clone semantics

#### 2. Preview-capability specialization 已有单独 local owner

当前 split 已经更清楚：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
  - 现在只负责：
  - preview-capability home dispatch
  - regular action-key switch rules

- [generation_action_filters_preview_capability_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_preview_capability_mutation.go:1)
  - 现在单独负责：
  - capability lookup
  - preview capability assignment
  - render-preview toggle
  - retryability reset
  - execution-quality reset
  - ideal-grade fallback delegation

这意味着 preview-capability specialization 不再继续和普通 action-key switch 共处一个 mixed local owner。

对应提交：

- `refactor: clarify listingkit preview capability filter mutation ownership`

#### 3. Broader mutation home 被完整保留

这一轮没有动：

- [generation_overview.go](/D:/code/task-processor/internal/listingkit/generation_overview.go:291)

也没有回流污染：

- action target filter clone aggregate home
- action target impact clone layering
- shared queue / retry clone homes

这让 `Phase 61` 刚收下来的 aggregate dispatch 层保持稳定，没有因为继续拆 preview 特例又被重新搅乱。

#### 4. Preview-capability mutation guardrail 已补齐

新增边界测试：

- [phase62_action_target_preview_capability_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase62_action_target_preview_capability_filter_mutation_boundary_test.go:1)

并同步把 `Phase 61` 的 broader mutation boundary 对齐到新的 preview-capability home：

- [phase61_action_target_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase61_action_target_filter_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- preview capability specialization 继续留在新的 narrow local home
- broader mutation home 继续只做 preview dispatch + regular action-key rules
- clone homes 继续留在既有 owner
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 62` 需要证明的核心点有四个：

1. preview-capability filter mutation outward behavior 保持稳定
2. preview-capability specialization 不再直接混在 broader mutation home 里
3. broader mutation / clone layering 没有被重新搅乱
4. preview-capability mutation guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 62` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续深挖 regular action-key switch

当前：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

普通 action-key rules 仍然共处同一个 switch home，但这是刻意保留的下一条 bounded 切片。

#### 2. 它没有把 ideal-review fallback 再单独拆成更细 owner

`applyAssetGenerationIdealReviewFilters(...)` 现在已经很薄。继续为了对称性再拆，不会带来同等级收益。

### Residual Responsibilities Still Present

`Phase 62` 收完之后，最明显的 residual hotspot 已经落在 regular action-key switch 本身：

- missing/provisional/review-ready/retry-section 这些 rule families 仍然共享一个 local owner
- 它们比 preview-capability specialization 更接近下一条真实的 ownership 切片

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit action target regular action-key filter mutation ownership

重点锚点：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
- regular action-key switch cases

原因很直接：

- preview-capability specialization 已经独立出来
- 剩下最明显的 mixed local owner 就是 regular action-key switch
- 这比继续抠已经很薄的 ideal-review helper 更像一个 bounded、收益清晰的小切片

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- preview-capability mutation outward behavior 保持稳定
- ownership split 已按预期落地
- guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
