# ListingKit Owner Reconciliation Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make reconciliation and identity preflight enforce the approved creator/store/system-owner rules while keeping unresolved user-owned data fail-closed.

**Architecture:** Add fixed candidate policies to the owner-reconciliation inventory, reuse those policies to build preflight aggregate predicates, and keep migration recovery as a read-only classification/report until an explicit identity migration is approved. The release driver remains independent and only improves failed-Job reporting.

**Tech Stack:** Go, `database/sql`, PostgreSQL aggregate queries, PowerShell migration tooling, Bash/Kubernetes release driver, Go tests with `sqlmock`.

## Global Constraints

- `listing_product_import_mapping` derives ownership only from its related store.
- A non-empty `creator` is authoritative; `created_by` is used only when `creator` is blank.
- No-candidate rows are system-owned and excluded from owner-scope preflight, but remain in the reconciliation audit report.
- A non-empty unmapped creator never silently falls back to `created_by`.
- Legacy identity mappings use existing `yudao_tenant_id` and `yudao_user_id` metadata.
- No arbitrary subject assignment, compatibility username fallback, or deployment-gate bypass.
- Reconciliation writes remain report-fingerprint confirmed, bounded, and blocked while unresolved user-owned rows remain.

---

### Task 1: Encode deterministic candidate policies

**Files:**
- Modify: `internal/listingkit/ownerreconcile/repository.go`
- Modify: `internal/listingkit/ownerreconcile/inventory.go`
- Test: `internal/listingkit/ownerreconcile/candidates_test.go`
- Test: `internal/listingkit/ownerreconcile/inventory_test.go`

**Interfaces:**
- Add a fixed `CandidatePolicy` enum on `TableSpec` with creator-first, store-only, and system-owned modes.
- Preserve existing `CandidateColumn` and `Resolution` fingerprints so report confirmation remains stable for unchanged data.

- [ ] **Step 1: Write failing policy tests**

  Add tests proving creator wins over a different `created_by`, a non-empty unmapped creator does not fall back, and store-only policy ignores row/task candidates for `listing_product_import_mapping`.

- [ ] **Step 2: Run focused tests and verify RED**

  Run `go test ./internal/listingkit/ownerreconcile -run 'Test.*(Creator|Store|CandidatePolicy)' -count=1`.
  Expected: failure because current resolver treats all non-empty candidates equally.

- [ ] **Step 3: Implement minimal policy selection**

  Add deterministic candidate selection before `ResolveCandidates`: creator-first selects creator when non-empty and only considers created_by when creator is blank; store-only selects the related store creator/created_by pair; system-owned emits no user resolution candidate. Keep all SQL identifiers compile-time constants.

- [ ] **Step 4: Run focused tests and package regression**

  Run the focused command, then `go test ./internal/listingkit/ownerreconcile -count=1`.
  Expected: PASS.

- [ ] **Step 5: Commit the policy unit**

  ```bash
  git add internal/listingkit/ownerreconcile/repository.go internal/listingkit/ownerreconcile/inventory.go internal/listingkit/ownerreconcile/candidates_test.go internal/listingkit/ownerreconcile/inventory_test.go
  git commit -m "fix: enforce owner candidate precedence"
  ```

### Task 2: Classify system-owned rows without weakening the gate

**Files:**
- Modify: `internal/listingkit/ownerreconcile/report.go`
- Modify: `internal/listingkit/ownerreconcile/repository.go`
- Test: `internal/listingkit/ownerreconcile/report_test.go`
- Test: `internal/listingkit/ownerreconcile/repository_test.go`

**Interfaces:**
- Extend `ReportSummary` with system-owned group/row totals and keep `UnresolvedRows` limited to user-owned blockers.
- Add a separate redacted `SystemOwnedFindings` collection; do not encode system-owned records as unresolved `Finding` values.

- [ ] **Step 1: Write failing report tests**

  Assert that no-candidate rows appear in the redacted audit output, increment system-owned totals, and do not increment `UnresolvedRows`; assert that unmapped and conflicting rows still increment unresolved totals.

- [ ] **Step 2: Run focused tests and verify RED**

  Run `go test ./internal/listingkit/ownerreconcile -run 'Test.*(SystemOwned|Unresolved|Report)' -count=1`.
  Expected: failure because all current findings are counted as unresolved.

- [ ] **Step 3: Implement separate system-owned classification**

  Update report construction, sorting, fingerprinting, and JSON output so system-owned findings are deterministic and included in the report fingerprint, while only unresolved user-owned findings block `ApplyUnique`.

- [ ] **Step 4: Run focused and full owner-reconcile tests**

  Run the focused command and `go test ./internal/listingkit/ownerreconcile -count=1`.
  Expected: PASS with report fingerprints stable for existing non-system cases.

- [ ] **Step 5: Commit the classification unit**

  ```bash
  git add internal/listingkit/ownerreconcile/report.go internal/listingkit/ownerreconcile/repository.go internal/listingkit/ownerreconcile/report_test.go internal/listingkit/ownerreconcile/repository_test.go
  git commit -m "fix: classify ownerless rows as system data"
  ```

### Task 3: Mirror candidate policies in identity preflight

**Files:**
- Modify: `internal/listingkit/identitypreflight/inventory.go`
- Modify: `internal/listingkit/identitypreflight/repository.go`
- Test: `internal/listingkit/identitypreflight/repository_test.go`
- Test: `internal/listingkit/identitypreflight/inventory_test.go`

**Interfaces:**
- Add fixed per-table blank-owner predicates that exclude only rows with no candidate source.
- Keep rows with non-empty unmapped candidates in the preflight aggregate so they remain release blockers.

- [ ] **Step 1: Write failing SQL contract tests**

  Assert that simple legacy tables filter only `creator`/`created_by`-both-blank rows, import mappings use only joined store candidates, and native tables with no candidate source are ignored. Assert that all identifiers remain compile-time validated.

- [ ] **Step 2: Run focused tests and verify RED**

  Run `go test ./internal/listingkit/identitypreflight -run 'Test.*(Blank|System|Import|OwnerTable)' -count=1`.
  Expected: failure because current aggregate queries include every blank owner.

- [ ] **Step 3: Implement fixed aggregate predicates**

  Extend inventory constants with explicit policy metadata and generate deterministic SQL. For import mappings, join the related store and test store creator/created_by only. Preserve SQLSTATE 42P01 handling and cancellation precedence.

- [ ] **Step 4: Run focused and package regressions**

  Run the focused command, then `go test ./internal/listingkit/identitypreflight ./internal/listingkit/userdirectory -count=1`.
  Expected: PASS.

- [ ] **Step 5: Commit the preflight unit**

  ```bash
  git add internal/listingkit/identitypreflight/inventory.go internal/listingkit/identitypreflight/repository.go internal/listingkit/identitypreflight/repository_test.go internal/listingkit/identitypreflight/inventory_test.go
  git commit -m "fix: align preflight with owner candidate policy"
  ```

### Task 4: Add migration recovery classification

**Files:**
- Modify: `scripts/migrate-yudao-users-to-zitadel.ps1`
- Modify: `scripts/yudao-zitadel-migration-dry-run.ps1`
- Test: `scripts/listingkit-owner-scope-dry-run.Tests.ps1`
- Create: `scripts/yudao-zitadel-owner-recovery-dry-run.ps1`

**Interfaces:**
- Produce a redacted report separating active legacy users missing metadata, inactive/deleted users referenced by owner candidates, and IDs absent from `system_users`.
- Reuse the existing migration metadata keys and source database access pattern; do not create ZITADEL users in dry-run mode.

- [ ] **Step 1: Write failing script contract tests**

  Assert the recovery report reads `system_users`, checks existing `yudao_user_id` metadata, emits only counts/fingerprints, and never emits access tokens or raw identifiers.

- [ ] **Step 2: Run tests and verify RED**

  Run `Invoke-Pester scripts/listingkit-owner-scope-dry-run.Tests.ps1 -EnableExit`.
  Expected: failure because no recovery classification command exists.

- [ ] **Step 3: Implement read-only recovery report**

  Reuse the migration script's active-user query and metadata contract, add referenced-owner classification, and write deterministic redacted JSON/CSV artifacts. Keep any future user creation as a separate explicitly approved migration action.

- [ ] **Step 4: Run PowerShell tests and static checks**

  Run the focused Pester command, `git diff --check`, and a dry-run with stubbed `kubectl`/`psql` inputs.
  Expected: PASS with no secret or raw-ID output.

- [ ] **Step 5: Commit the recovery report unit**

  ```bash
  git add scripts/migrate-yudao-users-to-zitadel.ps1 scripts/yudao-zitadel-migration-dry-run.ps1 scripts/yudao-zitadel-owner-recovery-dry-run.ps1 scripts/listingkit-owner-scope-dry-run.Tests.ps1
  git commit -m "feat: report legacy owner migration gaps"
  ```

### Task 5: Verify release driver and target-environment report

**Files:**
- Modify: `scripts/listingkit-identity-preflight-job.sh`
- Modify: `scripts/tests/listingkit-identity-preflight-job-test.sh`
- Verify: `.github/workflows/listingkit-deploy.yml`

- [ ] **Step 1: Run driver regression tests**

  Run `"C:\\Program Files\\Git\\bin\\bash.exe" scripts/tests/listingkit-identity-preflight-job-test.sh`.
  Expected: PASS and failed Jobs return immediately with logs/describe.

- [ ] **Step 2: Run scoped Go and workflow tests**

  Run `go test ./internal/listingkit/ownerreconcile ./internal/listingkit/identitypreflight ./internal/app/runtime/listingkitidentitypreflight ./tests -count=1` and the repository's workflow/YAML tests.
  Expected: PASS.

- [ ] **Step 3: Run target-environment read-only dry-run**

  Use the already approved temporary K8s port-forward and the redacted recovery report. Do not run `-Execute` while unresolved user-owned rows remain.

- [ ] **Step 4: Commit the complete implementation after review**

  Stage only the files from completed tasks and commit with a message describing the ownership-policy closure. Do not include generated `.local/tmp` reports or secrets.
