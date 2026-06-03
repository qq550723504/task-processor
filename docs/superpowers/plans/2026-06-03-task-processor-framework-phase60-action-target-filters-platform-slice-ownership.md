## Task Processor Framework Phase 60 ListingKit Action Target Filters Platform Slice Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the final `Platforms` slice ownership inside action target filters clone, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 59` intact. Do **not** reopen the shared retry request clone layering, do **not** reopen queue query clone ownership, do **not** reopen action target impact clone layering, and do **not** widen into a generic cloning framework or an action execute orchestration redesign.

Instead, focus on the filters platform-slice hotspot currently centered in:

- [generation_filters_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_filters_clone_shape.go:1)

and clarify:

- whether `Platforms` should keep living directly in one local shape home
- whether it deserves a clearer final local home
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` through `Phase 55` shared retry request clone layering
- reopening `Phase 56` through `Phase 58` impact clone layering
- reopening `Phase 59` filters aggregate split
- reopening navigation descriptor clone layering
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 59`, the action target filters clone layering is clearer, but the local shape home still retains one direct final responsibility:

- `Platforms` slice clone

The next ownership problem is no longer “who owns aggregate copy.” It is “should this last slice clone keep living directly in the shape home.”

### Target Outcome

At the end of `Phase 60`:

- action target filters platform-slice ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- filters platform-slice guardrails lock the clarified split

### Task 1: Lock current action target filters platform-slice behavior

**Files:**
- Use the existing behavior home that already covers filters clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - filters platform-slice behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit action target filters platform slice behavior"
```

### Task 2: Clarify action target filters platform-slice owner

**Files:**
- Modify:
  - `internal/listingkit/generation_filters_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct `Platforms` clone if that is now the clearest home
   - or split out a narrower final local home if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget" -count=1
```

4. Commit:

```bash
git add internal/listingkit/generation_filters_clone_shape.go internal/listingkit/*filters*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit action target filters platform slice ownership"
```

### Task 3: Lock action target filters platform-slice ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase60_action_target_filters_platform_slice_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - action target filters platform slice stays in the intended local place
   - filters aggregate home remains separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase60_action_target_filters_platform_slice_boundary_test.go internal/listingkit/*filters*clone*.go <affected tests>
git commit -m "test: lock listingkit action target filters platform slice boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionTarget|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
