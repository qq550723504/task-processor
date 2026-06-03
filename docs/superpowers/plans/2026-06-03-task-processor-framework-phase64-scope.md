## Task Processor Framework Phase 64 Scope

### Candidate Hotspots

After `Phase 63`, the next plausible follow-up slices are:

1. `ListingKit retry-oriented action-key filter mutation ownership`
2. `ListingKit missing/review-ready action-key filter mutation ownership`

### Recommended Next Slice

Recommend prioritizing `ListingKit retry-oriented action-key filter mutation ownership`.

Anchor it in:

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

Specifically around:

- `retry_failed_generation`
- `inspect_failed_renderer_tasks`
- `upgrade_fallback_assets`
- `retry_provisional_slots`
- `retry_section_generation`

### Why This Slice First

Because `Phase 63` already isolated the broader regular-action-key switch into its own local home. Inside that home, the retry-oriented family is now the clearest residual semantic cluster:

- it shares retryability semantics
- it shares provisional/failed grade rewriting patterns
- it is more cohesive than mixing it immediately with missing-slot or review-ready rules

By contrast, drilling into the remaining non-retry families first would produce a weaker, less coherent next cut.

### Out Of Scope

This next slice should still avoid:

- reopening `Phase 61` through `Phase 63` aggregate routing work
- reopening preview-capability mutation ownership
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing a generic action-rule strategy framework
- HTTP/bootstrap/runtime changes

### Desired Outcome

At the end of `Phase 64`:

- retry-oriented action-key mutation ownership is clearer
- broader regular-action-key home remains stable
- outward action-target behavior remains unchanged
- guardrails lock the narrowed retry-oriented split
