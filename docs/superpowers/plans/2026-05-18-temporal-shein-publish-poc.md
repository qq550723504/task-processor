# Temporal SHEIN Publish PoC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal Temporal-based workflow PoC for ListingKit SHEIN `publish`, proving durable execution, phase visibility, and idempotent replay without rewriting existing SHEIN business rules.

**Architecture:** Keep the current HTTP API, task repository, payload preparation, image upload, SHEIN remote API calls, and submission event/result model. Introduce a Temporal orchestration layer that owns phase progression and recovery for `publish` only. The existing synchronous `SubmitTask` path remains the fallback for `save_draft` and all non-PoC paths.

**Tech Stack:** Go, Temporal Go SDK, Gin, existing ListingKit repository/store layer, existing SHEIN submission helpers.

---

## File Structure

### New files

- `internal/listingkit/temporal/doc.go`
  - Package-level contract for Temporal integration in ListingKit.
- `internal/listingkit/temporal/types.go`
  - Shared workflow input/query/result types for the PoC.
- `internal/listingkit/temporal/task_queue.go`
  - Workflow ID, task queue, and signal/query name constants.
- `internal/listingkit/temporal/activities.go`
  - Activity implementations that wrap existing ListingKit service/repo logic.
- `internal/listingkit/temporal/workflow_publish.go`
  - Temporal workflow definition for SHEIN `publish`.
- `internal/listingkit/temporal/worker.go`
  - Worker registration and startup helpers.
- `internal/listingkit/temporal/client.go`
  - Thin client wrapper for starting workflows, querying state, and signaling retry.
- `internal/listingkit/temporal/workflow_publish_test.go`
  - Workflow unit tests via Temporal testsuite.
- `internal/listingkit/temporal/client_test.go`
  - Client/ID composition tests with mocks or stubs.
- `internal/app/runtime/temporal_runtime.go`
  - Runtime bootstrap for Temporal client + worker lifecycle.
- `docs/architecture/temporal-poc-runbook.md`
  - Local runbook for starting Temporal and validating the PoC.

### Modified files

- `go.mod`
  - Add Temporal Go SDK dependency.
- `go.sum`
  - Dependency lock update.
- `internal/listingkit/service_submit.go`
  - Route SHEIN `publish` PoC traffic into Temporal entrypoint instead of inline orchestration.
- `internal/listingkit/service.go`
  - Extend service dependencies/config to include Temporal client and PoC feature flag.
- `internal/listingkit/interfaces.go` or equivalent service contracts file
  - Add the orchestration dependency interface if needed.
- `internal/listingkit/api/submit_handler.go`
  - Preserve request shape; no behavior change except improved status mapping when workflow already exists or is in progress.
- `internal/listingkit/service_submit_lifecycle_test.go`
  - Add or adapt tests that now assert Temporal-backed phase/replay behavior for `publish`.
- `README.md`
  - Add minimal operator/developer note for Temporal PoC process startup if runtime entry is exposed from main services.

### Existing code to reuse directly

- `internal/listingkit/service_submit.go`
  - Existing business order for SHEIN submit phases.
- `internal/listingkit/submission/state.go`
  - Existing submission report and phase helpers.
- `internal/publishing/shein/submission.go`
  - Existing phase/status/result model.
- `internal/listingkit/service_submit_recovery.go`
  - Existing remote confirmation/recovery behavior to preserve where possible.
- `internal/listingkit/api/submit_handler.go`
  - Existing idempotency key extraction contract.

## Scope Rules

- PoC only covers `platform=shein` and `action=publish`.
- `save_draft` stays on the existing synchronous implementation.
- No production cutover in this plan.
- No migration of crawler, scheduler, or generic task flows.
- No database schema migration in the PoC.
- Do not redesign SHEIN payload rules, pricing, image semantics, or readiness rules.

## Design Decisions To Lock Before Coding

1. **Workflow identity**
   - Workflow ID: `shein-submit:<taskID>:publish`
   - One running workflow per task/action pair.
2. **Input identity**
   - Request idempotency key remains part of workflow input.
   - Same workflow can reject or replay duplicate request IDs using existing submission report state.
3. **State source of truth**
   - Task result persistence remains the durable UI-visible source of submission state.
   - Temporal history is orchestration durability, not a replacement for ListingKit result persistence.
4. **Activity boundaries**
   - All DB writes, HTTP/API calls, image upload, and remote confirmation stay in activities.
   - Workflow only decides sequence, retry, waiting, and phase transitions.
5. **Adoption path**
   - Service-level feature flag or nil-check chooses Temporal PoC path for SHEIN `publish`.

## Task 1: Add Temporal dependency and package skeleton

**Files:**
- Create: `internal/listingkit/temporal/doc.go`
- Create: `internal/listingkit/temporal/types.go`
- Create: `internal/listingkit/temporal/task_queue.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Test: `go test ./internal/listingkit/temporal`

- [ ] **Step 1: Write the package skeleton**

Create `internal/listingkit/temporal/types.go` with explicit PoC types:

```go
package temporal

import "time"

type PublishWorkflowInput struct {
	TaskID          string    `json:"task_id"`
	Platform        string    `json:"platform"`
	Action          string    `json:"action"`
	RequestID       string    `json:"request_id"`
	RequestedAt     time.Time `json:"requested_at"`
	TriggeredByUser string    `json:"triggered_by_user,omitempty"`
}

type SubmissionStateQueryResult struct {
	TaskID          string     `json:"task_id"`
	Action          string     `json:"action"`
	RequestID       string     `json:"request_id,omitempty"`
	CurrentPhase    string     `json:"current_phase,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	RemoteStatus    string     `json:"remote_status,omitempty"`
	WorkflowRunning bool       `json:"workflow_running"`
}
```

- [ ] **Step 2: Add constants**

Create `internal/listingkit/temporal/task_queue.go`:

```go
package temporal

const (
	TaskQueueSheinSubmitPublish = "listingkit-shein-submit-publish"
	QueryCurrentState           = "current_state"
	SignalRetry                 = "retry"
)

func WorkflowIDForPublish(taskID string) string {
	return "shein-submit:" + taskID + ":publish"
}
```

- [ ] **Step 3: Add package docs**

Create `internal/listingkit/temporal/doc.go`:

```go
// Package temporal contains ListingKit-specific Temporal orchestration glue.
// The package intentionally reuses existing ListingKit business logic and treats
// Temporal as the durable orchestration layer, not as a replacement for domain rules.
package temporal
```

- [ ] **Step 4: Add dependency**

Run:

```powershell
go get go.temporal.io/sdk@latest
```

Expected:

- `go.mod` gains `go.temporal.io/sdk`
- `go.sum` updates

- [ ] **Step 5: Verify compile**

Run:

```powershell
go test ./internal/listingkit/temporal
```

Expected: package compiles, no test files yet or empty pass.

## Task 2: Define activity seam around existing submit logic

**Files:**
- Create: `internal/listingkit/temporal/activities.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/interfaces.go` or equivalent
- Test: `internal/listingkit/temporal/workflow_publish_test.go`

- [ ] **Step 1: Define activity host dependencies**

In `internal/listingkit/temporal/activities.go`, define a narrow dependency carrier:

```go
package temporal

import (
	"context"
	"time"

	"task-processor/internal/listingkit"
)

type SubmitActivities struct {
	Service *listingkit.ServiceDependenciesAdapter
}

type LoadTaskInput struct {
	TaskID string
}

type PersistPhaseInput struct {
	TaskID    string
	Action    string
	RequestID string
	Phase     string
	Now       time.Time
}
```

If `listingkit.ServiceDependenciesAdapter` does not exist, add a similarly narrow adapter in `internal/listingkit/service.go` that exposes only what activities need.

- [ ] **Step 2: Split business actions into activity methods**

Define activity methods that wrap existing service helpers instead of reimplementing them:

```go
func (a *SubmitActivities) LoadTask(ctx context.Context, in LoadTaskInput) (*listingkit.Task, error)
func (a *SubmitActivities) BeginPublishAttempt(ctx context.Context, in PublishWorkflowInput) error
func (a *SubmitActivities) ValidateReadiness(ctx context.Context, in PublishWorkflowInput) error
func (a *SubmitActivities) PrepareProduct(ctx context.Context, in PublishWorkflowInput) (*listingkit.PreparedSubmitPayload, error)
func (a *SubmitActivities) UploadImages(ctx context.Context, in listingkit.UploadImagesInput) (*listingkit.PreparedSubmitPayload, error)
func (a *SubmitActivities) PreValidate(ctx context.Context, in listingkit.PreparedSubmitPayload) error
func (a *SubmitActivities) SubmitRemote(ctx context.Context, in listingkit.PreparedSubmitPayload) (*listingkit.RemoteSubmitResult, error)
func (a *SubmitActivities) PersistSuccess(ctx context.Context, in listingkit.PersistSubmitSuccessInput) error
func (a *SubmitActivities) PersistFailure(ctx context.Context, in listingkit.PersistSubmitFailureInput) error
func (a *SubmitActivities) ConfirmRemote(ctx context.Context, in listingkit.ConfirmRemoteInput) (*listingkit.RemoteConfirmResult, error)
func (a *SubmitActivities) BuildPreview(ctx context.Context, taskID string) (*listingkit.ListingKitPreview, error)
```

The exact helper input/output structs can live in `internal/listingkit` if needed, but keep them small and specific to orchestration handoff.

- [ ] **Step 3: Add the first failing workflow test**

Create a workflow test that asserts phase order through mocked activities:

```go
func TestPublishWorkflowRunsExpectedPhaseOrder(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()

	var phases []string
	env.OnActivity((*SubmitActivities).BeginPublishAttempt, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) { phases = append(phases, "validate") })
	env.OnActivity((*SubmitActivities).ValidateReadiness, mock.Anything, mock.Anything).
		Return(nil)
	env.OnActivity((*SubmitActivities).PrepareProduct, mock.Anything, mock.Anything).
		Return(&listingkit.PreparedSubmitPayload{}, nil).
		Run(func(args mock.Arguments) { phases = append(phases, "prepare_product") })
	// ...

	env.ExecuteWorkflow(PublishWorkflow, PublishWorkflowInput{
		TaskID: "task-1", Platform: "shein", Action: "publish", RequestID: "req-1",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []string{"validate", "prepare_product", "upload_images", "pre_validate", "submit_remote", "persist_result", "confirm_remote"}, phases)
}
```

- [ ] **Step 4: Implement only enough activity signatures to compile**

Stub activities with `panic("not implemented")` or `return nil, errors.New("not implemented")` just long enough to compile the test harness.

- [ ] **Step 5: Run targeted test**

Run:

```powershell
go test ./internal/listingkit/temporal -run TestPublishWorkflowRunsExpectedPhaseOrder -v
```

Expected: FAIL due to unimplemented workflow or activity registration.

## Task 3: Implement the Temporal workflow definition

**Files:**
- Create: `internal/listingkit/temporal/workflow_publish.go`
- Modify: `internal/listingkit/temporal/workflow_publish_test.go`
- Test: `go test ./internal/listingkit/temporal -run PublishWorkflow -v`

- [ ] **Step 1: Define the workflow**

Create `internal/listingkit/temporal/workflow_publish.go`:

```go
package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func PublishWorkflow(ctx workflow.Context, in PublishWorkflowInput) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	state := SubmissionStateQueryResult{
		TaskID:          in.TaskID,
		Action:          in.Action,
		RequestID:       in.RequestID,
		WorkflowRunning: true,
	}
	_ = workflow.SetQueryHandler(ctx, QueryCurrentState, func() (SubmissionStateQueryResult, error) {
		return state, nil
	})

	// execute activities in phase order, updating state.CurrentPhase before each call
	return nil
}
```

- [ ] **Step 2: Flesh out phase order**

Add explicit phase transitions before each activity execution:

```go
state.CurrentPhase = "validate"
if err := workflow.ExecuteActivity(ctx, (*SubmitActivities).BeginPublishAttempt, in).Get(ctx, nil); err != nil { return err }
if err := workflow.ExecuteActivity(ctx, (*SubmitActivities).ValidateReadiness, in).Get(ctx, nil); err != nil { return err }

state.CurrentPhase = "prepare_product"
var prepared listingkit.PreparedSubmitPayload
if err := workflow.ExecuteActivity(ctx, (*SubmitActivities).PrepareProduct, in).Get(ctx, &prepared); err != nil { return err }
```

Continue the same structure through `upload_images`, `pre_validate`, `submit_remote`, `persist_result`, and `confirm_remote`.

- [ ] **Step 3: Handle failures through persistence activity**

For each terminal activity error, invoke `PersistFailure` before returning:

```go
if err := workflow.ExecuteActivity(ctx, (*SubmitActivities).PreValidate, prepared).Get(ctx, nil); err != nil {
	_ = workflow.ExecuteActivity(ctx, (*SubmitActivities).PersistFailure, listingkit.PersistSubmitFailureInput{
		TaskID: in.TaskID, Action: in.Action, RequestID: in.RequestID, Phase: state.CurrentPhase, ErrorMessage: err.Error(),
	}).Get(ctx, nil)
	return err
}
```

- [ ] **Step 4: Add workflow tests for failure path and query state**

Add:

- success path test
- pre-validate failure test
- query state test while a delayed activity is running

Run:

```powershell
go test ./internal/listingkit/temporal -run PublishWorkflow -v
```

Expected: PASS.

## Task 4: Implement real activity logic by reusing existing ListingKit helpers

**Files:**
- Modify: `internal/listingkit/temporal/activities.go`
- Modify: `internal/listingkit/service_submit.go`
- Test: `go test ./internal/listingkit/temporal ./internal/listingkit -run Submit`

- [ ] **Step 1: Extract reusable helper methods from `service_submit.go` if needed**

If current logic is too entangled inside `SubmitTask`, extract helper methods into `internal/listingkit/service_submit.go` that can be called from both inline submit and Temporal activities:

```go
func (s *service) beginSheinPublishAttempt(ctx context.Context, taskID string, req *SubmitTaskRequest, startedAt time.Time) (*Task, string, error)
func (s *service) validateSheinPublishReadiness(ctx context.Context, task *Task, action string) error
func (s *service) prepareSheinPublishPayload(ctx context.Context, task *Task, pkg *SheinPackage, requestID string) (*PreparedSubmitPayload, error)
func (s *service) persistSheinPublishFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string, err error) error
```

Do not duplicate business logic into the Temporal package.

- [ ] **Step 2: Implement activities against those helpers**

In `internal/listingkit/temporal/activities.go`, call the extracted service methods and repository methods directly. Preserve existing phase writes through `internal/listingkit/submission/state.go`.

- [ ] **Step 3: Add a focused integration-style test**

Create or extend a test that runs the workflow with stubbed repo/API builders and asserts:

- remote publish called once
- phase transitions persisted
- preview rebuilt from saved result

Example assertion skeleton:

```go
func TestPublishWorkflowPersistsSubmissionStateAndReturnsPreview(t *testing.T) {
	// stub repo + stub shein API + activity host
	// execute workflow in testsuite
	// assert saved submission report current_phase cleared at end
	// assert publish record request_id matches input
	// assert remote publish call count == 1
}
```

- [ ] **Step 4: Run targeted tests**

Run:

```powershell
go test ./internal/listingkit/temporal ./internal/listingkit -run "PublishWorkflow|SubmitTask" -v
```

Expected: PASS.

## Task 5: Add Temporal client wrapper and runtime bootstrap

**Files:**
- Create: `internal/listingkit/temporal/client.go`
- Create: `internal/listingkit/temporal/worker.go`
- Create: `internal/app/runtime/temporal_runtime.go`
- Modify: `internal/listingkit/service.go`
- Test: `internal/listingkit/temporal/client_test.go`

- [ ] **Step 1: Add a thin client interface**

In `internal/listingkit/temporal/client.go`:

```go
package temporal

import (
	"context"

	sdkclient "go.temporal.io/sdk/client"
)

type Client interface {
	StartPublish(ctx context.Context, in PublishWorkflowInput) error
	QueryPublishState(ctx context.Context, taskID string) (*SubmissionStateQueryResult, error)
}

type clientImpl struct {
	sdk sdkclient.Client
}
```

- [ ] **Step 2: Implement `StartPublish` with stable workflow ID**

```go
func (c *clientImpl) StartPublish(ctx context.Context, in PublishWorkflowInput) error {
	_, err := c.sdk.ExecuteWorkflow(ctx, sdkclient.StartWorkflowOptions{
		ID:        WorkflowIDForPublish(in.TaskID),
		TaskQueue: TaskQueueSheinSubmitPublish,
	}, PublishWorkflow, in)
	return err
}
```

Handle already-started workflow by mapping it to a project-level in-progress error instead of leaking raw SDK text.

- [ ] **Step 3: Implement query helper**

```go
func (c *clientImpl) QueryPublishState(ctx context.Context, taskID string) (*SubmissionStateQueryResult, error) {
	resp, err := c.sdk.QueryWorkflow(ctx, WorkflowIDForPublish(taskID), "", QueryCurrentState)
	if err != nil {
		return nil, err
	}
	var state SubmissionStateQueryResult
	if err := resp.Get(&state); err != nil {
		return nil, err
	}
	return &state, nil
}
```

- [ ] **Step 4: Implement worker bootstrap**

In `internal/listingkit/temporal/worker.go`:

```go
func RegisterWorker(c sdkclient.Client, activities *SubmitActivities) worker.Worker {
	w := worker.New(c, TaskQueueSheinSubmitPublish, worker.Options{})
	w.RegisterWorkflow(PublishWorkflow)
	w.RegisterActivity(activities)
	return w
}
```

- [ ] **Step 5: Add runtime lifecycle helper**

In `internal/app/runtime/temporal_runtime.go`, create start/stop helpers that:

- dial Temporal client from config/env
- register worker
- start worker during runtime boot
- stop worker on shutdown

No need to wire every binary in the PoC if one obvious entrypoint exists; keep bootstrap scope narrow.

- [ ] **Step 6: Add client tests**

Test:

- workflow ID generation
- query name usage
- already-started error mapping

Run:

```powershell
go test ./internal/listingkit/temporal -run "Client|WorkflowID" -v
```

Expected: PASS.

## Task 6: Route SHEIN `publish` through Temporal client

**Files:**
- Modify: `internal/listingkit/service_submit.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/api/submit_handler.go`
- Test: `internal/listingkit/service_submit_lifecycle_test.go`

- [ ] **Step 1: Add service dependency for Temporal PoC**

In `internal/listingkit/service.go`, add a nullable dependency:

```go
type ServiceConfig struct {
	// ...
	TemporalSubmitClient temporal.Client
	TemporalPublishEnabled bool
}
```

- [ ] **Step 2: Short-circuit `publish` into Temporal start**

At the top of `SubmitTask` in `internal/listingkit/service_submit.go`, before inline orchestration:

```go
if platform == "shein" && action == "publish" && s.temporalPublishEnabled && s.temporalSubmitClient != nil {
	requestID := normalizedSubmitIdempotencyKey(req)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if err := s.temporalSubmitClient.StartPublish(ctx, temporal.PublishWorkflowInput{
		TaskID: taskID,
		Platform: "shein",
		Action: "publish",
		RequestID: requestID,
		RequestedAt: time.Now(),
	}); err != nil {
		return nil, mapTemporalSubmitStartError(err)
	}
	return s.GetTaskPreview(ctx, taskID, "shein")
}
```

Keep the existing inline path untouched for all other cases.

- [ ] **Step 3: Preserve API contract**

`internal/listingkit/api/submit_handler.go` should continue returning:

- `200` when workflow start succeeds
- `409` when the workflow is already running
- `400` for submit blocked / unsupported

Only change code if raw Temporal errors are leaking through.

- [ ] **Step 4: Add service tests**

Add tests for:

- publish routes to Temporal client when enabled
- save_draft still uses inline path
- repeated publish gets mapped to `ErrSubmitInProgress`

Run:

```powershell
go test ./internal/listingkit ./internal/listingkit/api -run "SubmitTask|SubmitHandler" -v
```

Expected: PASS.

## Task 7: Add local runbook and PoC verification flow

**Files:**
- Create: `docs/architecture/temporal-poc-runbook.md`
- Modify: `README.md` if runtime startup instructions are exposed there

- [ ] **Step 1: Write local Temporal run instructions**

Document:

- how to run local Temporal server or Temporal CLI dev server
- which service binary starts the worker
- required env/config keys
- how to submit a SHEIN `publish` PoC request

- [ ] **Step 2: Write manual verification cases**

Include:

1. Start workflow for `publish`
2. Query phase during image upload or delayed confirm
3. Kill worker during in-flight workflow
4. Restart worker
5. Confirm workflow resumes and does not duplicate remote publish

- [ ] **Step 3: Add exact verification commands**

Examples:

```powershell
go test ./internal/listingkit/temporal ./internal/listingkit ./internal/listingkit/api
go test ./...
```

and sample API invocation:

```bash
curl -X POST http://localhost:3000/api/v1/listing-kits/tasks/<task_id>/submit \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: temporal-poc-123" \
  -d '{"platform":"shein","action":"publish"}'
```

## Task 8: Final verification and cutover decision note

**Files:**
- Modify: `docs/architecture/temporal-workflow-evaluation.md`
- Test: full targeted and repo test pass

- [ ] **Step 1: Add post-PoC findings section**

After implementation, update `docs/architecture/temporal-workflow-evaluation.md` with:

- what was proven
- what still remains inline
- what blocked broader rollout

- [ ] **Step 2: Run targeted verification**

Run:

```powershell
go test ./internal/listingkit/temporal ./internal/listingkit ./internal/listingkit/api -v
```

Expected: PASS.

- [ ] **Step 3: Run broad safety verification**

Run:

```powershell
go test ./...
```

Expected: PASS, or document any unrelated pre-existing failures explicitly before closing the work.

## Execution Notes

- Reuse the existing SHEIN submit helpers wherever possible; do not fork business logic into the Temporal package.
- Keep workflow code deterministic: no direct `time.Now()`, random IDs, network calls, or repo access inside workflow definitions.
- Map Temporal-visible phase state back into the existing `SubmissionReport` model so UI consumers do not need a parallel state source.
- The first PoC is successful if it proves durability and replay safety for `publish`; it does not need to be production-ready infrastructure.

## Self-Review

### Spec coverage

- Temporal chosen as long-flow orchestration base: covered.
- Preserve current submit business rules: covered by activity reuse tasks.
- Limit scope to SHEIN `publish`: covered.
- Add PoC rollout/verification path: covered.
- Avoid replacing all existing flows at once: covered.

### Placeholder scan

- No `TODO`/`TBD` placeholders remain.
- Each task has explicit files, commands, and expected outcomes.

### Type consistency

- Uses `PublishWorkflowInput`, `SubmissionStateQueryResult`, `TaskQueueSheinSubmitPublish`, `QueryCurrentState`, and `SignalRetry` consistently.
- The plan assumes small adapter types may be added in `internal/listingkit`; they should be named consistently during implementation.

Plan complete and saved to `docs/superpowers/plans/2026-05-18-temporal-shein-publish-poc.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
