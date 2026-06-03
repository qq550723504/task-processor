## Task Processor Framework Phase 68 ListingKit Missing-Slot Action-Key Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the missing-slot action-key rule split inside the broader regular action-key mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 67` intact. Do **not** reopen retry-oriented ownership, do **not** reopen review-ready ownership, do **not** reopen preview-capability mutation ownership, do **not** reopen action target filter clone layering, and do **not** widen into a generic rule framework.

Instead, focus on the missing-slot rules currently centered in:

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

and clarify:

- whether `generate_missing_assets` and `review_missing_slots` deserve their own narrower local home
- whether the pair should remain colocated
- while keeping all current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 64` through `Phase 67` retry/review-ready work
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing strategy registries or generic mutation tables
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 67`, the retry-oriented side and the review-ready side are already isolated. The last obvious residual action-key mutation family still left in the broader regular action-key home is:

- `generate_missing_assets`
- `review_missing_slots`

These share one coherent missing-slot semantic cluster and now stand out as the remaining bounded ownership cut before a broader completion audit.

### Target Outcome

At the end of `Phase 68`:

- missing-slot mutation ownership is clearer
- regular-action-key home is reduced to thin routing only, or near-routing-only
- outward action-target behavior remains unchanged
- missing-slot guardrails lock the clarified layering

### Task 1: Lock current missing-slot behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - `generate_missing_assets`
   - `review_missing_slots`
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify missing-slot owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_regular_mutation.go`
- Add minimal helper file(s) only if justified

1. Move the missing-slot pair into a narrower local home if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock missing-slot ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase68_missing_slot_action_key_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - missing-slot home stays in the intended local place
   - retry-oriented and review-ready homes remain stable
   - broader regular-action-key home remains near-routing-only
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary|TestNonRetryRegularActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary|TestProvisionalVsSectionRetryActionKeyMutationBoundary|TestNonRetryRegularActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```
