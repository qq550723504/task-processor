## Task Processor Framework Phase 73 Plan

### Objective

Clarify `shein sale attribute freshness message shape ownership` by shrinking:

- [evaluateSheinSaleAttributeFreshnessResolution(...)](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:30)

### Proposed Cut

Prefer extracting one cohesive local home around:

1. issue aggregation and outward message shaping

Possible residual base checks can stay temporarily if that keeps the cut small and semantically clear.

### Guardrails

- keep current freshness messages stable unless existing tests already lock a different invariant
- preserve custom-repair success and failure semantics
- use explicit SHEIN freshness naming instead of generic “formatter” abstractions
- extend boundary coverage for the new ownership split

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinSaleAttributeFreshness.*|TestSheinSaleAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
