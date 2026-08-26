# Image Agent Automatic Evaluation and Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the image Agent workflow with candidate evaluation, duplicate detection, bounded slot repair, budget governance, final approval, and tenant-canary automatic mode.

**Architecture:** Deterministic and model-backed evaluators produce typed evidence for every candidate. A bounded Eino repair Agent may propose changes for rejected slots, while Temporal executes only validated repair commands and remains the owner of retries, budgets, approval waits, and durable state.

**Tech Stack:** Go 1.26, CloudWeGo Eino 0.9.15, goimagehash 1.1.0, Temporal Go SDK 1.43.0, existing ProductImage review and AI invocation ledger, React 19, Vitest.

**Spec:** `docs/superpowers/specs/2026-08-26-image-agent-workflow-design.md`

## Global Constraints

- Complete the core manual and assisted-planning implementation plans first.
- Pin `github.com/corona10/goimagehash` to `v1.1.0`; do not implement a new perceptual-hash algorithm.
- Technical retry, Agent reasoning retry, and semantic slot repair have separate owners and counters.
- Identity, authorization, IP, content-safety, tool, budget, and unknown-remote-state boundaries cannot be weakened by the Agent.
- Automatic mode publishes approved image assets only; it never publishes a listing or mutates canonical product facts.
- Automatic mode is disabled unless the capability flag and tenant allowlist both permit it.
- More than ten approved standard assets remain valid; target-platform selection is a later subset operation.

---

### Task 1: Add typed candidate evaluation and perceptual duplicate detection

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/imageagent/evaluation.go`
- Create: `internal/imageagent/evaluator.go`
- Create: `internal/imageagent/evaluator_test.go`
- Create: `internal/imageagent/tools/duplicate_detector.go`
- Create: `internal/imageagent/tools/duplicate_detector_test.go`
- Create: `internal/imageagent/tools/productimage_evaluator.go`
- Create: `internal/imageagent/tools/productimage_evaluator_test.go`

**Interfaces:**
- Produces: `CandidateEvaluator.Evaluate(context.Context, EvaluationInput) (EvaluationResult, error)` and `DuplicateDetector.Compare(context.Context, AssetRef, AssetRef) (DuplicateResult, error)`.
- Consumes: existing ProductImage review/quality capabilities and `internal/pkg/safeimagehttp` for bounded, SSRF-safe image bytes.

- [ ] **Step 1: Write failing authenticity and duplicate tests**

```go
func TestEvaluatorRejectsIdentityDriftEvenWhenQualityPasses(t *testing.T) {
    evaluator := NewCompositeEvaluator(stubQualityPass(), stubIdentityReview(false, "product_color_changed"), stubDuplicatePass())
    result, err := evaluator.Evaluate(context.Background(), evaluationInput("scene-2"))
    require.NoError(t, err)
    require.False(t, result.Accepted)
    require.Contains(t, result.ReasonCodes, "product_color_changed")
}

func TestDuplicateDetectorRejectsNearDuplicateCandidates(t *testing.T) {
    detector := NewPerceptualDuplicateDetector(safeLoaderForFixtures(t))
    result, err := detector.Compare(context.Background(), fixtureAsset("scene-a.png"), fixtureAsset("scene-a-resized.jpg"))
    require.NoError(t, err)
    require.True(t, result.Duplicate)
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent ./internal/imageagent/tools -run 'Test(EvaluatorRejectsIdentityDrift|DuplicateDetectorRejectsNearDuplicateCandidates)' -count=1`

Expected: FAIL because evaluation and duplicate contracts are absent.

- [ ] **Step 3: Pin goimagehash and implement composite evaluation**

Run: `go get github.com/corona10/goimagehash@v1.1.0`

```go
type EvaluationResult struct {
    Accepted bool
    QualityScore float64
    IdentityScore float64
    StyleScore float64
    ReasonCodes []string
    RepairHints []RepairHint
    DuplicateOfSlotID string
}

type EvaluationInput struct { RunID, SlotID string; Role SlotRole; Candidate AssetRef; AcceptedSiblings []AssetRef }
type RepairHint struct { Code, Message string }
type DuplicateResult struct { Duplicate bool; Distance int; Exact bool }
```

Use pHash distance through goimagehash with one configured threshold owned by `DuplicatePolicy`. Exact hashes short-circuit. Product identity review is a hard veto. Quality, role, format, style, IP, content-safety, duplicate, and lineage checks each emit stable reason codes.

- [ ] **Step 4: Verify malformed, inaccessible, and oversized images fail closed**

Run: `go test ./internal/imageagent ./internal/imageagent/tools -count=1`

Expected: PASS; duplicate analysis never fetches private hosts or unbounded bodies, and an evaluation outage blocks acceptance rather than silently passing.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/imageagent/evaluation.go internal/imageagent/evaluator.go internal/imageagent/evaluator_test.go internal/imageagent/tools
git commit -m "feat: evaluate image agent candidates"
```

### Task 2: Add a bounded repair Agent and slot-only repair loop

**Files:**
- Create: `internal/imageagent/repair.go`
- Create: `internal/imageagent/agent/eino_repair_planner.go`
- Create: `internal/imageagent/agent/eino_repair_planner_test.go`
- Modify: `internal/imageagent/temporal/workflow.go`
- Modify: `internal/imageagent/temporal/slot_workflow.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Create: `internal/imageagent/temporal/repair_workflow_test.go`

**Interfaces:**
- Produces: `RepairPlanner.PlanSlotRepair(context.Context, RepairInput) (RepairResult, error)`.
- Consumes: rejected slot, evaluation evidence, authorized sources/styles, and remaining budget.

- [ ] **Step 1: Write failing tests for slot-only repair and hard-rule protection**

```go
func TestRepairWorkflowRegeneratesOnlyRejectedSlot(t *testing.T) {
    env := newRepairWorkflowEnv(t, completedSlots(6), rejectedSlot("scene-2", "near_duplicate"))
    env.ExecuteWorkflow(ImageAgentWorkflow, env.Input())
    require.Equal(t, []string{"scene-2"}, env.ReexecutedSlotIDs())
    require.Equal(t, 6, env.UnchangedCompletedSlotCount())
}

func TestRepairPlannerCannotRelaxIdentityConstraint(t *testing.T) {
    planner := newRepairPlannerWithResult(t, RepairResult{ConstraintOverrides: map[string]bool{"preserve_product_identity": false}})
    _, err := planner.PlanSlotRepair(context.Background(), identityFailureInput())
    require.ErrorContains(t, err, "hard constraint")
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/agent ./internal/imageagent/temporal -run 'TestRepair' -count=1`

Expected: FAIL because repair types and loop are absent.

- [ ] **Step 3: Implement repair commands and bounded reasoning**

```go
type RepairResult struct {
    SlotID string
    SourceAssetIDs []string
    StyleReferenceIDs []string
    Brief string
    Strategy string
    Decision AgentDecision
}

type RepairInput struct {
    RunID string
    PlanRevision int64
    Slot Slot
    Evaluation EvaluationResult
    AuthorizedSourceAssetIDs []string
    AuthorizedStyleReferenceIDs []string
    RemainingBudget BudgetUsage
}
```

Allow only source/style replacement, reduced style influence, changed scene/props/lighting/composition, faithful-edit routing, or manual escalation. Validate every reference and strategy. The parent creates one new slot attempt and leaves completed siblings immutable.

- [ ] **Step 4: Verify loop termination and replay**

Run: `go test ./internal/imageagent/agent ./internal/imageagent/temporal -count=1`

Expected: PASS; per-slot repair limit, Agent step limit, timeout, and cost exhaustion enter explicit `blocked` states, and replay does not duplicate repaired attempts.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/repair.go internal/imageagent/agent/eino_repair_planner.go internal/imageagent/agent/eino_repair_planner_test.go internal/imageagent/temporal
git commit -m "feat: add bounded image slot repair"
```

### Task 3: Enforce budgets, retry ownership, and unknown remote reconciliation

**Files:**
- Create: `internal/imageagent/budget.go`
- Create: `internal/imageagent/budget_test.go`
- Create: `internal/imageagent/failure_policy.go`
- Create: `internal/imageagent/failure_policy_test.go`
- Modify: `internal/imageagent/temporal/slot_workflow.go`
- Create: `internal/imageagent/tools/remote_reconciler.go`
- Create: `internal/imageagent/tools/remote_reconciler_test.go`
- Modify: `internal/aicapability/invocation.go`

**Interfaces:**
- Produces: `BudgetPolicy`, `BudgetUsage`, `FailureDisposition`, and `RemoteReconciler`.
- Consumes: invocation-ledger usage and provider request/job IDs.

- [ ] **Step 1: Write failing budget and timeout tests**

```go
func TestBudgetRejectsNextAttemptBeforeSideEffect(t *testing.T) {
    budget := Budget{MaxImages: 7, MaxCostMicros: 700000, MaxRepairAttemptsPerSlot: 2}
    usage := BudgetUsage{Images: 7, EstimatedCostMicros: 650000}
    require.ErrorIs(t, budget.Authorize(usage, AttemptEstimate{Images: 1, CostMicros: 100000}), ErrBudgetExceeded)
}

func TestUnknownRemoteStateReconcilesBeforeResubmit(t *testing.T) {
    executor := newExecutorWithSubmitTimeoutAndRemoteSuccess(t)
    result, err := executor.Execute(context.Background(), slotAttempt())
    require.NoError(t, err)
    require.Equal(t, 1, executor.SubmitCalls())
    require.Equal(t, 1, executor.QueryCalls())
    require.Equal(t, "asset://remote-success", result.Candidates[0].AssetID)
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent ./internal/imageagent/tools -run 'Test(BudgetRejects|UnknownRemoteState)' -count=1`

Expected: FAIL because budget and reconciler types are absent.

- [ ] **Step 3: Implement preflight authorization and normalized dispositions**

```go
type AttemptEstimate struct { Images, ModelCalls int; CostMicros int64 }
type FailureDisposition string
const (
    FailureRetryTechnical FailureDisposition = "retry_technical"
    FailureNoRetry FailureDisposition = "no_retry"
    FailureReconcileRemote FailureDisposition = "reconcile_remote"
    FailureNeedsHuman FailureDisposition = "needs_human"
)
```

Budget dimensions are image count, estimated cost, Agent steps, model calls, repair attempts per slot, and elapsed time. Reserve before generation and settle from invocation usage. Map `rate_limited`, `provider_timeout`, and `provider_unavailable` to bounded technical retry; map invalid input, policy, credential, tool, identity, IP, and budget errors to no retry; map `unknown_remote_state` to reconciliation only.

- [ ] **Step 4: Verify no double owner retries**

Run: `go test ./internal/imageagent/... ./internal/aicapability/... -count=1`

Expected: PASS; one failure category has one retry owner, ledger records retain Agent run/parent IDs, and reconciliation never creates a second submit.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/budget.go internal/imageagent/budget_test.go internal/imageagent/failure_policy.go internal/imageagent/failure_policy_test.go internal/imageagent/temporal/slot_workflow.go internal/imageagent/tools/remote_reconciler.go internal/imageagent/tools/remote_reconciler_test.go internal/aicapability/invocation.go
git commit -m "feat: govern image agent retries and budgets"
```

### Task 4: Add tenant-canary automatic approval policy

**Files:**
- Create: `internal/imageagent/policy.go`
- Create: `internal/imageagent/policy_test.go`
- Modify: `internal/imageagent/temporal/types.go`
- Modify: `internal/imageagent/temporal/workflow.go`
- Create: `internal/imageagent/temporal/automatic_workflow_test.go`
- Modify: `internal/imageagent/httpapi/handler.go`
- Modify: `internal/imageagent/httpapi/routes.go`
- Modify: `internal/core/config/type_productimage.go`
- Modify: `internal/app/httpapi/composition_builder.go`

**Interfaces:**
- Produces: `ModePolicy` and automatic-mode route gating over the existing final-approval contract.
- Consumes: tenant capability flag, allowlist, budget, and candidate evaluations.

- [ ] **Step 1: Write failing policy and final-approval tests**

```go
func TestAutomaticModeFailsClosedWithoutTenantAllowlist(t *testing.T) {
    policy := NewModePolicy(Config{AutomaticEnabled: true, AutomaticTenantIDs: nil})
    require.ErrorIs(t, policy.Authorize("tenant-a", RunModeAutomatic), ErrModeDenied)
}

func TestAuthorizedAutomaticWorkflowPublishesOnlyAfterEveryHardCheckPasses(t *testing.T) {
    env := newFullyEvaluatedAutomaticEnv(t)
    env.ExecuteWorkflow(ImageAgentWorkflow, env.Input())
    require.Equal(t, 1, env.PublishedAssetCalls())
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent ./internal/imageagent/temporal -run 'Test(AutomaticMode|AuthorizedAutomaticWorkflow)' -count=1`

Expected: FAIL because mode policy and final approval are absent.

- [ ] **Step 3: Implement fail-closed mode policy and final publication gate**

```go
type ModePolicy struct { AutomaticEnabled bool; AutomaticTenantIDs map[string]struct{}; RequireFinalApproval bool }
func (p ModePolicy) Authorize(tenantID string, mode RunMode) error
```

Automatic mode requires `TASK_PROCESSOR_IMAGE_AGENT_AUTOMATIC_ENABLED=true` and a non-empty `TASK_PROCESSOR_IMAGE_AGENT_AUTOMATIC_ALLOWED_TENANT_IDS` containing the verified tenant. Manual and assisted modes continue to require the exact-revision final approval introduced in the core plan. Automatic publication receives only candidates that passed every hard check and writes standard assets; it has no listing-publication port.

- [ ] **Step 4: Verify route, workflow, and configuration behavior**

Run: `go test ./internal/imageagent/... ./internal/core/config ./internal/app/httpapi -count=1`

Expected: PASS; disabled or empty allowlist fails closed, assisted waits, and allowed automatic mode completes only when every hard evaluation passes.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/policy.go internal/imageagent/policy_test.go internal/imageagent/temporal internal/imageagent/httpapi internal/core/config/type_productimage.go internal/app/httpapi/composition_builder.go
git commit -m "feat: gate automatic image agent mode"
```

### Task 5: Complete review UI, more-than-ten acceptance, and canary runbook

**Files:**
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/candidate-review-card.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/candidate-review-card.test.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/agent-run-diagnostics.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/agent-run-diagnostics.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.tsx`
- Create: `internal/imageagent/automatic_acceptance_test.go`
- Create: `internal/listingkit/image_agent_asset_selection.go`
- Create: `internal/listingkit/image_agent_asset_selection_test.go`
- Create: `docs/development/image-agent-automatic-canary.md`

**Interfaces:**
- Consumes: complete run projection, candidate evaluations, allowed actions, and approved standard asset IDs.
- Produces: operator-visible evaluation/repair controls and controlled canary instructions.

- [ ] **Step 1: Write failing UI and standard-asset boundary tests**

```tsx
it("shows blocker, cost, retry owner, and next action", () => {
  render(<AgentRunDiagnostics run={budgetBlockedRun()} />);
  expect(screen.getByText("费用预算已用尽")).toBeInTheDocument();
  expect(screen.getByText("已产生费用 ¥0.65")).toBeInTheDocument();
  expect(screen.getByText("不会自动重试")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "切换为手动处理" })).toBeEnabled();
});
```

```go
func TestPlatformSelectionDoesNotRejectOrDeleteElevenStandardAssets(t *testing.T) {
    assets := approvedStandardAssets(11)
    selected, err := SelectForPlatform("shein", assets)
    require.NoError(t, err)
    require.NotEmpty(t, selected)
    require.Len(t, assets, 11)
}
```

- [ ] **Step 2: Verify RED**

Run backend: `go test ./internal/imageagent ./internal/listingkit -run 'TestPlatformSelectionDoesNotRejectOrDeleteElevenStandardAssets' -count=1`

Run frontend: `npm.cmd test -- --run src/components/listingkit/image-agent`

Working directory for frontend: `web/listingkit-ui`

- [ ] **Step 3: Implement evaluation, repair, and blocker surfaces**

Candidate cards show source/style lineage, quality/identity/style scores, stable reason codes, duplicate target, incurred cost, and accept/discard/regenerate actions. Diagnostics show current node, retry owner, next retry time, automatic retry decision, remaining budget, blocking reason, and allowed manual takeover.

Implement `SelectForPlatform(platform string, assets []asset.Asset) ([]asset.Asset, error)` as a pure platform-adaptation selector. It returns a platform-sized ordered subset without mutating or deleting the input standard assets.

- [ ] **Step 4: Run full focused acceptance**

```bash
go test ./internal/imageagent/... ./internal/productimage/... ./internal/aicapability/... ./internal/listingkit/... -count=1
go test ./internal/app/schema/productlisting ./internal/app/httpapi ./internal/app/runtime -count=1
cd web/listingkit-ui && npm.cmd test -- --run src/components/listingkit/image-agent src/components/listingkit/workspace && npm.cmd run typecheck
git diff --check
```

Expected: PASS. `automatic_acceptance_test.go` covers duplicate repair, identity rejection, unknown remote reconciliation, budget block, manual takeover, and tenant-canary completion.

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/image-agent internal/imageagent/automatic_acceptance_test.go internal/listingkit/image_agent_asset_selection.go internal/listingkit/image_agent_asset_selection_test.go docs/development/image-agent-automatic-canary.md
git commit -m "test: verify automatic image agent canary"
```

## Plan Self-Review

- Tasks 1-5 cover candidate evaluation, open-source duplicate detection, slot-only repair, budget/retry ownership, final approval, automatic-mode gating, complete diagnostics, and more-than-ten standard assets.
- `EvaluationResult`, `RepairResult`, `BudgetPolicy`, `FailureDisposition`, `ModePolicy`, and `ApproveResultsSignal` are defined before use and preserve the earlier plan contracts.
- Eino proposes repairs but cannot execute generation or relax hard rules; Temporal remains the only durable workflow owner.
- The automatic path cannot publish listings, access cross-tenant assets, silently fall back, or retry unknown remote submissions blindly.
