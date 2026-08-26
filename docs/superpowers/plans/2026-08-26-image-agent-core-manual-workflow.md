# Image Agent Core Manual Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a production-shaped manual image workflow with versioned slot plans, durable Temporal execution, per-slot recovery, authenticated APIs, SSE status, and a usable workbench.

**Architecture:** `internal/imageagent` owns the plan and run model. Temporal owns the durable parent/slot workflows; repository-owned ports call existing ProductImage capabilities. The UI edits one target per slot and consumes a complete run projection, so one renderer call can never stand for the entire gallery.

**Tech Stack:** Go 1.26, GORM 1.31.1, Temporal Go SDK 1.43.0, Gin, React 19, Next.js 16, TanStack Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-08-26-image-agent-workflow-design.md`

## Global Constraints

- This plan enables manual mode only; assisted and automatic modes remain rejected by policy until their plans land.
- Do not import Temporal, provider SDK, ListingKit platform, HTTP, or GORM types into `internal/imageagent` domain files.
- Do not impose a global ten-image limit; limits belong to provider invocation or later platform adaptation.
- Do not silently use source images or local canvas output as generated success.
- Every run, plan revision, slot, attempt, and publication operation has a stable idempotency key.
- Tenant and user identity come only from verified request context and are restored in Temporal activities.
- Existing `internal/localagent` and the dirty `codex/1688-image-source-fallback` worktree are out of scope.

---

### Task 1: Define the image-plan domain and state invariants

**Files:**
- Create: `internal/imageagent/model.go`
- Create: `internal/imageagent/validation.go`
- Create: `internal/imageagent/actions.go`
- Create: `internal/imageagent/model_test.go`

**Interfaces:**
- Produces: `Run`, `Plan`, `Slot`, `RunMode`, `RunStatus`, `SlotRole`, `SlotStatus`, `ValidatePlan`, and `AllowedActions`.
- Consumes: primitive IDs and asset references only.

- [ ] **Step 1: Write failing tests for independent slots and plan revisions**

```go
func TestValidatePlanAllowsMoreThanTenIndependentSlots(t *testing.T) {
    slots := make([]Slot, 11)
    for i := range slots {
        slots[i] = Slot{ID: fmt.Sprintf("slot-%d", i), Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}}
    }
    err := ValidatePlan(Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: slots})
    require.NoError(t, err)
}

func TestValidatePlanRejectsDuplicateSlotIDs(t *testing.T) {
    plan := Plan{Revision: 1, SourceAssetIDs: []string{"source-1"}, Slots: []Slot{
        {ID: "scene-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}},
        {ID: "scene-1", Role: SlotRoleScene, SourceAssetIDs: []string{"source-1"}},
    }}
    require.ErrorContains(t, ValidatePlan(plan), "duplicate slot id")
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent -run 'TestValidatePlan(AllowsMoreThanTenIndependentSlots|RejectsDuplicateSlotIDs)' -count=1`

Expected: FAIL because the package and types do not exist.

- [ ] **Step 3: Implement minimal domain types and validation**

```go
type RunMode string
const (
    RunModeManual RunMode = "manual"
    RunModeAssisted RunMode = "assisted"
    RunModeAutomatic RunMode = "automatic"
)

type RunStatus string
const (
    RunStatusPlanning RunStatus = "planning"
    RunStatusAwaitingPlanApproval RunStatus = "awaiting_plan_approval"
    RunStatusExecuting RunStatus = "executing"
    RunStatusEvaluating RunStatus = "evaluating"
    RunStatusRepairing RunStatus = "repairing"
    RunStatusAwaitingFinalApproval RunStatus = "awaiting_final_approval"
    RunStatusBlocked RunStatus = "blocked"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed RunStatus = "failed"
    RunStatusCancelled RunStatus = "cancelled"
)

type SlotStatus string
const (
    SlotStatusPending SlotStatus = "pending"
    SlotStatusExecuting SlotStatus = "executing"
    SlotStatusEvaluating SlotStatus = "evaluating"
    SlotStatusAccepted SlotStatus = "accepted"
    SlotStatusRejected SlotStatus = "rejected"
    SlotStatusBlocked SlotStatus = "blocked"
)

type SlotRole string
const (
    SlotRoleMain SlotRole = "main"
    SlotRoleScene SlotRole = "scene"
    SlotRoleDetail SlotRole = "detail"
    SlotRoleSellingPoint SlotRole = "selling_point"
    SlotRoleSize SlotRole = "size"
)

type Slot struct {
    ID string
    Role SlotRole
    SourceAssetIDs []string
    StyleReferenceIDs []string
    Brief string
    IdempotencyKey string
    Status SlotStatus
}

type Plan struct {
    Revision int64
    ParentRevision int64
    SourceAssetIDs []string
    StyleReferenceIDs []string
    Slots []Slot
    CreatedBy string
}

type Run struct {
    ID string
    BusinessTaskID string
    TenantID string
    UserID string
    Mode RunMode
    Status RunStatus
    CurrentNode string
    ActivePlanRevision int64
    Version int64
    Budget Budget
    Usage BudgetUsage
    Block *Block
}

type Block struct { Code, Message, SlotID string }
type Action string
const (
    ActionEditPlan Action = "edit_plan"
    ActionRetrySlot Action = "retry_slot"
    ActionApproveResults Action = "approve_results"
    ActionCancel Action = "cancel"
    ActionSwitchManual Action = "switch_manual"
)
type AssetRef struct { ID, URL, Kind string }
type ProductContextRef struct { ProductID, Title, ProductType string; Attributes map[string]string }
type AssetCandidate struct { AssetID, URL, SourceAssetID string; Metadata map[string]string }
type Budget struct { MaxImages, MaxAgentSteps, MaxModelCalls, MaxRepairAttemptsPerSlot int; MaxCostMicros int64; MaxElapsed time.Duration }
type BudgetUsage struct { Images, AgentSteps, ModelCalls int; EstimatedCostMicros int64; Elapsed time.Duration }
```

`ValidatePlan` trims IDs, requires revision `> 0`, at least one source and slot, unique slot IDs and idempotency keys, known roles, and source references contained in the plan. It deliberately has no total-slot maximum.

- [ ] **Step 4: Add transition and action-table tests; verify GREEN**

```go
func TestAllowedActionsForBlockedRunAreExplicit(t *testing.T) {
    run := Run{Mode: RunModeManual, Status: RunStatusBlocked, Block: &Block{Code: "slot_failed", SlotID: "scene-2"}}
    require.Equal(t, []Action{ActionEditPlan, ActionRetrySlot, ActionCancel}, AllowedActions(run))
}
```

Run: `go test ./internal/imageagent -count=1`

Expected: PASS for valid transitions and rejection of stale, terminal, or ambiguous commands.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/model.go internal/imageagent/validation.go internal/imageagent/actions.go internal/imageagent/model_test.go
git commit -m "feat: define image agent plan domain"
```

### Task 2: Persist runs, immutable plans, slots, and attempts

**Files:**
- Create: `internal/imageagent/repository.go`
- Create: `internal/imageagent/store/records.go`
- Create: `internal/imageagent/store/memory.go`
- Create: `internal/imageagent/store/gorm.go`
- Create: `internal/imageagent/store/events.go`
- Create: `internal/imageagent/store/repository_contract_test.go`
- Modify: `internal/app/schema/productlisting/runtime.go`
- Modify: `internal/app/schema/productlisting/runtime_test.go`

**Interfaces:**
- Consumes: Task 1 domain types.
- Produces: `Repository`, `store.NewMemoryRepository`, `store.NewGormRepository`, and `store.AutoMigrate`.

- [ ] **Step 1: Write the repository contract tests**

```go
type RunScope struct { TenantID, RunID string }
type RunMutation struct { Status RunStatus; CurrentNode string; ActivePlanRevision int64; Block *Block }
type SlotResult struct { SlotID string; Attempt int; Status SlotStatus; CandidateAssetIDs []string; ErrorCode string }
type StepAttempt struct { TenantID, RunID, SlotID, Node, IdempotencyKey string; Attempt int; Outcome, ErrorCategory string }
type RunEvent struct { TenantID, RunID, Type string; Cursor int64; ProjectionVersion int64; Payload json.RawMessage }

type Repository interface {
    CreateRun(context.Context, *Run) error
    GetRun(context.Context, RunScope) (*Run, error)
    UpdateRun(context.Context, RunScope, int64, RunMutation) error
    AppendPlan(context.Context, RunScope, int64, Plan) error
    SaveSlotResult(context.Context, RunScope, int64, SlotResult) error
    AppendAttempt(context.Context, StepAttempt) error
    AppendEvent(context.Context, RunEvent) error
    ListEvents(context.Context, RunScope, int64, int) ([]RunEvent, error)
}

func testStalePlanRevisionIsRejected(t *testing.T, repo Repository) {
    ctx := context.Background()
    require.NoError(t, repo.CreateRun(ctx, manualRun("run-1", "tenant-a")))
    scope := RunScope{TenantID: "tenant-a", RunID: "run-1"}
    require.NoError(t, repo.AppendPlan(ctx, scope, 0, planRevision(1)))
    err := repo.AppendPlan(ctx, scope, 0, planRevision(2))
    require.ErrorIs(t, err, ErrRevisionConflict)
}
```

Run the same contract against memory and SQLite-backed GORM repositories.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/store -count=1`

Expected: FAIL because repository implementations are absent.

- [ ] **Step 3: Implement normalized persistence and optimistic concurrency**

Use separate records for `image_agent_runs`, `image_agent_plans`, `image_agent_slots`, `image_agent_attempts`, and `image_agent_events`. Store plan/slot structured payloads as JSON only where fields are not queried; keep tenant, state, revision, role, cursor, timestamps, and idempotency keys as indexed columns. `AppendPlan` uses one transaction and an `active_plan_revision = expected` predicate. `UpdateRun` writes the run mutation and projection event atomically.

```go
func AutoMigrate(db *gorm.DB) error {
    if db == nil { return errors.New("database is nil") }
    return db.AutoMigrate(&runRecord{}, &planRecord{}, &slotRecord{}, &attemptRecord{}, &eventRecord{})
}
```

- [ ] **Step 4: Wire the product-listing migration and verify contracts**

Add `imageagentstore.AutoMigrate(db)` to `productlisting.AutoMigrateRuntime` and assert all five tables exist.

Run:

```bash
go test ./internal/imageagent/store ./internal/app/schema/productlisting -count=1
go test ./internal/app/runtime/productlistingschemamigrate -count=1
```

Expected: PASS, including cross-tenant reads returning `ErrRunNotFound`, idempotent duplicate attempt writes, and stale revision conflicts.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/repository.go internal/imageagent/store internal/app/schema/productlisting
git commit -m "feat: persist image agent runs and plans"
```

### Task 3: Add the manual Temporal parent and slot workflows

**Files:**
- Create: `internal/imageagent/ports.go`
- Create: `internal/imageagent/service.go`
- Create: `internal/imageagent/temporal/types.go`
- Create: `internal/imageagent/temporal/workflow.go`
- Create: `internal/imageagent/temporal/slot_workflow.go`
- Create: `internal/imageagent/temporal/activities.go`
- Create: `internal/imageagent/temporal/worker.go`
- Create: `internal/imageagent/temporal/workflow_test.go`
- Create: `internal/app/runtime/image_agent_temporal_runtime.go`
- Create: `internal/app/runtime/image_agent_temporal_runtime_test.go`
- Create: `cmd/image-agent-temporal-worker/main.go`
- Create: `cmd/image-agent-temporal-worker/main_test.go`

**Interfaces:**
- Consumes: `Repository`, `ValidatePlan`, verified execution identity, and ProductImage-facing `SlotExecutor`/`ApprovedAssetPublisher` ports.
- Produces: `ImageAgentWorkflow`, `ImageSlotWorkflow`, `WorkflowClient`, and a registered Temporal worker.

- [ ] **Step 1: Write failing workflow tests for seven independent slots and partial failure**

```go
func TestManualWorkflowExecutesEverySlotIndependently(t *testing.T) {
    env := newWorkflowEnv(t)
    env.OnActivity(activityExecuteSlot, mock.Anything, mock.Anything).Return(successfulSlotResult(), nil).Times(7)
    env.ExecuteWorkflow(ImageAgentWorkflow, WorkflowInput{RunID: "run-1", Plan: sevenSlotPlan()})
    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())
}

func TestManualWorkflowBlocksOnlyFailedSlot(t *testing.T) {
    env := newWorkflowEnvWithOneSlotFailure(t, "scene-2")
    env.ExecuteWorkflow(ImageAgentWorkflow, WorkflowInput{RunID: "run-1", Plan: sevenSlotPlan()})
    var result WorkflowResult
    require.NoError(t, env.GetWorkflowResult(&result))
    require.Equal(t, imageagent.RunStatusBlocked, result.Status)
    require.Equal(t, "scene-2", result.Block.SlotID)
    require.Len(t, result.CompletedSlotIDs, 6)
}

func TestManualWorkflowWaitsForFinalApprovalBeforePublishing(t *testing.T) {
    env := newWorkflowEnv(t)
    env.RegisterDelayedCallback(func() {
        require.Equal(t, 0, publishedAssetCalls(env))
        env.SignalWorkflow(signalApproveResults, ApproveResultsSignal{RunID: "run-1", PlanRevision: 1, ActionID: "approve-final-1"})
    }, time.Second)
    env.ExecuteWorkflow(ImageAgentWorkflow, WorkflowInput{RunID: "run-1", Plan: sevenSlotPlan()})
    require.Equal(t, 1, publishedAssetCalls(env))
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/temporal -run 'TestManualWorkflow(ExecutesEverySlotIndependently|BlocksOnlyFailedSlot)' -count=1`

Expected: FAIL because workflows and activities are absent.

- [ ] **Step 3: Implement deterministic orchestration**

```go
type SlotExecutor interface {
    ExecuteSlot(context.Context, SlotExecutionInput) (SlotExecutionResult, error)
}
type ApprovedAssetPublisher interface {
    PublishApproved(context.Context, PublishApprovedInput) error
}

type SlotExecutionInput struct {
    RunID, TenantID, UserID string
    PlanRevision int64
    Slot Slot
    Attempt int
    IdempotencyKey string
}
type SlotExecutionResult struct { SlotID string; Attempt int; Candidates []AssetCandidate }
type PublishApprovedInput struct { RunID, TenantID string; PlanRevision int64; CandidateAssetIDs []string; IdempotencyKey string }
type ApproveResultsSignal struct { RunID string; PlanRevision int64; ActorID, ActionID string }
```

The parent validates the plan, starts one child workflow per slot with bounded concurrency, gathers typed results, persists every terminal slot result, and blocks on failed slots. When all slots succeed it enters `awaiting_final_approval`; only an `ApproveResultsSignal` for the active plan revision invokes `ApprovedAssetPublisher`. Technical activity retries use Temporal `RetryPolicy`; semantic retry occurs only after a `retry_slot` Signal with the expected plan revision.

- [ ] **Step 4: Test replay, duplicate Signals, cancellation, and idempotency**

Run:

```bash
go test ./internal/imageagent/temporal -count=1
go test ./internal/app/runtime -run Temporal -count=1
go test ./cmd/image-agent-temporal-worker -count=1
```

Expected: PASS; replay does not repeat completed slot side effects, cancellation starts no new child, and duplicate retry Signals create one new attempt.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/ports.go internal/imageagent/service.go internal/imageagent/temporal internal/app/runtime/image_agent_temporal_runtime.go internal/app/runtime/image_agent_temporal_runtime_test.go cmd/image-agent-temporal-worker
git commit -m "feat: add manual image agent temporal workflow"
```

### Task 4: Adapt ProductImage capabilities to one-slot execution

**Files:**
- Create: `internal/imageagent/tools/productimage_executor.go`
- Create: `internal/imageagent/tools/productimage_executor_test.go`
- Modify: `internal/productimage/domain/scene_options.go`
- Modify: `internal/productimage/scene_request_context.go`

**Interfaces:**
- Consumes: `productimage.SubjectExtractor`, `WhiteBackgroundRenderer`, `SceneRenderer`, and `ProductContext`.
- Produces: `tools.NewProductImageSlotExecutor` implementing `imageagent.SlotExecutor`.

- [ ] **Step 1: Write failing role-routing tests**

```go
func TestExecutorCallsSceneRendererOncePerSceneSlot(t *testing.T) {
    renderer := &recordingSceneRenderer{result: []productimage.ImageAsset{{URL: "asset://scene-1"}}}
    executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: renderer, SubjectExtractor: stubSubjectExtractor()})
    for _, id := range []string{"scene-1", "scene-2", "scene-3", "scene-4"} {
        _, err := executor.ExecuteSlot(context.Background(), sceneSlotInput(id))
        require.NoError(t, err)
    }
    require.Equal(t, 4, renderer.calls)
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/tools -count=1`

Expected: FAIL because the adapter is absent.

- [ ] **Step 3: Implement role-specific calls without gallery fallback**

Main uses subject extraction plus white-background rendering. Scene, detail, and selling-point roles call the scene renderer with slot role, brief, and style references mapped into controlled scene options. Size role rejects missing reliable dimensions instead of fabricating them. The adapter requires at least one provider candidate for the slot; multiple provider outputs are persisted as candidates but do not create undeclared slots.

- [ ] **Step 4: Verify no source-success fallback and multi-slot behavior**

Run:

```bash
go test ./internal/imageagent/tools ./internal/productimage -run 'Slot|Scene|Fallback' -count=1
go test ./internal/imageagent/... -count=1
```

Expected: PASS; provider failure remains failure, and four scene slots produce four independent renderer calls.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/tools internal/productimage/domain/scene_options.go internal/productimage/scene_request_context.go
git commit -m "feat: execute product images by slot"
```

### Task 5: Expose authenticated commands, queries, and SSE

**Files:**
- Create: `internal/imageagent/httpapi/handler.go`
- Create: `internal/imageagent/httpapi/routes.go`
- Create: `internal/imageagent/httpapi/module.go`
- Create: `internal/imageagent/httpapi/handler_test.go`
- Modify: `internal/authz/listingkit.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_route_authorization.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_modules.go`

**Interfaces:**
- Routes: create run, get run, replace plan, retry slot, cancel, and SSE events under `/api/v1/image-agent/runs`.
- Permissions: `listingkit.image_agent.read` and `listingkit.image_agent.write`.

- [ ] **Step 1: Write failing identity, revision, and SSE tests**

```go
func TestReplacePlanRejectsStaleRevision(t *testing.T) {
    response := performVerifiedRequest(t, handler.ReplacePlan, http.MethodPut,
        "/api/v1/image-agent/runs/run-1/plan", `{"expected_revision":0,"plan":{"revision":2}}`, "tenant-a", "user-a")
    require.Equal(t, http.StatusConflict, response.Code)
}

func TestGetRunDoesNotTrustRequestTenant(t *testing.T) {
    response := performVerifiedRequest(t, handler.Get, http.MethodGet,
        "/api/v1/image-agent/runs/run-1?tenant_id=tenant-b", "", "tenant-a", "user-a")
    require.Equal(t, http.StatusOK, response.Code)
    require.NotContains(t, response.Body.String(), "tenant-b")
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/imageagent/httpapi ./internal/app/httpapi -run 'Test(ReplacePlanRejectsStaleRevision|GetRunDoesNotTrustRequestTenant)' -count=1`

Expected: FAIL because routes and permissions are absent.

- [ ] **Step 3: Implement the module and contracts**

```go
const ModuleName = "image-agent"
// POST /api/v1/image-agent/runs
// GET  /api/v1/image-agent/runs/:run_id
// PUT  /api/v1/image-agent/runs/:run_id/plan
// POST /api/v1/image-agent/runs/:run_id/slots/:slot_id/retry
// POST /api/v1/image-agent/runs/:run_id/results/approve
// POST /api/v1/image-agent/runs/:run_id/cancel
// GET  /api/v1/image-agent/runs/:run_id/events
```

Create accepts `mode=manual` only in this plan. Result approval requires the active plan revision and verified actor. SSE sends versioned projection events with monotonic cursor IDs and heartbeats; reconnect accepts `Last-Event-ID` and always permits a full GET snapshot. Map revision conflict to 409, validation to 400, missing identity to 401, authorization to 403, missing run to 404, and blocked command to 409.

- [ ] **Step 4: Verify mounted and authorized routes**

Run: `go test ./internal/imageagent/httpapi ./internal/authz ./internal/app/httpapi -count=1`

Expected: PASS and route catalog tests prove every image-agent route requires ZITADEL and the declared permission.

- [ ] **Step 5: Commit**

```bash
git add internal/imageagent/httpapi internal/authz/listingkit.go internal/listingkit/httpapi/zitadel_auth_route_authorization.go internal/app/httpapi
git commit -m "feat: expose image agent workflow api"
```

### Task 6: Build the manual image workbench and acceptance test

**Files:**
- Create: `web/listingkit-ui/src/lib/types/image-agent.ts`
- Create: `web/listingkit-ui/src/lib/api/image-agent.ts`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/use-image-agent-run.ts`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.tsx`
- Create: `web/listingkit-ui/src/components/listingkit/image-agent/image-agent-workbench.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen-views.tsx`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/[...path]/proxy-url.ts`
- Create: `internal/imageagent/manual_acceptance_test.go`

**Interfaces:**
- Consumes: Task 5 HTTP projection and SSE event schema.
- Produces: manual three-panel workbench embedded in the existing task workspace.

- [ ] **Step 1: Write failing UI tests for blockers and eleven slots**

```tsx
it("shows the exact blocked slot and keeps every planned slot", async () => {
  render(<ImageAgentWorkbench taskId="task-1" initialRun={blockedRunWithSlots(11, "scene-2")} />);
  expect(screen.getAllByTestId("image-slot-card")).toHaveLength(11);
  expect(screen.getByText("场景图 scene-2 生成失败")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "仅重试 scene-2" })).toBeEnabled();
});
```

- [ ] **Step 2: Verify RED**

Run: `npm.cmd test -- --run src/components/listingkit/image-agent/image-agent-workbench.test.tsx`

Working directory: `web/listingkit-ui`

Expected: FAIL because the workbench is absent.

- [ ] **Step 3: Implement three panels and resilient live state**

Render source materials separately from style references, slot cards in the center, and run/node/budget/block information on the right. `useImageAgentRun` opens one EventSource, applies only increasing cursor versions, and refetches the full snapshot after disconnect or version gap. Manual plan edits and final approval always include `expected_revision`.

- [ ] **Step 4: Run frontend and backend acceptance**

```bash
go test ./internal/imageagent/... -count=1
go test ./internal/app/schema/productlisting ./internal/app/httpapi -count=1
cd web/listingkit-ui && npm.cmd test -- --run src/components/listingkit/image-agent src/components/listingkit/workspace && npm.cmd run typecheck
```

Expected: PASS. `manual_acceptance_test.go` proves nine source assets plus a seven-slot plan invoke seven slot identities, preserve six successes when one fails, and permit eleven standard slots.

- [ ] **Step 5: Commit**

```bash
git add web/listingkit-ui/src/lib/types/image-agent.ts web/listingkit-ui/src/lib/api/image-agent.ts web/listingkit-ui/src/components/listingkit/image-agent web/listingkit-ui/src/components/listingkit/workspace/workspace-screen-views.tsx web/listingkit-ui/src/app/api/listing-kits/[...path]/proxy-url.ts internal/imageagent/manual_acceptance_test.go
git commit -m "feat: add manual image agent workbench"
```

## Plan Self-Review

- Tasks 1-6 cover the first rollout slice: domain, persistence, real one-slot execution, Temporal recovery, API/SSE, and manual UI.
- The shared types are `Run`, `Plan`, `Slot`, `SlotExecutionInput`, `SlotExecutionResult`, and the versioned run projection; names are consistent across tasks.
- Assisted planning, Eino, style catalogs, model evaluation, automatic repair, and automatic mode are intentionally delegated to the next two plans.
- The plan contains no output-count cap, no source-as-generated fallback, and no direct marketplace publication.
