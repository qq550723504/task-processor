# ListingKit Scoped System-Owned Exceptions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an auditable database-backed exception registry so only the 312 groups from report `648cdfab03c4` are classified as system-owned, while all future unmapped candidates remain release-blocking.

**Architecture:** A fixed ListingKit schema table stores active exception keys `(table_name, tenant_fingerprint, candidate_fingerprint)` plus the approving report fingerprint and reason. Owner reconciliation loads the active keys once per scan and classifies exact matches as system-owned; all non-matching unmapped candidates remain findings. A one-shot operator command revalidates the live report and inserts only the approved 312 keys, and the existing bounded ApplyUnique path then fills only the remaining verified mappings.

**Tech Stack:** Go, `database/sql`, GORM schema migration, PostgreSQL, sqlmock, PowerShell wrapper, existing owner-reconciliation runtime.

## Global Constraints

- Never update `creator`, `created_by`, `owner_user_id`, or any other business row while seeding exceptions.
- Exception matching is exact and fingerprint-only; raw tenant IDs, legacy user IDs, candidate values, and tokens never enter reports or logs.
- A missing/failed exception-table read fails closed in release preflight.
- Only report `648cdfab03c4` and its 312 `unmapped_candidate` groups may be seeded in this change.
- Future unmapped candidates remain blocking unless explicitly added through the same revalidated command.

---

### Task 1: Exception model and read-only store

**Files:**
- Create: `internal/listingkit/ownerreconcile/exceptions.go`
- Create: `internal/listingkit/ownerreconcile/exceptions_test.go`

**Interfaces:**
- Produces `type SystemOwnedException struct { Table, TenantFingerprint, CandidateFingerprint, ReportFingerprint, Reason string }`.
- Produces `type ExceptionStore interface { ListActive(ctx context.Context) ([]SystemOwnedException, error) }`.
- Produces `NewPostgresExceptionStore(*sql.DB) ExceptionStore` and a deterministic in-memory store for unit tests.

- [ ] **Step 1: Write the failing tests**

Add tests proving that a store loads active rows, rejects empty fingerprints, and returns a sanitized error when the registry query fails.

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/listingkit/ownerreconcile -run 'Test(PostgresExceptionStore|Exception)' -count=1`

Expected: FAIL because the store types and implementation do not exist.

- [ ] **Step 3: Implement the minimal store**

Use one fixed `SELECT table_name, tenant_fingerprint, candidate_fingerprint, report_fingerprint, reason FROM listingkit_owner_scope_system_owned_exceptions WHERE active = TRUE ORDER BY table_name, tenant_fingerprint, candidate_fingerprint`; validate all returned values before returning them.

- [ ] **Step 4: Run tests to verify GREEN**

Run the same focused command; expected PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/ownerreconcile/exceptions.go internal/listingkit/ownerreconcile/exceptions_test.go
git commit -m "feat(owner-reconcile): add scoped exception store"
```

### Task 2: Exact exception classification in reconciliation

**Files:**
- Modify: `internal/listingkit/ownerreconcile/repository.go`
- Modify: `internal/listingkit/ownerreconcile/candidates.go`
- Modify: `internal/listingkit/ownerreconcile/repository_test.go`
- Modify: `internal/listingkit/ownerreconcile/candidates_test.go`

**Interfaces:**
- Extend `Repository` with `Exceptions ExceptionStore`.
- `DryRun` loads and indexes active exceptions once, then classifies only exact table/tenant/candidate fingerprint matches as `system_owned`.

- [ ] **Step 1: Write the failing tests**

Add cases showing an exact exception removes a finding and does not create a resolution, while a different tenant, table, candidate fingerprint, or report fingerprint still produces `unmapped_candidate`.

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/listingkit/ownerreconcile -run 'Test(DryRun|ApplyUnique|Exception)' -count=1`

Expected: FAIL because `Repository` ignores exceptions.

- [ ] **Step 3: Implement minimal classification**

Compute the existing candidate fingerprint before classification; look up the exact key in an in-memory exception index. Exempted groups contribute only to `SystemOwnedFindings`; they never contribute to `AutoRows`, `Resolutions`, or `plannedGroup` entries.

- [ ] **Step 4: Run focused and package tests**

Run: `go test ./internal/listingkit/ownerreconcile -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/ownerreconcile/repository.go internal/listingkit/ownerreconcile/candidates.go internal/listingkit/ownerreconcile/*_test.go
git commit -m "feat(owner-reconcile): classify approved groups as system-owned"
```

### Task 3: Schema migration for the registry

**Files:**
- Modify: `internal/listingkit/schema/runtime.go`
- Create: `internal/listingkit/schema/system_owned_exceptions.go`
- Create: `internal/listingkit/schema/system_owned_exceptions_test.go`

- [ ] **Step 1: Write the failing migration test**

Assert the migration emits the fixed table name, the unique key on table/tenant/candidate fingerprints, the active index, and no business-table updates.

- [ ] **Step 2: Run the migration test to verify RED**

Run: `go test ./internal/listingkit/schema -run 'Test.*SystemOwnedException' -count=1`

Expected: FAIL because the migration is absent.

- [ ] **Step 3: Implement the migration**

Add a fixed GORM/raw-SQL migration that creates `listingkit_owner_scope_system_owned_exceptions` with the audited columns and indexes; invoke it from `schema.AutoMigrateRuntime`.

- [ ] **Step 4: Run schema and affected package tests**

Run: `go test ./internal/listingkit/schema ./internal/listingkit/ownerreconcile -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/schema
git commit -m "feat(schema): add owner exception registry"
```

### Task 4: Seed command with live report revalidation

**Files:**
- Create: `cmd/listingkit-owner-scope-exceptions/main.go`
- Create: `internal/app/runtime/listingkitownerexceptions/runtime.go`
- Create: `internal/app/runtime/listingkitownerexceptions/options.go`
- Create: `internal/app/runtime/listingkitownerexceptions/runtime_test.go`
- Create: `scripts/listingkit-owner-scope-exceptions.ps1`

**Interfaces:**
- CLI requires `--config`, `--report`, and `--confirm-report`.
- It loads the current report file, requires fingerprint `648cdfab03c4`, reruns read-only reconciliation against the live database, requires an identical fingerprint and exactly 312 `unmapped_candidate` groups, then inserts only those keys in one transaction with `ON CONFLICT DO NOTHING`.

- [ ] **Step 1: Write failing runtime tests**

Cover missing confirmation, changed report fingerprint, changed live report, non-312 group set, duplicate seed idempotence, and successful insert with no business-table SQL.

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./internal/app/runtime/listingkitownerexceptions -count=1`

Expected: FAIL because the command and runtime are absent.

- [ ] **Step 3: Implement the command and wrapper**

Use the existing non-validating config loader and strict non-creating DB connector. Keep all SQL identifiers fixed; use parameters for fingerprints and reason. Emit only `seeded_groups`, `report`, and `rows`.

- [ ] **Step 4: Run focused runtime and script syntax tests**

Run: `go test ./internal/app/runtime/listingkitownerexceptions ./internal/listingkit/ownerreconcile -count=1` and `pwsh -NoProfile -Command "[void][System.Management.Automation.Language.Parser]::ParseFile('scripts/listingkit-owner-scope-exceptions.ps1',[ref]$null,[ref]$null)"`.

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add cmd/listingkit-owner-scope-exceptions internal/app/runtime/listingkitownerexceptions scripts/listingkit-owner-scope-exceptions.ps1
git commit -m "feat(owner-reconcile): add reviewed exception seeder"
```

### Task 5: Wire preflight and owner-reconcile runtimes

**Files:**
- Modify: `internal/app/runtime/listingkitidentitypreflight/runtime.go`
- Modify: `internal/app/runtime/listingkitownerreconcile/runtime.go`
- Modify: corresponding runtime tests.

- [ ] **Step 1: Write failing wiring tests**

Assert both runtimes construct repositories with the exception store, use the non-validating loader, and fail closed when the exception table query fails.

- [ ] **Step 2: Run RED tests**

Run: `go test ./internal/app/runtime/listingkitidentitypreflight ./internal/app/runtime/listingkitownerreconcile -run 'Test.*Exception' -count=1`

Expected: FAIL because runtime wiring does not pass an exception store.

- [ ] **Step 3: Implement wiring**

Construct `NewPostgresExceptionStore` from the same read-only owner `*sql.DB` and pass it to the repository. Keep directory initialization after reconciliation and preserve existing error sanitization.

- [ ] **Step 4: Run affected tests and vet**

Run: `go test ./internal/app/runtime/listingkitidentitypreflight ./internal/app/runtime/listingkitownerreconcile ./internal/listingkit/identitypreflight ./internal/listingkit/ownerreconcile -count=1` and `go vet ./internal/app/runtime/listingkitidentitypreflight ./internal/app/runtime/listingkitownerreconcile ./internal/listingkit/ownerreconcile`.

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/app/runtime/listingkitidentitypreflight internal/app/runtime/listingkitownerreconcile
git commit -m "feat(identity-preflight): load scoped owner exceptions"
```

### Task 6: Seed the approved production snapshot and apply safe mappings

**Files:**
- Use: `.local/tmp/owner-scope-current.json` (read-only input, report `648cdfab03c4`).
- Use: `scripts/listingkit-owner-scope-exceptions.ps1`.
- Use: `scripts/listingkit-owner-scope-dry-run.ps1` with `-Execute` only after seeding.

- [ ] **Step 1: Run the seeder in production read-only validation mode**

Use the existing PostgreSQL port-forward and non-validating config; require the exact confirmation `648cdfab03c4`. Expected: live report matches and the command is ready to insert 312 keys.

- [ ] **Step 2: Insert the 312 exception keys**

Run the same command in apply mode. Expected: only the exception registry is written; business-table row counts remain unchanged.

- [ ] **Step 3: Re-run dry-run and verify classification**

Expected: `unresolved_rows=0`, `system_owned_rows=1,150,060` (existing 275,169 plus 874,891), and `auto_rows=6,992,644`.

- [ ] **Step 4: Apply only verified auto resolutions**

Run `scripts/listingkit-owner-scope-dry-run.ps1 -Execute -ConfirmReport <fresh fingerprint>` with a bounded batch size. Expected: only mapped `owner_user_id` values are filled; exception groups remain untouched.

- [ ] **Step 5: Verify final dry-run**

Expected: `unresolved_rows=0`, `auto_rows=0`, and the exception groups remain system-owned. Stop immediately on any fingerprint or count mismatch.

### Task 7: Verify release gate and deployment

**Files:**
- Modify: documentation for the required schema migration/exception seeding order.
- Test: existing owner-reconcile, identity-preflight, CI driver, and Kubernetes dry-run suites.

- [ ] **Step 1: Run focused verification**

Run the affected Go tests, `go vet`, `go test ./tests -count=1`, and `git diff --check`.

- [ ] **Step 2: Run the new preflight against production**

Expected: `status=ok identity_preflight=passed`, with no directory request before reconciliation completion.

- [ ] **Step 3: Trigger the immutable API deployment workflow**

Use the merged SHA and verify the preflight Job succeeds before the single immutable API apply.

- [ ] **Step 4: Verify rollout and invariants**

Check the running API image digest, Job logs, owner report counts, and that no business rows were deleted.

- [ ] **Step 5: Commit documentation and final evidence**

```powershell
git add docs deployments/kubernetes/listingkit-workbench/README.md
git commit -m "docs: record scoped owner exception release procedure"
```
