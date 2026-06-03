## Task Processor Framework Phase 76 Checkpoint

### Status

`Phase 76` continued the SHEIN attribute freshness framework line by shrinking:

- [evaluateSheinResolvedAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:12)

This round extracted the `invalid / missing-required collection` responsibility into its own local home.

### What Changed

- [shein_submit_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:12)
  - `evaluateSheinResolvedAttributeFreshness(...)` now keeps:
    - issue-state dispatch
    - drift-detail shaping
    - outward message assembly
- [shein_submit_attribute_freshness_issue_state.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_issue_state.go:1)
  - new local home for:
    - invalid resolved-attribute collection
    - missing-required collection
    - required-attribute reactivation checks
- [phase75_shein_attribute_freshness_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase75_shein_attribute_freshness_boundary_test.go:1)
  - boundary coverage updated to lock the new split

### Resulting Shape

Current layering for this hotspot is now:

- aggregate owner:
  - [shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:231)
- local evaluation home:
  - [shein_submit_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:12)
- issue-state collection home:
  - [shein_submit_attribute_freshness_issue_state.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_issue_state.go:1)

### Next Best Cut

The clearest remaining mixed ownership is now concentrated in the evaluation home around:

- drift-detail shaping
- outward issue/message assembly

So the strongest next slice is a `message-shape` extraction, not another collection split.

### Verification Summary

Fresh verification passed:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinAttributeFreshness.*|TestSheinAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
