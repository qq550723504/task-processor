## Task Processor Framework Phase 69 Scope

### Candidate Hotspots

After `Phase 68`, the next plausible follow-up directions are:

1. `ListingKit framework completion audit and residual hotspot review`
2. `ListingKit last-mile micro-ownership symmetry cleanup`

### Recommended Next Slice

Recommend prioritizing `ListingKit framework completion audit and residual hotspot review`.

### Why This Slice First

Because `Phase 61` through `Phase 68` have now isolated:

- preview-capability mutation
- retry-oriented mutation
- failed retry mutation
- provisional retry mutation
- section retry mutation
- review-ready / section-review mutation
- missing-slot mutation

and reduced the broader action-key mutation homes to routing-only or near-routing-only shape.

At this point, another mechanical decomposition phase is more likely to optimize for symmetry than to remove a real hotspot. A completion audit is now higher value:

- confirm whether any true mixed owners remain
- distinguish real residual hotspots from cosmetic asymmetry
- decide whether this refactor line is effectively done

### Out Of Scope

This next slice should still avoid:

- reopening already-stable action-key mutation homes without evidence
- redesigning `buildAssetGenerationActionTarget(...)`
- introducing generic frameworks just for consistency
- HTTP/bootstrap/runtime changes

### Desired Outcome

At the end of `Phase 69`:

- we have an evidence-backed assessment of whether the ListingKit framework refactor line is effectively complete
- any remaining real hotspots are explicitly named and ranked
- if no meaningful hotspots remain, the line can move toward closure instead of more mechanical slicing
