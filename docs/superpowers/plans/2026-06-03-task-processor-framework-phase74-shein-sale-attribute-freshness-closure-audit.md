## Task Processor Framework Phase 74 Plan

### Objective

Run a closure-style audit for the current SHEIN sale-attribute freshness ownership line.

### Audit Questions

1. Is the remaining base issue detection still a real mixed-owner hotspot?
2. Would another split reduce real change risk, or mainly improve symmetry?
3. Do current behavior tests and boundary tests already provide enough structural confidence to stop?

### Evidence To Review

- [shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:306)
- [shein_submit_sale_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:28)
- [shein_submit_sale_attribute_freshness_resolution_repair.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_resolution_repair.go:1)
- [shein_submit_sale_attribute_freshness_message_shape.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_message_shape.go:1)
- [phase71_shein_sale_attribute_freshness_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase71_shein_sale_attribute_freshness_boundary_test.go:1)
- [shein_submit_freshness_test.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness_test.go:1)

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinSaleAttributeFreshness.*|TestSheinSaleAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
