## Task Processor Framework Phase 62 Scope

### Candidate Hotspots

After `Phase 61`, the next plausible follow-up slices are:

1. `ListingKit action target preview capability filter mutation ownership`
2. `ListingKit action target action-key switch mutation ownership`

### Recommended Next Slice

Recommend prioritizing `ListingKit action target preview capability filter mutation ownership`.

Anchor it in:

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)

Specifically around:

- `applyAssetGenerationActionFiltersMutation(...)`
- `applyAssetGenerationPreviewCapabilityFilters(...)`
- preview capability ideal-grade defaulting and render-preview toggles

### Why This Slice First

Because `Phase 61` already proved that the next hotspot is no longer cloning or aggregate routing. The most distinct residual responsibility now is preview capability specialization:

- it is semantically different from the regular action-key switch
- it has its own capability lookup path
- it owns a coherent set of mutations
- it can be split without reopening the rest of the action-key cases

By contrast, jumping directly into the whole action-key switch would broaden the cut and mix several unrelated rule families back together.

### Out Of Scope

This next slice should still avoid:

- reopening `Phase 59` and `Phase 60` filter clone layering
- reopening action target impact clone layering
- reopening queue query / retry request shared clone owners
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing a generic filter mutation framework

### Desired Outcome

At the end of `Phase 62`:

- preview capability filter mutation has a clearer local owner
- `actionFiltersForKey(...)` and the broader mutation home stay stable
- current outward action-target behavior remains unchanged
- guardrails lock the narrowed split
