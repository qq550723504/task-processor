## Task Processor Framework Phase 68 Scope

### Candidate Hotspots

After `Phase 67`, the next plausible follow-up slices are:

1. `ListingKit missing-slot action-key mutation ownership`
2. `ListingKit framework completion audit and residual hotspot review`

### Recommended Next Slice

Recommend prioritizing `ListingKit missing-slot action-key mutation ownership`.

Anchor it in:

- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)

Specifically around:

- `generate_missing_assets`
- `review_missing_slots`

### Why This Slice First

Because `Phase 67` already isolated the review-ready / section-review family away from the broader regular-action-key home. The last obvious action-key mutation family still left in that home is the missing-slot pair.

This is the cleanest remaining bounded slice before a broader completion audit becomes more valuable than another refactor phase.

### Out Of Scope

This next slice should still avoid:

- reopening `Phase 64` through `Phase 67` retry/review-ready work
- reopening preview-capability mutation ownership
- reopening clone layering
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing a generic mutation rule framework
- HTTP/bootstrap/runtime changes

### Desired Outcome

At the end of `Phase 68`:

- missing-slot mutation ownership is clearer
- retry-oriented and review-ready homes remain stable
- outward action-target behavior remains unchanged
- guardrails lock the narrowed missing-slot split
