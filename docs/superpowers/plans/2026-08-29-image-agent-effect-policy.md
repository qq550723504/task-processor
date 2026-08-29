# Image Agent Effect Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish one pure owner for Image Agent v3 provider, budget, staging, publication, blocking, and recovery transitions while preserving repository, database, and Temporal contracts.

**Architecture:** Add `internal/imageagent/effectpolicy` below the existing domain model. Memory and GORM adapters load state under their existing locks, call typed pure functions, and atomically persist immutable absolute decisions. Temporal, HTTP, tools, and services continue using unchanged repository ports.

**Tech Stack:** Go, GORM, SQLite test harness, Temporal Go SDK, Go `testing`/`testify`, golangci-lint depguard.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-effect-policy-design.md`

## Global Constraints

- Complete `docs/superpowers/plans/2026-08-29-release-boundary-test-portability.md` first.
- Do not change repository interfaces, exported DTOs, JSON, schema, indexes, phase strings, blocked codes, fingerprints, or error classifications.
- Do not change Temporal names, queues, version gates, payloads, timeouts, or replay fixtures.
- Keep mutexes, transactions, row locks, database time, SQL race predicates, and `RowsAffected == 1` checks in `store`.
- `effectpolicy` imports only `$gostd` and `task-processor/internal/imageagent`; it accepts no context, repository, callback, GORM, Temporal, HTTP, provider, or config type.
- Every policy call returns a fresh value and uses `Changed=false` for exact idempotent repeats.
- Migrate memory and GORM together per transition family; never commit two rule owners.
- Stop for approval if implementation requires a public, persisted, or Temporal compatibility change.
- No push, PR, merge, deployment, production mutation, or unrelated cleanup is authorized.

## File Map

- Create `internal/imageagent/effectpolicy/{decision,validation,provider,staging,publication,recovery}.go` and matching tests.
- Modify `internal/imageagent/store/slot_effect_v3.go` to retain persistence mechanics and delegate eligibility.
- Modify `internal/imageagent/store/slot_effect_v3_repository_test.go` for shared memory/GORM conformance.
- Modify `.golangci.yml`, `tests/depguard_config_test.go`, and the two stable architecture documents for boundary enforcement.

---

### Task 1: Establish immutable decisions and the strict dependency guard

**Files:**
- Create: `internal/imageagent/effectpolicy/decision.go`
- Create: `internal/imageagent/effectpolicy/decision_test.go`
- Modify: `.golangci.yml`
- Modify: `tests/depguard_config_test.go`

- [ ] **Step 1: Write the failing depguard test**

Add `TestImageAgentEffectPolicyDepguardUsesStrictAllowlist`. Read `.golangci.yml` with the prerequisite `readRepositoryText`, isolate the `imageagent_effectpolicy_boundaries` block, and require:

```text
list-mode: strict
"**/internal/imageagent/effectpolicy/*.go"
"**/internal/imageagent/effectpolicy/**/*.go"
"$gostd"
"task-processor/internal/imageagent"
```

Also reject `gorm.io/`, `go.temporal.io/`, and `internal/imageagent/store` inside that block.

- [ ] **Step 2: Verify RED and add the guard**

Run `go test ./tests -run '^TestImageAgentEffectPolicyDepguardUsesStrictAllowlist$' -count=1`.

Expected: FAIL because the rule is absent. Then add adjacent to `aicapability_boundaries`:

```yaml
      imageagent_effectpolicy_boundaries:
        list-mode: strict
        files:
          - "**/internal/imageagent/effectpolicy/*.go"
          - "**/internal/imageagent/effectpolicy/**/*.go"
        allow:
          - "$gostd"
          - "task-processor/internal/imageagent"
```

- [ ] **Step 3: Write the immutable-value test and minimal types**

Add `TestEffectDecisionDoesNotAliasAttemptSlices`: copy an attempt containing staging, final-manifest, and published-candidate slices through the package clone helper, mutate the result, and prove the input is unchanged.

Implement:

```go
type EffectDecision struct {
	Attempt imageagent.SlotEffectV3Attempt
	Changed bool
}

type AccountingSnapshot struct {
	Policy    imageagent.BudgetPolicy
	Committed imageagent.UsageVector
	Reserved  imageagent.UsageVector
	Elapsed   time.Duration
	StartedAt time.Time
}

type AccountingDecision struct {
	EffectDecision
	Accounting        AccountingSnapshot
	AccountingChanged bool
}

type ProviderReservationDecision struct {
	AccountingDecision
	Acquired bool
}

type PublicationClaimDecision struct {
	EffectDecision
	Claim    imageagent.PublicationClaim
	Acquired bool
}

type PublicationLeaseDecision struct {
	EffectDecision
	Claim imageagent.PublicationClaim
}
```

Keep clone helpers private and explicit; do not clone through JSON.

- [ ] **Step 4: Verify and commit**

Run `go test ./internal/imageagent/effectpolicy ./tests -run '^(TestEffectDecisionDoesNotAliasAttemptSlices|TestImageAgentEffectPolicyDepguardUsesStrictAllowlist)$' -count=1`.

Expected: PASS and both tests execute.

```text
git add .golangci.yml tests/depguard_config_test.go internal/imageagent/effectpolicy/decision.go internal/imageagent/effectpolicy/decision_test.go
git commit -m "refactor: establish image effect policy boundary"
```

### Task 2: Extract provider and budget policy and adopt it in both adapters

**Files:**
- Create: `internal/imageagent/effectpolicy/validation.go`
- Create: `internal/imageagent/effectpolicy/provider.go`
- Create: `internal/imageagent/effectpolicy/provider_test.go`
- Modify: `internal/imageagent/store/slot_effect_v3.go`
- Modify: `internal/imageagent/store/slot_effect_v3_repository_test.go`

- [ ] **Step 1: Write table-driven RED tests**

Add `TestReserveProviderDecisionMatrix`, `TestRecordProviderNotDispatchedReleasesOnlyProvenReservation`, `TestSettleProviderDecisionMatrix`, `TestReleaseAndUnknownProviderBudgetDecisionMatrix`, and `TestProviderDecisionsDoNotMutateInputs`.

Cover new, exact repeat, reservation mismatch, provider-not-dispatched redispatch, released-budget reacquisition, persisted-policy mismatch, budget exceeded, quote/receipt validation, accounting overflow/underflow, committed repeat, and conflicting repeat.

- [ ] **Step 2: Verify RED and implement typed functions**

Run the five exact test names; expect build failure. Implement:

```go
func ReserveProvider(current *imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (ProviderReservationDecision, error)
func RecordProviderNotDispatched(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error)
func SettleProvider(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, receipt imageagent.SlotUsageReceipt, accounting AccountingSnapshot, observedAt time.Time) (AccountingDecision, error)
func ReleaseProviderBudget(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error)
func MarkProviderBudgetUnknown(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, accounting AccountingSnapshot) (AccountingDecision, error)
```

Move the rules in `authorizeSlotBudget`, reservation equality/validation, and receipt validation. Compare persisted and requested policies. Return absolute reserved/committed usage. Settlement elapsed is `max(accounting.Elapsed, observedAt.Sub(accounting.StartedAt))` when started.

- [ ] **Step 3: Add shared adapter conformance**

Extend the existing repository factory loop using `NewMemoryRepository()` and `NewGormRepository(newConcurrentSQLite(t))`. Assert the same returned attempt, acquisition flag, accounting, `errors.Is` classification, idempotent repeat, and conflicting repeat.

List and run the existing budget/provider tests first:

```text
go test ./internal/imageagent/store -list 'SlotEffectV3.*Budget|ProviderNotDispatched|RepositoryContract'
go test ./internal/imageagent/store -run '^(TestSlotEffectV3BudgetReservationLifecycle|TestSlotEffectV3BudgetReleaseAndUnknownAdmission|TestSlotEffectV3ProviderNotDispatchedReclaimsOnlyAfterProvenNoEffect|TestSlotEffectV3RepositoryContract)$' -count=1
```

- [ ] **Step 4: Migrate memory and GORM together**

Under the existing lock/transaction, load effect and run, map the run to `AccountingSnapshot`, call policy, and atomically persist returned absolute values. Retain not-found handling, uniqueness collision lookup, database time, row locks, SQL mapping, CAS predicates, and rollback. Delete provider predicates from store only after both paths delegate.

- [ ] **Step 5: Verify and commit**

Run `go test ./internal/imageagent/effectpolicy -count=1`, `go test ./internal/imageagent/store -run 'SlotEffectV3.*(Budget|Provider|RepositoryContract)' -count=1`, then `go test ./internal/imageagent/store -count=1`.

```text
git add internal/imageagent/effectpolicy/validation.go internal/imageagent/effectpolicy/provider.go internal/imageagent/effectpolicy/provider_test.go internal/imageagent/store/slot_effect_v3.go internal/imageagent/store/slot_effect_v3_repository_test.go
git commit -m "refactor: centralize image provider effect policy"
```

### Task 3: Extract staging policy and adopt it in both adapters

**Files:**
- Create: `internal/imageagent/effectpolicy/staging.go`
- Create: `internal/imageagent/effectpolicy/staging_test.go`
- Modify: `internal/imageagent/store/slot_effect_v3.go`
- Modify: `internal/imageagent/store/slot_effect_v3_repository_test.go`

- [ ] **Step 1: Write RED tests**

Add `TestPrepareStagingDecisionMatrix`, `TestCommitStagedDecisionMatrix`, and `TestStagingDecisionsDoNotMutateManifests`. Cover normalization, exact repeat, conflicting manifest, invalid phase, matching commit, wrong fingerprint, and staged repeat.

- [ ] **Step 2: Implement the two typed functions**

```go
func PrepareStaging(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, manifest imageagent.StagingManifest) (EffectDecision, error)
func CommitStaged(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation, manifestFingerprint string) (EffectDecision, error)
```

Normalize and fingerprint inside policy. Preserve validation, persisted-policy, and revision-conflict classifications.

- [ ] **Step 3: Migrate both adapters, verify, and commit**

Keep locks, mapping, and update predicates in store; extend shared conformance for both factories.

Run `go test ./internal/imageagent/effectpolicy -run 'Staging' -count=1`, `go test ./internal/imageagent/store -run 'SlotEffectV3.*(Staging|RepositoryContract)' -count=1`, and `go test ./internal/imageagent/store -count=1`.

```text
git add internal/imageagent/effectpolicy/staging.go internal/imageagent/effectpolicy/staging_test.go internal/imageagent/store/slot_effect_v3.go internal/imageagent/store/slot_effect_v3_repository_test.go
git commit -m "refactor: centralize image staging effect policy"
```

### Task 4: Extract publication lease, fencing, and completion policy

**Files:**
- Create: `internal/imageagent/effectpolicy/publication.go`
- Create: `internal/imageagent/effectpolicy/publication_test.go`
- Modify: `internal/imageagent/store/slot_effect_v3.go`
- Modify: `internal/imageagent/store/slot_effect_v3_repository_test.go`

- [ ] **Step 1: Write RED matrix tests**

Add `TestClaimPublicationDecisionMatrix`, `TestRenewPublicationDecisionMatrix`, `TestCompletePublicationDecisionMatrix`, and `TestPublicationDecisionsDoNotMutateInputs`.

Cover first fence `1`, active lease, expired handoff/fence increment, manifest/fingerprint conflict, stale owner/fence, renewal at/after expiry, ordered final-manifest bijection, exact completion repeat, conflicting completion, and post-completion claim. Use fixed timestamps.

- [ ] **Step 2: Implement typed functions**

```go
func ClaimPublication(current imageagent.SlotEffectV3Attempt, request imageagent.PublicationClaimRequest, observedAt time.Time) (PublicationClaimDecision, error)
func RenewPublication(current imageagent.SlotEffectV3Attempt, renewal imageagent.PublicationLeaseRenewal, observedAt time.Time) (PublicationLeaseDecision, error)
func CompletePublication(current imageagent.SlotEffectV3Attempt, completion imageagent.PublicationCompletion) (EffectDecision, error)
```

Move publication command validation, `evaluatePublicationClaimV3`, renewal, and completion equality. Normalize final manifests and published results before comparison.

- [ ] **Step 3: Migrate both adapters without weakening concurrency**

Pass the existing memory clock or database time explicitly. Preserve SQL phase/owner/fence predicates and `RowsAffected` checks only as race guards. Extend conformance and keep database-time/concurrency tests.

- [ ] **Step 4: Verify and commit**

List first with `go test ./internal/imageagent/store -list 'SlotEffectV3.*(Publication|Fence|Completion|Concurrent)'`. Then run the matching focused tests, `go test -race ./internal/imageagent/store -run 'SlotEffectV3.*Concurrent' -count=1`, and the full store package.

```text
git add internal/imageagent/effectpolicy/publication.go internal/imageagent/effectpolicy/publication_test.go internal/imageagent/store/slot_effect_v3.go internal/imageagent/store/slot_effect_v3_repository_test.go
git commit -m "refactor: centralize image publication effect policy"
```

### Task 5: Extract blocking, recovery, and corrupt fail-closed policy

**Files:**
- Create: `internal/imageagent/effectpolicy/recovery.go`
- Create: `internal/imageagent/effectpolicy/recovery_test.go`
- Modify: `internal/imageagent/store/slot_effect_v3.go`
- Modify: `internal/imageagent/store/slot_effect_v3_repository_test.go`

- [ ] **Step 1: Write RED tests**

Add `TestBlockDecisionMatrix`, `TestRestoreRecoveryBlockedDecisionMatrix`, `TestFailClosedCorruptDecisionMatrix`, and `TestRecoveryDecisionsDoNotMutateInputs`.

Cover every allowed/disallowed block pair, phase/code mismatch, publication-unknown owner/fence, exact blocked repeat, terminal completion rejection, exact redrive allowlist, provider redispatch rejection, missing recovery phase, stable corruption marker, and repeated fail-closed behavior.

- [ ] **Step 2: Implement typed functions**

```go
func Block(current imageagent.SlotEffectV3Attempt, transition imageagent.SlotEffectV3BlockTransition) (EffectDecision, error)
func RestoreRecoveryBlocked(current imageagent.SlotEffectV3Attempt, reservation imageagent.SlotEffectV3Reservation) (EffectDecision, error)
func FailClosedCorrupt(identity imageagent.SlotExternalEffectIdentity, marker string, current *imageagent.SlotEffectV3Attempt) (EffectDecision, error)
```

Move block validation, blocked-phase recognition, redrive eligibility, and block-transition eligibility. Preserve exact public codes and sentinel errors; never reconstruct corrupt data or authorize provider work.

- [ ] **Step 3: Migrate both adapters and remove old rule helpers**

Keep corrupt-row detection and deterministic marker creation in store. After both adapters delegate all families, run:

```text
rg -n 'authorizeSlotBudget|evaluatePublicationClaimV3|validateSlotEffectV3Reservation|validatePublicationClaimRequestV3|validatePublicationLeaseRenewalV3|validatePublicationCompletionV3|validateBlockTransitionV3|isBlockedV3Phase|isRedrivableRecoveryPhase|canBlockV3|sameSlotEffectV3Reservation|validateUsageReceipt|samePublicationCompletionV3' internal/imageagent/store
```

Expected: no store-owned domain helper remains.

- [ ] **Step 4: Verify and commit**

Run policy tests matching `Block|Recovery|Corrupt`; list and run store tests matching `RecoveryBlocked|Corrupt|BlockedPolicy|IllegalPhase`; then run the full store package.

```text
git add internal/imageagent/effectpolicy/recovery.go internal/imageagent/effectpolicy/recovery_test.go internal/imageagent/store/slot_effect_v3.go internal/imageagent/store/slot_effect_v3_repository_test.go
git commit -m "refactor: centralize image recovery effect policy"
```

### Task 6: Document the boundary and run compatibility verification

**Files:**
- Modify: `docs/architecture/project-boundaries.md`
- Modify: `docs/architecture/architecture-review-checklist.md`

- [ ] **Step 1: Update stable architecture policy**

Document that `effectpolicy` owns pure v3 transition eligibility, `store` owns locks/transactions/mapping/CAS, and callers use repository ports rather than importing policy. Add `depguard: imageagent_effectpolicy_boundaries` to the checklist Guard Baseline. Do not duplicate it in `next-steps.md`, which delegates baseline authority to the checklist.

- [ ] **Step 2: Run focused verification**

```text
go test ./internal/imageagent/effectpolicy -count=1
go test ./internal/imageagent/store -count=1
go test ./internal/imageagent/temporal -count=1
go test ./internal/imageagent/... -count=1
```

- [ ] **Step 3: Run unchanged Temporal replay fixtures**

List tests using `go test ./internal/imageagent/temporal -list 'Replay|Replays|RestartedExecution'`, then run exactly:

```text
go test ./internal/imageagent/temporal -run '^(TestReplayV2SlotInflightHistory|TestImageWorkflowReplaysPreFinalizationHistory|TestReplayV2AwaitingApprovalHistory|TestReplayV2PreAtomicAwaitingApprovalHistory|TestRestartedExecutionCanRepersistProjectionAndReplayedSlotResult|TestImageAgentWorkflowReplaysAfterPersistStateWorkflowTaskRestart)$' -count=1
```

Expected: PASS without fixture regeneration.

- [ ] **Step 4: Run architecture and full verification**

Run `go test ./tests -count=1`, the repository's current root-level CI-equivalent golangci/depguard command, `go test ./... -count=1`, `git diff --check`, and `git status --short`.

Expected: PASS. Reproduce environmental tool/network failures before classifying them as code failures.

- [ ] **Step 5: Inspect compatibility diff and commit docs**

Review `git diff` against the pre-refactor commit for `internal/imageagent/slot_effect_v3.go`, `internal/imageagent/temporal`, store records, and migrations. Expected: no exported port, Temporal wire/history, schema, migration, or unrelated change.

```text
git add docs/architecture/project-boundaries.md docs/architecture/architecture-review-checklist.md
git commit -m "docs: record image effect policy boundary"
```

## Final Acceptance Checklist

- [ ] Memory and GORM produce identical decisions, accounting, claims, and errors for the shared matrix.
- [ ] Store retains persistence/concurrency mechanics but no transition eligibility predicates.
- [ ] `effectpolicy` imports only `$gostd` and `internal/imageagent`.
- [ ] Errors, codes, phases, booleans, leases, fences, fingerprints, schema, and Temporal contracts are unchanged.
- [ ] Replay fixtures pass unchanged; prerequisite `./tests` and full `go test ./... -count=1` pass on the exact commit.
- [ ] Depguard and `git diff --check` pass.
- [ ] Report local verification, remote CI, review, merge, deployment, and runtime separately; this plan authorizes local implementation only.
