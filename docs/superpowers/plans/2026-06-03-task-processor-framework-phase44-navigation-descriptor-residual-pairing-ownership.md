## Task Processor Framework Phase 44 ListingKit Navigation Descriptor Residual Pairing Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics still belong inside the descriptor residual-pairing owner, and which should remain delegated to narrower existing clone homes.

### Architecture

Keep `Phase 43` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the aggregate clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the residual pairing currently centered in:

- [generation_navigation_descriptor_residual_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_residual_shape.go:1)

and clarify:

- which parts are truly residual descriptor pairing
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
- reopening `Phase 43` residual shape split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 43`, the descriptor shape seam is cleaner, but the residual-shape owner still aggregates several remaining decisions:

- conditional clone
- invalidates slice clone
- dispatch-plan clone delegation

The next ownership problem is no longer “who owns residual shape vs follow-up read shape.” It is “why does one residual descriptor seam still aggregate multiple remaining pairing decisions.”

### Target Outcome

At the end of `Phase 44`:

- descriptor residual pairing ownership is clearer
- existing nested clone home dispatch remains explicit
- current consumer-visible clone behavior remains unchanged
- residual pairing guardrails lock the clarified split

### Task 1: Lock current residual pairing behavior

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
git commit -m "test: lock listingkit descriptor residual pairing behavior"
```

### Task 2: Clarify residual pairing owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_residual_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep residual-shape home, but make residual pairing delegation more explicit
   - or split out one narrow residual-pairing seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_residual_shape.go internal/listingkit/*descriptor*pairing*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor residual pairing ownership"
```

### Task 3: Lock residual pairing ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase44_descriptor_residual_pairing_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - residual pairing home stays in the final intended local place
   - nested clone homes remain delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase44_descriptor_residual_pairing_boundary_test.go internal/listingkit/*descriptor*pairing*.go <affected tests>
git commit -m "test: lock listingkit descriptor residual pairing boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
