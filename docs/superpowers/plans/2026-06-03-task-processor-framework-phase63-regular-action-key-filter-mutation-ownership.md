## Task Processor Framework Phase 63 ListingKit Regular Action-Key Filter Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the regular action-key filter mutation rules inside the broader action-target mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 62` intact. Do **not** reopen action target filter clone layering, do **not** reopen action target impact clone layering, do **not** reopen shared queue/retry clone owners, and do **not** widen into a generic mutation framework.

Instead, focus on the regular action-key mutation rules currently centered in:

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

and clarify:

- whether the broader switch deserves one narrower local home for regular action-key routing
- whether rule families should remain colocated or only be locally delegated
- while keeping current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 61` and `Phase 62` aggregate and preview-capability work
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing an action-key registry or strategy framework
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 62`, preview capability specialization is already isolated. The next distinct residual responsibility is the regular action-key mutation switch:

- missing-slot style rules
- failed/provisional retry rules
- review-ready and section-review rules

These rule families are semantically different, but they still share the same broader local owner.

### Target Outcome

At the end of `Phase 63`:

- regular action-key mutation ownership is clearer
- preview-capability mutation home remains stable
- outward action-target behavior remains unchanged
- regular-switch mutation guardrails lock the clarified split

### Task 1: Lock current regular action-key mutation behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - missing/provisional/review-ready action-key mutation behavior
   - defensive clone semantics already relied upon here
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify regular action-key mutation owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_mutation.go`
- Add minimal helper file(s) only if justified

1. Move the regular action-key switch into a narrower local home if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock regular action-key mutation ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase63_regular_action_key_filter_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - regular action-key mutation stays in the intended local place
   - preview-capability mutation home remains separate
   - clone homes remain stable
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
