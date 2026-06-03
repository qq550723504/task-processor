## Task Processor Framework Phase 66 ListingKit Provisional-vs-Section Retry Action-Key Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the provisional-vs-section retry rule split inside the provisional retry action-key mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 65` intact. Do **not** reopen preview-capability mutation ownership, do **not** reopen broader regular-action-key routing, do **not** reopen failed retry ownership, do **not** reopen action target filter clone layering, and do **not** widen into a generic rule framework.

Instead, focus on the provisional retry rules currently centered in:

- [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)

and clarify:

- whether section retry deserves its own narrower local home
- whether the provisional retry pair should remain colocated
- while keeping all current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 64` and `Phase 65` aggregate routing work
- switching attention back to non-retry families
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing strategy registries or generic mutation tables
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 65`, failed retry semantics are already isolated. The next distinct residual responsibility inside the remaining provisional retry home is the split between:

- provisional retry semantics
- section retry semantics

These still share one local owner even though they are already more specific rule families.

### Target Outcome

At the end of `Phase 66`:

- provisional-vs-section retry mutation ownership is clearer
- provisional retry home remains stable
- outward action-target behavior remains unchanged
- provisional/section retry guardrails lock the clarified layering

### Task 1: Lock current provisional retry behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - provisional retry behavior
   - section retry behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify provisional retry owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_provisional_retry_mutation.go`
- Add minimal helper file(s) only if justified

1. Move the provisional-vs-section retry split into a narrower local home if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock provisional retry ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase66_provisional_vs_section_retry_action_key_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - provisional retry home stays in the intended local place
   - section retry split remains clear
   - failed retry home remains stable
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary|TestRetryOrientedActionKeyFilterMutationBoundary|TestFailedVsProvisionalRetryActionKeyMutationBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```
