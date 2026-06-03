## Task Processor Framework Phase 69 ListingKit Framework Completion Audit Plan

### Goal

Determine, with current-state evidence, whether the ListingKit framework refactor line has reached a practical completion point or whether real residual ownership hotspots still justify additional phases.

### Architecture

Do **not** start from the assumption that another refactor phase is needed.

Instead:

1. inspect the current mutation and clone homes
2. verify which broader homes are now routing-only or near-routing-only
3. identify any residual mixed owners that still create real maintenance risk
4. separate true hotspots from cosmetic symmetry gaps

### Out Of Scope For This Slice

- speculative decomposition done only for elegance
- broad redesigns of stable areas
- introducing new frameworks without a demonstrated hotspot
- runtime / HTTP changes

### Audit Targets

Primary targets:

- [generation_action_filters_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_mutation.go:1)
- [generation_action_filters_regular_mutation.go](/D:/code/task-processor/internal/listingkit/generation_action_filters_regular_mutation.go:1)
- all current local mutation homes under `internal/listingkit/generation_action_filters_*`
- the associated boundary suites under `internal/listingkit/phase6*_*.go`

Secondary targets:

- remaining action-target clone helpers
- any nearby mixed-owner helpers still touched by recent boundaries

### Questions To Answer

1. Are the current broader homes now essentially routing-only?
2. Do any remaining helpers still mix multiple distinct responsibilities in a way that materially hurts maintenance?
3. Are the remaining asymmetries real risks, or only cosmetic differences?
4. Is there an evidence-backed next phase that would still pay for itself?

### Expected Outcome

At the end of `Phase 69`:

- either a short ranked list of real remaining hotspots exists
- or we conclude the line is effectively complete and ready for closure/summary

### Verification

Use fresh source inspection plus at least the currently relevant boundary and behavior suites before making any completion claim.
