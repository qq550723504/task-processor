## Task Processor Framework Phase 72 Plan

### Objective

Clarify `shein sale attribute freshness resolution ownership` by shrinking:

- [evaluateSheinSaleAttributeFreshnessResolution(...)](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:30)

### Proposed Cut

Prefer extracting one of these cohesive homes first:

1. invalid-sale-attribute collection and repair retry routing
2. issue aggregation and outward message shaping

The exact choice should favor the smallest cut that most clearly removes mixed ownership while preserving existing freshness behavior tests.

### Guardrails

- keep outward readiness messages unchanged unless an existing test already proves a better invariant
- preserve current repair semantics for custom sale-attribute validation
- avoid introducing generic abstractions; use local homes with explicit freshness naming
- add boundary coverage for the new split

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinSaleAttributeFreshness.*|TestSheinSaleAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/temporal -count=1
```
