## Task Processor Framework Phase 66 Checkpoint

### Status

`Phase 66` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit provisional-vs-section retry action-key mutation ownership` 这条切片
- 它没有回头重开 failed retry ownership
- 它没有回头重开 broader regular-action-key routing
- 它没有回头重开 preview-capability mutation ownership
- 它没有回头重开 action target filter clone layering
- 它没有引入 generic mutation rule framework

对应计划文档：

- [2026-06-03-task-processor-framework-phase66-provisional-vs-section-retry-action-key-mutation-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase66-provisional-vs-section-retry-action-key-mutation-ownership.md:1)

### What Landed

#### 1. Provisional-vs-section retry outward behavior 继续保持稳定

这一轮没有再扩大 direct behavior fixture，而是沿用并 fresh 验证了现有测试：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

当前直接锁住的相关语义包括：

- failed retry mutation
- provisional retry mutation
- section retry mutation
- defensive clone semantics

#### 2. Section retry 已有单独 local owner

当前 split 已经更清楚：

- [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)
  - 现在只负责：
  - section retry home dispatch
  - provisional retry pair

- [generation_action_filters_section_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_section_retry_mutation.go:1)
  - 现在单独负责：
  - `retry_section_generation`

这意味着 section retry semantics 不再继续和 provisional retry pair 共处同一个 local owner。

对应提交：

- `refactor: clarify listingkit provisional-vs-section retry action-key mutation ownership`

#### 3. Failed retry 与 broader retry layering 被完整保留

这一轮没有动：

- [generation_action_filters_failed_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_failed_retry_mutation.go:1)
- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)

也没有回流污染：

- regular-action-key home
- preview-capability mutation home
- clone layering

这让 `Phase 65` 刚收下来的 failed-vs-provisional split 保持稳定，没有因为继续拆 section retry 又被重新搅乱。

#### 4. Provisional-vs-section retry guardrail 已补齐

新增边界测试：

- [phase66_provisional_vs_section_retry_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase66_provisional_vs_section_retry_action_key_mutation_boundary_test.go:1)

并同步把 `Phase 65` 的 provisional retry boundary 对齐到新的 section-retry routing 现实：

- [phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- provisional retry home 继续只做 section dispatch + provisional pair
- section retry family 继续留在新的 narrow local home
- failed retry family 继续独立
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 66` 需要证明的核心点有四个：

1. provisional-vs-section retry outward behavior 保持稳定
2. section retry family 不再直接混在 provisional retry home 里
3. failed retry / broader retry / regular-action-key layering 没有被重新搅乱
4. provisional-vs-section retry guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 66` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有转去 non-retry families

当前：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

仍然同时持有 missing-slot family 和 review-ready / section-review family。

#### 2. 它没有做更大的收官判断

这轮仍然沿着 retry-oriented 这条线继续收局部 ownership，没有在这一轮里做 broader framework completion audit。

### Residual Responsibilities Still Present

`Phase 66` 收完之后，最明显的 residual hotspot 已经更靠近 non-retry 这一侧：

- missing-slot family
- review-ready / section-review family

它们仍然共享 regular-action-key home。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit non-retry regular action-key mutation ownership

重点锚点：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

原因很直接：

- retry-oriented 这条线现在已经被压到比较清楚的粒度
- 剩下最明显的 mixed local owner 已经回到 non-retry families
- 这比继续在 retry line 为了对称性硬拆更有收益

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```

这些验证足以说明：

- provisional-vs-section retry outward behavior 保持稳定
- ownership split 已按预期落地
- guardrails 已按预期落地
- temporal 下游测试面没有被这次切片回归破坏
