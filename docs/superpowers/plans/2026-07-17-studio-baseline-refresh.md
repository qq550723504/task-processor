# Studio Baseline Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow SHEIN Studio task creation to recover SDS baselines blocked only by a completed SDS login.

**Architecture:** The Studio task gate will use the existing SDS baseline readiness service instead of a raw baseline-cache read. That service owns live login-state reconciliation and persistence; the gate will cache one normalized readiness result per baseline key for each creation run.

**Tech Stack:** Go, GORM, existing SDS login-status provider, Go unit tests.

## Global Constraints

- Do not force-update batch, design, or task-link status rows.
- Clear only recoverable login-state blocks when `HasAccessToken` is true and `LoginInProgress` is false.
- Preserve genuine login, cache-payload, and design-surface failures.

---

### Task 1: Replace the raw cache checker

**Files:**
- Modify: `internal/listingkit/studio_batch_task_gate.go:11-165,289-299`
- Modify: `internal/listingkit/task_studio_batch_service.go:9-38`
- Modify: `internal/listingkit/service_studio_batch_wiring_support.go:272-291`
- Test: `internal/listingkit/studio_batch_task_gate_test.go`

**Interfaces:**
- Consumes: `sdsBaselineService.GetReadiness(context.Context, *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error)`.
- Produces: `StudioBatchBaselineReadinessChecker.CheckStudioBatchBaselineReadiness(context.Context, *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error)`.

- [ ] **Step 1: Write the failing gate test**

```go
func TestStudioBatchTaskGateAllowsReconciledReadyBaseline(t *testing.T) {
    t.Parallel()
    eval := newEligibleStudioBatchGateEvaluation(t)
    eval.BaselineChecker = &stubStudioBatchBaselineReadinessChecker{
        readiness: &SDSBaselineReadiness{
            Status: SDSBaselineStatusReady,
            ValidationStatus: SDSBaselineValidationStatusReady,
        },
    }
    result, err := newStudioBatchTaskGate(eval.BaselineChecker, eval.StoreValidator).Evaluate(context.Background(), eval)
    if err != nil { t.Fatalf("Evaluate() error = %v", err) }
    if !result.Eligible { t.Fatalf("Evaluate() = %+v, want eligible", result) }
}
```

- [ ] **Step 2: Verify the test is red**

Run: `go test ./internal/listingkit -run TestStudioBatchTaskGateAllowsReconciledReadyBaseline -count=1`

Expected: FAIL because the current checker returns `*SDSBaselineCacheEntry`.

- [ ] **Step 3: Change the checker contract and gate evaluation**

```go
type StudioBatchBaselineReadinessChecker interface {
    CheckStudioBatchBaselineReadiness(context.Context, *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error)
}

func studioBatchReadinessIsReady(value *SDSBaselineReadiness) bool {
    return value != nil && value.Status == SDSBaselineStatusReady &&
        value.ValidationStatus == SDSBaselineValidationStatusReady
}
```

Keep `baselineCache` keyed by `sdsBaselineKey`, but cache the readiness result and error. Reject non-ready results with their existing `ReasonCode` and `Reason`.

- [ ] **Step 4: Delegate to the existing readiness service**

```go
func (c studioBatchBaselineReadinessChecker) CheckStudioBatchBaselineReadiness(
    ctx context.Context, query *SDSBaselineReadinessQuery,
) (*SDSBaselineReadiness, error) {
    if c.readinessService == nil { return nil, nil }
    return c.readinessService.GetReadiness(ctx, query)
}
```

Wire this checker through `taskStudioBatchServiceConfig`; do not duplicate SDS login-state reconciliation.

- [ ] **Step 5: Verify the gate suite is green**

Run: `go test ./internal/listingkit -run 'TestStudioBatchTaskGate' -count=1`

Expected: PASS.

### Task 2: Prove stale-login recovery in task creation

**Files:**
- Modify: `internal/listingkit/studio_batch_task_gate_test.go:330-430`
- Test: `internal/listingkit/sds_baseline_readiness_test.go:362-417`

**Interfaces:**
- Consumes: the Task 1 checker and `sdsBaselineService.reconcileCachedSDSLoginBaselineReadiness`.
- Produces: task creation that creates eligible work after stale login blocks are reconciled.

- [ ] **Step 1: Write the failing integration regression test**

Seed a valid cached baseline with `ValidationReasonCode: SDSBaselineReasonCodeLoginInProgress`; configure SDS login status as `HasAccessToken: true` and `LoginInProgress: false`; assert normal task creation creates exactly one task:

```go
if created != 1 {
    t.Fatalf("created tasks = %d, want 1 after stale login block is reconciled", created)
}
if len(result.RejectedTasks) != 0 {
    t.Fatalf("rejected tasks = %+v, want none", result.RejectedTasks)
}
```

- [ ] **Step 2: Verify the regression test is red**

Run: `go test ./internal/listingkit -run TestStudioBatchTaskGateCreatesTaskAfterStaleLoginBlockIsReconciled -count=1`

Expected: FAIL because current task creation reads the blocked cache entry directly.

- [ ] **Step 3: Add the active-login negative case**

```go
if result.RejectedTasks[0].ReasonCode != "baseline_not_ready" {
    t.Fatalf("reason = %q, want baseline_not_ready", result.RejectedTasks[0].ReasonCode)
}
```

Use `LoginInProgress: true` and assert zero created tasks.

- [ ] **Step 4: Verify targeted suites**

Run: `go test ./internal/listingkit -run 'TestStudioBatchTaskGate|TestGetSDSBaselineReadinessClearsCached' -count=1`

Expected: PASS.

- [ ] **Step 5: Verify package suites**

Run: `go test ./internal/listingkit ./internal/listingkit/httpapi -count=1 -timeout 5m`

Expected: PASS with exit code 0.

### Task 3: Recover batch `cc7ca86a-8e24-4c4e-90a6-38bb699b1bda`

**Files:**
- No repository-file changes.

**Interfaces:**
- Consumes: deployed Task 1 behavior and the existing Studio create-tasks endpoint.
- Produces: normal task-link outcomes without direct database state edits.

- [ ] **Step 1: Confirm SDS status**

With an authorized session, require `has_access_token=true` and `login_in_progress=false`; otherwise stop and report the status.

- [ ] **Step 2: Trigger normal task creation**

Submit the approved design IDs through the existing Studio create-tasks endpoint. Do not modify `listingkit_studio_batches`, `listingkit_studio_materialized_designs`, or task-link records directly.

- [ ] **Step 3: Verify the persisted result**

Use a read-only grouped query on `listingkit_studio_batch_task_links` and report created, reused, rejected, and failed counts. Any remaining blocked selection must retain its exact reason.

## Plan self-review

- Spec coverage: Task 1 removes the raw-cache bypass, Task 2 proves recovery and preserves active-login blocking, and Task 3 safely recovers the affected batch.
- Placeholder scan: no placeholder markers or ambiguous implementation steps remain.
- Type consistency: the gate checker returns `*SDSBaselineReadiness` throughout and delegates reconciliation to the existing readiness service.
