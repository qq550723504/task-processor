## Task Processor Framework Phase 78 Plan

### Objective

Run a closure audit for the current SHEIN attribute freshness ownership line.

### Audit Questions

1. Is the remaining evaluation home still a real mixed-owner hotspot?
2. Would another split reduce real change risk, or mostly improve symmetry?
3. Do current behavior tests and boundary tests already justify stopping here?

### Evidence To Review

- [shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:231)
- [shein_submit_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:8)
- [shein_submit_attribute_freshness_issue_state.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_issue_state.go:1)
- [shein_submit_attribute_freshness_message_shape.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_message_shape.go:1)
- [phase75_shein_attribute_freshness_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase75_shein_attribute_freshness_boundary_test.go:1)
- [shein_submit_freshness_test.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness_test.go:56)

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinAttributeFreshness.*|TestSheinAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
