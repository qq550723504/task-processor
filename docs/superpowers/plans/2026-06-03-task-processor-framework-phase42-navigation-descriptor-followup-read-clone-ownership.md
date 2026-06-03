## Task Processor Framework Phase 42 ListingKit Navigation Descriptor Follow-Up Read Clone Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics belong inside the descriptor follow-up read clone owner, and which should remain delegated to narrower shared clone homes.

### Architecture

Keep `Phase 41` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the aggregate clone splits that are already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the local follow-up read clone owner currently centered in:

- [generation_navigation_descriptor_clone_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_clone_shape.go:1)

and clarify:

- which parts are truly follow-up read clone shaping
- which parts should stay delegated to narrower shared clone homes
- how to keep outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 37` action-target aggregate clone split
- reopening `Phase 38` review-navigation target aggregate clone split
- reopening `Phase 39` descriptor aggregate clone split
- reopening `Phase 40` dispatch-plan aggregate clone split
- reopening `Phase 41` dispatch-plan step clone split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 41`, aggregate clone owners and dispatch-plan step clone are clearer, but descriptor follow-up read clone still knows too much at once:

- follow-up read slice clone
- nested query clone

The next ownership problem is no longer “who owns dispatch-plan step clone.” It is “why does one local descriptor shape seam still aggregate multiple nested clone-shaping decisions.”

### Target Outcome

At the end of `Phase 42`:

- descriptor follow-up read clone ownership is clearer
- shared queue-query clone delegation remains explicit
- current consumer-visible clone behavior remains unchanged
- follow-up read clone guardrails lock the clarified split

### Task 1: Lock current follow-up read clone behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - follow-up read clone shape
   - query defensive clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit descriptor follow-up read clone behavior"
```

### Task 2: Clarify follow-up read clone owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep descriptor shape home, but make follow-up read clone delegation more explicit
   - or split out one narrow read-local shaping seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_clone_shape.go internal/listingkit/*followup*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor follow-up read clone ownership"
```

### Task 3: Lock follow-up read clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase42_descriptor_followup_read_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - follow-up read clone home stays in the final intended local place
   - shared queue-query clone remains delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase42_descriptor_followup_read_clone_boundary_test.go internal/listingkit/*followup*clone*.go <affected tests>
git commit -m "test: lock listingkit descriptor follow-up read clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
