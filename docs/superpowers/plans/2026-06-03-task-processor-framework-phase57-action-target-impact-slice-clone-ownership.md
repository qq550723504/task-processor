## Task Processor Framework Phase 57 ListingKit Action Target Impact Slice Clone Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the slice-clone ownership inside action target impact clone shaping, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 56` intact. Do **not** reopen the shared retry request clone layering, do **not** reopen queue query clone ownership, do **not** reopen action target aggregate routing, and do **not** widen into a generic cloning framework or an action execute orchestration redesign.

Instead, focus on the impact slice-clone hotspot currently centered in:

- [task_generation_action_impact_clone_shape.go](/D:/code/task-processor/internal/listingkit/task_generation_action_impact_clone_shape.go:1)

and clarify:

- whether `Platforms / QualityGrades / States` should keep living together in one local shape home
- whether they deserve a clearer slice-clone split
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` through `Phase 55` shared retry request clone layering
- reopening `Phase 56` impact aggregate split
- reopening navigation descriptor clone layering
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 56`, the action target impact clone layering is clearer, but the local shape home still retains a mixed slice-clone responsibility:

- `Platforms` slice clone
- `QualityGrades` slice clone
- `States` slice clone

The next ownership problem is no longer “who owns impact aggregate copy.” It is “should one local shape home still directly own three distinct slice-clone responsibilities.”

### Target Outcome

At the end of `Phase 57`:

- action target impact slice-clone ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- impact slice-clone guardrails lock the clarified split

### Task 1: Lock current action target impact slice-clone behavior

**Files:**
- Use the existing behavior home that already covers impact clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - impact slice-clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit action target impact slice clone behavior"
```

### Task 2: Clarify action target impact slice-clone owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_impact_clone_shape.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct `Platforms / QualityGrades / States` slice clone if that is now the clearest home
   - or split out narrower local slice-clone homes if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_action_impact_clone_shape.go internal/listingkit/*impact*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit action target impact slice clone ownership"
```

### Task 3: Lock action target impact slice-clone ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase57_action_target_impact_slice_clone_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - action target impact slice clone stays in the intended local place
   - impact aggregate home remains separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase57_action_target_impact_slice_clone_boundary_test.go internal/listingkit/*impact*clone*.go <affected tests>
git commit -m "test: lock listingkit action target impact slice clone boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
