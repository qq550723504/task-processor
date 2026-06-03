## Task Processor Framework Phase 58 ListingKit Action Target Impact Final Slice Ownership Plan

### Goal

Reduce the next residual ownership hotspot in ListingKit by clarifying the final slice-clone ownership inside action target impact clone, while keeping outward clone behavior unchanged.

### Architecture

Keep `Phase 57` intact. Do **not** reopen the shared retry request clone layering, do **not** reopen queue query clone ownership, do **not** reopen action target aggregate routing, and do **not** widen into a generic cloning framework or an action execute orchestration redesign.

Instead, focus on the impact final slice-clone hotspot currently centered in:

- [task_generation_action_impact_slice_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_impact_slice_clone.go:1)

and clarify:

- whether `Platforms / QualityGrades / States` should keep living together in one local slice-clone home
- whether they deserve clearer final local homes
- while keeping outward clone behavior stable

### Out Of Scope For This Slice

- reopening `Phase 36` shared helper home move
- reopening `Phase 51` queue query clone split
- reopening `Phase 52` through `Phase 55` shared retry request clone layering
- reopening `Phase 56` impact aggregate split
- reopening `Phase 57` impact shape split
- reopening navigation descriptor clone layering
- redesigning action execute orchestration
- moving clone helpers into a generic utilities package
- HTTP/bootstrap/runtime changes

### Root Cause This Slice Addresses

After `Phase 57`, the action target impact clone layering is clear, but the local slice-clone home still retains a mixed final responsibility:

- `Platforms` slice clone
- `QualityGrades` slice clone
- `States` slice clone

The next ownership problem is no longer “who owns aggregate copy or shape routing.” It is “should one final local home still directly own three distinct slice-clone responsibilities.”

### Target Outcome

At the end of `Phase 58`:

- action target impact final slice ownership is clearer
- current direct consumers remain stable
- current consumer-visible clone behavior remains unchanged
- impact final slice guardrails lock the clarified split

### Task 1: Lock current action target impact final slice behavior

**Files:**
- Use the existing behavior home that already covers impact clone:
  - `internal/listingkit/service_generation_actions_test.go`

1. Add focused tests only if existing coverage is insufficient to lock:
   - impact final slice-clone behavior
   - current consumer-visible behavior
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact" -count=1
```

3. Commit:

```bash
git add <affected behavior test file(s)>
git commit -m "test: lock listingkit action target impact final slice behavior"
```

### Task 2: Clarify action target impact final slice owner

**Files:**
- Modify:
  - `internal/listingkit/task_generation_action_impact_slice_clone.go`
- Add minimal helper file(s) only if justified

1. Decide the minimal ownership move:
   - keep direct `Platforms / QualityGrades / States` final slice clone if that is now the clearest home
   - or split out narrower final local homes if that materially improves ownership
2. Preserve exact outward clone behavior
3. Re-run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact" -count=1
```

4. Commit:

```bash
git add internal/listingkit/task_generation_action_impact_slice_clone.go internal/listingkit/*impact*clone*.go <affected tests>
git commit -m "refactor: clarify listingkit action target impact final slice ownership"
```

### Task 3: Lock action target impact final slice ownership guardrails

**Files:**
- Create a new boundary test if justified, for example:
  - `internal/listingkit/phase58_action_target_impact_final_slice_boundary_test.go`
- Modify only the minimum existing boundary suite needed

1. Add ownership guardrails that lock:
   - action target impact final slice clone stays in the intended local place
   - impact aggregate and shape homes remain separate and stable
   - outward behavior remains intact
2. Run:

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

3. Commit:

```bash
git add internal/listingkit/phase58_action_target_impact_final_slice_boundary_test.go internal/listingkit/*impact*clone*.go <affected tests>
git commit -m "test: lock listingkit action target impact final slice boundaries"
```

### Expected Verification Matrix

```powershell
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact" -count=1
go test ./internal/listingkit -run "TestCloneAssetGenerationActionImpact|TestTaskGenerationAction.*Boundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
