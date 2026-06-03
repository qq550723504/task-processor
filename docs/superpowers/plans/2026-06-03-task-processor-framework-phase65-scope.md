## Task Processor Framework Phase 65 Scope

### Candidate Hotspots

After `Phase 64`, the next plausible follow-up slices are:

1. `ListingKit failed-vs-provisional retry action-key mutation ownership`
2. `ListingKit non-retry regular action-key mutation ownership`

### Recommended Next Slice

Recommend prioritizing `ListingKit failed-vs-provisional retry action-key mutation ownership`.

Anchor it in:

- [generation_action_filters_retry_oriented_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_retry_oriented_mutation.go:1)

Specifically around:

- `retry_failed_generation`
- `inspect_failed_renderer_tasks`
- `upgrade_fallback_assets`
- `retry_provisional_slots`
- `retry_section_generation`

### Why This Slice First

Because `Phase 64` already isolated the broader retry-oriented family into its own local home. Inside that home, the next clearest residual split is between:

- failed retry semantics
- provisional/section retry semantics

That is a tighter and more coherent next cut than immediately switching attention back to the remaining non-retry families.

### Out Of Scope

This next slice should still avoid:

- reopening `Phase 62` through `Phase 64` aggregate routing work
- reopening preview-capability mutation ownership
- reopening clone layering
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing a generic mutation rule framework
- HTTP/bootstrap/runtime changes

### Desired Outcome

At the end of `Phase 65`:

- failed-vs-provisional retry mutation ownership is clearer
- retry-oriented home remains stable
- outward action-target behavior remains unchanged
- guardrails lock the narrowed retry split
