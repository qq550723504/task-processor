## Task Processor Framework Phase 62 ListingKit Action Target Preview Capability Filter Mutation Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying preview capability filter mutation ownership inside the action-target filter mutation home, while keeping outward behavior unchanged.

### Architecture

Keep `Phase 61` intact. Do **not** reopen action target filters clone layering, do **not** reopen action target impact clone layering, do **not** reopen shared queue/retry clone owners, and do **not** widen into a generic mutation framework.

Instead, focus on the preview-capability specialization currently centered in:

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

and clarify:

- whether preview capability mutation deserves its own narrower local home
- whether ideal-grade defaulting for preview review stays colocated there or is only delegated
- while keeping all current consumer-visible behavior stable

### Out Of Scope For This Slice

- reopening `Phase 59` through `Phase 61` clone and mutation aggregate work
- redesigning `buildAssetGenerationActionTarget(...)`
- reworking the entire action-key switch
- introducing cross-package mutation helpers
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 61`, the broader mutation hotspot is already isolated into one local home. The next distinct residual responsibility inside that home is preview capability specialization:

- capability lookup
- preview capability assignment
- render preview toggles
- retryability reset
- ideal-grade fallback for preview review actions

That cluster is semantically distinct from the regular action-key-specific switch rules, but it still shares the same local owner.

### Target Outcome

At the end of `Phase 62`:

- preview capability mutation ownership is clearer
- regular action-key mutation rules remain stable
- outward action-target filter behavior remains unchanged
- preview-capability mutation guardrails lock the clarified split

### Task 1: Lock current preview capability mutation behavior

**Files:**
- `internal/listingkit/generation_overview_test.go`

1. Extend direct behavior coverage only as needed to lock:
   - preview capability mutation
   - ideal-grade fallback behavior
   - defensive clone semantics already relied upon here
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*" -count=1
```

### Task 2: Clarify preview capability mutation owner

**Files:**
- Modify:
  - `internal/listingkit/generation_action_filters_mutation.go`
- Add minimal helper file(s) only if justified

1. Move preview capability specialization into a narrower local home if that meaningfully improves ownership
2. Preserve exact outward behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
```

### Task 3: Lock preview capability mutation ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase62_action_target_preview_capability_filter_mutation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - preview capability mutation stays in the intended local place
   - broader mutation home remains stable
   - clone homes remain separate
2. Run:

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestActionFiltersForKey.*|TestTaskGenerationAction.*Boundary|TestActionTargetFilterMutationBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
