# Image Agent Recovery Lifecycle Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Close the remaining recovery lifecycle gaps so every recovery owner is durable, parent state is refreshed, explicit re-drive can create a new execution safely, and corrupt proof fails closed.

**Architecture:** Keep the projection-level `RecoverableEffects` registry as the source of truth for multiple owners. Recovery completion writes an exact effect result and emits an idempotent parent reconciliation command; the parent updates its projection only after reading durable effect state. Automatic starts retain `USE_EXISTING`, while authenticated explicit re-drive uses a distinct new execution identity/reuse policy that cannot attach a completed run. Recovery workflow IDs use injective encoded segments.

**Tech Stack:** Go, Temporal Go SDK, image-agent projection repository, existing HTTP/service adapters, testify.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-external-recovery-workflow-design.md`

## Global Constraints

- Provider generation is forbidden in recovery workflows.
- All effect and projection writes are scoped by tenant, owner, run, plan revision, slot, and attempt.
- Legacy v2 histories must replay without the recovery-start command.
- Automatic recovery start failures keep the parent blocked and durable.
- Explicit recovery commands accept only authenticated identity and server-resolved projection data.
- Every task must follow TDD: failing regression test first, then minimal production change, then focused and race verification.

---

### Task 1: Persist secondary recovery-start failures

**Files:**
- Modify: `internal/imageagent/temporal/workflow.go` (`commitPendingCancellation`, recovery handoff commit IDs)
- Modify: `internal/imageagent/temporal/activities.go`, `internal/imageagent/ports.go`, `internal/imageagent/store/projection.go`, `internal/imageagent/store/gorm.go` (atomic multi-slot projection mutation contract)
- Test: `internal/imageagent/temporal/workflow_test.go`, `internal/imageagent/store/repository_contract_test.go` (real repository-backed secondary start failure and atomic slot/run mutation)

**Interfaces:**
- Consumes: `WorkflowResult.RecoverableEffects`, `startEffectRecoveryV3`, `PersistRunState`
- Produces: an atomic projection commit that carries all changed slot mutations plus the recovery-owner registry, and idempotent commit IDs that include the complete recovery-owner registry/fingerprint, allowing a changed secondary owner code to persist without colliding with the initial handoff commit.

- [ ] **Step 1: Write the failing test**

Add `TestCommitProjectionPersistsRunAndMultipleSlotMutationsAtomically` at the repository contract layer, then add `TestManualWorkflowPersistsSecondaryRecoveryStartFailureWithRealRepository` that drives a two-slot cancellation where the first recovery starter succeeds and the second returns an error. Register `PersistRunState` against a real `MemoryRepository` adapter (not an always-success stub), then assert the final projection contains both registry entries, the primary remains `recovery_requested`, and the failed secondary is `recovery_start_failed`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/imageagent/temporal -run '^TestManualWorkflowPersistsSecondaryRecoveryStartFailureWithRealRepository$' -count=1`
Expected: the repository contract test FAIL because one `SlotMutation` cannot represent all changed slots, and the workflow test FAIL before the secondary failure can be durably persisted with `slot snapshot does not match normalized mutation` or `image agent revision conflict`.

- [ ] **Step 3: Write the minimal implementation**

Extend `ProjectionCommit` with an ordered `SlotMutations` collection and update memory/GORM validation/application paths to atomically apply every supplied slot mutation. Update `PersistRunState` to populate all changed slot mutations and include the complete recovery-owner registry and slot error-code projections in its idempotency key. Preserve identical commit IDs for retries of the same snapshot, but produce a distinct commit ID when a secondary start failure changes the durable registry.

- [ ] **Step 4: Run focused and race tests**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowPersistsSecondaryRecoveryStartFailureWithRealRepository|TestManualWorkflowStartsIndependentRecoveryOwnersForEachBlockedEffect)$' -count=1` and `go test -race ./internal/imageagent/temporal -run '^TestManualWorkflowPersistsSecondaryRecoveryStartFailureWithRealRepository$' -count=1`.
Expected: PASS.

- [ ] **Step 5: Commit**

```text
git add internal/imageagent/temporal/workflow.go internal/imageagent/temporal/workflow_test.go
git commit -m "fix: persist secondary recovery start failures"
```

### Task 2: Close recovery completion and parent projection reconciliation

**Files:**
- Modify: `internal/imageagent/temporal/effect_recovery_workflow.go`, `internal/imageagent/temporal/activities.go`, `internal/imageagent/temporal/workflow.go`, `internal/imageagent/temporal/types.go`
- Modify: `internal/imageagent/store/projection.go` (allow the exact same-attempt terminal publication reconciliation mutation)
- Test: `internal/imageagent/temporal/effect_recovery_workflow_test.go`, `internal/imageagent/temporal/workflow_test.go`

**Interfaces:**
- Consumes: exact `EffectRecoveryWorkflowInput`, `EffectRecoveryResult`, `RecoverableEffects`
- Produces: an idempotent parent-reconciliation activity/signal that reads durable effect state, removes or updates only the matching registry entry, and refreshes the blocked parent projection without provider calls.

- [ ] **Step 1: Write the failing tests**

Add `TestEffectRecoveryWorkflowReconcilesParentProjectionAfterPublication` and `TestEffectRecoveryWorkflowReconcilesUnknownPhaseWithoutClearingOwner`. The first must seed a blocked parent plus a staged effect, run recovery to `publication_complete`, and assert the exact registry entry is removed and the slot projection is accepted from durable published data. The second must assert an unknown phase leaves the registry entry and parent blocked.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/imageagent/temporal -run '^(TestEffectRecoveryWorkflowReconcilesParentProjectionAfterPublication|TestEffectRecoveryWorkflowReconcilesUnknownPhaseWithoutClearingOwner)$' -count=1`
Expected: FAIL because current recovery only updates the effect row and leaves the parent projection unchanged.

- [ ] **Step 3: Implement the reconciliation contract**

Add a provider-free reconciliation activity with an idempotency key derived from `(run, revision, slot, attempt, effect phase)`. Invoke it after recovery activity returns. For `publication_complete`, persist the durable candidates and remove only that owner; for unknown/recovery-blocked, persist the phase/code and keep the owner. Make retries return the already-applied projection instead of conflicting.

- [ ] **Step 4: Run focused and race tests**

Run: `go test ./internal/imageagent/temporal -run '^(TestEffectRecoveryWorkflowReconcilesParentProjectionAfterPublication|TestEffectRecoveryWorkflowReconcilesUnknownPhaseWithoutClearingOwner|TestEffectRecoveryWorkflowScopesToExactEffectIdentityWithoutProviderCall)$' -count=1` and the same command with `-race`.
Expected: PASS.

- [ ] **Step 5: Commit**

```text
git add internal/imageagent/temporal/effect_recovery_workflow.go internal/imageagent/temporal/activities.go internal/imageagent/temporal/workflow.go internal/imageagent/temporal/types.go internal/imageagent/temporal/effect_recovery_workflow_test.go internal/imageagent/temporal/workflow_test.go
git commit -m "fix: reconcile parent after image effect recovery"
```

### Task 3: Make explicit re-drive restartable and workflow IDs injective

**Files:**
- Modify: `internal/imageagent/temporal/worker.go`, `internal/imageagent/temporal/types.go`, `internal/imageagent/service.go`
- Test: `internal/imageagent/temporal/worker_test.go`, `internal/imageagent/service_commands_test.go`

**Interfaces:**
- Consumes: automatic `USE_EXISTING` starter and authenticated `RecoverEffect` service command
- Produces: deterministic injective workflow IDs and explicit re-drive semantics that can start a new execution after a completed recovery while duplicate in-flight starts still attach.

- [ ] **Step 1: Write failing tests**

Add `TestEffectRecoveryWorkflowIDSeparatesColonContainingIdentityFields` using two identities that previously collided, and `TestExplicitRecoverEffectStartsNewExecutionAfterCompletedRecovery` using a recording client that returns a completed first execution and verifies the second explicit request starts a new run rather than attaching the old one.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/imageagent/temporal -run '^(TestEffectRecoveryWorkflowIDSeparatesColonContainingIdentityFields|TestExplicitRecoverEffectStartsNewExecutionAfterCompletedRecovery)$' -count=1`
Expected: FAIL because IDs are colon-concatenated and the reuse policy is `ALLOW_DUPLICATE_FAILED_ONLY`.

- [ ] **Step 3: Implement minimal semantics**

Encode every identity segment with a stable length-prefix or URL-safe base64 representation before joining. Keep automatic starts on `USE_EXISTING`; add an explicit re-drive start mode that uses the same deterministic base identity plus a new execution generation recorded in the durable registry, and reject stale completed generations.

- [ ] **Step 4: Run focused and race tests**

Run: `go test ./internal/imageagent/temporal ./internal/imageagent -run '^(TestEffectRecoveryWorkflowIDSeparatesColonContainingIdentityFields|TestExplicitRecoverEffectStartsNewExecutionAfterCompletedRecovery|TestServiceRecoverEffectUsesVerifiedIdentityAndIgnoresClientWorkflowID)$' -count=1` and `go test -race ./internal/imageagent/temporal ./internal/imageagent -run '^TestExplicitRecoverEffectStartsNewExecutionAfterCompletedRecovery$' -count=1`.
Expected: PASS.

- [ ] **Step 5: Commit**

```text
git add internal/imageagent/temporal/worker.go internal/imageagent/temporal/types.go internal/imageagent/service.go internal/imageagent/temporal/worker_test.go internal/imageagent/service_commands_test.go
git commit -m "fix: make effect recovery redrive restartable"
```

### Task 4: Durable fail-closed handling for corrupt effect proof

**Files:**
- Modify: `internal/imageagent/store/slot_effect_v3.go`, `internal/imageagent/temporal/activities.go`
- Test: `internal/imageagent/store/slot_effect_v3_repository_test.go`, `internal/imageagent/temporal/effect_recovery_workflow_test.go`

**Interfaces:**
- Consumes: raw effect row identity and decode errors
- Produces: a minimal, exact-scoped `SlotEffectV3RecoveryBlocked` record with `SlotRecoveryBlockedCode`, no provider authorization, and an auditable corruption marker.

- [ ] **Step 1: Write failing test**

Add `TestRecoveryCorruptPolicyQuotePersistsRecoveryBlocked` by inserting an effect row with invalid policy/quote JSON, invoking recovery, and asserting a durable blocked phase/code is returned without provider calls.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/imageagent/store ./internal/imageagent/temporal -run '^TestRecoveryCorruptPolicyQuotePersistsRecoveryBlocked$' -count=1`
Expected: FAIL because decoding currently returns an error before a blocked record can be written.

- [ ] **Step 3: Implement fail-closed decode path**

Preserve identity, phase, attempt, and raw corruption marker separately from policy/quote decoding. When decode fails, reject all provider operations and atomically write `recovery_blocked` with the exact identity; repeated recovery reads the same durable blocked record.

- [ ] **Step 4: Run focused and race tests**

Run: `go test ./internal/imageagent/store ./internal/imageagent/temporal -run '^TestRecoveryCorruptPolicyQuotePersistsRecoveryBlocked$' -count=1` and `go test -race ./internal/imageagent/store ./internal/imageagent/temporal -run '^TestRecoveryCorruptPolicyQuotePersistsRecoveryBlocked$' -count=1`.
Expected: PASS.

- [ ] **Step 5: Commit**

```text
git add internal/imageagent/store/slot_effect_v3.go internal/imageagent/temporal/activities.go internal/imageagent/store/slot_effect_v3_repository_test.go internal/imageagent/temporal/effect_recovery_workflow_test.go
git commit -m "fix: persist corrupt effect proof as recovery blocked"
```
