## Task Processor Framework Phase 73 Checkpoint

### Status

`Phase 73` continued the SHEIN freshness framework line by shrinking:

- [evaluateSheinSaleAttributeFreshnessResolution(...)](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:28)

This round extracted `issue aggregation + outward message shaping` into its own local home.

### What Changed

- [shein_submit_sale_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:28)
  - `evaluateSheinSaleAttributeFreshnessResolution(...)` now keeps:
    - base issue detection
    - invalid-state dispatch
    - message-shape dispatch
- [shein_submit_sale_attribute_freshness_message_shape.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_message_shape.go:1)
  - new local home for:
    - issue aggregation
    - invalid-state sorting
    - outward message / outcome shaping
- [phase71_shein_sale_attribute_freshness_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase71_shein_sale_attribute_freshness_boundary_test.go:1)
  - boundary coverage updated to lock the new split

### Resulting Shape

Current layering for this hotspot is now:

- aggregate owner:
  - [shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:306)
- local evaluation home:
  - [shein_submit_sale_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:28)
- invalid-state / repair home:
  - [shein_submit_sale_attribute_freshness_resolution_repair.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_resolution_repair.go:1)
- message-shape home:
  - [shein_submit_sale_attribute_freshness_message_shape.go](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_message_shape.go:1)

### Next Best Cut

The remaining mixed ownership is now concentrated in the evaluation home's base issue detection:

- primary template drift
- secondary template drift

That means the next best cut is no longer mandatory. The line is already near-routing-only at the resolution layer, so the next phase should first judge whether a further base-check split is a real hotspot or cosmetic symmetry work.

### Verification Summary

Fresh verification passed:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinSaleAttributeFreshness.*|TestSheinSaleAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
