## Task Processor Framework Phase 65 Checkpoint

### Status

`Phase 65` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit failed-vs-provisional retry action-key mutation ownership` 这条切片
- 它没有回头重开 preview-capability mutation ownership
- 它没有回头重开 broader regular-action-key routing
- 它没有回头重开 action target filter clone layering
- 它没有引入 generic mutation rule framework
- 它没有扩大成 broader action execute redesign

对应计划文档：

- [2026-06-03-task-processor-framework-phase65-failed-vs-provisional-retry-action-key-mutation-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase65-failed-vs-provisional-retry-action-key-mutation-ownership.md:1)

### What Landed

#### 1. Retry-family outward behavior 已有更完整的直接夹具

这一轮继续补强了 direct behavior fixture：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

当前直接锁住了：

- failed retry mutation
- provisional retry mutation
- section retry mutation
- missing mutation
- review-only mutation
- preview-capability mutation
- defensive clone semantics

这让 retry-family 内部继续拆分时，不再依赖间接回归。

#### 2. Failed retry 与 provisional/section retry 已有单独 local owners

当前 split 已经更清楚：

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)
  - 现在只负责：
  - failed-retry home dispatch
  - provisional/section retry home dispatch

- [generation_action_filters_failed_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_failed_retry_mutation.go:1)
  - 现在单独负责：
  - `retry_failed_generation`
  - `inspect_failed_renderer_tasks`

- [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)
  - 现在单独负责：
  - `upgrade_fallback_assets`
  - `retry_provisional_slots`
  - `retry_section_generation`

这意味着 failed retry semantics 不再继续和 provisional/section retry semantics 共处同一个 local owner。

对应提交：

- `refactor: clarify listingkit failed-vs-provisional retry action-key mutation ownership`

#### 3. Retry-oriented home 被完整保留

这一轮没有动：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

也没有回流污染：

- preview-capability mutation home
- action target filter clone layering
- action target impact clone layering
- shared queue / retry clone homes

这让 `Phase 64` 刚收下来的 retry-oriented split 保持稳定，没有因为继续拆 failed/provisional layering 又被重新搅乱。

#### 4. Failed-vs-provisional retry guardrail 已补齐

新增边界测试：

- [phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go:1)

并同步把 `Phase 64` 的 retry-oriented boundary 对齐到新的 failed/provisional routing 现实：

- [phase64_retry_oriented_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase64_retry_oriented_action_key_filter_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- retry-oriented home 继续只做 failed/provisional dispatch
- failed retry family 继续留在新的 narrow local home
- provisional/section retry family 继续留在新的 sibling local home
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 65` 需要证明的核心点有四个：

1. failed-vs-provisional retry outward behavior 保持稳定
2. failed retry family 不再直接混在 provisional/section retry home 里
3. retry-oriented / regular-action-key / preview-capability layering 没有被重新搅乱
4. failed-vs-provisional retry guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 65` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续深挖 provisional retry vs section retry

当前：

- [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)

仍然同时持有 provisional retry 和 section retry 这两类更细 family。

#### 2. 它没有转去 non-retry families

missing-slot 和 review-ready / section-review 这两组 non-retry families 仍然保留在 regular-action-key home，本轮没有切回去动它们。

### Residual Responsibilities Still Present

`Phase 65` 收完之后，最明显的 residual hotspot 已经集中到两种候选：

- provisional retry vs section retry inside the provisional retry home
- remaining non-retry families inside the regular-action-key home

其中更自然的下一刀还是 provisional retry home 内部，因为这条线现在已经非常连续。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit provisional-vs-section retry action-key mutation ownership

重点锚点：

- [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)

原因很直接：

- provisional retry rules 和 section retry rule 已经形成清晰的下一层语义分组
- 它比立刻跳回 non-retry families 更像一个 bounded、收益清晰的小切片
- 顺着 retry-oriented 这条线继续收一层，结构会更稳定

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- failed-vs-provisional retry outward behavior 保持稳定
- ownership split 已按预期落地
- guardrails 已按预期落地
- temporal 下游测试面没有被这次切片回归破坏
