## Task Processor Framework Phase 69 Completion Audit Checkpoint

### Status

`Phase 69` 不是新的代码拆分阶段，而是对当前 ListingKit framework 收敛线的一次 completion audit。

这轮审计的结论是：

- 当前 action-target mutation 邻域已经**实质性收口**
- 继续机械式拆 phase 的收益已经明显低于做整线总结与收官判断
- 目前没有再发现一个同等级、同收益的真实 mixed-owner hotspot，足以自然支持 `Phase 70` 继续沿同一条 mutation 线拆下去

对应审计计划文档：

- [2026-06-03-task-processor-framework-phase69-completion-audit.md](/D:/code/task-processor/docs/superpowers/plans/2026-06-03-task-processor-framework-phase69-completion-audit.md:1)

### Audit Scope

本轮重点审计了：

- [generation_overview.go](/D:/code/task-processor/internal/listingkit/generation_overview.go:1)
- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
- 全部当前 local mutation homes：
  - [generation_action_filters_preview_capability_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_preview_capability_mutation.go:1)
  - [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)
  - [generation_action_filters_failed_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_failed_retry_mutation.go:1)
  - [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)
  - [generation_action_filters_section_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_section_retry_mutation.go:1)
  - [generation_action_filters_review_ready_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_review_ready_mutation.go:1)
  - [generation_action_filters_missing_slot_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_missing_slot_mutation.go:1)
- 关联 boundary suites：
  - [phase61_action_target_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase61_action_target_filter_mutation_boundary_test.go:1)
  - [phase62_action_target_preview_capability_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase62_action_target_preview_capability_filter_mutation_boundary_test.go:1)
  - [phase63_regular_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase63_regular_action_key_filter_mutation_boundary_test.go:1)
  - [phase64_retry_oriented_action_key_filter_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase64_retry_oriented_action_key_filter_mutation_boundary_test.go:1)
  - [phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go:1)
  - [phase66_provisional_vs_section_retry_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase66_provisional_vs_section_retry_action_key_mutation_boundary_test.go:1)
  - [phase67_non_retry_regular_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase67_non_retry_regular_action_key_mutation_boundary_test.go:1)
  - [phase68_missing_slot_action_key_mutation_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase68_missing_slot_action_key_mutation_boundary_test.go:1)

### What The Audit Found

#### 1. Broader homes are now routing-only or near-routing-only

当前三个更高层的 mutation homes 分别是：

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)

它们现在的形态已经非常明确：

- top-level mutation home：`preview-capability` dispatch + `regular-action-key` dispatch
- regular-action-key home：`retry-oriented` dispatch + `review-ready` dispatch + `missing-slot` dispatch
- retry-oriented home：`failed-retry` dispatch + `provisional/section-retry` dispatch

这些 broader homes 已经不再直接夹杂 field mutation 细节，基本都已经压成 routing 层。

#### 2. Local homes are now semantically coherent

当前每个 local home 承担的责任都很单一：

- preview-capability specialization
- failed retry
- provisional retry pair
- section retry
- review-ready / section-review
- missing-slot

从 source inspection 看，当前没有再发现某个 local home 同时持有两组以上高耦合但可自然分离的职责，而且拆开后的收益能明显超过新增层级成本。

#### 3. Remaining asymmetry is mostly cosmetic, not a real hotspot

还存在的几处不完全对称，主要是这种类型：

- 某些 local homes 用 `bool` 作为 “是否消费 actionKey” 的返回约定
- `applyAssetGenerationIdealReviewFilters(...)` 作为一个共享薄 helper 被 preview/review-ready 系列共同调用

这些点当然还可以继续“做得更对称”，但在当前证据下，它们更像：

- 结构风格选择
- 小规模复用 helper
- 局部编码习惯差异

而不是一个新的、会明显增加维护成本的 mixed-owner hotspot。

#### 4. Boundary coverage is now broad enough to support a stop decision

这一轮审计前后，fresh 跑过的测试已经覆盖了：

- direct mutation behavior
- local ownership boundaries
- adjacent action-target clone behavior
- `httpapi`
- `temporal`

这说明当前“停止继续拆 phase”并不是凭感觉，而是有当前状态证据支撑的。

### What The Audit Did Not Find

本轮没有再发现一个足以自然支持 `Phase 70` 的同等级热点，比如：

- 某个 broader home 仍然直接混着多个不同 rule families
- 某个 local home 仍然直接做 routing + field mutation + clone orchestration
- 某个 shared helper 仍然同时承载多条不相干语义链

换句话说，本轮没有找到一个“如果现在不拆，后续会明显拖累维护”的剩余点。

### Practical Conclusion

当前最合理的判断是：

#### 1. 这条 ListingKit framework refactor line 已经基本完成

至少在 `action-target mutation ownership` 这一条主线上，是这样。

#### 2. 如果继续开更多 phase，风险是开始为 phase 数而拆

那种继续拆更容易是在追求：

- 命名对称
- 层级对称
- helper 颗粒度对称

而不是继续移除真实 hotspot。

#### 3. 下一步更适合做收官动作，而不是继续拆代码

更高价值的下一步应该是：

- 整线总结
- merge / integration readiness 判断
- residual risk review
- 或者切换到另一个真正还热的 hotspot

### Verification Summary

本轮审计重新 fresh 通过了：

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary|TestNonRetryRegularActionKeyMutationBoundary|TestMissingSlotActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

这些 fresh 证据支持本轮 completion audit 的判断：

- 当前代码结构已实质收口
- 没有发现新的高信号 residual hotspot
- 继续机械拆 phase 的收益已经明显下降
