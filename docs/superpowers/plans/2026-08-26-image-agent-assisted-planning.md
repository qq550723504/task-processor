# Image Agent Assisted Planning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Eino-backed Agent planning, tenant-authorized style references, immutable plan approval, and the default Agent-assisted workbench flow on top of the manual workflow.

**Architecture:** CloudWeGo Eino runs only inside a bounded planner activity through repository-owned interfaces. It can call read-only and pure tools, then returns a strict `PlanningResult`; deterministic validation and Temporal approval remain authoritative before any generation side effect.

**Tech Stack:** Go 1.26, CloudWeGo Eino 0.9.15, Temporal Go SDK 1.43.0, existing AI capability routing/prompt ledger, React 19, Vitest.

**Spec:** `docs/superpowers/specs/2026-08-26-image-agent-workflow-design.md`

## Global Constraints

- Complete `2026-08-26-image-agent-core-manual-workflow.md` first.
- Pin `github.com/cloudwego/eino` to `v0.9.15`; do not depend on alpha `v0.10` releases.
- Eino and provider DTOs stay behind `imageagent.Planner`; domain, HTTP, and Temporal contracts remain framework-neutral.
- Planner tools are read-only or pure. Generation, persistence, publication, database access, and arbitrary network access are not Agent tools.
- Product source assets and style references are distinct typed inputs.
- Assisted mode is the default and must wait for approval of the exact plan revision.
- Do not store hidden model reasoning; store decision summary, evidence, tool calls, confidence, and stop reason.

---

### Task 1: Add a framework-neutral planner contract and Eino adapter

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/imageagent/planner.go`
- Create: `internal/imageagent/agent/eino_planner.go`
- Create: `internal/imageagent/agent/eino_planner_test.go`
- Create: `internal/imageagent/agent/capability_chat_model.go`
- Create: `internal/imageagent/agent/capability_chat_model_test.go`
- Create: `internal/imageagent/agent/boundary_test.go`

**Interfaces:**
- Consumes: `PlanningInput`, `PlanningTool`, and the existing provider-neutral chat-model capability adapter.
- Produces: `Planner.PlanImages(context.Context, PlanningInput) (PlanningResult, error)`.

- [ ] **Step 1: Write failing schema and boundary tests**

```go
type Planner interface {
    PlanImages(context.Context, PlanningInput) (PlanningResult, error)
}

type PlanningInput struct {
    RunID, TenantID, UserID string
    ProductContext ProductContextRef
    SourceAssets []AssetRef
    AuthorizedStyleReferenceIDs []string
    Budget Budget
}

type PlanningTool interface {
    Name() string
    Schema() json.RawMessage
    Invoke(context.Context, json.RawMessage) (json.RawMessage, error)
}

func TestEinoPlannerReturnsValidatedStructuredPlan(t *testing.T) {
    planner := newPlannerWithScriptedModel(t, validSevenSlotResponse())
    result, err := planner.PlanImages(context.Background(), planningInputWithNineSources())
    require.NoError(t, err)
    require.Len(t, result.Plan.Slots, 7)
    require.Equal(t, imageagent.SlotRoleMain, result.Plan.Slots[0].Role)
}

func TestImageAgentDomainDoesNotImportEino(t *testing.T) {
    assertNoImportPrefix(t, "task-processor/internal/imageagent", "github.com/cloudwego/eino", []string{"internal/imageagent/agent"})
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/agent ./internal/imageagent -run 'Test(EinoPlannerReturnsValidatedStructuredPlan|ImageAgentDomainDoesNotImportEino)' -count=1`

Expected: FAIL because the planner adapter and dependency are absent.

- [ ] **Step 3: Pin Eino and implement the bounded adapter**

Run: `go get github.com/cloudwego/eino@v0.9.15`

```go
type PlanningResult struct {
    Plan Plan
    Decision AgentDecision
}

type AgentDecision struct {
    Summary string
    EvidenceRefs []string
    ToolCalls []ToolCallRecord
    Confidence float64
    StopReason string
}
```

Use Eino's Agent/tool abstractions with a maximum of eight steps. `capability_chat_model.go` implements Eino's tool-calling chat-model interface by translating requests through the existing governed `aicapability` route and recording the returned invocation identity; it does not select providers itself. Parse the final response into the strict result schema, reject unknown roles or references, run `ValidatePlan`, and map tool denial, step limit, malformed structured output, timeout, and budget errors to existing `aicapability.ErrorCategory` values.

- [ ] **Step 4: Verify tool allowlisting and no hidden reasoning persistence**

Run: `go test ./internal/imageagent/agent ./internal/imageagent -count=1`

Expected: PASS; an attempted unknown tool returns `agent_tool_denied`, and persisted decisions contain no message scratchpad or chain-of-thought field.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/imageagent/planner.go internal/imageagent/agent
git commit -m "feat: add bounded eino image planner"
```

### Task 2: Normalize tenant-authorized style references

**Files:**
- Create: `internal/imageagent/style.go`
- Create: `internal/imageagent/style_repository.go`
- Create: `internal/imageagent/store/style_records.go`
- Create: `internal/imageagent/store/style_repository_test.go`
- Create: `internal/imageagent/tools/style_catalog.go`
- Create: `internal/imageagent/tools/style_catalog_test.go`
- Modify: `internal/app/schema/productlisting/runtime.go`

**Interfaces:**
- Produces: `StyleReference`, `StyleSource`, `StyleRepository`, and read-only `ListAuthorizedStyles` Agent tool.
- Consumes: existing ListingKit upload/asset references; it does not create a new file store.

- [ ] **Step 1: Write failing source-normalization and tenant tests**

```go
func TestStyleRepositoryReturnsBuiltInUploadedAndSavedStyles(t *testing.T) {
    repo := newStyleRepository(t)
    seedBuiltIn(t, repo, "modern-home")
    seedTenantStyle(t, repo, "tenant-a", StyleSourceUpload, "upload-1")
    seedTenantStyle(t, repo, "tenant-a", StyleSourceSavedResult, "saved-1")
    got, err := repo.List(context.Background(), "tenant-a")
    require.NoError(t, err)
    require.ElementsMatch(t, []StyleSource{StyleSourceBuiltIn, StyleSourceUpload, StyleSourceSavedResult}, sources(got))
}

func TestStyleRepositoryRejectsCrossTenantReference(t *testing.T) {
    _, err := repo.Resolve(context.Background(), "tenant-b", "tenant-a-style")
    require.ErrorIs(t, err, imageagent.ErrStyleNotFound)
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/store ./internal/imageagent/tools -run 'TestStyleRepository' -count=1`

Expected: FAIL because style contracts are absent.

- [ ] **Step 3: Implement the unified style contract**

```go
type StyleReference struct {
    ID string
    TenantID string
    Source StyleSource
    Name string
    AssetIDs []string
    Brief StyleBrief
    Authorized bool
}

type StyleSource string
const (
    StyleSourceBuiltIn StyleSource = "built_in"
    StyleSourceUpload StyleSource = "upload"
    StyleSourceSavedResult StyleSource = "saved_result"
)
type StyleBrief struct { Family, Tone, Background, Composition, PropsLevel string }
```

Built-in styles have empty tenant ownership and explicit catalog enablement. Uploaded and saved-result styles require current tenant ownership and asset access. A provider input-reference limit selects and records an invocation subset; it never truncates stored style references or output slots.

- [ ] **Step 4: Migrate and verify the read-only Agent tool**

Run:

```bash
go test ./internal/imageagent/store ./internal/imageagent/tools ./internal/app/schema/productlisting -count=1
```

Expected: PASS; tool output excludes inaccessible styles and returns stable IDs rather than raw arbitrary URLs.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/style.go internal/imageagent/style_repository.go internal/imageagent/store internal/imageagent/tools/style_catalog.go internal/imageagent/tools/style_catalog_test.go internal/app/schema/productlisting/runtime.go
git commit -m "feat: add image agent style references"
```

### Task 3: Add assisted planning and exact-revision approval to Temporal

**Files:**
- Modify: `internal/imageagent/temporal/types.go`
- Modify: `internal/imageagent/temporal/workflow.go`
- Modify: `internal/imageagent/temporal/activities.go`
- Create: `internal/imageagent/temporal/assisted_workflow_test.go`
- Modify: `internal/imageagent/service.go`

**Interfaces:**
- Consumes: `Planner` and `PlanningResult` from Task 1.
- Produces: `ApprovePlanSignal{RunID, Revision, ActorID, ActionID}` and assisted workflow states.

- [ ] **Step 1: Write failing approval-gate tests**

```go
func TestAssistedWorkflowCreatesNoSlotSideEffectsBeforeApproval(t *testing.T) {
    env := newAssistedWorkflowEnv(t, validSevenSlotPlanningResult())
    env.RegisterDelayedCallback(func() {
        require.Equal(t, 0, slotExecutionCalls(env))
        env.SignalWorkflow(signalApprovePlan, ApprovePlanSignal{RunID: "run-1", Revision: 1, ActorID: "user-a", ActionID: "approve-1"})
    }, time.Second)
    env.ExecuteWorkflow(ImageAgentWorkflow, assistedInput("run-1"))
    require.Equal(t, 7, slotExecutionCalls(env))
}

func TestAssistedWorkflowRejectsStaleApproval(t *testing.T) {
    // Active plan is revision 2; revision 1 approval must leave the workflow waiting.
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run 'TestAssistedWorkflow' -count=1`

Expected: FAIL because assisted planning and approval Signals are absent.

- [ ] **Step 3: Implement planner activity and approval state**

The parent enters `planning`, invokes one planner activity, persists an immutable plan revision and decision, enters `awaiting_plan_approval`, and waits for a Signal matching run ID and active revision. Duplicate `ActionID` values are ignored. Rejecting a plan enters manual-editable `blocked`; replacing the plan creates a new revision and invalidates prior approval.

- [ ] **Step 4: Verify replay and identity behavior**

Run: `go test ./internal/imageagent/temporal ./internal/imageagent -count=1`

Expected: PASS; workflow replay does not call the planner twice, stale approval starts no slot, and the approver is persisted without trusting payload tenant identity.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/temporal internal/imageagent/service.go
git commit -m "feat: add assisted image plan approval"
```

### Task 4: Expose assisted commands and style-aware workbench controls

**Files:**
- Modify: `internal/imageagent/httpapi/handler.go`
- Modify: `internal/imageagent/httpapi/routes.go`
- Modify: `internal/imageagent/httpapi/handler_test.go`
- Modify: `web/listingkit-ui/src/lib/types/image-agent.ts`
- Modify: `web/listingkit-ui/src/lib/api/image-agent.ts`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/style-reference-picker.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/style-reference-picker.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.test.tsx`

**Interfaces:**
- Adds: style list, assisted create, plan approve/reject, and mode-switch commands.
- Default UI mode: `assisted`.

- [ ] **Step 1: Write failing backend and frontend tests**

```tsx
it("defaults to Agent assisted and separates source from style", () => {
  render(<ImageAgentWorkbench taskId="task-1" initialRun={planningRun()} />);
  expect(screen.getByRole("radio", { name: "Agent 辅助" })).toBeChecked();
  expect(screen.getByRole("heading", { name: "商品源图" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "参考风格" })).toBeInTheDocument();
});
```

Backend tests assert approval ignores request tenant/user fields and returns 409 for stale revision.

- [ ] **Step 2: Verify RED**

Run backend: `go test ./internal/imageagent/httpapi -run 'Assisted|ApprovePlan|Style' -count=1`

Run frontend: `npm.cmd test -- --run src/components/listingkit/image-agent`

Working directory for frontend: `web/listingkit-ui`

- [ ] **Step 3: Implement commands and plan-preview UI**

Add:

```text
GET  /api/v1/image-agent/styles
POST /api/v1/image-agent/runs/:run_id/plan/approve
POST /api/v1/image-agent/runs/:run_id/plan/reject
POST /api/v1/image-agent/runs/:run_id/mode
```

Show global style plus per-slot override, Agent decision summary/evidence/confidence, planned slot counts, and an explicit approval button. Do not display or request hidden reasoning.

- [ ] **Step 4: Verify type, API, and UI behavior**

```bash
go test ./internal/imageagent/... ./internal/app/httpapi -count=1
cd web/listingkit-ui && npm.cmd test -- --run src/components/listingkit/image-agent && npm.cmd run typecheck
```

Expected: PASS; manual mode remains usable, assisted is default, and plan approval is version-safe.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/httpapi web/listingkit-ui/src/lib/types/image-agent.ts web/listingkit-ui/src/lib/api/image-agent.ts web/listingkit-ui/src/components/listingkit/image-agent
git commit -m "feat: add assisted image planning workbench"
```

### Task 5: Prove the nine-source assisted acceptance path

**Files:**
- Create: `internal/imageagent/assisted_acceptance_test.go`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/assisted-flow.test.tsx`
- Create: `docs/development/image-agent-assisted-acceptance.md`

**Interfaces:**
- Consumes: all previous tasks.
- Produces: a deterministic fixture and operator runbook for the first assisted canary.

- [ ] **Step 1: Write the end-to-end fixture**

```go
func TestAssistedNineSourceProductProducesSevenApprovedSlotExecutions(t *testing.T) {
    env := newAssistedAcceptanceEnv(t, sourceAssets(9), plannerPlan(1, 4, 2))
    result := env.RunAndApprovePlan(1)
    require.Len(t, result.CompletedSlots, 7)
    require.Equal(t, 7, env.SlotExecutorCalls())
    require.Equal(t, 4, env.SceneRendererCalls())
}
```

- [ ] **Step 2: Verify RED before completing fixtures**

Run: `go test ./internal/imageagent -run TestAssistedNineSourceProductProducesSevenApprovedSlotExecutions -count=1`

Expected: FAIL until the full assisted composition is wired.

- [ ] **Step 3: Complete composition and acceptance documentation**

Document feature flag, tenant allowlist, migration job, Temporal worker requirement, provider governance, expected seven slots, approval action, and rollback that disables only the new route. Do not include credentials or production mutations.

- [ ] **Step 4: Run final assisted verification**

```bash
go test ./internal/imageagent/... ./internal/app/schema/productlisting ./internal/app/httpapi -count=1
cd web/listingkit-ui && npm.cmd test -- --run src/components/listingkit/image-agent && npm.cmd run typecheck
git diff --check
```

Expected: PASS with no skipped target tests and no whitespace errors.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/assisted_acceptance_test.go web/listingkit-ui/src/components/listingkit/image-agent/assisted-flow.test.tsx docs/development/image-agent-assisted-acceptance.md
git commit -m "test: verify assisted image agent workflow"
```

## Plan Self-Review

- Tasks 1-5 cover Eino isolation, three style sources, exact-revision approval, assisted-default UI, and the nine-source/seven-slot acceptance path.
- `Planner.PlanImages`, `PlanningInput`, `PlanningResult`, `StyleReference`, and `ApprovePlanSignal` are defined before use and remain provider/framework-neutral outside the adapter.
- No Agent tool performs generation, persistence, publication, or arbitrary network/database access.
- Automatic approval, model-based candidate evaluation, duplicate detection, and semantic repair remain in the third plan.
