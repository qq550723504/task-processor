## Task Processor Framework Phase 49 ListingKit Navigation Descriptor Follow-Up Read Slice Clone Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how follow-up read pairing owns slice orchestration separately from the already-stable item clone home, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 48` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the follow-up read routing pairing split that is already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the remaining follow-up read slice hotspot currently centered in:

- [generation_navigation_descriptor_followup_read_routing_pairing.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_followup_read_routing_pairing.go:1)

and clarify:

- whether this home should keep direct slice orchestration and item clone dispatch together
- whether slice orchestration deserves a narrower local home
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 42` follow-up read item clone split
- reopening `Phase 47` clone-shape pairing split
- reopening `Phase 48` follow-up read routing pairing split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 48`, follow-up read routing layering is much clearer, but the follow-up read pairing home still retains the final mixed ownership:

- follow-up read slice clone orchestration
- follow-up read item clone home dispatch

The next ownership problem is no longer “who owns routing pairing.” It is “should one follow-up read pairing seam still directly own both slice orchestration and item clone dispatch.”

### Target Outcome

At the end of `Phase 49`:

- follow-up read slice clone ownership is clearer
- existing item clone home remains stable
- current consumer-visible clone behavior remains unchanged
- follow-up read slice clone guardrails lock the clarified split

### Task 1: Lock current follow-up read slice clone behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - follow-up read slice clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit descriptor follow-up read slice clone behavior"
```

### Task 2: Clarify follow-up read slice clone owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_descriptor_followup_read_routing_pairing.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct slice orchestration if that is now the clearest home
   - or split out one narrow slice-clone seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_descriptor_followup_read_routing_pairing.go internal/listingkit/*followup*read*slice*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit descriptor follow-up read slice clone ownership"
```

### Task 3: Lock follow-up read slice clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase49_descriptor_followup_read_slice_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - follow-up read slice clone stays in the final intended local place
   - existing item clone home remains delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase49_descriptor_followup_read_slice_clone_boundary_test.go internal/listingkit/*followup*read*slice*clone*.go <affected tests>
git commit -m "test: lock listingkit descriptor follow-up read slice clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationDescriptor|TestCloneGenerationNavigationDispatchPlan|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
