## Task Processor Framework Phase 66 Scope

### Candidate Hotspots

After `Phase 65`, the next plausible follow-up slices are:

1. `ListingKit provisional-vs-section retry action-key mutation ownership`
2. `ListingKit non-retry regular action-key mutation ownership`

### Recommended Next Slice

Recommend prioritizing `ListingKit provisional-vs-section retry action-key mutation ownership`.

Anchor it in:

- [generation_action_filters_provisional_retry_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_provisional_retry_mutation.go:1)

Specifically around:

- `upgrade_fallback_assets`
- `retry_provisional_slots`
- `retry_section_generation`

### Why This Slice First

Because `Phase 65` already isolated failed retry semantics from the broader retry-oriented home. Inside the remaining provisional retry home, the next clearest residual split is:

- provisional retry semantics
- section retry semantics

That is a tighter and more coherent next cut than immediately switching attention back to the non-retry families.

### Out Of Scope

This next slice should still avoid:

- reopening `Phase 63` through `Phase 65` aggregate routing work
- reopening preview-capability mutation ownership
- reopening clone layering
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing a generic mutation rule framework
- HTTP/bootstrap/runtime changes

### Desired Outcome

At the end of `Phase 66`:

- provisional-vs-section retry mutation ownership is clearer
- failed retry home remains stable
- outward action-target behavior remains unchanged
- guardrails lock the narrowed provisional/section retry split
