## Task Processor Framework Phase 38 ListingKit Review Navigation Target Clone Aggregate Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying which clone-shaping semantics belong inside the aggregate review-navigation target clone owner, and which should remain delegated to narrower nested clone homes.

### Architecture

Keep `Phase 37` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the action-target aggregate clone split, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the aggregate review-navigation clone owner currently centered in:

- [service_generation_navigation_dispatch_helpers.go](/D:/code-task-processor/internal/listingkit/service_generation_navigation_dispatch_helpers.go:1)

and clarify:

- which parts are truly aggregate review-navigation shaping
- which parts should stay delegated to narrower nested clone homes
- how to keep outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 37` action-target aggregate clone split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 37`, action-target aggregate clone is clear, but review-navigation aggregate clone still knows too much at once:

- conditional clone
- descriptor clone
- queue/session/preview query clone
- nested action target clone

The next ownership problem is no longer “who owns action-target aggregate clone.” It is “why does one review-navigation clone seam still aggregate so many nested clone-shaping decisions.”

### Target Outcome

At the end of `Phase 38`:

- review-navigation aggregate clone ownership is clearer
- nested clone delegation becomes more explicit if justified
- current consumer-visible clone behavior remains unchanged
- review-navigation aggregate clone guardrails lock the clarified split

### Task 1: Lock current aggregate review-navigation clone behavior

**Files:**
- Modify the smallest existing test home that already covers review-navigation clone behavior, likely:
  - `internal/listingkit/generation_review_navigation_target_test.go`
  - and/or `internal/listingkit/service_generation_actions_test.go` only if needed

1. Add focused tests only if existing coverage is insufficient to lock:
   - aggregate review-navigation clone shape
   - nested pointer/slice defensive clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit review navigation clone aggregate behavior"
```

### Task 2: Clarify aggregate review-navigation clone owner

**Files:**
- Modify:
  - `internal/listingkit/service_generation_navigation_dispatch_helpers.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep aggregate home, but make nested delegation more explicit
   - or split out one narrow nested shaping seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/service_generation_navigation_dispatch_helpers.go internal/listingkit/*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit review navigation clone aggregate ownership"
```

### Task 3: Lock aggregate review-navigation clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase38_review_navigation_clone_aggregate_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - aggregate review-navigation clone home stays in the final intended local place
   - nested clone helpers remain delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase38_review_navigation_clone_aggregate_boundary_test.go internal/listingkit/*clone*.go <affected tests>
git commit -m "test: lock listingkit review navigation clone aggregate boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestGenerationReviewActionNavigationTarget.*|TestCloneAssetGenerationActionTarget.*|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
