## Task Processor Framework Phase 48 ListingKit Navigation Descriptor Follow-Up Read Routing Pairing Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how follow-up read routing pairs slice orchestration with the already-stable item clone home, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 47` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the descriptor clone-shape pairing split that is already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the remaining follow-up read routing pairing currently centered in:

- [generation_navigation_descriptor_followup_read_routing.go](/D:/code/task-processor/internal/listingkit/generation_navigation_descriptor_followup_read_routing.go:1)

and clarify:

- whether this home should keep direct pairing of slice orchestration and item clone dispatch
- whether a narrower local pairing seam makes ownership clearer
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 42` follow-up read item clone split
- reopening `Phase 46` clone-shape routing split
- reopening `Phase 47` clone-shape pairing split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 47`, clone-shape layering is much clearer, but the follow-up read routing home still retains the final orchestration pairing:

- follow-up read slice clone
- follow-up read item clone home dispatch

The next ownership problem is no longer “who owns clone-shape routing.” It is “should one follow-up read routing seam still directly pair slice orchestration with item clone delegation.”

### Target Outcome

At the end of `Phase 48`:

- follow-up read routing pairing ownership is clearer
- existing item clone home remains stable
- current consumer-visible clone behavior remains unchanged
- follow-up read routing guardrails lock the clarified split

### Task 1: Lock current follow-up read routing behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - follow-up read routing pairing behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit descriptor follow-up read routing behavior"
```

### Task 2: Clarify follow-up read routing owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_followup_read_routing.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct pairing if that is now the clearest home
   - or split out one narrow pairing seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_followup_read_routing.go internal/listingkit/*followup*read*routing*pairing*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor follow-up read routing pairing ownership"
```

### Task 3: Lock follow-up read routing ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase48_descriptor_followup_read_routing_pairing_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - follow-up read routing pairing stays in the final intended local place
   - existing item clone home remains delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase48_descriptor_followup_read_routing_pairing_boundary_test.go internal/listingkit/*followup*read*routing*pairing*.go <affected tests>
git commit -m "test: lock listingkit descriptor follow-up read routing pairing boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
