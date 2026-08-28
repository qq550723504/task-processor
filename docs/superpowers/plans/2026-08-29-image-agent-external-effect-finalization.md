# Image Agent External-Effect Finalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure an in-flight v3 provider effect reaches a durable, auditable outcome before cancellation or deadline becomes terminal, while retaining a recoverable approval path after publication.

**Architecture:** Treat provider dispatch and provider-outcome persistence as separate lifecycle phases. The provider call keeps the caller context and is bounded by the authorised deadline; only post-dispatch database transitions receive a one-minute, cancellation-detached finalization context. A workflow-level version marker carries the new protocol through child workflows, drains started children before committing a cancellation, and admits commands according to the irreversible approval-publication boundary.

**Tech Stack:** Go 1.26, Temporal Go SDK, GORM repositories, Go `context.WithoutCancel`, testify, existing Temporal workflow test environment.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-external-effect-finalization-design.md`

## Global Constraints

- Provider invocation, artifact transfer, staging, and publication must always use the caller context; the finalization context is only for durable provider-effect and budget-state writes.
- Provider dispatch must stop at `DeadlineAt`; a v3 activity gets at most 60 seconds after that deadline to persist the known provider outcome.
- Cancellation is accepted promptly, stops future slot dispatch, waits for all already-started children to finish finalizing, and only then persists `cancelled`.
- If approval publication succeeded and the completed projection write failed, cancel is rejected and only resuming that exact approval command is accepted.
- Preserve existing Temporal histories with `workflow.GetVersion`; do not reinterpret an old history under the new lifecycle protocol.
- Keep `ProviderUnknown` cancel-only and do not add a provider reconciliation API.
- Stage and commit only files named in each task; do not push, merge, or deploy during implementation.

---

## File Structure

- `internal/imageagent/temporal/activities.go` owns v3 provider invocation plus durable post-dispatch provider/budget state transitions.
- `internal/imageagent/temporal/slot_workflow.go` owns v3 activity timeout calculation and forwards protocol capability through `SlotWorkflowV3Input`.
- `internal/imageagent/temporal/types.go` owns the serializable child-workflow input field for the finalization protocol.
- `internal/imageagent/temporal/workflow.go` owns versioned parent cancellation draining, command admission, effect-owner fencing, and approval recovery policy.
- `internal/imageagent/temporal/worker.go` owns external Update API stage selection for `Client.Cancel`.
- `internal/imageagent/temporal/slot_effect_v3_activity_test.go`, `budget_authorization_test.go`, and `workflow_test.go` prove the protocol at activity, child workflow, and parent/client boundaries.

### Task 1: Persist post-dispatch provider outcomes after caller cancellation

**Files:**
- Modify: `internal/imageagent/temporal/activities.go`
- Test: `internal/imageagent/temporal/slot_effect_v3_activity_test.go`

**Interfaces:**
- Produces `providerFinalizationContext(ctx context.Context) (context.Context, context.CancelFunc)`, a one-minute `context.WithTimeout(context.WithoutCancel(ctx), time.Minute)` context retaining values but not cancellation.
- Consumes the existing `RecordSlotProviderNotDispatchedV3`, `MarkSlotProviderBudgetUnknownV3`, `SettleSlotProviderEffectV3`, and `blockSlotEffectV3` repository paths.

- [ ] **Step 1: Write the failing cancellation-after-dispatch test**

Add a fake provider that records it has begun, waits for `ctx.Done()`, then returns a known `ProviderDispatched` error. Make the repository fake reject writes if it receives a cancelled context, then cancel the activity context after the provider begins:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
provider.started = make(chan struct{})
go func() { <-provider.started; cancel() }()
err := activities.ExecuteSlotV3(ctx, input)
require.Error(t, err)
require.True(t, repository.recordedUnknown)
require.False(t, repository.writeSawCancelledContext)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/imageagent/temporal -run '^TestExecuteSlotV3PersistsDispatchedOutcomeAfterCallerCancellation$' -count=1`

Expected: FAIL because the provider/budget transition is attempted with the cancelled activity context.

- [ ] **Step 3: Write minimal implementation**

In `activities.go`, add the helper and use it only after the provider has returned or the dispatch state is known:

```go
const providerFinalizationTimeout = time.Minute

func providerFinalizationContext(ctx context.Context) (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.WithoutCancel(ctx), providerFinalizationTimeout)
}
```

At each post-dispatch branch, create `finalizationCtx, cancel := providerFinalizationContext(ctx)`, `defer cancel()`, and call the existing record/mark/settle/block method with `finalizationCtx`. Keep `provider.Generate`, staging, artifact I/O, and publication on `ctx` or their existing deadline-limited provider context.

- [ ] **Step 4: Run focused tests to verify they pass**

Run: `go test ./internal/imageagent/temporal -run '^(TestExecuteSlotV3PersistsDispatchedOutcomeAfterCallerCancellation|TestExecuteSlotV3CancelsBlockingSDKCallBeforeLeaseExpiryWithoutTakeover)$' -count=1`

Expected: PASS; the provider receives cancellation while the durable outcome write remains uncancelled and bounded.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/imageagent/temporal/activities.go internal/imageagent/temporal/slot_effect_v3_activity_test.go
git commit -m "fix: finalize image provider effects after cancellation"
```

### Task 2: Reserve deadline grace for provider-outcome finalization

**Files:**
- Modify: `internal/imageagent/temporal/types.go`
- Modify: `internal/imageagent/temporal/slot_workflow.go`
- Modify: `internal/imageagent/temporal/workflow.go`
- Test: `internal/imageagent/temporal/budget_authorization_test.go`
- Test: `internal/imageagent/temporal/history_replay_test.go`

**Interfaces:**
- Produces `ExternalEffectFinalization bool` on `SlotWorkflowV3Input`.
- Produces `externalEffectFinalizationPatch` as the parent workflow version marker; new histories use the finalization protocol and old histories retain their previous timeout semantics.
- Consumes `DeadlineAt`, `workflow.Now(ctx)`, and the existing 10-minute `StartToCloseTimeout` activity contract.

- [ ] **Step 1: Write the failing timeout-boundary tests**

Add a new-history child workflow test that sets a near-future `DeadlineAt`, enables `ExternalEffectFinalization`, and asserts activity options reserve finalization grace:

```go
env.ExecuteWorkflow(ImageSlotWorkflowV3, SlotWorkflowV3Input{
    SlotID: "slot-1", BudgetAuthorization: true,
    DeadlineAt: env.Now().Add(2 * time.Minute),
    ExternalEffectFinalization: true,
})
require.NoError(t, env.GetWorkflowError())
require.Equal(t, 3*time.Minute, observedStartToCloseTimeout)
```

Add a replay/version test proving the old marker path still supplies only the pre-existing remaining-time timeout.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/imageagent/temporal -run '^(TestImageSlotWorkflowV3ReservesFinalizationGraceAfterBudgetDeadline|TestImageWorkflowReplaysPreFinalizationHistory)$' -count=1`

Expected: FAIL because the current activity timeout is exactly the remaining provider budget and no protocol field/version branch exists.

- [ ] **Step 3: Write minimal implementation**

Add the serializable input field:

```go
type SlotWorkflowV3Input struct {
    // existing fields
    ExternalEffectFinalization bool
}
```

In the parent workflow, obtain the marker once and pass it to every v3 child input. In `ImageSlotWorkflowV3`, retain the existing immediate `BudgetElapsed` block when `remaining <= 0`, but calculate the activity timeout as:

```go
startToClose := 10 * time.Minute
if input.BudgetAuthorization && !input.DeadlineAt.IsZero() {
    providerWindow := input.DeadlineAt.Sub(workflow.Now(ctx))
    if providerWindow <= 0 {
        return SlotWorkflowV3Result{Status: imageagent.SlotStatusBlocked, ErrorCode: imageagent.BudgetElapsedCode}, nil
    }
    activityWindow := providerWindow
    if input.ExternalEffectFinalization { activityWindow += providerFinalizationTimeout }
    if activityWindow < startToClose { startToClose = activityWindow }
}
```

Use the same one-minute constant as Task 1 (move it to a shared Temporal package file only if Go visibility requires it); do not start provider work after `DeadlineAt`.

- [ ] **Step 4: Run focused tests to verify they pass**

Run: `go test ./internal/imageagent/temporal -run '^(TestImageSlotWorkflowV3ReservesFinalizationGraceAfterBudgetDeadline|TestImageSlotWorkflowV3BudgetDeadlineBlocksWithoutActivity|TestImageWorkflowReplaysPreFinalizationHistory)$' -count=1`

Expected: PASS; current histories obtain grace, already-expired budgets dispatch nothing, and replayed histories preserve the former decision path.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/imageagent/temporal/types.go internal/imageagent/temporal/slot_workflow.go internal/imageagent/temporal/workflow.go internal/imageagent/temporal/budget_authorization_test.go internal/imageagent/temporal/history_replay_test.go
git commit -m "fix: reserve image effect finalization grace"
```

### Task 3: Drain started effects before committing a cancellation

**Files:**
- Modify: `internal/imageagent/temporal/workflow.go`
- Modify: `internal/imageagent/temporal/worker.go`
- Test: `internal/imageagent/temporal/workflow_test.go`

**Interfaces:**
- Produces versioned cancellation state with an accepted/pending intent and a terminal commit only after `executeInitialSlots` has drained every already-started child.
- Produces `Client.Cancel` behavior that waits for `WorkflowUpdateStageAccepted`, matching the existing retry acknowledgement semantics.
- Consumes the existing `updates.cancelRequested`, child cancellation context, and `persistTerminalRunState` terminal projection function.

- [ ] **Step 1: Write the failing parent-workflow cancellation tests**

Add a workflow test with one started child that reports an irreversible provider outcome only after its cancellation signal. Issue cancel while that child is outstanding, then assert no terminal cancel projection is written before the child future completes:

```go
env.SignalWorkflow(ImageRunWorkflow, updateChannelName, cancelCommand)
env.AdvanceTime(time.Second)
require.Empty(t, repository.terminalStatuses())
child.complete(SlotWorkflowResult{SlotID: "slot-1", State: imageagent.SlotStateBlocked})
env.AdvanceTime(time.Second)
require.Equal(t, []imageagent.RunStatus{imageagent.RunStatusCancelled}, repository.terminalStatuses())
```

Extend `TestTemporalClientRetryWaitsForAcceptedWhileOtherCommandsWaitForCompleted` so its cancel command also calls Update with `WorkflowUpdateStageAccepted` and does not call `Get`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowCancellationWaitsForStartedSlotFinalization|TestTemporalClientRetryWaitsForAcceptedWhileOtherCommandsWaitForCompleted)$' -count=1`

Expected: FAIL because cancel currently persists `RunStatusCancelled` before child workflows drain and `Client.Cancel` waits for completion.

- [ ] **Step 3: Write minimal implementation**

For histories after `externalEffectFinalizationPatch`, change cancel handling into these concrete transitions:

```go
// update acceptance path
updates.cancelRequested = true
updates.cancelPending = true
updates.wake = true

// scheduler path, after every in-flight child future has completed
if updates.cancelPending && allStartedChildrenDrained {
    err := persistTerminalRunState(ctx, input.RunID, imageagent.RunStatusCancelled)
    if err != nil { return err }
    updates.cancelCommitted = true
    updates.cancelPending = false
}
```

Keep `cancelChildren()` in `executeInitialSlots` when `cancelRequested` is observed, stop launching new slots, and continue selecting each started child future until all are recorded. Route terminal persistence through the existing effect owner so only one terminal state wins. Retain the existing eager cancellation path for pre-marker histories.

In `worker.go`, implement `Client.Cancel` with `executeAcceptedCommandUpdate`, using the existing cancel update name and request payload; do not alter approve, resume, or replace semantics.

- [ ] **Step 4: Run focused tests to verify they pass**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowCancellationWaitsForStartedSlotFinalization|TestManualWorkflowCancelSupersedesFailedApprovalCommand|TestTemporalClientRetryWaitsForAcceptedWhileOtherCommandsWaitForCompleted)$' -count=1`

Expected: PASS; cancellation is promptly accepted, no further slots start, the child finalizes before the run is terminally cancelled, and a pre-publication failed approval remains cancellable.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/imageagent/temporal/workflow.go internal/imageagent/temporal/worker.go internal/imageagent/temporal/workflow_test.go
git commit -m "fix: drain image effects before cancellation"
```

### Task 4: Encode the irreversible approval-publication boundary

**Files:**
- Modify: `internal/imageagent/temporal/workflow.go`
- Test: `internal/imageagent/temporal/workflow_test.go`

**Interfaces:**
- Produces `approvalPublicationCommitted(record workflowUpdateRecord) bool`, true only for the existing `updatePhaseApprovalPersistComplete` phase after `PublishApproved` has succeeded.
- Produces effect-owner fencing where a terminal identity is committed only after terminal persistence succeeds; a failed terminal action does not itself permanently fence later valid actions.
- Consumes `failedPendingActionCanBeSuperseded`, `applyApproveResults`, `workflowEffectOwner`, and the existing same-command resume identity.

- [ ] **Step 1: Write the failing irreversible-boundary tests**

Add a test that makes approval publication succeed, forces the completed-state persistence write to fail, and then sends cancel:

```go
publishApprovedSucceedsOnce()
failNextCompletedProjectionWrite()
require.Error(t, approveRun())
err := cancelRun()
require.ErrorContains(t, err, "approval publication is already committed")
require.Equal(t, 1, publisher.approvedCalls)
require.Equal(t, 0, repository.cancelledWrites)
```

Then resume the original approval command and assert the completed projection is persisted without a second `PublishApproved` call. Add a separate effect-owner test where a terminal persistence failure leaves no blanket identity fence, proving that the admission rule—not a stale owner reservation—selects the permitted follow-up action.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowRejectsCancelAfterApprovalPublicationBeforeProjection|TestManualWorkflowApprovalUpdateResumesAfterCompletedStateFailureWithoutRepublishing|TestWorkflowEffectOwnerDoesNotFenceFailedTerminalPersistence)$' -count=1`

Expected: FAIL because a failed terminal identity is reserved before `persistTerminalRunState` succeeds and cancellation admission does not recognise the publication boundary.

- [ ] **Step 3: Write minimal implementation**

Add the phase predicate:

```go
func approvalPublicationCommitted(record workflowUpdateRecord) bool {
    return record.kind == updateKindApprove &&
        record.phase == updatePhaseApprovalPersistComplete
}
```

Before accepting a cancel or superseding failed command, reject it with an application error when `approvalPublicationCommitted(record)` is true; accept only a resume whose command identity equals that record's approval identity. In `workflowEffectOwner`, set terminal intent only after `request.execute(ownerCtx)` succeeds for new histories:

```go
err := request.execute(ownerCtx)
if err != nil { continue }
if request.terminalIdentity != "" {
    terminalIntentIdentity = request.terminalIdentity
    terminalSucceeded = true
}
```

Preserve the legacy effect-owner order behind the same version marker for old histories. Do not change the pre-publication failure policy: a failed approval that never published remains cancellable.

- [ ] **Step 4: Run focused tests to verify they pass**

Run: `go test ./internal/imageagent/temporal -run '^(TestManualWorkflowRejectsCancelAfterApprovalPublicationBeforeProjection|TestManualWorkflowApprovalUpdateResumesAfterCompletedStateFailureWithoutRepublishing|TestManualWorkflowCancelSupersedesFailedApprovalCommand|TestWorkflowEffectOwnerDoesNotFenceFailedTerminalPersistence)$' -count=1`

Expected: PASS; publish-once recovery completes, cancellation cannot strand a post-publication failure, and pre-publication cancellation remains available.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/imageagent/temporal/workflow.go internal/imageagent/temporal/workflow_test.go
git commit -m "fix: preserve approval recovery after publication"
```

### Task 5: Verify the complete protocol and close the review loop

**Files:**
- Modify: `internal/imageagent/temporal/activities.go` only if focused test fixes are necessary
- Modify: `internal/imageagent/temporal/slot_workflow.go` only if focused test fixes are necessary
- Modify: `internal/imageagent/temporal/types.go` only if focused test fixes are necessary
- Modify: `internal/imageagent/temporal/workflow.go` only if focused test fixes are necessary
- Modify: `internal/imageagent/temporal/worker.go` only if focused test fixes are necessary
- Test: `internal/imageagent/temporal/*_test.go`

**Interfaces:**
- Consumes all four completed protocol slices and the existing image-agent repository contract suite.
- Produces local evidence suitable for updating the three P1 review threads; remote CI and thread resolution remain separate, explicit follow-up actions.

- [ ] **Step 1: Run package-level race and replay verification**

Run: `go test -race ./internal/imageagent/temporal -count=1`

Expected: PASS with all workflow, activity, client, budget, and history replay tests executing.

- [ ] **Step 2: Run the image-agent boundary suites**

Run: `go test ./internal/imageagent/... -count=1`

Expected: PASS; no package relies on eager terminal cancellation or stale terminal owner fencing.

- [ ] **Step 3: Inspect staged scope and commit any verification-driven correction**

Run: `git status --short` and `git diff --check`

Expected: only the explicitly named Temporal protocol files are modified; `git diff --check` is silent. If a focused correction is present, stage the exact changed paths and commit it as `test: verify image effect finalization protocol`.

- [ ] **Step 4: Push only the completed code commits and inspect the current PR state**

Run: `git push origin codex/image-agent-core-manual` followed by `gh pr view 239 --json url,headRefName,mergeStateStatus,statusCheckRollup,reviewThreads`

Expected: the pushed branch is `codex/image-agent-core-manual`; report CI state and unresolved review-thread count without claiming merge or deployment.

- [ ] **Step 5: Reply to and resolve the three P1 threads only after remote checks pass**

For thread IDs `PRRT_kwDOQg5lB86dOfdz`, `PRRT_kwDOQg5lB86dOfd8`, and `PRRT_kwDOQg5lB86dOfd9`, post concise replies that state the specific durable-finalization, deadline-grace, and approval-recovery evidence. Resolve each thread only after the relevant remote CI is successful; if CI fails or new threads appear, leave them open and diagnose the new evidence first.
