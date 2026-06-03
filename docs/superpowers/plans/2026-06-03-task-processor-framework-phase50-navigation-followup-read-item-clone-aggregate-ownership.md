## Task Processor Framework Phase 50 ListingKit Navigation Follow-Up Read Item Clone Aggregate Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying how follow-up read item clone owns top-level field copy separately from the already-stable shared queue query clone helper, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 49` intact. Do **not** reopen the shared clone helper home move, do **not** reopen the follow-up read slice clone split that is already stable, and do **not** widen into a generic cloning framework or a navigation dispatch redesign.

Instead, focus on the remaining follow-up read item hotspot currently centered in:

- [generation_navigation_followup_read_clone.go](/D:/code-task-processor/internal/listingkit/generation_navigation_followup_read_clone.go:1)

and clarify:

- whether this home should keep direct top-level field copy and query clone delegation together
- whether nested query delegation deserves a narrower local aggregate split
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 42` follow-up read item clone split itself
- reopening `Phase 48` follow-up read routing pairing split
- reopening `Phase 49` follow-up read slice clone split
- redesigning navigation dispatch execution flow
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 49`, follow-up read slice layering is much clearer, but the follow-up read item clone home still retains the final mixed aggregate ownership:

- top-level follow-up read field copy
- nested queue query clone delegation

The next ownership problem is no longer “who owns slice orchestration.” It is “should one follow-up read item clone seam still directly own both top-level field copy and nested query clone delegation.”

### Target Outcome

At the end of `Phase 50`:

- follow-up read item clone aggregate ownership is clearer
- shared queue query clone helper remains stable
- current consumer-visible clone behavior remains unchanged
- follow-up read item clone guardrails lock the clarified split

### Task 1: Lock current follow-up read item clone behavior

**Files:**
- Modify the smallest existing test home that already covers follow-up read clone behavior:
  - `internal/listingkit/generation_navigation_descriptor_clone_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - follow-up read item clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationFollowUpRead|TestCloneGenerationQueueQuery.*" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit follow-up read item clone behavior"
```

### Task 2: Clarify follow-up read item clone owner

**Files:**
- Modify:
  - `internal/listingkit/generation_navigation_followup_read_clone.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct aggregate ownership if that is now the clearest home
   - or split out one narrow aggregate seam if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationFollowUpRead|TestCloneGenerationQueueQuery.*" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_navigation_followup_read_clone.go internal/listingkit/*followup*read*item*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit follow-up read item clone aggregate ownership"
```

### Task 3: Lock follow-up read item clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase50_followup_read_item_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - follow-up read item clone stays in the final intended local place
   - shared queue query clone helper remains delegated rather than re-inlined
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationFollowUpRead|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase50_followup_read_item_clone_boundary_test.go internal/listingkit/*followup*read*item*clone*.go <affected tests>
git commit -m "test: lock listingkit follow-up read item clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneGenerationNavigationFollowUpRead|TestCloneGenerationQueueQuery.*" -count=1
go test ./internal/listingkit -run "TestCloneGenerationNavigationFollowUpRead|TestCloneGenerationQueueQuery.*|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
