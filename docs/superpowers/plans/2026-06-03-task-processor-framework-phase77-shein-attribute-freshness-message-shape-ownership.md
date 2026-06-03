## Task Processor Framework Phase 77 Plan

### Objective

Clarify `shein attribute freshness message shape ownership` by shrinking:

- [evaluateSheinResolvedAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:12)

### Proposed Cut

Prefer extracting one cohesive local home around:

1. drift-detail shaping and outward issue/message assembly

Possible residual dispatch can stay in the evaluation home if that keeps the cut small and semantically clear.

### Guardrails

- keep current attribute-freshness messages stable unless existing tests already lock a different invariant
- preserve required-attribute reactivation behavior
- use explicit SHEIN freshness naming instead of generic formatter abstractions
- extend boundary coverage for the new ownership split

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinAttributeFreshness.*|TestSheinAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
