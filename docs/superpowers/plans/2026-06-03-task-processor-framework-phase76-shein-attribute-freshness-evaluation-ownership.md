## Task Processor Framework Phase 76 Plan

### Objective

Clarify `shein attribute freshness evaluation ownership` by shrinking:

- [evaluateSheinResolvedAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:38)

### Proposed Cut

Prefer extracting one of these cohesive homes first:

1. invalid and missing-required collection
2. issue aggregation and outward message shaping

The chosen cut should stay small, preserve current test-proven messages, and avoid generic abstractions.

### Guardrails

- keep outward attribute-freshness messages unchanged unless existing tests already prove a better invariant
- preserve current required-attribute reactivation semantics
- keep explicit SHEIN freshness naming in any new local home
- add boundary coverage for the new ownership split

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinAttributeFreshness.*|TestSheinAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
