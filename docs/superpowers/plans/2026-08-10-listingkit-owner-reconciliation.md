# ListingKit Owner Reconciliation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore a safe ListingKit owner reconciliation command that defaults to a redacted dry-run, reports unresolved ownership, and only applies uniquely verified canonical-subject backfills after an explicit report confirmation.

**Architecture:** Keep candidate resolution as a pure package with no database or identity-client side effects. The command opens the application database and the existing ZITADEL metadata database through strict non-creating read-only connections, streams fixed owner-table aggregates, writes a fingerprinted report, and gates optional batched updates on that exact fingerprint. The existing PowerShell wrapper remains the operator entry point.

**Tech Stack:** Go, `database/sql`, PostgreSQL, existing config/database helpers, PowerShell wrapper, `sqlmock` for SQL behavior tests.

## Global Constraints

- ZITADEL `yudao_user_id` + `yudao_tenant_id` metadata is the only legacy-to-canonical mapping source.
- Old `creator`, `created_by`, task creator, and store creator are candidates only; automatic resolution requires one unique subject.
- Default mode performs no `UPDATE`, DDL, database creation, or ZITADEL write.
- SQL identifiers are compile-time inventory constants; no user-supplied table or column names.
- Reports never contain raw tenant IDs, legacy IDs, subjects, names, email, tokens, or SQL bodies.
- Unresolved and conflicting rows remain release blockers; no arbitrary tenant-member assignment is allowed.

---

### Task 1: Pure candidate resolution and redacted report model

**Files:**
- Create: `internal/listingkit/ownerreconcile/candidates.go`
- Create: `internal/listingkit/ownerreconcile/candidates_test.go`
- Create: `internal/listingkit/ownerreconcile/report.go`
- Create: `internal/listingkit/ownerreconcile/report_test.go`

**Interfaces:**
- `type LegacyIdentity struct { TenantID, LegacyUserID, Subject string }`
- `type Candidate struct { Source, Subject string }`
- `func ResolveCandidates(candidates []Candidate) (subject, reason string)` returns a subject only when every non-empty candidate is identical; returns `("", "no_candidate")` or `("", "conflicting_candidates")` otherwise.
- `type Finding struct { Table, TenantFingerprint, OwnerFingerprint, Reason string; Rows int64 }`
- `func NewReport(...) Report` and `func (Report) Fingerprint() (string, error)` produce deterministic, redacted JSON metadata.

- [ ] **Step 1: Write the failing tests** for no candidate, one candidate, equal candidates, conflicting candidates, empty-source suppression, deterministic sorting, six-byte fingerprints, and raw-value redaction.
- [ ] **Step 2: Run `go test ./internal/listingkit/ownerreconcile -count=1` and verify the package is missing or tests fail for the expected undefined symbols.**
- [ ] **Step 3: Implement the pure resolver and report types with stable sorting and SHA-256-derived short fingerprints.**
- [ ] **Step 4: Re-run the focused tests and verify all pass without database access.**
- [ ] **Step 5: Commit `feat: add safe owner candidate resolution primitives`.**

### Task 2: Read-only PostgreSQL inventory and dry-run aggregation

**Files:**
- Create: `internal/listingkit/ownerreconcile/repository.go`
- Create: `internal/listingkit/ownerreconcile/repository_test.go`
- Modify: `internal/listingkit/identitypreflight/inventory.go` only if a shared fixed inventory seam is required

**Interfaces:**
- `type MetadataDirectory interface { ListLegacyIdentities(context.Context) ([]LegacyIdentity, error) }`
- `type Repository struct { Queryer Queryer; Inventory []TableSpec }`
- `func (Repository) DryRun(context.Context, MetadataDirectory) (Report, error)` executes only fixed SELECT aggregates and returns findings plus safe-resolution counts.
- `type Queryer interface { QueryContext(context.Context, string, ...any) (*sql.Rows, error) }`

- [ ] **Step 1: Add SQL-mock RED tests** proving all identifiers are fixed, dry-run issues no UPDATE/DDL, missing metadata fails closed, and rows are closed/cancellation preserved.
- [ ] **Step 2: Run `go test ./internal/listingkit/ownerreconcile -run 'TestRepository.*' -count=1` and confirm the expected failures.**
- [ ] **Step 3: Implement metadata loading from `projections.user_metadata5` and `projections.org_metadata2`, rejecting missing, duplicate, or tenant-mismatched mappings.**
- [ ] **Step 4: Implement fixed table queries for ListingAdmin legacy tables and ListingKit native tables; use candidate sources only where the schema defines them.**
- [ ] **Step 5: Re-run repository tests and verify only SELECT behavior and sanitized errors.**
- [ ] **Step 6: Commit `feat: add read-only owner reconciliation repository`.**

### Task 3: CLI/runtime wiring and existing wrapper compatibility

**Files:**
- Create: `cmd/listingkit-owner-scope-dry-run/main.go`
- Create: `internal/app/runtime/listingkitownerreconcile/options.go`
- Create: `internal/app/runtime/listingkitownerreconcile/runtime.go`
- Create: `internal/app/runtime/listingkitownerreconcile/runtime_test.go`
- Modify: `scripts/listingkit-owner-scope-dry-run.ps1`

**Interfaces:**
- Preserve the wrapper's existing flags: `--config`, `--output`, `--sql-output`, `--schema-output`, `--backfill-output`, `--safe-backfill-output`, `--manual-review-output`, and unresolved report paths.
- Add `--execute` (default false), `--confirm-report`, and `--batch-size` (default 500).
- `Run(context.Context, Options) error` loads config without full production validation, opens only strict non-creating connections, runs dry-run, and writes redacted artifacts.

- [ ] **Step 1: Add runtime RED tests** for default dry-run, missing config/database/ZITADEL metadata, and deterministic output paths.
- [ ] **Step 2: Run `go test ./internal/app/runtime/listingkitownerreconcile -count=1` and verify expected failures.**
- [ ] **Step 3: Wire `LoadConfigFromFileWithoutValidation`, `NewDatabaseFromConfigWithoutCreate`, and `identitypreflight`-style cleanup; never use the creating database opener.**
- [ ] **Step 4: Reconnect the PowerShell wrapper to the command and ensure the wrapper does not pass `--execute` implicitly.**
- [ ] **Step 5: Run the focused runtime tests and `go test ./cmd/listingkit-owner-scope-dry-run -run '^$' -count=1`.**
- [ ] **Step 6: Commit `feat: restore safe ListingKit owner dry-run command`.**

### Task 4: Explicitly gated batched backfill

**Files:**
- Modify: `internal/listingkit/ownerreconcile/repository.go`
- Modify: `internal/listingkit/ownerreconcile/repository_test.go`
- Modify: `internal/app/runtime/listingkitownerreconcile/runtime.go`
- Modify: `internal/app/runtime/listingkitownerreconcile/runtime_test.go`

**Interfaces:**
- `func (Repository) ApplyUnique(ctx context.Context, reportFingerprint string, expected string, batchSize int) (ApplySummary, error)` updates only rows whose current owner is blank and whose candidates resolve uniquely.
- Execute mode requires `--confirm-report 12-hex-report-fingerprint` equal to the freshly generated report fingerprint; mismatch aborts before the first UPDATE.

- [ ] **Step 1: Add SQL-mock RED tests** for confirmation mismatch, invalid batch size, conflict exclusion, allowed-column-only UPDATE, transaction rollback, and post-batch count verification.
- [ ] **Step 2: Run the focused tests and verify they fail before implementation.**
- [ ] **Step 3: Implement per-table primary-key batch updates with transaction boundaries and fail-closed postcondition checks.**
- [ ] **Step 4: Re-run focused tests and verify dry-run still has zero UPDATE statements.**
- [ ] **Step 5: Add a shell/PowerShell boundary test proving `-Execute` is never implicit and raw identifiers are absent from report files.**
- [ ] **Step 6: Commit `feat: gate ListingKit owner backfill on report fingerprint`.**

### Task 5: Documentation and verification

**Files:**
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`
- Create: `scripts/listingkit-owner-scope-dry-run.Tests.ps1`
- Create: `docs/superpowers/verification/2026-08-10-listingkit-owner-reconciliation.md`

- [ ] **Step 1: Document dry-run invocation, report review, unresolved policy, and the separate execute authorization gate without embedding credentials or raw identities.**
- [ ] **Step 2: Run PowerShell tests, focused Go tests, `go vet` on changed packages, and `git diff --check`.**
- [ ] **Step 3: Run the command against a disposable/local fixture only; do not execute against the production database in this implementation pass.**
- [ ] **Step 4: Verify the final diff contains no runtime compatibility fallback, no broad writes, and no raw production identifiers.**
- [ ] **Step 5: Commit `docs: document ListingKit owner reconciliation runbook`.**
