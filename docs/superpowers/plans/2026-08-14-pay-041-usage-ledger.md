# PAY-041 Idempotent Usage Ledger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an auditable, idempotent usage ledger with an atomic local reservation boundary while keeping current subscription guards and OpenMeter shadow behavior unchanged.

**Architecture:** Keep `internal/listingsubscription` as the owner of plan, entitlement, quota, and usage domain contracts. Add a ledger beside the existing mutable counters: an event row records the business fact and a locked usage bucket records committed and reserved quantities. A transaction creates the event, reserves quota, and creates an OpenMeter outbox record together; later commit/release transitions are idempotent. PAY-042 will wire entrypoints to this ledger, and PAY-043 will handle manual commercial subscriptions/payment records separately.

**Tech Stack:** Go, GORM, PostgreSQL production schema, SQLite in-memory repository tests, existing ListingKit schema-migration command, table-driven and concurrency tests.

## Global Constraints

- The only initial commercial plan remains `paid_pilot`; do not add checkout, payment, invoice, tax, or provider SDK code in PAY-041.
- Preserve existing `CheckUsage`, `AuthorizeUsage`, `RecordUsage`, `UsageCounter`, and current HTTP behavior until PAY-042 has an explicit cutover PR.
- Use `int64` quantities; `storage_bytes_current` accepts signed deltas, while successful-job metrics require positive quantities.
- Event identity is unique by `(tenant_id, idempotency_key)`; retries and recovery return the original event instead of creating a second charge.
- Event lifecycle values are exactly `reserved`, `committed`, `released`, and `reversed`.
- Reservation must enforce entitlement status/time bounds and the projected limit under a database transaction; `hasAccess` alone is not a hard-quota reservation.
- OpenMeter delivery is asynchronous through an outbox; an OpenMeter outage must not roll back the local business reservation.
- No business handler may import the OpenMeter SDK or call provider APIs directly.
- Extend the existing ListingKit schema migrator; do not introduce a second migration framework in this plan.
- Every externally visible error must preserve tenant, metric, limit, and current/reserved usage context without exposing secrets or other tenants.

---

### Task 1: Define the ledger domain contract and invariants

**Files:**
- Modify: `internal/listingsubscription/types.go`
- Create: `internal/listingsubscription/usage_ledger.go`
- Test: `internal/listingsubscription/usage_ledger_test.go`

**Interfaces:**
- Consumes: existing `Module`, `Entitlement`, `UsageCounter`, and subscription status values.
- Produces: `UsageEvent`, `UsageEventStatus`, `ReserveUsageInput`, `ReserveUsageResult`, `UsageLedger`, `UsageOutboxItem`, and typed validation/quota errors for repository and PAY-042 callers.

Define these exact public contracts:

```go
type UsageEventStatus string

const (
    UsageEventReserved  UsageEventStatus = "reserved"
    UsageEventCommitted UsageEventStatus = "committed"
    UsageEventReleased  UsageEventStatus = "released"
    UsageEventReversed  UsageEventStatus = "reversed"
)

type UsageEvent struct {
    EventID       string
    TenantID      string
    ModuleCode    string
    Metric        string
    Quantity      int64
    SourceType    string
    SourceID      string
    IdempotencyKey string
    Status        UsageEventStatus
    OccurredAt    time.Time
    ReversalOf    string
    Metadata      map[string]string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type ReserveUsageInput struct {
    TenantID       string
    ModuleCode     string
    Metric         string
    Quantity       int64
    PeriodKey      string
    SourceType     string
    SourceID       string
    IdempotencyKey string
    OccurredAt     time.Time
    Metadata       map[string]string
}

type ReserveUsageResult struct {
    Event          UsageEvent
    Existing       bool
    CommittedUsage int64
    ReservedUsage  int64
    Limit          *int64
}

type UsageOutboxItem struct {
    ID            int64
    EventID       string
    Destination   string
    Status        string
    Attempts      int
    NextAttemptAt *time.Time
    LastError     string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type UsageLedger interface {
    Reserve(ctx context.Context, input ReserveUsageInput) (ReserveUsageResult, error)
    Commit(ctx context.Context, eventID string) (UsageEvent, error)
    Release(ctx context.Context, eventID, reason string) (UsageEvent, error)
    Reverse(ctx context.Context, eventID, idempotencyKey, reason string) (UsageEvent, error)
    Get(ctx context.Context, tenantID, idempotencyKey string) (*UsageEvent, error)
    ListPendingOutbox(ctx context.Context, limit int) ([]UsageOutboxItem, error)
}
```

- [ ] **Step 1: Write failing validation tests.** Cover blank tenant/module/metric/idempotency key, zero quantity, invalid status transitions, negative quantity for job metrics, and a negative storage delta that would make projected usage less than zero.
- [ ] **Step 2: Run the focused tests and verify they fail.**

Run: `go test ./internal/listingsubscription -run 'TestUsageLedger|TestReserveUsage' -count=1`

Expected: FAIL because the new contract types and validation functions do not exist.
- [ ] **Step 3: Implement the minimal domain types and pure validation.** Normalize all string identifiers with `strings.TrimSpace`; clone metadata maps before storing; use `errors.Is`-compatible sentinel errors for invalid input, duplicate identity, invalid transition, and quota exceeded.
- [ ] **Step 4: Add state-machine transition tests.** Assert `reserved -> committed`, `reserved -> released`, `committed -> reversed`, and rejection of `released -> committed`, `committed -> released`, and a second reversal.
- [ ] **Step 5: Run the focused tests and verify they pass.**

Run: `go test ./internal/listingsubscription -run 'TestUsageLedger|TestReserveUsage' -count=1`

- [ ] **Step 6: Commit the contract slice.**

```powershell
git add internal/listingsubscription/types.go internal/listingsubscription/usage_ledger.go internal/listingsubscription/usage_ledger_test.go
git commit -m "feat: define subscription usage ledger contract"
```

### Task 2: Add durable ledger, bucket, and outbox schema

**Files:**
- Modify: `internal/listingsubscription/gorm_repository.go`
- Modify: `internal/listingkit/schema/runtime.go:115-125`
- Test: `internal/listingsubscription/gorm_usage_ledger_test.go`
- Test: `internal/listingkit/schema/runtime_test.go`

**Interfaces:**
- Consumes: Task 1 domain types and existing `GormRepository` database handle.
- Produces: durable tables and repository methods used by the service implementation.

Add focused GORM rows with these table names and constraints:

```text
saas_usage_events
  event_id primary key
  tenant_id, module_code, metric, quantity bigint
  period_key, source_type, source_id, idempotency_key
  status, occurred_at, reversal_of, metadata, created_at, updated_at
  unique (tenant_id, idempotency_key)

saas_usage_buckets
  tenant_id, module_code, period_key, metric composite primary key
  committed bigint not null default 0
  reserved bigint not null default 0
  updated_at

saas_usage_event_outbox
  id primary key
  event_id unique not null
  destination not null default 'openmeter'
  status not null default 'pending'
  attempts not null default 0
  next_attempt_at, last_error, created_at, updated_at
```

- [ ] **Step 1: Write migration-shape tests.** Open the existing SQLite test database, call `listingsubscription.AutoMigrateRepository`, and assert all three tables and the unique indexes exist. Add a runtime schema test proving `listingkitschema.AutoMigrateRuntime` includes the ledger migration and remains idempotent on a second run.
- [ ] **Step 2: Run the migration tests and verify they fail.**

Run: `go test ./internal/listingsubscription ./internal/listingkit/schema -run 'Test.*Ledger|Test.*Usage.*Schema' -count=1`

Expected: FAIL because the ledger rows are not registered.
- [ ] **Step 3: Implement the GORM rows and register them in `AutoMigrateRepository`.** Keep existing subscription tables unchanged. Add indexes for tenant/metric/status and outbox status/next-attempt ordering.
- [ ] **Step 4: Wire the existing ListingKit runtime schema migration.** Add the ledger migration to the existing subscription migration call; do not add a new command or auto-run migration from API startup.
- [ ] **Step 5: Run the migration tests and verify they pass.**

Run: `go test ./internal/listingsubscription ./internal/listingkit/schema -run 'Test.*Ledger|Test.*Usage.*Schema' -count=1`

- [ ] **Step 6: Commit the schema slice.**

```powershell
git add internal/listingsubscription/gorm_repository.go internal/listingsubscription/gorm_usage_ledger_test.go internal/listingkit/schema/runtime.go internal/listingkit/schema/runtime_test.go
git commit -m "feat: add usage ledger persistence schema"
```

### Task 3: Implement atomic reservation and idempotent transitions

**Files:**
- Create: `internal/listingsubscription/usage_ledger_gorm.go`
- Modify: `internal/listingsubscription/gorm_repository.go`
- Test: `internal/listingsubscription/gorm_usage_ledger_test.go`

**Interfaces:**
- Consumes: Task 1 `UsageLedger` contract and Task 2 rows.
- Produces: `NewGormUsageLedger(repo *GormRepository) UsageLedger` with transactional reservation, commit, release, reverse, lookup, and outbox listing.

- [ ] **Step 1: Write the duplicate and transition tests.** Assert that two `Reserve` calls with the same tenant and idempotency key return one event, only one bucket reservation, and `Existing == true` on the second call. Assert commit/release retries return the same final event without changing counters twice.
- [ ] **Step 2: Write the quota tests.** Seed an active entitlement with a limit, reserve up to the limit, reject the next positive reservation, allow a negative `storage_bytes_current` delta that stays non-negative, and reject a negative delta below zero. Assert error fields expose metric, limit, committed, and reserved values.
- [ ] **Step 3: Run the tests and verify they fail.**

Run: `go test ./internal/listingsubscription -run 'TestGormUsageLedger|TestUsageLedgerQuota|TestUsageLedgerIdempotency' -count=1`

Expected: FAIL because the GORM ledger implementation is absent.
- [ ] **Step 4: Implement `Reserve` in one database transaction.** Normalize input, load the entitlement, lock or create the bucket row, calculate `projected = committed + reserved + quantity`, enforce active/trialing status and the configured limit, insert the event with `reserved` status, increment bucket `reserved`, and insert one pending outbox row. On a unique-key conflict, load and return the existing event without changing the bucket.
- [ ] **Step 5: Implement `Commit` and `Release` idempotently.** Lock the event and bucket; only `reserved` may commit or release. Commit moves quantity from `reserved` to `committed` and sets `committed_at` through `updated_at`; release subtracts from reserved and stores a redacted reason in outbox metadata or audit payload. Repeating the same operation returns the existing terminal event.
- [ ] **Step 6: Implement `Reverse`.** Require a committed source event, create a new event with `ReversalOf` set to the source event and a unique idempotency key, apply the negative quantity to committed bucket usage in the same transaction, and mark the new event `reversed`; never mutate or delete the original committed event.
- [ ] **Step 7: Run the focused GORM tests and verify they pass.**

Run: `go test ./internal/listingsubscription -run 'TestGormUsageLedger|TestUsageLedgerQuota|TestUsageLedgerIdempotency' -count=1`

- [ ] **Step 8: Commit the durable implementation.**

```powershell
git add internal/listingsubscription/usage_ledger_gorm.go internal/listingsubscription/gorm_repository.go internal/listingsubscription/gorm_usage_ledger_test.go
git commit -m "feat: enforce atomic subscription usage reservations"
```

### Task 4: Add deterministic in-memory ledger and concurrency proof

**Files:**
- Create: `internal/listingsubscription/usage_ledger_mem.go`
- Test: `internal/listingsubscription/usage_ledger_mem_test.go`
- Modify: `internal/listingsubscription/mem_repository.go` only if the existing test fixture needs a shared entitlement setup helper.

**Interfaces:**
- Consumes: Task 1 `UsageLedger` contract.
- Produces: a mutex-protected `NewMemUsageLedger` for service and handler tests with the same transition and idempotency semantics as GORM.

- [ ] **Step 1: Write the concurrency test.** Start 20 goroutines reserving one unit against a limit of 10 with unique idempotency keys; assert exactly 10 succeed, the committed-plus-reserved total is 10, and no event is duplicated.
- [ ] **Step 2: Write replay tests.** Run the same idempotency key concurrently from 20 goroutines; assert all callers observe the same event ID and only one reservation exists.
- [ ] **Step 3: Run the tests and verify they fail.**

Run: `go test -race ./internal/listingsubscription -run 'TestMemUsageLedger|TestUsageLedgerConcurrent' -count=1`

Expected: FAIL because the in-memory ledger is absent.
- [ ] **Step 4: Implement the mutex-protected ledger.** Reuse the Task 1 validation and transition helpers; keep bucket keys and event identity maps explicit so tests mirror the durable model rather than using a separate simplified policy.
- [ ] **Step 5: Run race-enabled tests and verify they pass.**

Run: `go test -race ./internal/listingsubscription -run 'TestMemUsageLedger|TestUsageLedgerConcurrent' -count=1`

- [ ] **Step 6: Commit the deterministic test implementation.**

```powershell
git add internal/listingsubscription/usage_ledger_mem.go internal/listingsubscription/usage_ledger_mem_test.go internal/listingsubscription/mem_repository.go
git commit -m "test: prove usage ledger idempotency under concurrency"
```

### Task 5: Add service wiring and shadow outbox boundary without changing paid entrypoints

**Files:**
- Modify: `internal/listingsubscription/service.go`
- Modify: `internal/listingsubscription/types.go`
- Create: `internal/listingsubscription/usage_outbox.go`
- Test: `internal/listingsubscription/service_usage_ledger_test.go`
- Test: `internal/listingkit/api/subscription_guard_test.go`

**Interfaces:**
- Consumes: Task 3/4 `UsageLedger` implementations.
- Produces: an optional ledger on `Service`, explicit `ReserveUsage`, `CommitUsage`, `ReleaseUsage`, and `ReverseUsage` methods for PAY-042; existing `CheckUsage` and `RecordUsage` remain behaviorally unchanged.

- [ ] **Step 1: Write service wiring tests.** Assert `NewServiceWithLedger(repo, ledger)` rejects nil dependencies, exposes the ledger methods, and leaves a service created by existing `NewService(repo)` on the legacy counter path.
- [ ] **Step 2: Write outbox tests.** Assert a successful local commit creates exactly one pending OpenMeter outbox item, a retry reads the same event identity, and a failed delivery leaves the local committed event intact for later retry.
- [ ] **Step 3: Run tests and verify they fail.**

Run: `go test ./internal/listingsubscription ./internal/listingkit/api -run 'Test.*Ledger|Test.*Outbox|Test.*SubscriptionGuard' -count=1`

Expected: FAIL because service wiring and outbox helpers are absent.
- [ ] **Step 4: Add optional ledger wiring.** Keep the existing constructor signature stable; add `NewServiceWithLedger` and delegate only explicit new methods to the ledger. Do not replace `CheckUsage`, `AuthorizeUsage`, `RecordUsage`, `requireSubscriptionUsage`, or any current handler call.
- [ ] **Step 5: Add redacted outbox payload construction.** Store only event identity, tenant-scoped metric, quantity, occurred time, and safe metadata required by the OpenMeter adapter; reject secrets, authorization headers, provider credentials, and arbitrary request bodies.
- [ ] **Step 6: Run tests and verify they pass.**

Run: `go test ./internal/listingsubscription ./internal/listingkit/api -run 'Test.*Ledger|Test.*Outbox|Test.*SubscriptionGuard' -count=1`

- [ ] **Step 7: Commit the compatibility boundary.**

```powershell
git add internal/listingsubscription/service.go internal/listingsubscription/types.go internal/listingsubscription/usage_outbox.go internal/listingsubscription/service_usage_ledger_test.go internal/listingkit/api/subscription_guard_test.go
git commit -m "feat: expose usage ledger without changing paid entrypoints"
```

### Task 6: Add migration, reconciliation, and PAY-042 handoff evidence

**Files:**
- Create: `docs/architecture/pay-041-usage-ledger.md`
- Modify: `docs/product/listingkit-paid-pilot-execution-plan.md` only to link the completed PAY-041 evidence and keep PAY-042 as the next gate.
- Test: `internal/listingsubscription/usage_ledger_reconciliation_test.go`

**Interfaces:**
- Consumes: Task 3 ledger and outbox query methods.
- Produces: documented rebuild/reconciliation commands and an explicit handoff contract for PAY-042.

- [ ] **Step 1: Write reconciliation tests.** Seed committed events, bucket totals, a duplicate idempotency attempt, a released event, and an outbox failure; assert the reconciliation report identifies zero false positives and reports each mismatch category with tenant, metric, event ID, and safe reason.
- [ ] **Step 2: Run the tests and verify they fail.**

Run: `go test ./internal/listingsubscription -run 'TestUsageLedgerReconciliation' -count=1`

Expected: FAIL because the report builder is absent.
- [ ] **Step 3: Implement report generation and a dry-run command/helper.** Compare bucket committed totals against committed event sums, reserved totals against reserved event sums, and outbox states against event states. Never mutate production data in the dry-run path.
- [ ] **Step 4: Document the exact PAY-042 handoff.** List the entrypoints requiring reservation/commit integration: Studio design jobs, product image jobs, SHEIN save draft, SHEIN publish, storage upload/delete, and batch/recovery paths. Record that failures, cancellations, platform rejection, and unknown remote status do not commit customer usage.
- [ ] **Step 5: Run the full ledger and repository gates.**

Run:

```powershell
go test ./internal/listingsubscription ./internal/listingkit/schema ./internal/listingkit/api -count=1
go test -race ./internal/listingsubscription -count=1
./scripts/test-all.ps1 -count=1
```

- [ ] **Step 6: Commit the evidence and handoff.**

```powershell
git add docs/architecture/pay-041-usage-ledger.md docs/product/listingkit-paid-pilot-execution-plan.md internal/listingsubscription/usage_ledger_reconciliation_test.go
git commit -m "docs: record PAY-041 usage ledger handoff"
```

## Completion Gates

- Duplicate business events produce one ledger event and one outbox item under concurrent replay.
- A hard quota cannot be exceeded by concurrent reservations in the durable repository test.
- Commit, release, and reverse operations are auditable and idempotent.
- Existing paid entrypoints still use the legacy path; no production behavior changes until PAY-042.
- Local ledger totals reconcile from events without silent `RecordUsage` errors.
- No payment provider, checkout, invoice, tax, or customer billing API is added by this plan.

## Explicitly Deferred to Separate Plans

- PAY-042: wire the ledger into every paid entrypoint and remove ignored usage-write errors.
- PAY-043: manual commercial subscription registry and payment-provider adapter/webhook state machine.
- PAY-044: scheduled reconciliation, export, manual adjustments, and billing support reports.
- OpenMeter production deployment, capacity planning, retention, and SLO design.


