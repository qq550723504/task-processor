## Task Processor Framework Phase 43 ListingKit Navigation Descriptor Residual Shape Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics still belong inside the descriptor residual-shape owner, and which should remain delegated to narrower existing clone homes.

### Architecture

Keep `Phase 42` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the aggregate clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the residual descriptor shape currently centered in:

- [generation_navigation_descriptor_clone_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_clone_shape.go:1)

and clarify:

- which parts are truly residual descriptor shape
- which parts should stay delegated to narrower existing clone homes
- how to keep outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 37` action-target aggregate clone split
- reopening `Phase 38` review-navigation target aggregate clone split
- reopening `Phase 39` descriptor aggregate clone split
- reopening `Phase 40` dispatch-plan aggregate clone split
- reopening `Phase 41` dispatch-plan step clone split
- reopening `Phase 42` follow-up read clone split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 42`, the descriptor shape seam no longer owns follow-up read query clone, but it still aggregates several residual shape decisions:

- conditional clone
- invalidates slice clone
- dispatch-plan clone delegation

The next ownership problem is no longer “who owns follow-up read clone.” It is “why does one descriptor shape seam still aggregate multiple remaining shape decisions.”

### Target Outcome

At the end of `Phase 43`:

- descriptor residual shape ownership is clearer
- existing nested clone home dispatch remains explicit
- current consumer-visible clone behavior remains unchanged
- descriptor residual-shape guardrails lock the clarified split

### Task 1: Lock current residual descriptor shape behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - conditional clone shape
   - invalidates slice clone
   - dispatch-plan clone delegation behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit descriptor residual shape behavior"
```

### Task 2: Clarify residual descriptor shape owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep descriptor shape home, but make residual shape delegation more explicit
   - or split out one narrow residual-shape seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_clone_shape.go internal/listingkit/*descriptor*shape*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor residual shape ownership"
```

### Task 3: Lock residual descriptor shape ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase43_descriptor_residual_shape_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - residual descriptor shape home stays in the final intended local place
   - nested clone homes remain delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase43_descriptor_residual_shape_boundary_test.go internal/listingkit/*descriptor*shape*.go <affected tests>
git commit -m "test: lock listingkit descriptor residual shape boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
