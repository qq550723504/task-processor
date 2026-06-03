## Task Processor Framework Phase 80 Checkpoint

### Status

`Phase 80` continued the architecture-level readiness contract work by extracting a second shared seam on top of the shared summary and shared gate slices:

- shared readiness guidance resolver ownership

### Why This Is A Framework Slice

This cut is framework-level because it centralizes duplicated contract wiring used by multiple readiness builders:

- [buildSheinSubmitReadinessWithPodForAction(...)](/D:/code-task-processor/internal/listingkit/shein_submit_readiness.go:24)
- [buildSheinSubmitFreshnessReadiness(...)](/D:/code-task-processor/internal/listingkit/shein_submit_freshness.go:164)

Before this round, both builders locally owned the same closure that:

- resolved readiness guidance from `buildSheinReadinessGuidance(...)`
- cloned reason artifacts
- cloned repair-hint artifacts
- adapted them into the `sheinworkspace.Guidance[...]` contract expected by `BuildSubmitReadiness(...)`

That made the duplicated resolver closure a real shared-contract hotspot.

### What Changed

- [shein_submit_readiness_guidance.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_guidance.go:1)
  - new shared seam:
    - `buildSheinSubmitReadinessGuidanceResolver(...)`
- [shein_submit_readiness.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness.go:248)
  - standard submit readiness now delegates guidance wiring to the shared resolver seam
- [shein_submit_freshness.go](/D:/code-task-processor/internal/listingkit/shein_submit_freshness.go:168)
  - freshness readiness now delegates the same wiring to the shared resolver seam
- [phase80_submit_readiness_guidance_boundary_test.go](/D:/code-task-processor/internal/listingkit/phase80_submit_readiness_guidance_boundary_test.go:1)
  - new boundary guardrail for the shared guidance resolver seam

### Resulting Shared Contract Set

The readiness architecture now has three explicit shared seams:

1. summary post-shaping:
   - [shein_submit_readiness_summary.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_summary.go:1)
2. cross-flow readiness gating:
   - [shein_submit_readiness_gate.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_gate.go:1)
3. readiness guidance resolver wiring:
   - [shein_submit_readiness_guidance.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_guidance.go:1)

This is materially more architectural than the earlier local freshness helper slicing because these seams span:

- multiple builders
- multiple submit flows
- multiple consumers of outward readiness semantics

### Why This Matters

This reduces shared-contract drift in three places:

- readiness guidance wiring can no longer diverge between submit readiness and freshness readiness
- future readiness builders have a single resolver contract to reuse
- the readiness architecture is now easier to reason about as a small layered contract set instead of repeated local closures

### Verification Summary

Fresh verification passed:

```powershell
go test ./internal/listingkit -run "TestSheinSubmitReadinessGuidanceBoundary|TestSheinSubmitReadinessSummaryBoundary|TestSheinSubmitReadinessGateBoundary|TestShapeSheinSubmitReadinessSummary.*|TestValidateSheinSubmitReadinessGates.*|TestTaskTemporalSubmissionAdapterValidateReadinessBlocksOnFreshnessGate" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
