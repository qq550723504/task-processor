## Task Processor Framework Phase 39 ListingKit Navigation Descriptor Clone Aggregate Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics belong inside the aggregate navigation descriptor clone owner, and which should remain delegated to narrower nested clone homes.

### Architecture

Keep `Phase 38` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the action-target aggregate clone split, do **not** reopen the review-navigation target aggregate clone split, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the aggregate descriptor clone owner currently centered in:

- [generation_navigation_target_conditional.go](/D:/code-task-processor/internal/listingkit/generation_navigation_target_conditional.go:1)

and clarify:

- which parts are truly aggregate descriptor shaping
- which parts should stay delegated to narrower nested clone homes
- how to keep outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 37` action-target aggregate clone split
- reopening `Phase 38` review-navigation target aggregate clone split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 38`, target-level aggregate clones are clear, but descriptor aggregate clone still knows too much at once:

- conditional clone
- dispatch plan clone
- invalidates slice clone
- follow-up reads clone

The next ownership problem is no longer “who owns review-navigation target aggregate clone.” It is “why does one navigation descriptor clone seam still aggregate so many nested clone-shaping decisions.”

### Target Outcome

At the end of `Phase 39`:

- navigation descriptor aggregate clone ownership is clearer
- nested clone delegation becomes more explicit if justified
- current consumer-visible clone behavior remains unchanged
- descriptor aggregate clone guardrails lock the clarified split

### Task 1: Lock current aggregate descriptor clone behavior

**Files:**
- Modify the smallest existing test home that already covers descriptor clone behavior, likely:
  - `internal/listingkit/generation_review_navigation_target_test.go`
  - and/or `internal/listingkit/service_generation_actions_test.go` only if needed

1. Add focused tests only if existing coverage is insufficient to lock:
   - aggregate descriptor clone shape
   - nested pointer/slice defensive clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestCloneGenerationQueueQuery.*|TestCloneGenerationNavigationDispatchPlan.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit navigation descriptor clone aggregate behavior"
```

### Task 2: Clarify aggregate descriptor clone owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_target_conditional.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep aggregate home, but make nested delegation more explicit
   - or split out one narrow nested shaping seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestCloneGenerationQueueQuery.*|TestCloneGenerationNavigationDispatchPlan.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_target_conditional.go internal/listingkit/*descriptor*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit navigation descriptor clone aggregate ownership"
```

### Task 3: Lock aggregate descriptor clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase39_navigation_descriptor_clone_aggregate_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - aggregate descriptor clone home stays in the final intended local place
   - nested clone helpers remain delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestCloneGenerationQueueQuery.*|TestCloneGenerationNavigationDispatchPlan.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase39_navigation_descriptor_clone_aggregate_boundary_test.go internal/listingkit/*descriptor*clone*.go <affected tests>
git commit -m "test: lock listingkit navigation descriptor clone aggregate boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestCloneGenerationQueueQuery.*|TestCloneGenerationNavigationDispatchPlan.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationReviewNavigationTarget|TestCloneGenerationQueueQuery.*|TestCloneGenerationNavigationDispatchPlan.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
