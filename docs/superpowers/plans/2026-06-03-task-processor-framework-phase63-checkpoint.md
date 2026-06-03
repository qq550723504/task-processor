## Task Processor Framework Phase 63 Checkpoint

### Status

`Phase 63` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit regular action-key filter mutation ownership` 这条切片
- 它没有回头重开 preview-capability mutation home
- 它没有回头重开 action target filter clone layering
- 它没有回头重开 action target impact clone layering
- 它没有引入 generic action-key registry/framework
- 它没有扩大成 broader action execute redesign

对应计划文档：

- [2026-06-03-task-processor-framework-phase63-regular-action-key-filter-mutation-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase63-regular-action-key-filter-mutation-ownership.md:1)

### What Landed

#### 1. Regular action-key mutation outward behavior 已有直接夹具

这一轮把 direct behavior fixture 补得更完整了：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

当前直接锁住了：

- preview-capability mutation
- review-only mutation preserving existing grade
- missing-action mutation rewriting to `missing`
- failed-retry mutation rewriting to `provisional + failed + retryable`
- defensive clone semantics

这让 regular action-key rule family 的继续拆分不再只能依赖间接回归。

#### 2. Regular action-key switch 已有单独 local owner

当前 split 已经更清楚：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
  - 现在只负责：
  - preview-capability mutation home dispatch
  - regular action-key mutation home dispatch

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
  - 现在单独负责：
  - missing-slot style rules
  - failed/provisional retry rules
  - review-ready and section-review rules

这意味着 broader mutation home 不再继续直接持有 regular action-key switch 本体。

对应提交：

- `refactor: clarify listingkit regular action-key filter mutation ownership`

#### 3. Preview-capability home 被完整保留

这一轮没有动：

- [generation_action_filters_preview_capability_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_preview_capability_mutation.go:1)

也没有回流污染：

- action target filter clone aggregate home
- impact clone layering
- shared queue / retry clone homes

这让 `Phase 62` 刚收下来的 preview-capability specialization 保持稳定，没有因为继续拆 regular switch 又被重新搅乱。

#### 4. Regular action-key mutation guardrail 已补齐

新增边界测试：

- [phase63_regular_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase63_regular_action_key_filter_mutation_boundary_test.go:1)

并同步把 `Phase 61` 的 broader mutation boundary 对齐到新的 routing 现实：

- [phase61_action_target_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase61_action_target_filter_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- broader mutation home 继续只做 preview + regular-action-key dispatch
- regular action-key switch 继续留在新的 local home
- preview-capability mutation home 继续独立
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 63` 需要证明的核心点有四个：

1. regular action-key mutation outward behavior 保持稳定
2. regular action-key switch 不再直接混在 broader mutation home 里
3. preview-capability mutation home 和 clone layering 没有被重新搅乱
4. regular action-key mutation guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 63` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续深挖 regular action-key switch 内部的 rule families

当前：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

仍然同时持有 missing-slot、failed/provisional retry、review-ready/section-review 这些不同 family。

#### 2. 它没有把 ideal-review helper 再单独拆开

`applyAssetGenerationIdealReviewFilters(...)` 仍然保持当前薄 helper 形态。本轮没有为了对称性去切更细。

### Residual Responsibilities Still Present

`Phase 63` 收完之后，最明显的 residual hotspot 已经落在 regular action-key switch 内部：

- missing-slot style rules 是一类语义
- failed/provisional retry rules 是另一类语义
- review-ready / section-review rules 又是另一类语义

它们仍然共享一个 local owner。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit retry-oriented action-key filter mutation ownership

重点锚点：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
- failed/provisional retry-oriented rule families

原因很直接：

- retry-oriented rules 自成一组，语义上最集中
- 它比继续抠 ideal-review helper 更像下一条 bounded、收益清晰的小切片
- 先把 retry-oriented family 独立出来，再看剩余 missing/review families 是否还值得继续拆，会更稳

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- regular action-key mutation outward behavior 保持稳定
- ownership split 已按预期落地
- guardrails 已按预期落地
- HTTP / temporal 下游测试面没有被这次切片回归破坏
