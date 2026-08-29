# Image Agent Budget Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Enforce PR #239 run budgets before every governed ProductImage effect while closing the four related identity, identifier, retry-transport, and SSE review findings.

**Architecture:** Add provider-neutral quote and receipt contracts to the v3 slot executor, then bind budget reservation to the existing v3 provider-effect transaction. Temporal carries immutable policy and identity data, uses a version marker for new behavior, and projects committed usage; remote metering remains downstream of the local lock.

**Tech Stack:** Go 1.x, Temporal Go SDK, GORM, SQLite/PostgreSQL repository contracts, Next.js, React, TypeScript, Vitest.

**Spec:** docs/superpowers/specs/2026-08-28-image-agent-budget-authorization-design.md

## Global Constraints

- Preserve frozen v2 Temporal payloads, activity names, and replay behavior.
- Existing histories without the new policy version remain uncapped.
- Omitted limits are disabled, zero is a hard zero, positive values are hard limits, and negative values are invalid.
- No provider call may occur without a durable matching reservation.
- Unknown provider outcomes retain their reservation and are never retried automatically.
- OpenMeter and subscription entitlements are not part of the authorization transaction.
- Stage only files owned by the current task; do not deploy or alter production configuration.

---

### Task 1: Shared durable IDs and execution identity

**Files:**
- Modify: internal/imageagent/artifact.go
- Modify: internal/imageagent/validation.go
- Modify: internal/imageagent/service.go
- Modify: internal/imageagent/ports.go
- Modify: internal/imageagent/temporal/types.go
- Modify: internal/imageagent/temporal/worker.go
- Modify: internal/imageagent/temporal/workflow.go
- Modify: internal/imageagent/temporal/slot_workflow.go
- Modify: internal/imageagent/temporal/activities.go
- Test: internal/imageagent/service_commands_test.go
- Test: internal/imageagent/artifact_compatibility_test.go
- Test: internal/imageagent/temporal/slot_effect_v3_activity_test.go

**Interfaces:**
- Produces: ValidateArtifactKeyIdentifier(string) error
- Produces: ExecutionIdentity.BusinessTaskID string
- Produces: WorkflowInput.BusinessTaskID and v3 child/activity propagation

- [ ] **Step 1: Add failing identifier-boundary tests**

    func TestStartRejectsRunIDOutsideArtifactGrammar(t *testing.T) {
        service, workflows := newServiceFixture(t)
        err := service.Start(verifiedContext(), startInputWithRunID("run:1"))
        require.ErrorIs(t, err, imageagent.ErrValidation)
        require.Zero(t, workflows.startCalls)
    }

    func TestValidatePlanRejectsSlotIDOutsideArtifactGrammar(t *testing.T) {
        plan := validPlan()
        plan.Slots[0].ID = "slot/1"
        require.Error(t, imageagent.ValidatePlan(plan))
    }

- [ ] **Step 2: Run the identifier tests and confirm RED**

Run: go test ./internal/imageagent -run 'Test(StartRejectsRunIDOutsideArtifactGrammar|ValidatePlanRejectsSlotIDOutsideArtifactGrammar)$' -count=1

Expected: tests fail because run:1 and slot/1 are currently accepted.

- [ ] **Step 3: Centralize the artifact identifier grammar**

Move the existing artifactKeyIdentifierPattern ownership behind:

    func ValidateArtifactKeyIdentifier(value string) error {
        if !artifactKeyIdentifierPattern.MatchString(value) {
            return ErrValidation
        }
        return nil
    }

Use it in Start, ValidatePlan, RetrySlot, and published-artifact validation.

- [ ] **Step 4: Add a failing ProductImage identity restoration test**

The v3 activity test executor reads productimage.AIIdentityFromContext and
asserts tenant, user, and business task all match the WorkflowStart run.

- [ ] **Step 5: Run the activity identity test and confirm RED**

Run: go test ./internal/imageagent/temporal -run 'TestExecuteSlotV3RestoresProductImageBusinessIdentity$' -count=1

Expected: business task and ProductImage identity are empty.

- [ ] **Step 6: Propagate and restore the unified identity**

Add BusinessTaskID to ExecutionIdentity, populate it in Service.Start, carry it
through WorkflowInput, SlotWorkflowV3Input, ExecuteSlotV3ActivityInput, and
restore both authidentity and productimage.WithAIIdentity in activities.

- [ ] **Step 7: Verify Task 1**

Run: go test ./internal/imageagent ./internal/imageagent/temporal -run 'Artifact|Identifier|Identity|Start|ExecuteSlotV3' -count=1

- [ ] **Step 8: Commit Task 1**

    git add internal/imageagent
    git commit -m "fix: enforce image agent durable identity"

---

### Task 2: Retry acceptance and SSE lifecycle

**Files:**
- Modify: internal/imageagent/temporal/worker.go
- Test: internal/imageagent/temporal/canary_test.go
- Modify: web/listingkit-ui/src/lib/api/image-agent.ts
- Modify: web/listingkit-ui/src/components/listingkit/image-agent/use-image-agent-run.ts
- Test: web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.test.tsx

**Interfaces:**
- Produces: RetrySlot waits for WorkflowUpdateStageAccepted without handle.Get
- Produces: imageAgentEventsUrl(runID, afterCursor)

- [ ] **Step 1: Add a failing Temporal client test**

Use a fake sdkWorkflowClient that records UpdateWorkflowOptions and returns a
handle whose Get fails the test. Assert RetrySlot uses
WorkflowUpdateStageAccepted and never calls Get.

- [ ] **Step 2: Run the client test and confirm RED**

Run: go test ./internal/imageagent/temporal -run 'TestClientRetrySlotReturnsAfterUpdateAccepted$' -count=1

Expected: current code records WorkflowUpdateStageCompleted and calls Get.

- [ ] **Step 3: Implement accepted-stage retry submission**

Add executeAcceptedCommandUpdate and use it only from RetrySlot. Keep replace,
approve, cancel, and resume on their current completed semantics.

- [ ] **Step 4: Add a failing SSE retention test**

After two increasing projection events, assert the snapshot endpoint is
refreshed twice, FakeEventSource.instances has length one, and the first source
is not closed. Add a separate onerror test that expects a reconnect URL with
after_cursor equal to the committed cursor.

- [ ] **Step 5: Run the Vitest file and confirm RED**

Run from web/listingkit-ui:

    npm.cmd test -- --run src/components/listingkit/image-agent/image-agent-workbench.test.tsx

Expected: a healthy event closes the source and creates another instance.

- [ ] **Step 6: Separate refresh from reconnect**

Healthy projection events call refreshSnapshot only. onerror closes and
reconnects. imageAgentEventsUrl appends the positive stored cursor as
after_cursor; initial connections omit it.

- [ ] **Step 7: Verify Task 2**

Run the focused Go test and focused Vitest file again.

- [ ] **Step 8: Commit Task 2**

    git add internal/imageagent/temporal/worker.go internal/imageagent/temporal/canary_test.go web/listingkit-ui/src/lib/api/image-agent.ts web/listingkit-ui/src/components/listingkit/image-agent
    git commit -m "fix: align image agent command transport"

---

### Task 3: Explicit budget policy and usage vectors

**Files:**
- Create: internal/imageagent/budget.go
- Test: internal/imageagent/budget_test.go
- Modify: internal/imageagent/model.go
- Modify: internal/imageagent/httpapi/dto.go
- Test: internal/imageagent/httpapi/handler_test.go
- Modify: web/listingkit-ui/src/lib/types/image-agent.ts
- Modify: web/listingkit-ui/src/lib/api/image-agent.ts

**Interfaces:**
- Produces: Limit with Enabled bool and Value int64
- Produces: BudgetPolicy with Images, AgentSteps, ModelCalls, RepairAttemptsPerSlot, CostMicros, MaxElapsed
- Produces: UsageVector and CheckedAddUsage
- Produces: BudgetPolicy.Allows(committed, reserved, quote) error
- Extends: public budget projection with additive enabled_limits metadata

- [ ] **Step 1: Add failing domain tests**

Cover omitted versus explicit zero, negative rejection, overflow rejection,
and denial when any single dimension exceeds its cap.

    policy := BudgetPolicy{Images: Limit{Enabled: true, Value: 1}}
    require.NoError(t, policy.Allows(UsageVector{}, UsageVector{}, UsageVector{Images: 1}))
    require.ErrorIs(t, policy.Allows(UsageVector{Images: 1}, UsageVector{}, UsageVector{Images: 1}), ErrBudgetExceeded)

- [ ] **Step 2: Run budget tests and confirm RED**

Run: go test ./internal/imageagent -run 'TestBudget' -count=1

Expected: policy and usage-vector types do not exist.

- [ ] **Step 3: Implement normalized policy types**

Use explicit Limit fields internally. Split input and output DTOs: decode
request limits with pointers so explicit zero is preserved, while retaining all
existing numeric response keys for compatibility. Add an optional
enabled_limits list to the response so new clients can distinguish disabled
from explicit zero; older snapshots infer positive values as enabled and zero
as disabled. Reject negative and overflowing durations.

- [ ] **Step 4: Add and run HTTP transport tests**

Verify omitted is disabled, zero is enabled with value zero, negative returns
HTTP 400 before Application.Start, and response JSON keeps every existing
numeric key plus the additive enabled_limits list.

- [ ] **Step 5: Verify Task 3**

Run: go test ./internal/imageagent ./internal/imageagent/httpapi -run 'Budget|StartRun' -count=1

- [ ] **Step 6: Commit Task 3**

    git add internal/imageagent/budget.go internal/imageagent/budget_test.go internal/imageagent/model.go internal/imageagent/httpapi web/listingkit-ui/src/lib
    git commit -m "feat: define image agent budget policy"

---

### Task 4: Provider quote and receipt contract

**Files:**
- Modify: internal/imageagent/ports.go
- Modify: internal/imageagent/slot_effect_v3.go
- Modify: internal/imageagent/tools/productimage_executor.go
- Test: internal/imageagent/tools/productimage_executor_test.go
- Modify: internal/productimage/providers/interfaces.go
- Modify: internal/productimage/interfaces.go
- Modify: internal/productimage/default_components.go
- Modify: internal/productimage/model_subject_extractor.go
- Modify: internal/productimage/model_white_background_renderer.go
- Modify: internal/productimage/model_scene_renderer.go
- Modify: internal/productimage/real_components.go
- Modify: internal/productimage/scene_renderer.go
- Modify: internal/productimage/httpapi/image_agent_capabilities.go
- Test: internal/productimage/*_test.go

**Interfaces:**
- Produces: SlotUsageQuote with Maximum UsageVector, Operations, PricingVersion, Fingerprint
- Produces: SlotUsageReceipt with Actual UsageVector, ProviderRequestIDs, CostBasis
- Produces: ProductImage CapabilityUsageQuoter without importing imageagent
- Produces: StagedSlotExecutor.QuoteSlot and generated output UsageReceipt
- Produces: typed provider dispatch classification for failure settlement

- [ ] **Step 1: Add failing quote coverage tests**

Assert main quotes its two governed operations while deriving model-call,
output, and cost quantities from the configured subject/white-background
capability quoters. Assert scene derives the same quantities from its renderer
quoter, including a finite maximum output count that may be greater than one.
Quote fingerprints change when slot input or pricing version changes. A capped
dimension without a trustworthy capability quote returns
ErrBudgetQuoteUnavailable before renderer invocation.

- [ ] **Step 2: Run ProductImage tests and confirm RED**

Run: go test ./internal/imageagent/tools -run 'TestProductImageSlotUsage' -count=1

Expected: QuoteSlot and usage receipt contracts do not exist.

- [ ] **Step 3: Implement deterministic ProductImage quotes**

Quote the existing operation graph. Local deterministic components return zero
model calls and zero cost; model-backed components return their conservative
upper bound. Bind the aggregate quote fingerprint to
SlotExecutionFingerprint, component quote fingerprints, operation names,
per-operation output maxima, and pricing version. Settle observed output and
operation counts while retaining the conservative cost/model-call upper bound
where the provider does not return trustworthy actual usage.

Keep ProductImage independent from imageagent by defining its capability-level
quote contract in internal/productimage/providers and mapping it in the
ProductImage slot adapter. Every constructor used by the image-agent bootstrap
must return a component that implements the quote contract; an injected legacy
component without a quote remains usable only by legacy uncapped workflows.

- [ ] **Step 4: Enforce quoted output count**

Before provider invocation, derive the maximum output allowance from the
component quote and pass it through when the provider supports an output-count
request. Preserve the existing multi-candidate SceneRenderer contract. Reject
any returned result above the quoted allowance as a provider contract violation
and classify the reserved effect as unknown, not released.

Consume each ordered operation immediately before invoking its component.
Return a typed execution failure carrying not-dispatched, rejected-before-
effect, or dispatched-unknown state plus any provider request identities; the
activity must never infer release safety from error text.

- [ ] **Step 5: Verify Task 4**

Run: go test ./internal/imageagent/tools -count=1

- [ ] **Step 6: Commit Task 4**

    git add internal/imageagent/ports.go internal/imageagent/slot_effect_v3.go internal/imageagent/tools internal/productimage/providers/interfaces.go internal/productimage/interfaces.go internal/productimage/default_components.go internal/productimage/model_subject_extractor.go internal/productimage/model_white_background_renderer.go internal/productimage/model_scene_renderer.go internal/productimage/real_components.go internal/productimage/scene_renderer.go internal/productimage/httpapi/image_agent_capabilities.go internal/productimage/*_test.go
    git commit -m "feat: quote image agent provider usage"

---

### Task 5: Atomic v3 budget reservation

**Files:**
- Modify: internal/imageagent/slot_effect_v3.go
- Modify: internal/imageagent/store/memory.go
- Modify: internal/imageagent/store/records.go
- Modify: internal/imageagent/store/gorm.go
- Modify: internal/imageagent/store/slot_effect_v3.go
- Modify: internal/imageagent/store/projection.go
- Test: internal/imageagent/store/slot_effect_v3_repository_test.go
- Test: internal/imageagent/store/gorm_concurrency_test.go
- Test: internal/imageagent/store/records_postgres_test.go

**Interfaces:**
- Produces: SlotBudgetStatus values reserved, committed, released, unknown
- Extends: SlotEffectV3Reservation with Policy and Quote
- Produces: SettleSlotProviderV3 and MarkSlotProviderBudgetUnknownV3
- Persists: separate committed and reserved aggregate vectors on the run row
- Changes: Repository.GetProjection overlays authoritative run budget/usage

- [ ] **Step 1: Add repository contract tests**

For memory and GORM repositories, assert:

- a quote inside the last unit is claimed;
- a concurrent second quote is denied before claim;
- idempotent replay does not add reserved usage twice;
- mismatched quote fingerprint conflicts;
- commit moves reserved to Run.Usage exactly once;
- release removes reservation without committed usage;
- unknown remains counted against admission.
- the run-row reserved aggregate changes in the same transaction as every
  reservation transition.

- [ ] **Step 2: Run repository tests and confirm RED**

Run: go test ./internal/imageagent/store -run 'TestSlotEffectV3Budget' -count=1

Expected: repository has no budget reservation lifecycle.

- [ ] **Step 3: Extend the v3 effect record**

Add budget status, quote JSON/fingerprint, receipt JSON, pricing version, and
settlement timestamps to image_agent_v3_slot_external_effects. Additive schema
migration only; do not alter frozen v2 records.

- [ ] **Step 4: Implement memory atomicity**

Under the existing repository mutex, load run policy plus committed and
reserved aggregate vectors, authorize, then create the provider claim and
increment the run's reserved vector atomically. Commit/release/unknown
transitions validate the existing quote and update both authoritative run
aggregates idempotently. Effect-row scans are reconciliation evidence, not the
normal admission counter.

- [ ] **Step 5: Implement GORM atomicity**

Inside one transaction, lock the owner-scoped run row, authorize against its
committed and reserved vectors, insert the v3 effect, and increment the reserved
aggregate. Retry supported SQLite busy/unique races using the existing
repository patterns. Settlement locks the same run and effect records and
moves or releases the reserved aggregate exactly once. GetProjection overlays
Budget and Usage from the authoritative run row so a settlement cannot be
hidden by an older serialized projection snapshot.

- [ ] **Step 6: Verify concurrency and schema**

Run:

    go test ./internal/imageagent/store -run 'Budget|Concurrent|Postgres|AutoMigrate' -count=1
    go test -race ./internal/imageagent/store -run 'TestSlotEffectV3Budget' -count=1

- [ ] **Step 7: Commit Task 5**

    git add internal/imageagent/slot_effect_v3.go internal/imageagent/store
    git commit -m "feat: reserve image agent budget atomically"

---

### Task 6: Temporal authorization, settlement, and replay

**Files:**
- Modify: internal/imageagent/temporal/types.go
- Modify: internal/imageagent/temporal/worker.go
- Modify: internal/imageagent/temporal/workflow.go
- Modify: internal/imageagent/temporal/slot_workflow.go
- Modify: internal/imageagent/temporal/activities.go
- Test: internal/imageagent/temporal/workflow_test.go
- Test: internal/imageagent/temporal/slot_effect_v3_activity_test.go
- Test: internal/imageagent/temporal/history_replay_test.go
- Test: internal/imageagent/temporal/manual_acceptance_test.go

**Interfaces:**
- Produces: imageAgentBudgetAuthorizationPatch Temporal marker
- Carries: BudgetPolicy, StartedAt, DeadlineAt, BusinessTaskID
- Produces: budget_exhausted, budget_quote_unavailable, and budget_elapsed block codes

- [ ] **Step 1: Add failing workflow/activity tests**

Cover initial slot denial before activity execution, repair-attempt denial,
elapsed denial, successful quote-reserve-generate-settle, activity retry
idempotency, and provider ambiguity retaining the reservation.

- [ ] **Step 2: Run focused Temporal tests and confirm RED**

Run: go test ./internal/imageagent/temporal -run 'Test.*Budget' -count=1

Expected: workflow input drops policy and activities do not reserve.

- [ ] **Step 3: Add the replay marker and additive inputs**

StartManual passes the persisted policy and absolute timestamps. New workflows
select budget authorization through GetVersion; DefaultVersion retains the
legacy uncapped behavior for captured histories.

- [ ] **Step 4: Enforce workflow-only limits**

Before starting a child or retry, enforce deadline and repair attempts. Project
deterministic budget block codes without scheduling a provider activity.

- [ ] **Step 5: Enforce activity reservation and settlement**

Quote first, reserve and claim atomically, run provider only for an owned
claim, settle with the receipt during staging preparation, and mark unknown
for every post-dispatch ambiguous error. Apply the absolute deadline to the
activity context.

- [ ] **Step 6: Verify replay and recovery**

Run:

    go test ./internal/imageagent/temporal -run 'Budget|Replay|Recovery|ExecuteSlotV3' -count=1
    go test ./internal/imageagent/temporal -run 'TestImageAgentWorkflowReplays' -count=1

- [ ] **Step 7: Commit Task 6**

    git add internal/imageagent/temporal
    git commit -m "feat: enforce image agent budget in temporal"

---

### Task 7: Projection and end-to-end verification

**Files:**
- Modify as needed: internal/imageagent/projection_json.go
- Modify as needed: internal/imageagent/httpapi/dto.go
- Test: internal/imageagent/projection_test.go
- Test: internal/imageagent/temporal/manual_acceptance_test.go
- Test: web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.test.tsx

**Interfaces:**
- Consumes: committed Run.Usage from Task 5
- Produces: stable public budget/usage projection

- [ ] **Step 1: Add a failing projection test**

Commit a budgeted v3 attempt, reload through Repository.GetProjection and the
HTTP DTO, and assert committed usage survives while reserved/unknown internal
details are not exposed.

- [ ] **Step 2: Run projection tests and confirm RED**

Run: go test ./internal/imageagent ./internal/imageagent/httpapi -run 'Budget|Usage|Projection' -count=1

- [ ] **Step 3: Complete projection mapping**

Map only committed vectors and elapsed duration to the public Run.Usage. Keep
quote, pricing, request identities, and reservation states internal.

- [ ] **Step 4: Run complete focused verification**

Run:

    go test ./internal/imageagent/... -count=1
    go test -race ./internal/imageagent/... -count=1
    go vet ./internal/imageagent/...

From web/listingkit-ui:

    npm.cmd test -- --run src/components/listingkit/image-agent/image-agent-workbench.test.tsx src/app/api/listing-kits/route.test.ts
    npm.cmd run lint
    npm.cmd run typecheck

- [ ] **Step 5: Inspect the final diff**

Run:

    git diff origin/main...HEAD --check
    git status --short

Confirm no unrelated files, secrets, deployment changes, or frozen v2 payload
changes are present.

- [ ] **Step 6: Commit projection changes**

    git add internal/imageagent web/listingkit-ui/src/components/listingkit/image-agent web/listingkit-ui/src/lib/api/image-agent.ts web/listingkit-ui/src/lib/types/image-agent.ts
    git commit -m "test: verify image agent budget recovery"

- [ ] **Step 7: Push and close review threads**

Push codex/image-agent-core-manual, wait for required CI, reply to each inline
review thread with the exact root fix and verification, and resolve only after
the updated checks and local evidence support the claim.
