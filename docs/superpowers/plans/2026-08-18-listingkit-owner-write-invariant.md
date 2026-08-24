# ListingKit Owner Write Invariant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every new or updated row in ListingKit's legacy owner-scoped tables require a non-empty canonical `owner_user_id`, while preserving the existing system-owned policy for native tables.

**Architecture:** Add one owner-resolution boundary in `internal/listingadmin` that prefers the verified request identity, accepts an explicit owner only for trusted internal calls, trims it, and returns `ErrOwnerUserIDRequired` when absent. Route every owner-scoped repository write through that boundary, route local task-RPC task creation through the import-task repository after deriving the owner from its trusted store parent, and add PostgreSQL `CHECK ... NOT VALID` constraints as the final defense. Keep the existing owner reconciliation backfill as a separately authorized operational action.

**Tech Stack:** Go, GORM, Gin request contexts, PostgreSQL, SQLite test databases, `sqlmock`, existing owner-reconciliation inventory and reports.

**Spec:** `docs/superpowers/specs/2026-08-18-listingkit-owner-write-invariant-design.md`

## Global Constraints

- A missing canonical subject must return an error before any owner-scoped insert or update.
- A verified request identity takes precedence over a model or DTO owner value; HTTP clients cannot select another owner through the payload.
- Internal/background writers must pass an explicit canonical owner or derive it from a trusted persisted parent; they must not depend on an empty HTTP context.
- Native system-owned tables are not changed to require a fabricated user identity.
- The preflight gate continues to block both unresolved rows and auto-resolvable rows.
- Historical production backfill, constraint validation, deployment, rerun, merge, and PR publication remain separate authorized operations.
- No new dependency is introduced; reuse the existing GORM repositories, owner reconciliation, and test database helpers.

---

### Task 1: Establish the shared owner-resolution contract

**Files:**
- Create: `internal/listingadmin/owner_scope_test.go`
- Modify: `internal/listingadmin/access_scope.go:35-95`

**Interfaces:**
- Produces `var ErrOwnerUserIDRequired = errors.New("owner user id is required")`.
- Produces `func WithOwnerUserID(ctx context.Context, ownerUserID string) context.Context` for trusted internal callers.
- Produces `func requireOwnerUserID(ctx context.Context, explicitOwner string) (string, error)` for owner-scoped repositories.

- [ ] **Step 1: Write failing contract tests**

```go
func TestRequireOwnerUserIDRejectsMissingAndWhitespaceOwners(t *testing.T) {
	for _, explicit := range []string{"", " ", "\t\n"} {
		if got, err := requireOwnerUserID(context.Background(), explicit); !errors.Is(err, ErrOwnerUserIDRequired) || got != "" {
			t.Fatalf("requireOwnerUserID(%q) = %q, %v; want empty owner error", explicit, got, err)
		}
	}
}

func TestRequireOwnerUserIDUsesVerifiedContextIdentityOverPayload(t *testing.T) {
	ctx := withRequestUserID(context.Background(), " verified-sub ")
	got, err := requireOwnerUserID(ctx, "payload-subject")
	if err != nil || got != "verified-sub" {
		t.Fatalf("requireOwnerUserID() = %q, %v; want verified-sub", got, err)
	}
}

func TestRequireOwnerUserIDAcceptsExplicitTrustedInternalOwner(t *testing.T) {
	got, err := requireOwnerUserID(context.Background(), " internal-sub ")
	if err != nil || got != "internal-sub" {
		t.Fatalf("requireOwnerUserID() = %q, %v; want internal-sub", got, err)
	}
}

func TestWithOwnerUserIDSuppliesInternalOwner(t *testing.T) {
	got, err := requireOwnerUserID(WithOwnerUserID(context.Background(), "job-sub"), "")
	if err != nil || got != "job-sub" {
		t.Fatalf("requireOwnerUserID(WithOwnerUserID()) = %q, %v; want job-sub", got, err)
	}
}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/listingadmin -run 'TestRequireOwnerUserID|TestWithOwnerUserID' -count=1`

Expected: FAIL because `ErrOwnerUserIDRequired`, `WithOwnerUserID`, and `requireOwnerUserID` do not yet exist.

- [ ] **Step 3: Implement the minimal shared contract**

Use the existing `requestUserIDFromContext` and `withRequestUserID`; do not create a second identity context key:

```go
var ErrOwnerUserIDRequired = errors.New("owner user id is required")

func WithOwnerUserID(ctx context.Context, ownerUserID string) context.Context {
	return withRequestUserID(ctx, ownerUserID)
}

func requireOwnerUserID(ctx context.Context, explicitOwner string) (string, error) {
	if owner := strings.TrimSpace(requestUserIDFromContext(ctx)); owner != "" {
		return owner, nil
	}
	if owner := strings.TrimSpace(explicitOwner); owner != "" {
		return owner, nil
	}
	return "", ErrOwnerUserIDRequired
}
```

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `go test ./internal/listingadmin -run 'TestRequireOwnerUserID|TestWithOwnerUserID' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the shared contract**

```bash
git add internal/listingadmin/access_scope.go internal/listingadmin/owner_scope_test.go
git commit -m "feat(listingkit): require canonical owner for writes"
```

### Task 2: Close owner gaps in the ListingKit admin repositories

**Files:**
- Modify: `internal/listingadmin/store_repository.go`
- Modify: `internal/listingadmin/category_repository.go`
- Modify: `internal/listingadmin/filter_rule_repository.go`
- Modify: `internal/listingadmin/generation_topic_policy_repository.go`
- Modify: `internal/listingadmin/generation_topic_override_repository.go`
- Modify: `internal/listingadmin/operation_strategy_repository.go`
- Modify: `internal/listingadmin/pricing_rule_repository.go`
- Modify: `internal/listingadmin/profit_rule_repository.go`
- Modify: `internal/listingadmin/scheduled_task_config_repository.go`
- Modify: `internal/listingadmin/sensitive_word_repository.go`
- Create: `internal/listingadmin/owner_scoped_repository_test.go`

**Interfaces:**
- Each owner-scoped create/upsert method resolves its owner with `requireOwnerUserID` before GORM `Create` or `Save`.
- Existing HTTP handlers continue to populate request identity; internal tests and callers can use `WithOwnerUserID`.
- `SaveActivityStrategy` must either require the initiating owner or be moved out of the legacy owner-scoped inventory after confirming its shared semantics. The implementation must not leave a blank owner path.

- [ ] **Step 1: Write failing repository tests for missing-owner rejection**

Use the existing SQLite migration helpers and repository constructors. In `owner_scoped_repository_test.go`, create one test per repository family so the test names identify the exact write path: `TestCreateStoreRejectsOwnerlessWrite`, `TestCreateCategoryRejectsOwnerlessWrite`, `TestCreateFilterRuleRejectsOwnerlessWrite`, `TestCreateGenerationTopicPolicyRejectsOwnerlessWrite`, `TestCreateGenerationTopicOverrideRejectsOwnerlessWrite`, `TestCreateOperationStrategyRejectsOwnerlessWrite`, `TestCreatePricingRuleRejectsOwnerlessWrite`, `TestCreateProfitRuleRejectsOwnerlessWrite`, `TestUpsertScheduledTaskConfigRejectsOwnerlessWrite`, and `TestCreateSensitiveWordRejectsOwnerlessWrite`. Each test calls its repository method with `context.Background()` and no explicit owner, asserts `errors.Is(err, ErrOwnerUserIDRequired)`, and asserts the target table count remains zero.

The test table should make the invariant visible without hiding the method-specific setup:

```go
func newOwnerScopedSQLiteDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate owner-scoped model: %v", err)
	}
	return db
}

func TestCreateStoreRejectsOwnerlessWrite(t *testing.T) {
	db := newOwnerScopedSQLiteDB(t, &listingStore{})
	repo := NewGormStoreRepository(db)
	_, err := repo.CreateStore(context.Background(), &Store{TenantID: 101, StoreID: "store-1", Name: "Store"})
	if !errors.Is(err, ErrOwnerUserIDRequired) {
		t.Fatalf("CreateStore() error = %v, want ErrOwnerUserIDRequired", err)
	}
	var count int64
	db.Table("listing_store").Count(&count)
	if count != 0 {
		t.Fatalf("listing_store rows = %d, want 0", count)
	}
}
```

Repeat the same shape with the concrete constructor/model fixture for each named repository test; do not combine unrelated write paths behind reflection.

Also add one positive case using `WithOwnerUserID(context.Background(), "internal-sub")` and verify the persisted owner is exactly `internal-sub`.

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/listingadmin -run TestOwnerScopedCreatesRejectOwnerlessWrites -count=1`

Expected: FAIL because current repositories still call `Create` after an empty request identity.

- [ ] **Step 3: Implement the minimal repository changes**

At the beginning of each create/upsert method, resolve and assign the owner before building the insert/update map:

```go
ownerUserID, err := requireOwnerUserID(ctx, row.OwnerUserID)
if err != nil {
	return nil, err
}
applyStoreCreateAuditFields(&row, ownerUserID)
```

Use the corresponding existing audit helper for each model. For update methods, resolve the trusted request owner or the already-loaded explicit owner before putting `owner_user_id` into the update map. Preserve existing authorization and tenant-scoping behavior. Do not add a new repository abstraction for each table.

- [ ] **Step 4: Run the repository tests to verify they pass**

Run: `go test ./internal/listingadmin -run 'TestOwnerScopedCreatesRejectOwnerlessWrites|TestGorm.*(Create|Upsert)' -count=1`

Expected: PASS, with the existing repository lifecycle tests still green.

- [ ] **Step 5: Commit the admin repository changes**

```bash
git add internal/listingadmin/*repository.go internal/listingadmin/owner_scoped_repository_test.go
git commit -m "fix(listingkit): reject ownerless admin writes"
```

### Task 3: Make import-task, mapping, and product-data writes owner-safe and atomic

**Files:**
- Modify: `internal/listingadmin/import_task_repository.go`
- Modify: `internal/listingadmin/product_import_mapping_repository.go`
- Modify: `internal/listingadmin/product_data_repository.go`
- Modify: `internal/listingadmin/import_task_dispatch_test.go`
- Modify: `internal/listingadmin/product_data_query_test.go`
- Create: `internal/listingadmin/owner_scoped_batch_test.go`

**Interfaces:**
- `BatchCreateImportTasks` rejects the entire batch if any row has no owner and does so before its duplicate check or insert.
- `UpsertProductDataBatch` rejects an ownerless item before any item is written; the method must not partially persist a batch.
- Product-import-mapping keeps its existing sentinel behavior, but uses the shared resolver and applies the verified request identity over a payload owner.

- [ ] **Step 1: Write failing batch tests**

Add these cases:

```go
func TestBatchCreateImportTasksRejectsOwnerlessBatchBeforeInsert(t *testing.T) {
	// Seed/migrate the import-task table, submit two tasks with context.Background(),
	// and assert ErrOwnerUserIDRequired plus COUNT(*) = 0.
}

func TestUpsertProductDataBatchRejectsOwnerlessItemWithoutPartialWrite(t *testing.T) {
	// Submit one valid item followed by one ownerless item and assert the error
	// plus COUNT(*) = 0; a failed batch must not leave the valid prefix behind.
}

func TestProductImportMappingRequestIdentityCannotBeOverriddenByPayload(t *testing.T) {
	// Create with context identity "verified-sub" and payload owner "other-sub";
	// assert persisted owner is "verified-sub".
}
```

- [ ] **Step 2: Run the batch tests to verify they fail**

Run: `go test ./internal/listingadmin -run 'TestBatchCreateImportTasksRejectsOwnerlessBatchBeforeInsert|TestUpsertProductDataBatchRejectsOwnerlessItemWithoutPartialWrite|TestProductImportMappingRequestIdentityCannotBeOverriddenByPayload' -count=1`

Expected: FAIL because import-task and product-data paths currently continue with blank owners, and product mapping accepts an explicit payload owner before checking the request identity.

- [ ] **Step 3: Implement preflight validation and transaction boundaries**

Validate all rows before any database query or write:

```go
for i := range tasks {
	ownerUserID, err := requireOwnerUserID(ctx, tasks[i].OwnerUserID)
	if err != nil {
		return nil, fmt.Errorf("import task %d: %w", i, err)
	}
	tasks[i].OwnerUserID = ownerUserID
}
```

For product-data upsert, validate and normalize the complete input slice first, then execute the existing per-row operations inside the existing transaction or a new transaction around the whole batch. Preserve duplicate/idempotency behavior. For product-import-mapping, use the shared resolver in create and update so a verified request identity always wins.

- [ ] **Step 4: Run the batch and existing repository tests**

Run: `go test ./internal/listingadmin -run '(TestBatchCreateImportTasks|TestUpsertProductDataBatch|Test.*ProductImportMapping|TestGormImportTaskRepository)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the batch write changes**

```bash
git add internal/listingadmin/import_task_repository.go internal/listingadmin/product_import_mapping_repository.go internal/listingadmin/product_data_repository.go internal/listingadmin/*repository_test.go internal/listingadmin/owner_scoped_batch_test.go
git commit -m "fix(listingkit): make import and product writes owner-safe"
```

### Task 4: Remove the local task-RPC direct-write bypass

**Files:**
- Modify: `internal/taskrpcapi/types.go` only if an explicit owner must be added to the internal RPC contract; prefer parent derivation when the store is authoritative
- Modify: `internal/listingruntime/local/local_task_rpc_provider.go`
- Modify: `internal/listingruntime/local/local_task_rpc_provider_test.go`
- Create: `internal/listingadmin/import_task_owner.go`
- Create: `internal/listingadmin/import_task_owner_test.go`

**Interfaces:**
- Local task submission no longer calls `p.db.Table("listing_product_import_task").Create` directly.
- The provider derives a non-empty owner from the same-tenant `listing_store` parent, then delegates creation to `GormImportTaskRepository.BatchCreateImportTasks`.
- Missing or cross-tenant store returns the existing `ErrStoreNotFound`; a found store with a blank owner returns `ErrOwnerUserIDRequired`; neither case persists a task.

- [ ] **Step 1: Write failing local-RPC tests**

Add cases to `local_task_rpc_provider_test.go`:

```go
func TestLocalTaskRPCSubmitTaskDerivesOwnerFromStoreAndUsesRepository(t *testing.T) {
	// Seed listing_store with tenant_id=246, id=986, owner_user_id="store-owner".
	// Submit a task and assert listing_product_import_task.owner_user_id is store-owner.
}

func TestLocalTaskRPCSubmitTaskRejectsStoreWithoutOwner(t *testing.T) {
	// Seed the same store with NULL/blank owner_user_id, submit, and assert the
	// owner-required error plus zero new task rows.
}
```

- [ ] **Step 2: Run the local-RPC tests to verify they fail**

Run: `go test ./internal/listingruntime/local -run 'TestLocalTaskRPCSubmitTask(DerivesOwnerFromStoreAndUsesRepository|RejectsStoreWithoutOwner)' -count=1`

Expected: FAIL because the current path writes `localImportTaskRow` directly and never populates `owner_user_id`.

- [ ] **Step 3: Implement parent-derived ownership and repository delegation**

Use `ResolveStoreOwnerUserID(ctx, p.db, req.TenantID, req.StoreID)` for a constrained lookup by both tenant and store ID, normalize the owner, convert the request to the existing `listingadmin.ImportTask`, and call `NewGormImportTaskRepository(p.db).BatchCreateImportTasks` with `WithOwnerUserID(ctx, owner)`. Keep the current task response mapping and uniqueness error translation. Do not duplicate import-task insert SQL in the local package.

- [ ] **Step 4: Run all local task-RPC tests**

Run: `go test ./internal/listingruntime/local -run 'TestLocalTaskRPC' -count=1`

Expected: PASS, including the existing platform-routing and uniqueness tests.

- [ ] **Step 5: Commit the bypass removal**

```bash
git add internal/listingruntime/local/local_task_rpc_provider.go internal/listingruntime/local/local_task_rpc_provider_test.go internal/listingadmin/import_task_owner.go internal/listingadmin/import_task_owner_test.go internal/taskrpcapi/types.go
git commit -m "fix(listingkit): route local task rpc through owner-safe repository"
```

### Task 5: Add database-level owner checks without breaking historical rollout

**Files:**
- Modify: `internal/listingadmin/schema_migrate.go`
- Modify: `internal/listingadmin/schema_migrate_test.go`
- Create: `internal/listingadmin/owner_constraint.go`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md` to document post-backfill validation SQL

**Interfaces:**
- `ensureOwnerAuditColumns` installs an idempotent PostgreSQL check for owner-scoped tables only:

```sql
CHECK (NULLIF(BTRIM(owner_user_id::text), '') IS NOT NULL) NOT VALID
```

- SQLite and other existing test dialects keep their current migration behavior; application-level tests still enforce the invariant.
- Documentation lists the controlled validation step after the historical backfill and does not make deployment silently mutate production data.

- [ ] **Step 1: Write failing migration tests**

Add `TestEnsureOwnerAuditColumnsAddsPostgresOwnerCheckNotValid` using a PostgreSQL `sqlmock` database. Expect the existing `HasColumn` probes, owner index statement, and the idempotent `ADD CONSTRAINT ... NOT VALID` statement for `listing_store`. Add `TestAutoMigrateOwnerScopedRepositoryIsIdempotentOnSQLite` that runs the normal store/import-task auto-migrations twice and verifies both calls succeed.

- [ ] **Step 2: Run migration tests to verify the new behavior is absent**

Run: `go test ./internal/listingadmin -run 'Test.*Owner.*Constraint|TestAutoMigrate.*ImportTaskRepository' -count=1`

Expected: the new PostgreSQL expectation fails because no owner constraint is currently installed.

- [ ] **Step 3: Implement the idempotent PostgreSQL constraint**

Use the fixed internal table inventory, a stable constraint name such as `ck_<table>_owner_user_id_nonblank`, and a PostgreSQL-only `DO` block guarded by `pg_constraint`. Keep `NOT VALID` so existing 2,500 historical rows do not break schema migration; PostgreSQL still enforces the check for every new insert/update. Do not add constraints to native system-owned tables.

- [ ] **Step 4: Add the controlled validation procedure to operations documentation**

Document that after the reconciliation report is zero, an operator validates each named constraint with:

```sql
ALTER TABLE "<owner-scoped-table>"
  VALIDATE CONSTRAINT "ck_<owner-scoped-table>_owner_user_id_nonblank";
```

The deployment workflow must not execute this validation until the separately authorized backfill has completed.

- [ ] **Step 5: Run migration tests and commit**

Run: `go test ./internal/listingadmin -run 'Test.*Owner.*Constraint|TestAutoMigrate.*ImportTaskRepository' -count=1`

Expected: PASS.

```bash
git add internal/listingadmin/schema_migrate.go internal/listingadmin/schema_migrate_test.go internal/listingadmin/owner_constraint.go deployments/kubernetes/listingkit-workbench/README.md
git commit -m "fix(listingkit): add database owner write guard"
```

### Task 6: Correct preflight diagnostics while keeping the fail-closed gate

**Files:**
- Modify: `internal/app/runtime/listingkitidentitypreflight/runtime.go`
- Modify: `internal/app/runtime/listingkitidentitypreflight/runtime_test.go`

**Interfaces:**
- The gate condition remains `UnresolvedRows > 0 || AutoRows > 0`.
- The blocked output names `unresolved_rows`, `auto_rows`, and `system_owned_rows` independently and retains the fingerprint.

- [ ] **Step 1: Write the failing output assertion**

Update the auto-row test to require:

```go
if !strings.Contains(output.String(), "status=blocked unresolved_rows=0 auto_rows=4 system_owned_rows=0") {
	t.Fatalf("output = %q, want distinct owner reconciliation counts", output.String())
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/runtime/listingkitidentitypreflight -run 'TestRunBlocksOnAutoResolvableOwnerRowsBeforeDirectory' -count=1`

Expected: FAIL because the current log contains `owner_reconciliation=unresolved rows=0`.

- [ ] **Step 3: Implement the diagnostic-only change**

Replace only the output format string; preserve the blocker condition, report fingerprinting, redaction, and directory short-circuit.

- [ ] **Step 4: Run the preflight package tests**

Run: `go test ./internal/app/runtime/listingkitidentitypreflight -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the preflight change**

```bash
git add internal/app/runtime/listingkitidentitypreflight/runtime.go internal/app/runtime/listingkitidentitypreflight/runtime_test.go
git commit -m "fix(identity-preflight): report owner row categories accurately"
```

### Task 7: Run staged verification and record the production boundary

**Files:**
- Modify: none unless a test exposes a contract mismatch; any such change must be added to the relevant task commit instead of hidden here

- [ ] **Step 1: Run focused owner-write tests**

Run: `go test ./internal/listingadmin ./internal/listingruntime/local ./internal/app/runtime/listingkitidentitypreflight ./internal/listingkit/ownerreconcile -count=1`

Expected: PASS.

- [ ] **Step 2: Run repository structure and formatting checks**

Run: `gofmt -w` on only the changed Go files, then `git diff --check` and the repository's existing structure test command if present.

Expected: no formatting or whitespace errors.

- [ ] **Step 3: Run the relevant full module tests**

Run: `GOWORK=off go test ./internal/listingadmin ./internal/listingruntime/local ./internal/app/runtime/listingkitidentitypreflight ./internal/listingkit/ownerreconcile -count=1`

Expected: PASS. If an unrelated baseline failure appears, report the exact package and failure without weakening this invariant.

- [ ] **Step 4: Review the final diff for bypasses**

Run: `rg -n 'Table\("listing_(store|category|filter_rule|generation_topic_override|generation_topic_policy|operation_strategy|pricing_rule|profit_rule|scheduled_task_config|sensitive_word|product_import_task|product_import_mapping|product_data)"\)\.(Create|Save|Updates|UpdateColumn)' internal`

Expected: no owner-scoped direct write remains outside the validated repository boundary; any intentional exception must be a read or a documented system-owned table.

- [ ] **Step 5: Commit verification-only adjustments if required and hand off**

Report the branch, commit list, focused test commands and results, and explicitly state that the 2,500 historical rows still require the separately authorized `ApplyUnique` reconciliation before constraint validation. Do not rerun the failed deployment or mutate production in this task.
