## Task Processor Framework Phase 41 ListingKit Navigation Dispatch-Plan Step Clone Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics belong inside the dispatch-plan step clone owner, and which should remain delegated to narrower shared clone homes.

### Architecture

Keep `Phase 40` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the target/descriptor/dispatch-plan aggregate clone splits, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the local step clone owner currently centered in:

- [generation_navigation_dispatch_plan_clone_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_dispatch_plan_clone_shape.go:1)

and clarify:

- which parts are truly step clone shaping
- which parts should stay delegated to narrower shared clone homes
- how to keep outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 37` action-target aggregate clone split
- reopening `Phase 38` review-navigation target aggregate clone split
- reopening `Phase 39` descriptor aggregate clone split
- reopening `Phase 40` dispatch-plan aggregate clone split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 40`, aggregate clone owners are clearer, but dispatch-plan step clone still knows too much at once:

- step slice clone
- step-level query clone

The next ownership problem is no longer “who owns dispatch-plan aggregate clone.” It is “why does one local step clone seam still aggregate multiple nested clone-shaping decisions.”

### Target Outcome

At the end of `Phase 41`:

- dispatch-plan step clone ownership is clearer
- shared queue-query clone delegation remains explicit
- current consumer-visible clone behavior remains unchanged
- step clone guardrails lock the clarified split

### Task 1: Lock current step clone behavior

**Files:**
- Modify the smallest existing test home that already covers dispatch-plan step clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - step clone shape
   - query defensive clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit navigation dispatch plan step clone behavior"
```

### Task 2: Clarify step clone owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_dispatch_plan_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep step clone home, but make shared query clone delegation more explicit
   - or split out one narrow step-local shaping seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_dispatch_plan_clone_shape.go internal/listingkit/*step*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit navigation dispatch plan step clone ownership"
```

### Task 3: Lock step clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase41_navigation_dispatch_plan_step_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - step clone home stays in the final intended local place
   - shared queue-query clone remains delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase41_navigation_dispatch_plan_step_clone_boundary_test.go internal/listingkit/*step*clone*.go <affected tests>
git commit -m "test: lock listingkit navigation dispatch plan step clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
