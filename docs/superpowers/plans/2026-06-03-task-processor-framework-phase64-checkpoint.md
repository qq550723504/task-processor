## Task Processor Framework Phase 64 Checkpoint

### Status

`Phase 64` 已按计划完成，可以作为当前 ListingKit framework 收敛工作的有效 checkpoint。

本阶段严格落在既定范围内：

- 它完成了 `ListingKit retry-oriented action-key filter mutation ownership` 这条切片
- 它没有回头重开 preview-capability mutation ownership
- 它没有回头重开 broader aggregate routing from `Phase 61` and `Phase 63`
- 它没有回头重开 action target filter clone layering
- 它没有引入 generic action-rule registry/framework
- 它没有扩大成 broader action execute redesign

对应计划文档：

- [2026-06-03-task-processor-framework-phase64-retry-oriented-action-key-filter-mutation-ownership.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase64-retry-oriented-action-key-filter-mutation-ownership.md:1)

### What Landed

#### 1. Retry-oriented mutation outward behavior 已有直接夹具

这一轮继续补强了 direct behavior fixture：

- [generation_overview_test.go](/D:/code/task-processor/internal/listingkit/generation_overview_test.go:1)

当前直接锁住了：

- preview-capability mutation
- missing-action mutation
- failed-retry mutation
- retry-section mutation
- review-only mutation preserving existing grade
- defensive clone semantics

这让 retry-oriented family 的继续拆分不再只能依赖间接 action-target 回归。

#### 2. Retry-oriented rules 已有单独 local owner

当前 split 已经更清楚：

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
  - 现在只负责：
  - retry-oriented home dispatch
  - missing-slot family
  - review-ready / section-review family

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)
  - 现在单独负责：
  - `retry_failed_generation`
  - `inspect_failed_renderer_tasks`
  - `upgrade_fallback_assets`
  - `retry_provisional_slots`
  - `retry_section_generation`

这意味着 retry-oriented rule family 不再继续和 non-retry families 共处同一个 local owner。

对应提交：

- `refactor: clarify listingkit retry-oriented action-key filter mutation ownership`

#### 3. Regular-action-key home 被完整保留

这一轮没有动：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

也没有回流污染：

- preview-capability mutation home
- action target filter clone layering
- action target impact clone layering
- shared queue / retry clone homes

这让 `Phase 63` 刚收下来的 regular-action-key split 保持稳定，没有因为继续拆 retry-oriented family 又被重新搅乱。

#### 4. Retry-oriented mutation guardrail 已补齐

新增边界测试：

- [phase64_retry_oriented_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase64_retry_oriented_action_key_filter_mutation_boundary_test.go:1)

并同步把 `Phase 63` 的 regular-action-key boundary 对齐到新的 retry-oriented routing 现实：

- [phase63_regular_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase63_regular_action_key_filter_mutation_boundary_test.go:1)

当前 guardrail 锁住了 4 件事：

- regular-action-key home 继续只做 retry-oriented dispatch + non-retry families
- retry-oriented family 继续留在新的 narrow local home
- preview-capability mutation home 继续独立
- outward behavior 继续保持稳定

### Acceptance Check

`Phase 64` 需要证明的核心点有四个：

1. retry-oriented mutation outward behavior 保持稳定
2. retry-oriented family 不再直接混在 regular-action-key home 里
3. regular-action-key / preview-capability / clone layering 没有被重新搅乱
4. retry-oriented mutation guardrails 已把新 split 钉住

这四件事现在都成立。

因此，`Phase 64` 可以明确认定为已完成。

### What This Phase Did Not Try To Solve

#### 1. 它没有继续深挖 retry-oriented family 内部的 failed/provisional pairing

当前：

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)

仍然同时持有 failed-retry、provisional retry、section retry 这几种更细 family。

#### 2. 它没有继续深挖 non-retry families

missing-slot 和 review-ready / section-review 这两组 non-retry rules 仍然保留在 regular-action-key home，本轮没有为了对称性继续拆它们。

### Residual Responsibilities Still Present

`Phase 64` 收完之后，最明显的 residual hotspot 已经分成两种候选：

- retry-oriented home 内部的 failed/provisional/section retry family
- regular-action-key home 里剩下的 non-retry families

其中更自然的下一刀是 retry-oriented home 内部，因为它仍然保留着更强的语义聚合。

### What Should Move To The Next Phase

下一阶段最值得推进的是：

#### 1. ListingKit failed-vs-provisional retry action-key mutation ownership

重点锚点：

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)

原因很直接：

- failed retry rules 和 provisional retry rules 已经形成清晰的下一层语义分组
- 它比马上转去 non-retry family 更像一个 bounded、收益清晰的小切片
- 先把 retry-oriented home 内部再收一层，会让这条线更顺地收尾

### Verification Summary

本阶段已通过的验证如下：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
go test ./internal/listingkit/temporal -count=1
```

说明：

- 这轮 `httpapi + temporal` 联合验证第一次运行时，`temporal` 包出现了一次瞬时失败，表现为 `PublishWorkflow` 相关测试报 potential deadlock 和 failure phase 为空
- 在同一工作区、零代码改动前提下 fresh 单跑 `go test ./internal/listingkit/temporal -count=1` 后恢复通过
- 因为本阶段没有修改 `internal/listingkit/temporal`，而且单跑立即恢复，当前证据更支持一次瞬时波动，而不是由这次切片引入的稳定回归

这些验证足以说明：

- retry-oriented mutation outward behavior 保持稳定
- ownership split 已按预期落地
- guardrails 已按预期落地
- 本阶段切片没有证据表明对 temporal 下游造成稳定回归
