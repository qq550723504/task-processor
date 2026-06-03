## Task Processor Framework Phase 63 Scope

### Candidate Hotspots

After `Phase 62`, the next plausible follow-up slices are:

1. `ListingKit action target regular action-key filter mutation ownership`
2. `ListingKit ideal-review defaulting helper ownership`

### Recommended Next Slice

Recommend prioritizing `ListingKit action target regular action-key filter mutation ownership`.

Anchor it in:

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

Specifically around:

- `generate_missing_assets` / `review_missing_slots`
- `retry_failed_generation` / `inspect_failed_renderer_tasks`
- `upgrade_fallback_assets` / `retry_provisional_slots`
- `review_ready_assets` / `continue_publish_review`
- `retry_section_generation`
- `defer_section_review` / `approve_section_review`

### Why This Slice First

Because `Phase 62` already isolated preview-capability specialization into its own local home. The most obvious residual mixed owner now is the broader action-key switch itself:

- several distinct rule families still share one local home
- each family carries a coherent mutation pattern
- it is the next bounded ownership hotspot with visible structure

By contrast, drilling into `applyAssetGenerationIdealReviewFilters(...)` now would mostly optimize for symmetry, not for the next highest-value ownership split.

### Out Of Scope

This next slice should still avoid:

- reopening `Phase 59` through `Phase 62` clone and preview-capability work
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing a generic action-key mutation registry/framework
- HTTP/bootstrap/runtime changes

### Desired Outcome

At the end of `Phase 63`:

- regular action-key filter mutation ownership is clearer
- preview-capability mutation home remains stable
- outward action-target behavior remains unchanged
- guardrails lock the narrowed regular-switch split
