## Task Processor Framework Phase 46 ListingKit Navigation Descriptor Clone Shape Routing Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how descriptor clone-shape orchestration routes into the already-stable local clone homes, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 45` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the content-level clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the remaining clone-shape orchestration currently centered in:

- [generation_navigation_descriptor_clone_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_clone_shape.go:1)

and clarify:

- whether this home should keep direct follow-up read slice orchestration
- whether a narrower local routing seam makes ownership clearer
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
- reopening `Phase 45` dispatch-plan delegation split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 45`, the content-level ownership is much clearer, but the descriptor clone-shape seam still retains orchestration knowledge:

- residual shape dispatch
- follow-up read slice clone
- follow-up read clone home dispatch

The next ownership problem is no longer “who owns residual content.” It is “should one clone-shape seam still directly orchestrate multiple local clone homes.”

### Target Outcome

At the end of `Phase 46`:

- descriptor clone-shape routing ownership is clearer
- existing local clone homes remain stable
- current consumer-visible clone behavior remains unchanged
- clone-shape routing guardrails lock the clarified split

### Task 1: Lock current clone-shape routing behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - clone-shape routing behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit descriptor clone shape routing behavior"
```

### Task 2: Clarify clone-shape routing owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct orchestration if that is now the clearest home
   - or split out one narrow routing seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_clone_shape.go internal/listingkit/*clone*shape*routing*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor clone shape routing ownership"
```

### Task 3: Lock clone-shape routing ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase46_descriptor_clone_shape_routing_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - descriptor clone-shape routing stays in the final intended local place
   - existing local clone homes remain delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase46_descriptor_clone_shape_routing_boundary_test.go internal/listingkit/*clone*shape*routing*.go <affected tests>
git commit -m "test: lock listingkit descriptor clone shape routing boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
