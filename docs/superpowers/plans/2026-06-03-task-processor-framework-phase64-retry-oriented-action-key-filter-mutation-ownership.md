## Task Processor Framework Phase 64 ListingKit Retry-Oriented Action-Key Filter Mutation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the retry-oriented action-key filter mutation rules inside the regular action-key mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 63` intact. Do **not** reopen preview-capability mutation ownership, do **not** reopen action target filter clone layering, do **not** reopen action target impact clone layering, and do **not** widen into a generic action-rule framework.

Instead, focus on the retry-oriented rules currently centered in:

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

and clarify:

- whether retry-oriented rules deserve their own narrower local home
- whether provisional/failed variants should remain colocated there
- while keeping all current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 62` preview-capability work
- reopening the broader aggregate routing from `Phase 61` and `Phase 63`
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing strategy registries or generic mutation tables
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 63`, the broader regular action-key switch is already isolated. The next distinct residual responsibility inside that home is the retry-oriented family:

- `retry_failed_generation`
- `inspect_failed_renderer_tasks`
- `upgrade_fallback_assets`
- `retry_provisional_slots`
- `retry_section_generation`

These rules share retry-oriented semantics, but they still live together with non-retry action-key families.

### Target Outcome

At the end of `Phase 64`:

- retry-oriented action-key mutation ownership is clearer
- missing/review-ready families remain stable
- outward action-target behavior remains unchanged
- retry-oriented mutation guardrails lock the clarified split

### Task 1: Lock current retry-oriented mutation behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - failed/provisional retry behavior
   - retry section behavior if current direct coverage is insufficient
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify retry-oriented mutation owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_regular_mutation.go`
- Add minimal helper file(s) only if justified

1. Move retry-oriented rules into a narrower local home if that materially improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock retry-oriented mutation ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase64_retry_oriented_action_key_filter_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - retry-oriented mutation stays in the intended local place
   - broader regular-action-key home remains stable
   - preview-capability mutation home remains separate
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary|TestActionTargetPreviewCapabilityFilterMutationBoundary|TestRegularActionKeyFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
