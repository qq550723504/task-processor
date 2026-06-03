## Task Processor Framework Phase 75 Checkpoint

### Status

`Phase 75` opened the next real hotspot after the closure of the SHEIN sale-attribute freshness sub-line.

This round selected:

- [evaluateSheinAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:231)

as the next mixed-owner target inside `shein_submit_freshness.go`.

### Why This Became The Next Line

Discovery showed that `evaluateSheinAttributeFreshness(...)` was still directly mixing:

- template filtering
- template context build
- invalid resolved-attribute detection
- required-attribute reactivation logic
- drift-detail shaping
- outward message assembly

That made it a clear next ownership hotspot with existing behavior tests already in place.

### What Changed

- [shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:231)
  - `evaluateSheinAttributeFreshness(...)` now acts as:
    - guard
    - template-context dispatch
    - resolved-attribute evaluation dispatch
- [shein_submit_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:1)
  - new local home for:
    - `buildSheinAttributeFreshnessTemplateContext(...)`
    - `evaluateSheinResolvedAttributeFreshness(...)`
- [phase75_shein_attribute_freshness_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase75_shein_attribute_freshness_boundary_test.go:1)
  - locks the new aggregate vs evaluation ownership split

### Resulting Shape

Current shape for this new line is:

- aggregate owner:
  - `evaluateSheinAttributeFreshness(...)`
- local evaluation home:
  - `buildSheinAttributeFreshnessTemplateContext(...)`
  - `evaluateSheinResolvedAttributeFreshness(...)`

The local evaluation home still mixes:

- invalid resolved-attribute detection
- required-attribute reactivation logic
- drift-detail shaping
- outward message assembly

### Next Best Cut

The strongest next slice is to keep narrowing `evaluateSheinResolvedAttributeFreshness(...)`, especially around:

- invalid / missing-required collection
- issue aggregation and outward message shaping

### Verification Summary

Fresh verification passed:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinAttributeFreshness.*|TestSheinAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
