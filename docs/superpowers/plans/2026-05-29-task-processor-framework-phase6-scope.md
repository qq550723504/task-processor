# Task Processor Framework Phase 6 Scope Recommendation

## Recommendation

`Phase 6` should focus on **ListingKit platform finalization pressure**, not on introducing a broader workflow execution-context model yet.

The highest-value next hotspot is:

- continuing to narrow [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)
- separating platform post-processing, deferred asset dispatch, and summary/finalization concerns inside the finalization seam

In short:

- `Phase 5B` already made workflow branching explicit enough for now
- `Phase 6` should reduce the biggest remaining behavior hotspot inside the new finalization seam

## Why This Is The Right Next Step

After `Phase 5B`, the biggest remaining asymmetry is no longer:

- canonical acquisition ownership
- media/SDS branching ownership
- asset planning ownership
- public workflow entry readability

Those four are now on explicit phase seams.

The biggest remaining asymmetry is that one seam now carries too many downstream responsibilities:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

This seam currently decides, in one place:

1. SHEIN post-assembly optimization
2. default pricing shaping
3. SDS official-image overlay
4. studio AI image overlay
5. review decoration and workflow issues
6. variant image coverage guards
7. deferred platform asset generation dispatch
8. generation-task persistence
9. summary finalization and preview sync

That is the next root-cause hotspot because it keeps several different categories of behavior coupled behind one entry:

- platform-specific content shaping
- asset-generation completion behavior
- result-summary finalization

The code is still much better than before `Phase 5B`, but responsibility is now concentrated in one new seam.

## Current Hotspot

The main hotspot is:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)

This file is not large by line count alone, but its `run(...)` signature and behavior surface show concentrated pressure:

- it takes `snapshot`, `recipesByPlatform`, `generationPlan`, `inventory`, `persistedGenerationTasks`, `enableAssetGeneration`, and `sdsOptions`
- it mixes platform content shaping with asset-dispatch side effects
- it also owns summary finalization and final logging

That is a stronger signal than “it still has 100+ lines.” The real problem is mixed behavior ownership inside one execution seam.

## Candidate Phase 6 Directions

There are two realistic directions from the current branch state.

### Option 1: Platform finalization decomposition

Keep the work feature-owned inside ListingKit and split `workflow_platform_finalize_phase.go` along its real behavior boundaries.

This would likely mean:

- one bounded collaborator or helper for SHEIN/platform post-processing
- one bounded collaborator or helper for deferred asset dispatch and generation-task persistence
- one bounded collaborator or helper for final summary/preview synchronization
- a thinner `platformFinalizePhase.run(...)` that becomes orchestration rather than the primary home of all three concerns

**Pros**

- directly targets the hottest remaining workflow seam
- follows the same bounded-seam pattern that already worked in `Phase 4B`, `Phase 5A`, and `Phase 5B`
- lowers future change risk in the part of ListingKit most likely to keep evolving
- avoids speculative new workflow abstractions

**Cons**

- needs careful staging because finalization behavior is broad and touches multiple downstream helpers
- can easily drift into cosmetic extraction if not driven by real responsibility lines

**Recommendation:** `Yes`

This is the best `Phase 6` target.

### Option 2: Shared workflow execution-context model

Introduce a richer shared context object across canonical/media/asset/finalize seams so fewer values are threaded through signatures.

This would likely mean:

- replacing several explicit parameters with a mutable workflow context struct
- centralizing execution-stage data such as `inventory`, `generationPlan`, `sdsOptions`, and `snapshot`
- possibly reworking `standardWorkflowState` into a broader multi-phase context

**Pros**

- can reduce parameter lists
- may look architecturally cleaner
- could help if multiple seams begin sharing more stage data soon

**Cons**

- currently more speculative than the finalization hotspot
- risks hiding data flow that is currently still readable and explicit
- can create a “god context” before a second wave of shared-pressure actually appears

**Recommendation:** `Not yet`

This should wait until shared context pressure appears across multiple seams, not just because one seam takes many parameters.

## Why Not Introduce Execution Context Immediately

`Phase 5B` already solved the real workflow branching root cause:

- canonical acquisition no longer lives inline in `workflow_standard.go`
- media/SDS branching no longer lives inline in `workflow_standard.go`
- asset planning no longer lives inline in `workflow_standard.go`
- platform finalization no longer lives inline in `workflow_platform_adaptation.go`

What remains is mostly:

- one heavy downstream finalization seam
- explicit parameter passing between seams
- potential future reuse pressure

Only the first item is a first-order problem today.

By contrast, a broader execution-context model is still a second-order design concern. The current parameter flow is verbose, but it is explicit and still bounded inside ListingKit.

## Suggested Phase 6 Goal

The concrete `Phase 6` goal should be:

> Split ListingKit platform finalization into smaller feature-owned responsibilities so `workflow_platform_finalize_phase.go` stops being the primary home of platform post-processing, deferred asset dispatch, and result-summary finalization at the same time.

That goal is specific enough to implement incrementally and narrow enough to keep the work anchored in one real hotspot.

## Suggested Phase 6 Success Criteria

`Phase 6` should be considered successful when:

1. `workflow_platform_finalize_phase.go` is materially thinner or reduced to orchestration
2. platform post-processing and deferred asset-dispatch logic no longer live in the same primary method body
3. summary finalization and preview sync are clearly owned by a narrower collaborator or helper
4. existing workflow asset/finalization tests still pass unchanged in behavior
5. no generic cross-workflow context abstraction is introduced unless a second concrete pressure point appears during implementation

## Suggested Non-Goals For Phase 6

To keep the next slice disciplined, `Phase 6` should explicitly avoid:

- introducing a repo-wide workflow engine
- replacing `standardWorkflowState` with a universal execution context
- redesigning canonical/media/asset seam contracts unless finalization work proves it necessary
- moving workflow concerns into HTTP/runtime/bootstrap layers
- changing asset generation or SHEIN review business semantics

## Expected File Hotspots

If we take the recommended direction, the first likely hotspots are:

- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)
- [internal/listingkit/workflow_platform_adaptation.go](/D:/code/task-processor/internal/listingkit/workflow_platform_adaptation.go:1)
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
- [internal/listingkit/phase5b_workflow_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase5b_workflow_boundary_test.go:1)

Possible new files, if the split is warranted, would likely stay feature-local, for example:

- `internal/listingkit/workflow_platform_postprocess_phase.go`
- `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- `internal/listingkit/workflow_platform_summary_phase.go`

The design pressure should be:

- thinner finalization orchestration
- explicit feature-owned sub-seams
- no speculative context framework

## Recommendation Summary

Proceed to `Phase 6`, but scope it narrowly:

- choose **ListingKit platform finalization pressure** as the next hotspot
- defer a broader execution-context model until multiple seams show real shared-state pressure
- keep the work feature-owned inside ListingKit

That keeps the next slice aligned with actual behavior concentration in the codebase rather than abstraction symmetry for its own sake.
