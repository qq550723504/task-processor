# PAY-042 ListingKit generation usage cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace legacy generation usage writes for `GenerateListingKit` with a durable `Reserve -> Commit/Release` ledger lifecycle while preserving unrelated entrypoints.

**Architecture:** Keep the HTTP subscription check as an access gate, then inject an optional generation-settlement port into the ListingKit service. The processor reserves against the stable task ID before workflow execution, commits only after the terminal result is persisted, and releases safe failures. A `usage_commit_pending` retryable state is settled through `RecoverTaskNow` without re-running providers.

**Tech Stack:** Go, GORM, existing `listingsubscription.UsageLedger`, ListingKit task repository, Gin handler tests, in-memory and SQLite durable test repositories.

## Global Constraints

- First slice is only `POST /api/v1/listing-kits/generate` and its `ProcessListingKit` execution.
- Use `module_code=studio`, `metric=studio_design_jobs_succeeded`, `quantity=1`.
- Use `source_type=listingkit_generation`, `source_id=task.ID`, and `idempotency_key=listingkit:generation:<task.ID>`.
- Both `completed` and `needs_review` are billable terminal results.
- Never call a provider before a successful reservation.
- Do not call OpenMeter directly; retain the existing durable outbox boundary.
- Do not modify payment/provider integration, SHEIN submit, 1688 create, storage upload/delete, or Studio standalone async endpoints.
- Keep the new dependency optional and the configuration flag disabled by default.
- Any unsafe release or settlement uncertainty remains durable for reconciliation; do not guess or silently discard usage.

---

### Task 1: Define the generation settlement port and subscription adapter

**Files:**
- Create: `internal/listingkit/generation_usage.go`
- Modify: `internal/listingkit/service_types.go`
- Modify: `internal/listingkit/service_task_dependencies.go`
- Modify: `internal/listingkit/service_config_groups.go`
- Modify: `internal/listingsubscription/service.go`
- Create: `internal/listingkit/httpapi/generation_usage_adapter.go`
- Modify: `internal/listingkit/httpapi/bootstrap_service_config.go`
- Modify: `internal/listingkit/httpapi/bootstrap_repositories_core.go`
- Test: `internal/listingkit/generation_usage_test.go`
- Test: `internal/listingkit/httpapi/generation_usage_adapter_test.go`
- Test: `internal/listingsubscription/service_usage_ledger_test.go`

**Interfaces:**
- Produces `listingkit.GenerationUsageSettlement` for processor tasks.
- Produces `(*listingsubscription.Service).GetUsage(ctx, tenantID, idempotencyKey) (*UsageEvent, error)` for idempotent settlement lookup.

- [ ] **Step 1: Write the failing contract tests.**

Add tests that assert the port accepts `(ctx, tenantID, taskID, occurredAt)`, normalizes the key to `listingkit:generation:<taskID>`, and that the subscription adapter maps the canonical metric and source fields into `ReserveUsageInput`. Add a service test proving `GetUsage` delegates to the configured ledger and returns `ErrUsageLedgerNotConfigured` when absent.

- [ ] **Step 2: Run the focused tests to verify RED.**

Run:

```powershell
go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/listingsubscription -run 'Test(GenerationUsage|SubscriptionUsageLedger)' -count=1
```

Expected: compile failures for the missing port, adapter, and `GetUsage` method.

- [ ] **Step 3: Implement the port and adapter.**

Use these exact public contracts:

```go
type GenerationUsageReservation struct {
    EventID         string
    AlreadyCommitted bool
}

type GenerationUsageSettlement interface {
    ReserveGeneration(context.Context, string, string, time.Time) (GenerationUsageReservation, error)
    CommitGeneration(context.Context, string, string) error
    ReleaseGeneration(context.Context, string, string, string) error
}
```

The adapter computes the stable key, calls `ReserveUsage`, uses `GetUsage` for
commit/release lookup, and treats an existing committed event as
`AlreadyCommitted=true`. Add the dependency to `ServiceCoreDependencies` and
`taskDependencies`; leave it nil unless the bootstrap flag and a concrete
ledger-backed subscription service are both available.

- [ ] **Step 4: Run the focused tests to verify GREEN.**

Run the same command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit the contract slice.**

```powershell
git add internal/listingkit/generation_usage.go internal/listingkit/service_types.go internal/listingkit/service_task_dependencies.go internal/listingkit/service_config_groups.go internal/listingsubscription/service.go internal/listingkit/httpapi/generation_usage_adapter.go internal/listingkit/httpapi/bootstrap_service_config.go internal/listingkit/httpapi/bootstrap_repositories_core.go internal/listingkit/generation_usage_test.go internal/listingkit/httpapi/generation_usage_adapter_test.go internal/listingsubscription/service_usage_ledger_test.go
git commit -m "feat: add ListingKit generation usage settlement port"
```

### Task 2: Reserve before workflow and settle terminal results

**Files:**
- Create: `internal/listingkit/service_process_usage.go`
- Modify: `internal/listingkit/service_process_runner_helper.go`
- Test: `internal/listingkit/service_process_usage_test.go`
- Test: `internal/listingkit/service_process_status_test.go`

**Interfaces:**
- Consumes `service.taskDeps.generationUsage` from Task 1.
- Produces processor behavior: reserve before `runWorkflow`, release on safe failure, commit after `persistProcessSuccess`.

- [ ] **Step 1: Write failing processor tests.**

Add a recording settlement fake and tests with these names and assertions:

```go
func TestProcessListingKitReservesBeforeWorkflow(t *testing.T)
func TestProcessListingKitQuotaRejectionSkipsWorkflow(t *testing.T)
func TestProcessListingKitReleasesReservationOnWorkflowFailure(t *testing.T)
func TestProcessListingKitCommitsAfterCompletedResultPersistence(t *testing.T)
func TestProcessListingKitCommitsNeedsReviewResult(t *testing.T)
func TestProcessListingKitDoesNotDoubleReserveOrRunOnCommittedReplay(t *testing.T)
```

The fake must record call order (`reserve`, `workflow`, `persist`, `commit`), and
the workflow stub must fail the test if called after a quota rejection or an
already committed replay.

- [ ] **Step 2: Run the processor tests to verify RED.**

```powershell
go test ./internal/listingkit -run 'TestProcessListingKit(Reserves|Quota|Releases|Commits|DoesNotDouble)' -count=1
```

Expected: failures because no settlement calls exist.

- [ ] **Step 3: Implement the processor settlement sequence.**

After `claimTask`, call `ReserveGeneration(ctx, task.TenantID, task.ID, task.CreatedAt)` before `runWorkflow`. On `ErrUsageQuotaExceeded`, persist a safe failed task and return without invoking the workflow. On workflow failure, call `ReleaseGeneration` and preserve the existing classified failure behavior. After `persistProcessSuccess` returns nil for either completed or needs-review, call `CommitGeneration`.

If reservation reports `AlreadyCommitted`, only return the already persisted terminal result; if the task has no persisted result, fail closed with a settlement-pending error and never rerun the workflow.

- [ ] **Step 4: Run focused, package, and race tests.**

```powershell
go test ./internal/listingkit -run 'TestProcessListingKit(Reserves|Quota|Releases|Commits|DoesNotDouble)' -count=1
go test ./internal/listingkit -count=1
go test -race ./internal/listingkit -run 'TestProcessListingKit' -count=1
```

Expected: PASS for all commands.

- [ ] **Step 5: Commit the processor slice.**

```powershell
git add internal/listingkit/service_process_usage.go internal/listingkit/service_process_runner_helper.go internal/listingkit/service_process_usage_test.go internal/listingkit/service_process_status_test.go
git commit -m "feat: settle ListingKit generation usage at task boundary"
```

### Task 3: Persist a settlement-only retryable state

**Files:**
- Modify: `internal/listingkit/interfaces_dependencies.go`
- Modify: `internal/listingkit/retryable_block.go`
- Modify: `internal/listingkit/store/task_repo_status.go`
- Modify: `internal/listingkit/store/mem_store.go`
- Modify: `internal/listingkit/store/task_repo_contracts.go`
- Test: `internal/listingkit/store/task_repo_usage_settlement_test.go`
- Test: `internal/listingkit/service_process_usage_test.go`

**Interfaces:**
- Adds `Repository.ResolveUsageSettlement(ctx, taskID string) error`.
- Produces `RetryableBlock{ReasonCode: "usage_commit_pending", RecoveryScope: "listingkit_usage_settlement"}` while retaining the already persisted result.

- [ ] **Step 1: Write the failing repository tests.**

Test that `MarkBlockedRetryable` after a terminal result retains `Task.Result`, changes status to `blocked_retryable`, and stores only the safe reason code. Test that `ResolveUsageSettlement` restores status from `Task.Result.Status` (`completed` or `needs_review`), clears `RetryableBlock` and `Error`, and never deletes the result.

- [ ] **Step 2: Run the repository tests to verify RED.**

```powershell
go test ./internal/listingkit/store -run 'Test.*UsageSettlement' -count=1
```

Expected: compile failures for the missing repository method and persistence behavior.

- [ ] **Step 3: Implement durable and memory persistence.**

Add `ResolveUsageSettlement` to the repository interface and both implementations. Use the persisted result status as the only source for the restored terminal status; reject resolution when the task/result is missing or non-terminal. In the processor, when `CommitGeneration` fails after terminal result persistence, call `MarkBlockedRetryable` with the exact reason code and return the commit error without invoking the workflow again.

- [ ] **Step 4: Run focused and package tests.**

```powershell
go test ./internal/listingkit/store -run 'Test.*UsageSettlement' -count=1
go test ./internal/listingkit ./internal/listingkit/store -count=1
go vet ./internal/listingkit ./internal/listingkit/store
```

Expected: PASS.

- [ ] **Step 5: Commit the persistence slice.**

```powershell
git add internal/listingkit/interfaces_dependencies.go internal/listingkit/retryable_block.go internal/listingkit/store/task_repo_status.go internal/listingkit/store/mem_store.go internal/listingkit/store/task_repo_contracts.go internal/listingkit/store/task_repo_usage_settlement_test.go internal/listingkit/service_process_usage_test.go
git commit -m "feat: persist ListingKit usage settlement retries"
```

### Task 4: Add settlement-only recovery

**Files:**
- Modify: `internal/listingkit/task_recovery_service.go`
- Modify: `internal/listingkit/service_submit_routing.go`
- Test: `internal/listingkit/task_recovery_usage_settlement_test.go`
- Test: `internal/listingkit/api/task_recovery_handler_test.go`

**Interfaces:**
- Consumes `GenerationUsageSettlement` and `Repository.ResolveUsageSettlement`.
- Produces `RecoverTaskNow` behavior for `usage_commit_pending` that never calls `TaskSubmitter.Submit`.

- [ ] **Step 1: Write failing recovery tests.**

Add tests named:

```go
func TestRecoverTaskNowSettlesUsageCommitWithoutSubmittingTask(t *testing.T)
func TestRecoverTaskNowLeavesUsageSettlementBlockedWhenEventMissing(t *testing.T)
func TestRecoverTaskNowUsesGenericSubmitRecoveryForNonUsageBlocks(t *testing.T)
```

The first test must seed a terminal result plus `usage_commit_pending`, make the
settlement fake commit successfully, assert the task returns to its original
terminal status, and assert the task submitter call count remains zero. The
second test must preserve the block when lookup/commit is ambiguous.

- [ ] **Step 2: Run recovery tests to verify RED.**

```powershell
go test ./internal/listingkit -run 'TestRecoverTaskNow(Se|Leaves|UsesGeneric)' -count=1
```

Expected: failures because recovery currently always requeues blocked tasks.

- [ ] **Step 3: Implement the settlement-only branch.**

Before constructing the generic recovery request, load the task. If
`RetryableBlock.ReasonCode == "usage_commit_pending"`, call
`CommitGeneration(ctx, task.TenantID, task.ID)`, then `ResolveUsageSettlement`.
Return the refreshed task. Do not call `RecoverBlockedTaskNow` or submit the
generation task. For lookup, missing event, or unresolved delivery errors, keep
the block and return the error. All other retryable reasons continue through the
existing generic recovery service unchanged.

- [ ] **Step 4: Run focused, race, and package recovery gates.**

```powershell
go test ./internal/listingkit -run 'TestRecoverTaskNow(Se|Leaves|UsesGeneric)' -count=1
go test -race ./internal/listingkit -run 'TestRecoverTaskNow' -count=1
go test ./internal/listingkit ./internal/listingkit/api -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the recovery slice.**

```powershell
git add internal/listingkit/task_recovery_service.go internal/listingkit/service_submit_routing.go internal/listingkit/task_recovery_usage_settlement_test.go internal/listingkit/api/task_recovery_handler_test.go
git commit -m "feat: recover ListingKit usage settlements without rerun"
```

### Task 5: Wire the opt-in flag and preserve legacy behavior

**Files:**
- Modify: `internal/core/config/type_listingkit.go`
- Modify: `internal/core/config/defaults.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/listingkit/httpapi/bootstrap_repositories_core.go`
- Modify: `internal/listingkit/httpapi/bootstrap_service_config.go`
- Modify: `internal/listingkit/httpapi/bootstrap_module_service.go`
- Test: `internal/core/config/config_env_test.go`
- Test: `internal/listingkit/httpapi/bootstrap_test.go`
- Test: `internal/listingkit/api/generation_tasks_handler_test.go`

**Interfaces:**
- Configuration key: `listingkit.generationUsageLedgerEnabled`, default `false`.
- When false, `ServiceCoreDependencies.GenerationUsageLedger` remains nil and existing generation behavior is unchanged.
- When true, a concrete GORM subscription repository receives `NewGormUsageLedger` and the ListingKit service receives the adapter.

- [ ] **Step 1: Write failing configuration and wiring tests.**

Assert the default is false, Viper binds `LISTINGKIT_GENERATIONUSAGELEDGERENABLED`, and bootstrap does not inject a settlement dependency when disabled. Add an API test proving task creation still performs only the active-subscription check and does not call `RecordUsage`.

- [ ] **Step 2: Run the tests to verify RED.**

```powershell
go test ./internal/core/config ./internal/listingkit/httpapi ./internal/listingkit/api -run 'Test.*GenerationUsage|TestGenerateListingKit' -count=1
```

Expected: compile or assertion failures for the missing config field and wiring.

- [ ] **Step 3: Implement config and bootstrap wiring.**

Parse the bool in `BuildConfig`, bind the default in `setDefaults`, pass it through `BuildServiceInput` into `buildListingKitServiceConfig`, and only construct the adapter when the flag is true and the subscription service has a concrete ledger. Keep custom repositories without a ledger on the legacy path with a startup diagnostic rather than a panic.

- [ ] **Step 4: Run focused and full affected-package gates.**

```powershell
go test ./internal/core/config ./internal/listingkit/httpapi ./internal/listingkit/api -run 'Test.*GenerationUsage|TestGenerateListingKit' -count=1
go test ./internal/core/config ./internal/listingkit/httpapi ./internal/listingkit/api ./internal/listingkit -count=1
go vet ./internal/core/config ./internal/listingkit/httpapi ./internal/listingkit/api ./internal/listingkit
```

Expected: PASS.

- [ ] **Step 5: Commit the wiring slice.**

```powershell
git add internal/core/config/type_listingkit.go internal/core/config/defaults.go internal/core/config/loader_builder.go internal/listingkit/httpapi/bootstrap_repositories_core.go internal/listingkit/httpapi/bootstrap_service_config.go internal/listingkit/httpapi/bootstrap_module_service.go internal/core/config/config_env_test.go internal/listingkit/httpapi/bootstrap_test.go internal/listingkit/api/generation_tasks_handler_test.go
git commit -m "feat: gate ListingKit generation ledger cutover"
```

### Task 6: Record handoff and execute final verification

**Files:**
- Modify: `docs/architecture/pay-041-usage-ledger.md`
- Create: `docs/architecture/pay-042-listingkit-generation-usage-cutover.md`
- Modify: `docs/architecture/README.md`
- Test: `internal/listingkit/phase_pay042_generation_usage_boundary_test.go`

**Interfaces:**
- Documents the exact first-slice boundary and explicitly lists PAY-043 and PAY-044 as follow-up owners.

- [ ] **Step 1: Write the architecture-boundary regression test.**

Assert the handoff names `GenerateListingKit`, the stable task-derived idempotency key, the canonical metric, reserve-before-workflow ordering, terminal commit, failure release, and settlement-only recovery. Assert that payment/provider and unrelated entrypoints are not listed as completed.

- [ ] **Step 2: Update architecture documentation and index.**

Record the enabled flag, lifecycle diagram, rollback (disable the flag), reconciliation expectations, and the next entrypoint order. Add the new document to `docs/architecture/README.md` so repository architecture-index tests remain green.

- [ ] **Step 3: Run final verification before claiming completion.**

```powershell
go test ./internal/listingsubscription ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/listingkit/store -count=1
go test -race ./internal/listingkit ./internal/listingkit/store -count=1
go vet ./internal/listingsubscription ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/listingkit/store
./scripts/test-all.ps1 -count=1
git diff --check HEAD~5..HEAD
```

Expected: all commands exit 0; if the repository-wide harness exposes an
unrelated pre-existing failure, record the exact module and error without
weakening the PAY-042 gates.

- [ ] **Step 4: Commit the handoff.**

```powershell
git add docs/architecture/pay-041-usage-ledger.md docs/architecture/pay-042-listingkit-generation-usage-cutover.md docs/architecture/README.md internal/listingkit/phase_pay042_generation_usage_boundary_test.go
git commit -m "docs: record PAY-042 ListingKit generation cutover"
```

