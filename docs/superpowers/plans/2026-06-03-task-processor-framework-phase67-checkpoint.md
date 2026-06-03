## Task Processor Framework Phase 67 Checkpoint

### Status

`Phase 67` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit non-retry regular action-key mutation ownership` 这条切片
- 它没有回头重开 retry-oriented ownership
- 它没有回头重开 preview-capability mutation ownership
- 它没有回头重开 action target filter clone layering
- 它没有引入 generic mutation rule framework
- 它没有扩大成 broader action execute redesign

对应计划文档：

- [2026-06-03-task-processor-framework-phase67-non-retry-regular-action-key-mutation-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase67-non-retry-regular-action-key-mutation-ownership.md:1)

### What Landed

#### 1. Non-retry outward behavior 已有直接夹具

这一轮 direct behavior fixture 又补强了一步：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

当前直接锁住了：

- preview-capability mutation
- missing-slot mutation
- review-ready defaulting
- section-review preserving existing grade
- retry-oriented mutations
- defensive clone semantics

这让 non-retry 这条线继续拆时，不再依赖间接 action-target 回归。

#### 2. Review-ready / section-review family 已有单独 local owner

当前 split 已经更清楚：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
  - 现在只负责：
  - retry-oriented home dispatch
  - review-ready home dispatch
  - missing-slot family

- [generation_action_filters_review_ready_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_review_ready_mutation.go:1)
  - 现在单独负责：
  - `review_ready_assets`
  - `continue_publish_review`
  - `defer_section_review`
  - `approve_section_review`

这意味着 review-ready / section-review semantics 不再继续和 missing-slot family 共处同一个 local owner。

对应提交：

- `refactor: clarify listingkit non-retry regular action-key mutation ownership`

#### 3. Retry-oriented 与 preview-capability layering 被完整保留

这一轮没有动：

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)
- [generation_action_filters_preview_capability_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_preview_capability_mutation.go:1)

也没有回流污染：

- clone layering
- action target aggregate routing

这让 `Phase 64` 到 `Phase 66` 刚收下来的 retry-oriented split 保持稳定，没有因为继续拆 non-retry family 又被重新搅乱。

#### 4. Non-retry guardrail 已补齐

新增边界测试：

- [phase67_non_retry_regular_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase67_non_retry_regular_action_key_mutation_boundary_test.go:1)

并同步把 `Phase 63` 和 `Phase 64` 的 regular/retry boundary 对齐到新的 review-ready routing 现实：

- [phase63_regular_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase63_regular_action_key_filter_mutation_boundary_test.go:1)
- [phase64_retry_oriented_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase64_retry_oriented_action_key_filter_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- regular-action-key home 继续只做 retry dispatch + review-ready dispatch + missing-slot family
- review-ready / section-review family 继续留在新的 narrow local home
- retry-oriented homes 继续稳定
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 67` 需要证明的核心点有四个：

1. non-retry outward behavior 保持稳定
2. review-ready / section-review family 不再直接混在 broader regular-action-key home 里
3. retry-oriented / preview-capability / clone layering 没有被重新搅乱
4. non-retry guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 67` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续拆 missing-slot family

当前：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

仍然直接持有 `generate_missing_assets` / `review_missing_slots` 这组 missing-slot family。

#### 2. 它没有做 broader completion audit

虽然 residual hotspot 已经明显变少，但这轮仍然优先完成最后这条高信号 ownership 切片，没有在这一轮里做全局收官审计。

### Residual Responsibilities Still Present

`Phase 67` 收完之后，最明显的 residual hotspot 已经大幅收敛，只剩两种比较真实的后续方向：

- missing-slot family 的最后一层 local ownership
- broader framework completion audit / residual hotspot review

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit missing-slot action-key mutation ownership

重点锚点：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

原因很直接：

- retry-oriented 和 review-ready 这两侧都已经独立出来
- 剩下最明显的最后一块 action-key family 就是 missing-slot
- 再收这一刀之后，就很接近一个值得做 broader completion audit 的收官点

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary|TestNonRetryRegularActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- non-retry outward behavior 保持稳定
- ownership split 已按预期落地
- guardrails 已按预期落地
- temporal 下游测试面没有被这次切片回归破坏
