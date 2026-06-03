## Task Processor Framework Phase 81 Plan

### Objective

Run a closure audit for the current shared readiness contract line.

### Audit Questions

1. Are there still duplicated shared-contract responsibilities across submit readiness and freshness readiness builders?
2. Are direct and temporal submit flows still duplicating gating semantics outside the shared seam?
3. Would another extraction reduce real cross-flow drift, or mostly optimize symmetry?

### Evidence To Review

- [shein_submit_readiness.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness.go:24)
- [shein_submit_freshness.go](/D:/code-task-processor/internal/listingkit/shein_submit_freshness.go:164)
- [shein_submit_readiness_summary.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_summary.go:1)
- [shein_submit_readiness_gate.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_gate.go:1)
- [shein_submit_readiness_guidance.go](/D:/code-task-processor/internal/listingkit/shein_submit_readiness_guidance.go:1)
- [task_direct_submission_service.go](/D:/code-task-processor/internal/listingkit/task_direct_submission_service.go:46)
- [task_temporal_submission_adapter.go](/D:/code-task-processor/internal/listingkit/task_temporal_submission_adapter.go:87)

### Verification

At minimum, rerun:

```powershell
go test ./internal/listingkit -run "TestSheinSubmitReadinessGuidanceBoundary|TestSheinSubmitReadinessSummaryBoundary|TestSheinSubmitReadinessGateBoundary|TestShapeSheinSubmitReadinessSummary.*|TestValidateSheinSubmitReadinessGates.*|TestTaskTemporalSubmissionAdapterValidateReadinessBlocksOnFreshnessGate" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```
