# Image Agent Effect Terminalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure cancellation cannot project a run as cancelled while a v3 effect remains claimed.

**Architecture:** Provider calls retain their original deadline context. Once a provider result is known, bounded finalization owns durable artifact/staging/publication transitions and records an unknown phase on failure; the parent only commits cancellation after all started effects have terminal outcomes.

**Tech Stack:** Go, Temporal, GORM, object store, testify.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-effect-terminalization-design.md`

## Global Constraints

- Provider calls never exceed `DeadlineAt`.
- New behavior uses `externalEffectFinalizationPatch`; old histories retain current decisions.
- A claimed provider/publication effect must end published or durable-unknown before cancellation is terminal.

---

### Task 1: Finalize successful provider and staging effects

**Files:**
- Modify: `internal/imageagent/temporal/activities.go`
- Test: `internal/imageagent/temporal/slot_effect_v3_activity_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestExecuteSlotV3CancellationTerminalizesSuccessfulProviderEffect(t *testing.T) {
    // cancel after generated result; assert recovery/staging uses finalization context
    // and no provider_claimed effect remains
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run '^TestExecuteSlotV3CancellationTerminalizesSuccessfulProviderEffect$' -count=1`

Expected: FAIL because recovery persistence sees the cancelled activity context.

- [ ] **Step 3: Implement minimal terminalization**

Route recovery-bundle preservation, staging transitions, and durable `StagingUnknown` fallback through `providerFinalizationContext` after provider success. Keep `GenerateQuotedSlot` and `GenerateSlot` unchanged on their original contexts.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/imageagent/temporal -run '^(TestExecuteSlotV3CancellationTerminalizesSuccessfulProviderEffect|TestExecuteSlotV3PersistsDispatchedOutcomeAfterCallerCancellation)$' -count=1`

Expected: PASS.

Run: `git add internal/imageagent/temporal/activities.go internal/imageagent/temporal/slot_effect_v3_activity_test.go` then `git commit -m "fix: terminalize cancelled provider effects"`.

### Task 2: Finalize publication and preserve full deadline grace

**Files:**
- Modify: `internal/imageagent/temporal/activities.go`
- Modify: `internal/imageagent/temporal/slot_workflow.go`
- Test: `internal/imageagent/temporal/slot_effect_v3_activity_test.go`
- Test: `internal/imageagent/temporal/budget_authorization_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestExecuteSlotV3CancellationTerminalizesPublicationClaim(t *testing.T) {}
func TestImageSlotWorkflowV3UsesProviderWindowPlusGraceBeyondTenMinutes(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run '^(TestExecuteSlotV3CancellationTerminalizesPublicationClaim|TestImageSlotWorkflowV3UsesProviderWindowPlusGraceBeyondTenMinutes)$' -count=1`

Expected: FAIL because publication uses cancelled context and activity timeout is capped at ten minutes.

- [ ] **Step 3: Implement minimal protocol**

Use bounded finalization context for claimed-publication lease renewal, object finalization, completion, and `PublicationUnknown`. For new budget-authorized histories make `StartToCloseTimeout` equal `providerWindow + providerFinalizationTimeout`; retain old and unbudgeted paths.

- [ ] **Step 4: Verify GREEN and commit**

Run: `go test ./internal/imageagent/temporal -run '^(TestExecuteSlotV3CancellationTerminalizesPublicationClaim|TestImageSlotWorkflowV3UsesProviderWindowPlusGraceBeyondTenMinutes|TestImageSlotWorkflowV3BudgetDeadlineBlocksWithoutActivity)$' -count=1`

Expected: PASS.

Run: `git add internal/imageagent/temporal/activities.go internal/imageagent/temporal/slot_workflow.go internal/imageagent/temporal/slot_effect_v3_activity_test.go internal/imageagent/temporal/budget_authorization_test.go` then `git commit -m "fix: preserve image effect finalization grace"`.

### Task 3: Gate terminal cancellation on effect outcome

**Files:**
- Modify: `internal/imageagent/temporal/workflow.go`
- Test: `internal/imageagent/temporal/workflow_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestManualWorkflowDoesNotProjectCancelledForUnterminalizedEffect(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run '^TestManualWorkflowDoesNotProjectCancelledForUnterminalizedEffect$' -count=1`

Expected: FAIL because child drain alone permits cancellation projection.

- [ ] **Step 3: Implement and verify**

Make the versioned cancellation scheduler require a durable published/unknown child outcome before `persistTerminalRunState(...Cancelled)`; retain a blocked pending command otherwise.

Run: `go test -race ./internal/imageagent/temporal -count=1`

Expected: PASS.

- [ ] **Step 4: Commit**

Run: `git add internal/imageagent/temporal/workflow.go internal/imageagent/temporal/workflow_test.go` then `git commit -m "fix: gate cancellation on effect terminalization"`.
