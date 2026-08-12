# SHEIN Import Platform Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Canonicalize SHEIN import-task routing without startup failures on historical mixed-case rows, and provide an explicit audited recovery path for the store-986 pending cohort.

**Architecture:** Normalize source, target, and legacy platform values at write and read boundaries while preserving their separate meanings. Ordinary auto-migration remains non-mutating; only a dry-run-by-default recovery command may update a strictly verified cohort in one transaction.

**Tech Stack:** Go, GORM, PostgreSQL/SQLite-compatible SQL, existing listing-admin repository, Go tests.

## Global Constraints

- Preserve `source_platform=amazon` and `target_platform=shein`; never collapse them.
- Canonical form is `strings.ToLower(strings.TrimSpace(value))`.
- `AutoMigrateImportTaskRepository` must not mutate, reject, or add constraints for historical mixed-case rows.
- Recovery requires `store_id=986`, positive `expected_count`, and explicit `-execute`; dry-run is the default.
- Recovery only normalizes pending rows whose case-folded legacy fields are amazon, amazon, and shein.
- Recovery must not publish RabbitMQ messages or change status.
- Exclude `deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard/configmap-heavy.yaml`; it requires independent capacity evidence.

## Execution Status (2026-08-12)

- Task 1 complete in `b525d6652`: ordinary startup migration no longer validates historical platform case, replaces indexes, or adds platform constraints.
- Task 2 verified as already present in `origin/master` through merged PR #123: canonical writes, normalized projections, case-compatible queries, and runtime routing passed focused coverage without copying older local-worktree changes.
- Task 3 complete in `444e5dc07`: the recovery API is store-986-only, expected-count gated, transactional, dry-run by default, and does not change status or publish messages.
- Task 4 complete in `b60688d0d`: the CLI uses the existing non-creating database factory and rejects invalid scope before configuration or database access.
- Task 5 verification passed for all relevant Go packages and repository architecture tests. A live dry-run was intentionally not run because `config/config-dev.yaml` has not been confirmed as a non-production database. It needs separately verified environment scope before any database read.

## File Structure

- `internal/domain/task/platform.go`: canonical platform helper.
- `internal/listingadmin/import_task_*.go`: write normalization and case-compatible predicates.
- `internal/listingadmin/platform_recovery.go`: transactional cohort validation and update.
- `internal/listingadmin/platform_recovery_test.go`: dry-run, scope, duplicate, rollback, and success tests.
- `internal/app/runtime/sheinplatformrecovery/runtime.go`: configuration and database dependency injection.
- `internal/app/runtime/sheinplatformrecovery/runtime_test.go`: explicit execution-flag tests.
- `cmd/shein-import-platform-recovery/main.go`: flag-only CLI entrypoint.

### Task 1: Remove startup-gating schema hardening

**Files:**
- Modify: `internal/listingadmin/import_task_repository.go`
- Modify: `internal/listingadmin/schema_migrate.go`
- Modify: `internal/listingadmin/schema_migrate_test.go`

**Interfaces:** Consumes `AutoMigrateImportTaskRepository(db *gorm.DB) error`; produces schema compatibility that tolerates historical rows.

- [ ] **Step 1: Write the failing regression**

```go
func TestAutoMigrateImportTaskRepositoryAllowsHistoricalMixedCasePlatforms(t *testing.T) {
    db := newImportTaskTestDB(t)
    seedHistoricalImportTask(t, db, "Amazon", "Amazon", "SHEIN")
    if err := AutoMigrateImportTaskRepository(db); err != nil {
        t.Fatalf("AutoMigrateImportTaskRepository() error = %v", err)
    }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/listingadmin -run TestAutoMigrateImportTaskRepositoryAllowsHistoricalMixedCasePlatforms -count=1`

Expected: FAIL because auto-migration invokes the platform-integrity guard.

- [ ] **Step 3: Implement the minimal startup-safe change**

Remove the integrity-guard call from ordinary auto-migration and delete its case-constraint/index-replacement helpers. Retain nullable-column and dispatch-event migrations. Do not add startup data repair.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/listingadmin -run 'TestAutoMigrateImportTaskRepository|Test.*Schema' -count=1`

Expected: PASS with a mixed-case row still readable.

- [ ] **Step 5: Commit**

```bash
git add internal/listingadmin/import_task_repository.go internal/listingadmin/schema_migrate.go internal/listingadmin/schema_migrate_test.go
git commit -m "fix(listingadmin): keep platform cleanup out of startup"
```

### Task 2: Apply canonical write, projection, and query compatibility

**Files:**
- Modify/Test: `internal/domain/task/platform.go`, `internal/domain/task/platform_test.go`
- Modify/Test: `internal/listingadmin/import_task_handler.go`, `internal/listingadmin/import_task_handler_test.go`
- Modify/Test: `internal/listingadmin/import_task_model.go`, `internal/listingadmin/import_task_model_test.go`
- Modify/Test: `internal/listingadmin/import_task_query.go`, `internal/listingadmin/import_task_repository.go`, `internal/listingadmin/import_task_dispatch_test.go`
- Modify: `internal/listingruntime/types.go`, `internal/listingruntime/local/local_data_provider.go`, `internal/app/consumer/task_handler.go`
- Test: `internal/app/consumer/task_handler_platform_test.go`

**Interfaces:** Produces `task.NormalizePlatform(value string) string`; consumes existing source/target runtime fields without changing their meaning.

- [ ] **Step 1: Write failing behavior tests**

```go
func TestNormalizePlatformTrimsAndLowercases(t *testing.T) {
    if got := task.NormalizePlatform(" SHEIN "); got != "shein" { t.Fatalf("got %q", got) }
}
func TestListDispatchCandidatesFairMatchesMixedCaseTargetPlatform(t *testing.T) {
    seedImportTask(t, db, ImportTask{Platform: "Amazon", SourcePlatform: "Amazon", TargetPlatform: "SHEIN"})
    got, err := repo.ListDispatchCandidatesFair(ctx, DispatchCandidateRequest{Platform: "shein"})
    if err != nil || len(got) != 1 { t.Fatalf("got %#v, err %v", got, err) }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/domain/task ./internal/listingadmin ./internal/app/consumer -run 'TestNormalizePlatformTrimsAndLowercases|TestListDispatchCandidatesFairMatchesMixedCaseTargetPlatform|TestModelTaskFromRuntime' -count=1`

Expected: FAIL because platform values are only trimmed and SQL predicates are case-sensitive.

- [ ] **Step 3: Implement boundary normalization**

Use `task.NormalizePlatform` at API creation, GORM conversion/defaults, query input, runtime mapping, and consumer routing. Query stored values with `LOWER(TRIM(...))`; prefer target and use legacy platform only as fallback.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/domain/task ./internal/listingadmin ./internal/listingruntime/local ./internal/app/consumer -count=1`

Expected: PASS with source and target values preserved separately.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/task internal/listingadmin/import_task_handler.go internal/listingadmin/import_task_model.go internal/listingadmin/import_task_query.go internal/listingadmin/import_task_repository.go internal/listingruntime internal/app/consumer/task_handler.go
git commit -m "fix(shein): normalize import task platform routing"
```

### Task 3: Add explicit transactional cohort recovery

**Files:**
- Create: `internal/listingadmin/platform_recovery.go`
- Create: `internal/listingadmin/platform_recovery_test.go`

**Interfaces:**

```go
type PlatformRecoveryRequest struct { StoreID int64; ExpectedCount int; Execute bool }
type PlatformRecoveryReport struct { SelectedIDs []int64; UpdatedIDs []int64; ConflictingIDs []int64; DryRun bool }
func (r *GormImportTaskRepository) RecoverStore986PlatformCohort(ctx context.Context, req PlatformRecoveryRequest) (PlatformRecoveryReport, error)
```

- [ ] **Step 1: Write RED tests**

```go
func TestRecoverStore986PlatformCohortDryRunDoesNotWrite(t *testing.T) {
    repo, db := newRecoveryRepository(t)
    id := seedPendingImportTask(t, db, 986, "Amazon", "Amazon", "SHEIN", "P-1")
    report, err := repo.RecoverStore986PlatformCohort(ctx, PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 1})
    if err != nil || !report.DryRun || !slices.Equal(report.SelectedIDs, []int64{id}) { t.Fatalf("report=%+v err=%v", report, err) }
    assertImportTaskPlatforms(t, db, id, "Amazon", "Amazon", "SHEIN")
}
func TestRecoverStore986PlatformCohortRejectsWrongCountAndRollsBack(t *testing.T) {
    repo, db := newRecoveryRepository(t)
    id := seedPendingImportTask(t, db, 986, "Amazon", "Amazon", "SHEIN", "P-1")
    _, err := repo.RecoverStore986PlatformCohort(ctx, PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 200, Execute: true})
    if err == nil { t.Fatal("expected count mismatch") }
    assertImportTaskPlatforms(t, db, id, "Amazon", "Amazon", "SHEIN")
}
func TestRecoverStore986PlatformCohortRejectsCaseFoldedDuplicate(t *testing.T) {
    repo, db := newRecoveryRepository(t)
    id := seedPendingImportTask(t, db, 986, "Amazon", "Amazon", "SHEIN", "P-1")
    seedPendingImportTask(t, db, 986, "amazon", "amazon", "shein", "P-1")
    _, err := repo.RecoverStore986PlatformCohort(ctx, PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 2, Execute: true})
    if err == nil { t.Fatal("expected duplicate rejection") }
    assertImportTaskPlatforms(t, db, id, "Amazon", "Amazon", "SHEIN")
}
func TestRecoverStore986PlatformCohortNormalizesOnlyEligiblePendingRows(t *testing.T) {
    repo, db := newRecoveryRepository(t)
    id := seedPendingImportTask(t, db, 986, "Amazon", "Amazon", "SHEIN", "P-1")
    skipped := seedPublishedImportTask(t, db, 986, "Amazon", "Amazon", "SHEIN", "P-2")
    report, err := repo.RecoverStore986PlatformCohort(ctx, PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 1, Execute: true})
    if err != nil || !slices.Equal(report.UpdatedIDs, []int64{id}) { t.Fatalf("report=%+v err=%v", report, err) }
    assertImportTaskPlatforms(t, db, id, "amazon", "amazon", "shein")
    assertImportTaskPlatforms(t, db, skipped, "Amazon", "Amazon", "SHEIN")
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/listingadmin -run TestRecoverStore986PlatformCohort -count=1`

Expected: FAIL because the recovery API does not exist.

- [ ] **Step 3: Implement fail-closed recovery**

Reject non-986 stores and non-positive expected counts. In one transaction select only the case-folded pending Amazon-to-SHEIN cohort, sort IDs, require exact count, reject active case-folded duplicate `(target_platform, product_id, region, store_id)` keys, and return without writing during dry-run. With `Execute=true`, update only the selected IDs' three platform columns.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/listingadmin -run 'TestRecoverStore986PlatformCohort|TestListDispatchCandidatesFair' -count=1`

Expected: PASS; rejected requests leave all rows unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/listingadmin/platform_recovery.go internal/listingadmin/platform_recovery_test.go
git commit -m "feat(shein): add audited import platform recovery"
```

### Task 4: Expose recovery through an explicit CLI runtime

**Files:**
- Create: `internal/app/runtime/sheinplatformrecovery/runtime.go`
- Create: `internal/app/runtime/sheinplatformrecovery/runtime_test.go`
- Create: `cmd/shein-import-platform-recovery/main.go`

**Interfaces:** Consumes `PlatformRecoveryRequest`; produces a command with `-config`, `-store-id`, `-expected-count`, and `-execute` flags.

- [ ] **Step 1: Write runtime RED tests**

```go
func TestRunDefaultsToDryRun(t *testing.T) {
    repo := &recoveryRepoStub{}
    _, err := runWithDependencies(ctx, Options{StoreID: 986, ExpectedCount: 200}, Dependencies{Recover: repo.Recover})
    if err != nil || repo.request.Execute { t.Fatalf("request=%+v err=%v", repo.request, err) }
}
func TestRunRejectsAnyStoreExcept986(t *testing.T) {
    repo := &recoveryRepoStub{}
    _, err := runWithDependencies(ctx, Options{StoreID: 985, ExpectedCount: 200}, Dependencies{Recover: repo.Recover})
    if err == nil || repo.called { t.Fatalf("called=%v err=%v", repo.called, err) }
}
func TestRunPassesExplicitExpectedCountAndExecuteFlag(t *testing.T) {
    repo := &recoveryRepoStub{}
    _, err := runWithDependencies(ctx, Options{StoreID: 986, ExpectedCount: 200, Execute: true}, Dependencies{Recover: repo.Recover})
    if err != nil || repo.request != (listingadmin.PlatformRecoveryRequest{StoreID: 986, ExpectedCount: 200, Execute: true}) { t.Fatalf("request=%+v err=%v", repo.request, err) }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/app/runtime/sheinplatformrecovery -count=1`

Expected: FAIL because the runtime package does not exist.

- [ ] **Step 3: Implement dependency-injected runtime and CLI**

Use the established config loader and shared database factory. Reject scope before mutation, print aggregate counts and IDs only, and call the recovery repository directly. Do not instantiate scheduler, publisher, or worker dependencies.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/app/runtime/sheinplatformrecovery -count=1 && go build ./cmd/shein-import-platform-recovery`

Expected: PASS and build exit 0.

- [ ] **Step 5: Commit**

```bash
git add internal/app/runtime/sheinplatformrecovery cmd/shein-import-platform-recovery
git commit -m "feat(shein): add platform recovery command"
```

### Task 5: Final verification and scoped handoff

**Files:**
- Modify: `docs/refactoring/next-phase-plan.md` only if a recovery-command status link is needed.
- Do not modify: `deployments/kubernetes/shein-listing/overlays/prod-store-auto-shard/configmap-heavy.yaml`.

- [ ] **Step 1: Format and inspect scope**

Run: `gofmt -w internal/domain/task/platform.go internal/listingadmin/*.go internal/listingruntime/*.go internal/listingruntime/local/local_data_provider.go internal/app/consumer/task_handler.go internal/app/runtime/sheinplatformrecovery/*.go cmd/shein-import-platform-recovery/main.go`

Run: `git diff --check && git diff --name-only origin/master...HEAD`

Expected: only routing/recovery files and design/plan documents are in scope.

- [ ] **Step 2: Run broader verification**

Run: `go test ./internal/domain/task ./internal/listingadmin ./internal/listingruntime/... ./internal/app/consumer ./internal/app/runtime/sheinplatformrecovery ./tests -count=1`

Expected: PASS.

- [ ] **Step 3: Run dry-run command only against non-production configuration**

Run: `go run ./cmd/shein-import-platform-recovery -config config/config-dev.yaml -store-id 986 -expected-count 200`

Expected: the command makes no write; an unavailable local database is reported without mutation.

- [ ] **Step 4: Review and publish only with separate authorization**

Confirm the command cannot publish RabbitMQ messages or alter status. Do not run `-execute` against production, deploy, push, or create a PR without explicit authorization.
