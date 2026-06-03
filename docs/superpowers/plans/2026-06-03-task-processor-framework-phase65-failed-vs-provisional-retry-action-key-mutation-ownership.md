## Task Processor Framework Phase 65 ListingKit Failed-vs-Provisional Retry Action-Key Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the failed-vs-provisional retry rule split inside the retry-oriented action-key mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 64` intact. Do **not** reopen preview-capability mutation ownership, do **not** reopen broader regular-action-key routing, do **not** reopen action target filter clone layering, and do **not** widen into a generic rule framework.

Instead, focus on the retry-oriented rules currently centered in:

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)

and clarify:

- whether failed retry rules deserve their own narrower local home
- whether provisional retry and section retry should remain colocated or only be locally delegated
- while keeping all current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 63` and `Phase 64` aggregate routing work
- switching attention back to non-retry families
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing strategy registries or generic mutation tables
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 64`, retry-oriented rules are already isolated from non-retry families. The next distinct residual responsibility inside that home is the split between:

- failed retry semantics
- provisional retry semantics
- section retry semantics

These still share one local owner even though they are already more specific rule families.

### Target Outcome

At the end of `Phase 65`:

- failed-vs-provisional retry mutation ownership is clearer
- retry-oriented home remains stable
- outward action-target behavior remains unchanged
- retry split guardrails lock the clarified layering

### Task 1: Lock current retry-family behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - failed retry behavior
   - provisional retry behavior
   - section retry behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify retry-family owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_retry_oriented_mutation.go`
- Add minimal helper file(s) only if justified

1. Move the failed/provisional retry split into a narrower local home if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock retry-family ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase65_failed_vs_provisional_retry_action_key_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - retry-oriented home stays in the intended local place
   - failed/provisional split remains clear
   - broader regular-action-key home remains stable
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
