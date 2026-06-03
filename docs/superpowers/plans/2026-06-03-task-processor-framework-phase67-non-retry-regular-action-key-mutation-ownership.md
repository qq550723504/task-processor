## Task Processor Framework Phase 67 ListingKit Non-Retry Regular Action-Key Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the non-retry regular action-key rule split inside the broader regular action-key mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 66` intact. Do **not** reopen retry-oriented ownership, do **not** reopen preview-capability mutation ownership, do **not** reopen action target filter clone layering, and do **not** widen into a generic rule framework.

Instead, focus on the non-retry rules currently centered in:

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

and clarify:

- whether missing-slot rules deserve their own narrower local home
- whether review-ready and section-review rules should remain colocated or only be locally delegated
- while keeping all current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 64` through `Phase 66` retry-oriented work
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing strategy registries or generic mutation tables
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 66`, the retry-oriented side is already isolated to a much cleaner shape. The next distinct residual responsibility is the non-retry family still left inside the broader regular action-key home:

- missing-slot semantics
- review-ready semantics
- section-review semantics

These still share one local owner even though they are already distinct rule families.

### Target Outcome

At the end of `Phase 67`:

- non-retry regular action-key mutation ownership is clearer
- retry-oriented homes remain stable
- outward action-target behavior remains unchanged
- non-retry guardrails lock the clarified layering

### Task 1: Lock current non-retry behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - missing-slot behavior
   - review-ready / section-review behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify non-retry owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_regular_mutation.go`
- Add minimal helper file(s) only if justified

1. Move the non-retry split into a narrower local home if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock non-retry ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase67_non_retry_regular_action_key_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - non-retry home stays in the intended local place
   - retry-oriented homes remain stable
   - preview-capability home remains stable
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```
