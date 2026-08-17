# PAY-042 Generation Reservation Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recover a ListingKit generation worker interruption without stranding quota or automatically replaying a provider call.

**Architecture:** Add a task-owned reservation intent plus lease before PAY-041 `ReserveGeneration`. An expiry sweep checks the deterministic ledger event, releases only a known reserved event, and atomically blocks the task for retry; unknown or settled states become reconciliation blocks. Admission is applied only to a new intent, so a pre-existing intent can unwind after a cohort flag change.

**Tech Stack:** Go, GORM, SQLite tests, ListingKit task repository, PAY-041 ledger.

## Global Constraints

- PAY-041 remains the local atomic quota authority; OpenMeter access is never used as a quota lock.
- New metered work requires durable intent before reserve and fails closed without a durable repository.
- A blank `BillingTenantID` never newly enters the cohort; existing intent always settles or reconciles.
- Internal intent fields use `json:"-"` and persist no provider credentials or usage payload.
- Recovery must not invoke the provider in the same sweep that identifies an expired intent.

---

## File Structure

- `internal/listingkit/model_task.go`: hidden intent state and lease.
- `internal/listingkit/interfaces_dependencies.go`: narrow repository and ledger lookup interfaces.
- `internal/listingkit/store/task_repo_status.go`: tenant-scoped intent lifecycle and atomic expiry resolution.
- `internal/listingkit/service_process_usage.go`: admission, reserve, and lease renewal.
- `internal/listingkit/service_process_runner_helper.go`: legacy retry path when usage is disabled.
- `internal/listingkit/task_recovery_service.go`: expiry reconciliation before generic recovery.
- `internal/listingkit/httpapi/generation_usage_adapter.go`: event status lookup.
- `internal/listingsubscription/errors.go`, `internal/listingsubscription/gorm_usage_ledger.go`: shared event-not-found sentinel.
- Associated focused `_test.go` files and `docs/architecture/pay-042-listingkit-generation-usage-cutover.md`.

### Task 1: Durable reservation intent

**Files:**
- Modify: `internal/listingkit/model_task.go`
- Modify: `internal/listingkit/interfaces_dependencies.go`
- Modify: `internal/listingkit/store/task_repo_status.go`
- Test: `internal/listingkit/store/task_repo_status_test.go`

**Interfaces:**
- Produces `GenerationUsageReservationRepository` with begin, mark-reserved, renew, clear, list-expired, and atomic resolve methods.
- Produces `GenerationUsageLedgerLookup.LookupGeneration(context.Context, string, string) (GenerationUsageEventState, bool, error)`.

- [ ] **Step 1: Write the failing lifecycle tests**

```go
func TestGenerationUsageReservationLeaseLifecycle(t *testing.T) {
    task := createProcessingTask(t)
    require.NoError(t, repo.BeginGenerationUsageReservation(ctx, task.ID, now.Add(10*time.Minute)))
    require.NoError(t, repo.MarkGenerationUsageReserved(ctx, task.ID, now.Add(10*time.Minute)))
    require.NoError(t, repo.RenewGenerationUsageReservation(ctx, task.ID, now.Add(20*time.Minute)))
    require.NoError(t, repo.ClearGenerationUsageReservation(ctx, task.ID))
    require.Empty(t, loadTask(t, task.ID).GenerationUsageReservationState)
}

func TestListExpiredGenerationUsageReservationsOnlyReturnsProcessingLeases(t *testing.T) {
    expired := createProcessingReservation(t, "reserved", now.Add(-time.Minute))
    createProcessingReservation(t, "reserved", now.Add(time.Minute))
    createBlockedReservation(t, "reserved", now.Add(-time.Minute))
    got, err := repo.ListExpiredGenerationUsageReservations(ctx, now, 10)
    require.NoError(t, err)
    require.Equal(t, []string{expired.ID}, taskIDs(got))
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./internal/listingkit/store -run 'TestGenerationUsageReservationLeaseLifecycle|TestListExpiredGenerationUsageReservationsOnlyReturnsProcessingLeases' -count=1`

Expected: FAIL because fields and methods are absent.

- [ ] **Step 3: Implement model, interface, and atomic storage operations**

```go
type GenerationUsageReservationRepository interface {
    BeginGenerationUsageReservation(context.Context, string, time.Time) error
    MarkGenerationUsageReserved(context.Context, string, time.Time) error
    RenewGenerationUsageReservation(context.Context, string, time.Time) error
    ClearGenerationUsageReservation(context.Context, string) error
    ListExpiredGenerationUsageReservations(context.Context, time.Time, int) ([]Task, error)
    ResolveExpiredGenerationUsageReservation(context.Context, string, RetryableBlock, string, bool) error
}
```

Add hidden state/lease fields and use a GORM transaction in `ResolveExpiredGenerationUsageReservation` to move `processing` to blocked, write the retryable block/error, and either clear or retain intent. Query only expired `processing` rows with a non-empty state and expired lease.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/listingkit/store -run 'TestGenerationUsageReservation' -count=1; go test ./internal/listingkit -count=1`

Expected: PASS.

```powershell
git add internal/listingkit/model_task.go internal/listingkit/interfaces_dependencies.go internal/listingkit/store/task_repo_status.go internal/listingkit/store/task_repo_status_test.go
git commit -m "feat: persist generation usage reservations"
```

### Task 2: Replay-safe worker admission and retry

**Files:**
- Modify: `internal/listingkit/service_process_usage.go`
- Modify: `internal/listingkit/service_process_runner_helper.go`
- Test: `internal/listingkit/service_process_usage_test.go`
- Test: `internal/listingkit/service_process_runner_helper_test.go`

**Interfaces:**
- Consumes `GenerationUsageReservationRepository` from Task 1.
- Produces `reserveGenerationUsage` that creates or resumes exactly one durable intent.

- [ ] **Step 1: Write failing service tests**

```go
func TestReserveGenerationUsageReplaysExistingIntentOutsideCohort(t *testing.T) {
    task := taskWithReservation("pending", tenantInCohort)
    admission.Allows = false
    _, enabled, err := service.reserveGenerationUsage(ctx, task)
    require.NoError(t, err); require.True(t, enabled); require.Equal(t, 1, settlement.reserveCalls)
}

func TestReserveGenerationUsageSkipsNewTaskWithoutBillingTenant(t *testing.T) {
    _, enabled, err := service.reserveGenerationUsage(ctx, taskWithoutReservation(""))
    require.NoError(t, err); require.False(t, enabled); require.Zero(t, settlement.reserveCalls)
}

func TestRetryableProviderFailureUsesLegacyPersistenceWhenUsageDisabled(t *testing.T) {
    task := runRetryableProviderFailure(t, false)
    require.Empty(t, task.NextRetryAt); require.True(t, task.RetryableBlock.AutoResumeEnabled)
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./internal/listingkit -run 'TestReserveGenerationUsageReplaysExistingIntentOutsideCohort|TestReserveGenerationUsageSkipsNewTaskWithoutBillingTenant|TestRetryableProviderFailureUsesLegacyPersistenceWhenUsageDisabled' -count=1`

Expected: FAIL because current admission precedes replay and disabled usage schedules the new retry path.

- [ ] **Step 3: Implement intent-first admission and lease renewal**

```go
if task.GenerationUsageReservationState == "" {
    if admission != nil && (task.BillingTenantID == "" || !admission.AllowsGenerationUsage(task.BillingTenantID)) {
        return nil, false, nil
    }
    if err := reservations.BeginGenerationUsageReservation(ctx, task.ID, leaseUntil()); err != nil { return nil, false, err }
}
reservation, err := settlement.ReserveGeneration(ctx, billingTenantID(task), task.ID, usage)
if err != nil { return nil, true, err }
if err := reservations.MarkGenerationUsageReserved(ctx, task.ID, leaseUntil()); err != nil { return nil, true, err }
```

Require a reservation repository for a newly admitted metered task. Renew a created lease every three minutes with a five-second persistence context and stop the ticker before the worker returns. When `usageEnabled` is false, route retryable provider failure to existing `persistProcessFailure`, not `persistProcessRetryableFailure`.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/listingkit -run 'TestReserveGenerationUsage|TestRetryableProviderFailureUsesLegacyPersistence' -count=1; go test ./internal/listingkit -count=1`

Expected: PASS.

```powershell
git add internal/listingkit/service_process_usage.go internal/listingkit/service_process_runner_helper.go internal/listingkit/service_process_usage_test.go internal/listingkit/service_process_runner_helper_test.go
git commit -m "fix: make generation usage retries replay safe"
```

### Task 3: Expired reservation recovery and deterministic lookup

**Files:**
- Modify: `internal/listingkit/task_recovery_service.go`
- Modify: `internal/listingkit/httpapi/generation_usage_adapter.go`
- Modify: `internal/listingsubscription/errors.go`
- Modify: `internal/listingsubscription/gorm_usage_ledger.go`
- Test: `internal/listingkit/task_recovery_service_test.go`
- Test: `internal/listingkit/httpapi/generation_usage_adapter_test.go`
- Test: `internal/listingsubscription/gorm_usage_ledger_test.go`

**Interfaces:**
- Consumes Task 1 interfaces.
- Produces reason codes `generation_usage_worker_interrupted` and `generation_usage_reconciliation_pending`.

- [ ] **Step 1: Write failing recovery and lookup tests**

```go
func TestRecoverySweepReleasesExpiredReservedGenerationBeforeRetry(t *testing.T) {
    task := expiredProcessingReservation(t, "reserved")
    lookup.state, lookup.found = GenerationUsageEventReserved, true
    _, err := service.RunRecoverySweep(ctx, 10)
    require.NoError(t, err); require.Equal(t, 1, settlement.releaseCalls)
    requireBlocked(t, task.ID, "generation_usage_worker_interrupted"); requireReservationCleared(t, task.ID)
}

func TestRecoverySweepBlocksUnknownLedgerStateWithoutProviderReplay(t *testing.T) {
    task := expiredProcessingReservation(t, "reserved")
    lookup.err = errors.New("ledger unavailable")
    _, err := service.RunRecoverySweep(ctx, 10)
    require.NoError(t, err); requireBlocked(t, task.ID, "generation_usage_reconciliation_pending")
    requireReservationRetained(t, task.ID); require.Zero(t, provider.calls)
}

func TestAdapterLookupGenerationTreatsLedgerNotFoundAsAbsent(t *testing.T) {
    state, found, err := adapter.LookupGeneration(ctx, tenantID, eventID)
    require.NoError(t, err); require.False(t, found); require.Empty(t, state)
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/listingsubscription -run 'TestRecoverySweep.*Generation|TestAdapterLookupGenerationTreatsLedgerNotFoundAsAbsent' -count=1`

Expected: FAIL because expiry reconciliation and lookup are absent.

- [ ] **Step 3: Implement safe reconciliation**

```go
switch {
case err != nil || (found && state != GenerationUsageEventReserved):
    err = reservations.ResolveExpiredGenerationUsageReservation(ctx, task.ID, reconciliationBlock(), "generation usage requires reconciliation", false)
case found:
    err = settlement.ReleaseGeneration(ctx, billingTenantID(task), task.ID)
    if err == nil { err = reservations.ResolveExpiredGenerationUsageReservation(ctx, task.ID, workerInterruptedBlock(), "generation worker interrupted", true) }
default:
    err = reservations.ResolveExpiredGenerationUsageReservation(ctx, task.ID, workerInterruptedBlock(), "generation worker interrupted before reserve", true)
}
```

Expose `listingsubscription.ErrUsageEventNotFound`; map both memory and GORM absence to it. The adapter returns `found=false` only for that sentinel and errors for unknown status. Run this expiry pass before the generic recoverable-task query.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/listingsubscription -run 'TestRecoverySweep.*Generation|TestAdapterLookupGeneration|TestGormUsageLedger' -count=1; go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/listingsubscription -count=1`

Expected: PASS.

```powershell
git add internal/listingkit/task_recovery_service.go internal/listingkit/httpapi/generation_usage_adapter.go internal/listingsubscription/errors.go internal/listingsubscription/gorm_usage_ledger.go internal/listingkit/task_recovery_service_test.go internal/listingkit/httpapi/generation_usage_adapter_test.go internal/listingsubscription/gorm_usage_ledger_test.go
git commit -m "fix: recover expired generation usage reservations"
```

### Task 4: Document and hand off the reviewed change

**Files:**
- Modify: `docs/architecture/pay-042-listingkit-generation-usage-cutover.md`
- Modify: `docs/architecture/README.md` only if the index guard requires it.

- [ ] **Step 1: Update the operator contract**

Document the ten-minute lease and three-minute renewal; the two recovery reason codes; that a cohort change does not abandon an existing intent; that new blank-`BillingTenantID` tasks stay legacy; that backfill is a separate migration; and that reconciliation blocks never auto-replay the provider.

- [ ] **Step 2: Run final checks**

Run: `go test ./internal/listingkit ./internal/listingkit/store ./internal/listingkit/httpapi ./internal/listingsubscription -count=1; go test -race ./internal/listingkit ./internal/listingkit/store -count=1; go vet ./internal/listingkit/... ./internal/listingsubscription; ./scripts/test-all.ps1 -count=1; git diff --check origin/main...HEAD`

Expected: every command exits 0; record exact output for any unrelated pre-existing failure before changing its scope.

- [ ] **Step 3: Commit, push, reply, and resolve**

```powershell
git add docs/architecture/pay-042-listingkit-generation-usage-cutover.md docs/architecture/README.md
git commit -m "docs: define generation reservation recovery"
git push
```

Reply with test evidence and resolve only these implemented review threads: `PRRT_kwDOQg5lB86Zr0eZ`, `PRRT_kwDOQg5lB86Zr0ea`, `PRRT_kwDOQg5lB86Zr0ec`, and `PRRT_kwDOQg5lB86Zr0ee`.
