# Image Agent External Effect Recovery Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Assign every unterminalized image-agent effect to an idempotent independent Temporal recovery workflow that can be automatically started and explicitly re-driven.

**Architecture:** The main workflow persists a blocked recovery request and invokes a worker-owned starter activity. The starter uses a deterministic workflow ID and `USE_EXISTING` conflict handling to attach to an already-running recovery execution. The recovery workflow performs provider-free reconciliation and durable terminalization; an authenticated API can re-drive the same ID.

**Tech Stack:** Go, Temporal SDK, Gin, GORM/object store adapters, testify.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-external-recovery-workflow-design.md`

## Global Constraints

- Recovery never invokes provider generation and never extends `DeadlineAt`.
- Workflow ID is exactly `image-agent-effect-recovery:<tenant>:<owner>:<run>:<revision>:<slot>:<attempt>`; duplicate starts attach to the existing execution.
- Recovery start failure persists `recovery_start_failed` and leaves the run blocked; it never projects `cancelled`.
- Every recovery write is scoped to the exact effect identity and is idempotent; exhaustion records `recovery_blocked` or an unknown phase.
- Client input cannot supply tenant/user/workflow IDs, provider inputs, or a second effect identity.

---

### Task 1: Recovery workflow and provider-free reconciliation

**Files:**
- Create: `internal/imageagent/temporal/effect_recovery_workflow.go`
- Modify: `internal/imageagent/temporal/types.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Test: `internal/imageagent/temporal/effect_recovery_workflow_test.go`
- Test: `internal/imageagent/temporal/slot_effect_v3_activity_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestEffectRecoveryWorkflowReconcilesClaimWithoutProviderCall(t *testing.T) {}
func TestEffectRecoveryWorkflowUsesDeterministicIDAndAttachesDuplicateStart(t *testing.T) {}
func TestEffectRecoveryWorkflowPersistsRecoveryBlockedAfterBoundedExhaustion(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run '^(TestEffectRecoveryWorkflowReconcilesClaimWithoutProviderCall|TestEffectRecoveryWorkflowUsesDeterministicIDAndAttachesDuplicateStart|TestEffectRecoveryWorkflowPersistsRecoveryBlockedAfterBoundedExhaustion)$' -count=1`

Expected: FAIL because no recovery workflow, deterministic ID helper, or recovery activity exists.

- [ ] **Step 3: Implement minimal workflow**

Add `EffectRecoveryWorkflowInput`, `EffectRecoveryResult`, `EffectRecoveryWorkflowName`, and `EffectRecoveryWorkflowID(imageagent.ExecutionIdentity, int64, string, int) string`. Add an activity that loads the exact v3 effect, rehydrates the recovery bundle, renews/finalizes/completes staging/publication without provider calls, and persists `published`, `provider_unknown`, `staging_unknown`, `publication_unknown`, or `recovery_blocked`. Configure bounded retries and deterministic idempotency keys from the input identity.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/imageagent/temporal -run '^(TestEffectRecoveryWorkflowReconcilesClaimWithoutProviderCall|TestEffectRecoveryWorkflowUsesDeterministicIDAndAttachesDuplicateStart|TestEffectRecoveryWorkflowPersistsRecoveryBlockedAfterBoundedExhaustion)$' -count=1`

Expected: PASS.

Run: `git add internal/imageagent/temporal/effect_recovery_workflow.go internal/imageagent/temporal/types.go internal/imageagent/temporal/activities.go internal/imageagent/temporal/effect_recovery_workflow_test.go internal/imageagent/temporal/slot_effect_v3_activity_test.go` then `git commit -m "feat: add image effect recovery workflow"`.

### Task 2: Automatic recovery handoff and worker registration

**Files:**
- Modify: `internal/imageagent/temporal/workflow.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Modify: `internal/imageagent/temporal/worker.go`
- Test: `internal/imageagent/temporal/workflow_test.go`
- Test: `internal/imageagent/temporal/worker_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestManualWorkflowStartsExternalRecoveryForUnterminalizedEffect(t *testing.T) {}
func TestManualWorkflowKeepsBlockedWhenRecoveryStartFails(t *testing.T) {}
func TestRegisterWorkerIncludesEffectRecoveryWorkflowAndActivity(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowStartsExternalRecoveryForUnterminalizedEffect|TestManualWorkflowKeepsBlockedWhenRecoveryStartFails|TestRegisterWorkerIncludesEffectRecoveryWorkflowAndActivity)$' -count=1`

Expected: FAIL because blocked cancellation has no external recovery owner or worker registration.

- [ ] **Step 3: Implement automatic handoff**

Add `RecoveryWorkflowStarter` to `ActivityDependencies`; its activity calls Temporal `ExecuteWorkflow` with the exact deterministic ID, `WorkflowIDConflictPolicy=USE_EXISTING`, `WorkflowIDReusePolicy=ALLOW_DUPLICATE_FAILED_ONLY`, and `EffectRecoveryWorkflowName`. In the parent’s unsafe-cancellation branch, persist `recovery_requested`, start the recovery workflow, and record `recovery_start_failed` on error while retaining blocked status. Register the recovery workflow/activity in every non-frozen worker mode; frozen v2 registration remains unchanged.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowStartsExternalRecoveryForUnterminalizedEffect|TestManualWorkflowKeepsBlockedWhenRecoveryStartFails|TestRegisterWorkerIncludesEffectRecoveryWorkflowAndActivity)$' -count=1`

Expected: PASS.

Run: `git add internal/imageagent/temporal/workflow.go internal/imageagent/temporal/activities.go internal/imageagent/temporal/worker.go internal/imageagent/temporal/workflow_test.go internal/imageagent/temporal/worker_test.go` then `git commit -m "feat: hand off blocked effects to recovery"`.

### Task 3: Authenticated explicit re-drive API

**Files:**
- Modify: `internal/imageagent/ports.go`
- Modify: `internal/imageagent/service.go`
- Modify: `internal/imageagent/httpapi/handler.go`
- Modify: `internal/imageagent/httpapi/routes.go`
- Modify: `internal/imageagent/temporal/worker.go`
- Test: `internal/imageagent/service_commands_test.go`
- Test: `internal/imageagent/httpapi/handler_test.go`
- Test: `internal/imageagent/temporal/worker_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestServiceRecoverEffectUsesVerifiedIdentityAndIgnoresClientWorkflowID(t *testing.T) {}
func TestRecoverEffectHandlerRejectsCrossTenantAndNonBlockedEffect(t *testing.T) {}
func TestTemporalClientRecoverEffectUsesDeterministicWorkflowID(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent ./internal/imageagent/httpapi ./internal/imageagent/temporal -run '^(TestServiceRecoverEffectUsesVerifiedIdentityAndIgnoresClientWorkflowID|TestRecoverEffectHandlerRejectsCrossTenantAndNonBlockedEffect|TestTemporalClientRecoverEffectUsesDeterministicWorkflowID)$' -count=1`

Expected: FAIL because no recovery command, service method, route, or Temporal client method exists.

- [ ] **Step 3: Implement authenticated re-drive**

Add `RecoverEffectCommand{RunID, PlanRevision, SlotID, Attempt, ActionID, Identity}` and `WorkflowClient.RecoverEffect`. Verify actor/tenant/owner from the request context, require the current projection to be blocked with a matching effect identity, derive the deterministic workflow ID server-side, and call the same `USE_EXISTING` starter. Reject client-supplied workflow IDs/provider data and cross-tenant identities before any Temporal call. Expose `POST /api/v1/image-agent/runs/:run_id/slots/:slot_id/attempts/:attempt/recover` through the existing authenticated handler/router.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/imageagent ./internal/imageagent/httpapi ./internal/imageagent/temporal -count=1`

Expected: PASS.

Run: `git add internal/imageagent/ports.go internal/imageagent/service.go internal/imageagent/httpapi/handler.go internal/imageagent/httpapi/routes.go internal/imageagent/temporal/worker.go internal/imageagent/service_commands_test.go internal/imageagent/httpapi/handler_test.go internal/imageagent/temporal/worker_test.go` then `git commit -m "feat: expose image effect recovery redrive"`.

### Task 4: End-to-end recovery and replay acceptance

**Files:**
- Modify: `internal/imageagent/temporal/history_replay_test.go`
- Modify: `internal/imageagent/temporal/manual_acceptance_test.go`
- Test: `internal/imageagent/temporal/effect_recovery_workflow_test.go`

- [ ] **Step 1: Write failing acceptance tests**

```go
func TestManualWorkflowRecoveryOwnerCompletesAfterWorkerRestart(t *testing.T) {}
func TestLegacyHistoryDoesNotStartExternalRecoveryWorkflow(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowRecoveryOwnerCompletesAfterWorkerRestart|TestLegacyHistoryDoesNotStartExternalRecoveryWorkflow)$' -count=1`

Expected: FAIL because no captured recovery handoff/replay acceptance exists.

- [ ] **Step 3: Implement acceptance coverage**

Use the repository’s `WorkflowReplayer` fixture for a legacy history and a Temporal test environment for worker restart/re-drive. Assert no provider invocation, deterministic duplicate attach, exact effect identity scoping, terminal/unknown phase persistence, and no `cancelled` projection before recovery ownership is durable.

- [ ] **Step 4: Verify and commit**

Run: `go test -race ./internal/imageagent/temporal -count=1`

Expected: PASS.

Run: `git add internal/imageagent/temporal/history_replay_test.go internal/imageagent/temporal/manual_acceptance_test.go internal/imageagent/temporal/effect_recovery_workflow_test.go` then `git commit -m "test: verify external effect recovery"`.

