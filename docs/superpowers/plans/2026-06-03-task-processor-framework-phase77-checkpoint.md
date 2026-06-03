## Task Processor Framework Phase 77 Checkpoint

### Status

`Phase 77` continued the SHEIN attribute freshness framework line by shrinking:

- [evaluateSheinResolvedAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:8)

This round extracted `drift-detail shaping + outward issue/message assembly` into its own local home.

### What Changed

- [shein_submit_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:8)
  - `evaluateSheinResolvedAttributeFreshness(...)` now keeps:
    - issue-state dispatch
    - message-shape dispatch
- [shein_submit_attribute_freshness_message_shape.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_message_shape.go:1)
  - new local home for:
    - drift-detail shaping
    - issue aggregation
    - outward message assembly
- [phase75_shein_attribute_freshness_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase75_shein_attribute_freshness_boundary_test.go:1)
  - boundary coverage updated to lock the new split

### Resulting Shape

Current layering for this hotspot is now:

- aggregate owner:
  - [shein_submit_freshness.go](/D:/code/task-processor/internal/listingkit/shein_submit_freshness.go:231)
- local evaluation home:
  - [shein_submit_attribute_freshness_evaluation.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:8)
- issue-state collection home:
  - [shein_submit_attribute_freshness_issue_state.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_issue_state.go:1)
- message-shape home:
  - [shein_submit_attribute_freshness_message_shape.go](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_message_shape.go:1)

### Closure Signal

At this point the attribute freshness sub-line is close to `near-routing-only`:

- aggregate owner is thin
- local evaluation home is just issue-state dispatch + message-shape dispatch
- collection logic has its own home
- message shaping has its own home

That means the next best step is likely a closure audit, not another default decomposition phase.

### Verification Summary

Fresh verification passed:

```powershell
go test ./internal/listingkit -run "TestEvaluateSheinAttributeFreshness.*|TestSheinAttributeFreshnessBoundary" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
