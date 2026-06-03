## Task Processor Framework Phase 45 ListingKit Navigation Descriptor Dispatch-Plan Delegation Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how descriptor residual shape delegates dispatch-plan clone ownership, while keeping dispatch-plan clone behavior and its existing local homes stable.

### Architecture

Keep `Phase 44` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the aggregate clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the remaining descriptor residual dispatch-plan delegation currently centered in:

- [generation_navigation_descriptor_residual_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_residual_shape.go:1)

and clarify:

- whether the residual shape home should keep direct dispatch-plan delegation
- or whether a narrower local delegation seam makes ownership clearer
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 37` action-target aggregate clone split
- reopening `Phase 38` review-navigation target aggregate clone split
- reopening `Phase 39` descriptor aggregate clone split
- reopening `Phase 40` dispatch-plan aggregate clone split
- reopening `Phase 41` dispatch-plan step clone split
- reopening `Phase 42` follow-up read clone split
- reopening `Phase 43` residual shape split
- reopening `Phase 44` residual pairing split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 44`, the residual pairing has been isolated, but the descriptor residual-shape home still retains one visible delegation decision:

- dispatch-plan clone delegation

The next ownership problem is no longer “who owns residual pairing.” It is “should the descriptor residual-shape home still directly own dispatch-plan delegation, or should that seam be made more explicit.”

### Target Outcome

At the end of `Phase 45`:

- descriptor residual dispatch-plan delegation ownership is clearer
- dispatch-plan clone home remains stable
- current consumer-visible clone behavior remains unchanged
- dispatch-plan delegation guardrails lock the clarified split

### Task 1: Lock current dispatch-plan delegation behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - dispatch-plan clone delegation behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit descriptor dispatch plan delegation behavior"
```

### Task 2: Clarify dispatch-plan delegation owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_residual_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct delegation if that is now the clearest home
   - or split out one narrow delegation seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_residual_shape.go internal/listingkit/*dispatch*delegation*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor dispatch plan delegation ownership"
```

### Task 3: Lock dispatch-plan delegation ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase45_descriptor_dispatch_plan_delegation_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - descriptor residual dispatch-plan delegation stays in the final intended local place
   - dispatch-plan clone home remains delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase45_descriptor_dispatch_plan_delegation_boundary_test.go internal/listingkit/*dispatch*delegation*.go <affected tests>
git commit -m "test: lock listingkit descriptor dispatch plan delegation boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
