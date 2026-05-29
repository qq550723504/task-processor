# Task Processor Framework Phase 5A ListingKit Process Phase Model Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit task processing flow through explicit process phases so task claiming, workflow execution, terminal-state persistence, and worker retry gating stop living as implicit logic spread across `service_process.go` and `processor.go`.

**Architecture:** Reuse the repo’s already-proven `productenrich/productimage` state-machine mindset instead of inventing a new generic framework. Keep the seam feature-owned inside ListingKit: first extract deterministic process-outcome and terminal-transition logic, then align the worker processor on a small ListingKit-specific process state helper so service and worker side stop drifting independently.

**Tech Stack:** Go, ListingKit service/repository layer, existing workflow/result model, existing process status tests, existing worker processor pattern from `internal/productenrich/pipeline` and `internal/productimage/pipeline`

**Out of Scope For This Slice:**

- redesigning `runStandardProductWorkflow(...)`
- redesigning `runPlatformAdaptation(...)`
- changing submit/runtime context behavior from `Phase 4B`
- introducing a repo-wide generic process engine
- changing retry policy semantics beyond making current behavior explicit

---

## Root Cause This Slice Addresses

After `Phase 4B`, ListingKit still has one process-layer hotspot left:

- [internal/listingkit/service_process.go](/D:/code/task-processor/internal/listingkit/service_process.go:1)
- [internal/listingkit/processor.go](/D:/code/task-processor/internal/listingkit/processor.go:1)

Today these files jointly decide:

1. when a task can be claimed
2. how workflow execution is invoked
3. how partial failure results are persisted
4. how `needs_review` vs `completed` terminal states are derived
5. when retry counters are incremented and tasks are requeued

The real problem is not file size. The problem is crossed ownership between:

- service-side process phase orchestration
- terminal-state persistence rules
- worker-side retry/skip gating

That makes future changes risky because a behavior change in “process completion” can easily leak across both files without one explicit seam to test.

The repo already has a mature local precedent for this kind of problem:

- [internal/productenrich/pipeline/state_machine.go](/D:/code/task-processor/internal/productenrich/pipeline/state_machine.go:1)
- [internal/productimage/pipeline/state_machine.go](/D:/code/task-processor/internal/productimage/pipeline/state_machine.go:1)

`Phase 5A` should reuse that idea, but keep the implementation local to ListingKit’s own process semantics.

---

## Target Outcome

At the end of `Phase 5A`:

- service-side process phases are explicit
- terminal-state derivation and persistence are grouped coherently
- worker-side retry/skip gating flows through a small ListingKit-owned state helper
- current process behavior remains unchanged
- narrow boundary tests lock the new ownership split

---

## Task 1: Isolate deterministic process-outcome and terminal-state rules

**Files:**
- Create: `internal/listingkit/service_process_outcome.go`
- Modify: `internal/listingkit/service_process.go`
- Modify: `internal/listingkit/service_process_status_test.go`

- [ ] **Step 1: Write failing tests for process terminal outcome rules**

Extend `service_process_status_test.go` so it explicitly locks:

1. `Summary.NeedsReview` maps to `TaskStatusNeedsReview`
2. successful non-review results map to `TaskStatusCompleted`
3. workflow errors still persist partial results before marking failed
4. review reasons continue to come from `reviewReasonsFromResult(...)`

Prefer extending the existing `ProcessListingKit` status tests instead of introducing a parallel suite.

- [ ] **Step 2: Run focused process-status tests**

Run:

```powershell
go test ./internal/listingkit -run "TestProcessListingKit(MarksNeedsReviewWhenSummaryRequiresReview|MarksSheinCookieUnavailableAsBlockingIssue|InitializesDefaultSheinPricing|ReusesPublishedSheinPricingCache)" -count=1
```

Expected: PASS before the refactor, establishing the behavior baseline.

- [ ] **Step 3: Extract explicit process-outcome helpers**

Create `service_process_outcome.go` and move deterministic terminalization logic out of `ProcessListingKit(...)` into focused helpers such as:

- `deriveProcessTerminalStatus(result *ListingKitResult) TaskStatus`
- `applyProcessTerminalResult(result *ListingKitResult, status TaskStatus) *ListingKitResult`
- `persistProcessFailure(ctx context.Context, taskID string, result *ListingKitResult, err error) error`
- `persistProcessSuccess(ctx context.Context, taskID string, result *ListingKitResult) error`

Important:

- do not change terminal-state precedence
- do not change review-reason derivation
- keep repository contracts unchanged

- [ ] **Step 4: Re-run process terminal verification**

Run:

```powershell
go test ./internal/listingkit -run "TestProcessListingKit(MarksNeedsReviewWhenSummaryRequiresReview|MarksSheinCookieUnavailableAsBlockingIssue)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/service_process_outcome.go internal/listingkit/service_process.go internal/listingkit/service_process_status_test.go
git commit -m "refactor: extract listingkit process outcome rules"
```

---

## Task 2: Introduce an explicit service-side process phase runner

**Files:**
- Create: `internal/listingkit/service_process_flow.go`
- Modify: `internal/listingkit/service_process.go`
- Modify: `internal/listingkit/service_process_status_test.go`
- Modify: `internal/listingkit/phase4a_collaborator_boundary_test.go`

- [ ] **Step 1: Write failing boundary tests for process-phase ownership**

Add narrow source/behavior tests that prove:

1. `service_process.go` stops being the primary home of claim/execute/finalize orchestration bodies
2. service-side process flow is routed through an explicit helper seam
3. terminal persistence helpers from Task 1 are consumed through that seam

Lock at least one concrete indicator such as:

- `ProcessListingKit(...)` delegating to `runListingKitProcessFlow(...)`
- `service_process.go` no longer containing inline `MarkNeedsReview(...)` / `MarkCompleted(...)` orchestration branches

- [ ] **Step 2: Run focused process-status and source-boundary tests**

Run:

```powershell
go test ./internal/listingkit -run "Test(ProcessListingKit.*|ServiceProcessFileUsesExplicitFlowSeam)" -count=1
```

Expected: PASS before the refactor except for the new failing boundary assertion.

- [ ] **Step 3: Add a ListingKit-owned process flow seam**

Create `service_process_flow.go` with an explicit process-phase runner, for example:

- `type listingKitProcessFlow struct { service *service }`
- `func buildListingKitProcessFlow(s *service) *listingKitProcessFlow`

This seam should own coordinated service-side phases across:

- task claim
- workflow execution
- failure persistence
- success terminalization

Important:

- reuse existing repository and workflow helpers internally first
- do not move workflow internals out of `runWorkflow(...)` in this step
- keep the seam feature-owned inside ListingKit

- [ ] **Step 4: Rewire `ProcessListingKit(...)` through the process flow seam**

Update `ProcessListingKit(...)` so the method becomes the public entry point while the actual service-side phase orchestration runs through the new flow seam.

- [ ] **Step 5: Re-run focused service process verification**

Run:

```powershell
go test ./internal/listingkit -run "TestProcessListingKit.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/service_process_flow.go internal/listingkit/service_process.go internal/listingkit/service_process_status_test.go internal/listingkit/phase4a_collaborator_boundary_test.go
git commit -m "refactor: introduce listingkit process flow seam"
```

---

## Task 3: Align worker retry/skip gating on a ListingKit-owned process state helper

**Files:**
- Create: `internal/listingkit/processor_state_machine.go`
- Create: `internal/listingkit/processor_state_machine_test.go`
- Create: `internal/listingkit/processor_process_test.go`
- Modify: `internal/listingkit/processor.go`

- [ ] **Step 1: Write failing tests for worker-side process gating**

Add focused processor tests that lock:

1. non-pending tasks are skipped
2. `ErrTaskNotPending` is treated as a safe skip
3. retryable failures increment retry count and prepare retry
4. retry scheduling still respects `maxRetries`
5. tenant/OpenAI identity injection still happens before service execution

Use small stub repos/services rather than a new end-to-end harness.

- [ ] **Step 2: Run focused processor tests**

Run:

```powershell
go test ./internal/listingkit -run "TestProcessor(ProcessTask.*|StateMachine.*)" -count=1
```

Expected: FAIL first because the new tests do not exist yet.

- [ ] **Step 3: Introduce a ListingKit-owned processor state helper**

Create `processor_state_machine.go` using the same bounded idea already proven in:

- [internal/productenrich/pipeline/state_machine.go](/D:/code/task-processor/internal/productenrich/pipeline/state_machine.go:1)
- [internal/productimage/pipeline/state_machine.go](/D:/code/task-processor/internal/productimage/pipeline/state_machine.go:1)

Keep it local to ListingKit and focused on current behavior, for example:

- `type ProcessorStateMachine struct { maxRetries int }`
- `func NewProcessorStateMachine(maxRetries int) *ProcessorStateMachine`
- `func (sm *ProcessorStateMachine) CanProcess(task *Task) error`
- `func (sm *ProcessorStateMachine) ShouldRetry(task *Task) bool`

Do not over-generalize failure classification if current ListingKit semantics do not need a richer model yet.

- [ ] **Step 4: Rewire `processor.go` to consume the explicit state helper**

Update `processor.go` so:

- status skip logic
- retry gating
- retry scheduling decision

flow through the state helper instead of living inline beside identity injection and service invocation.

- [ ] **Step 5: Re-run focused processor verification**

Run:

```powershell
go test ./internal/listingkit -run "TestProcessor(ProcessTask.*|StateMachine.*)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/processor_state_machine.go internal/listingkit/processor_state_machine_test.go internal/listingkit/processor_process_test.go internal/listingkit/processor.go
git commit -m "refactor: align listingkit processor retry gating"
```

---

## Task 4: Lock the ListingKit process phase ownership boundary

**Files:**
- Create: `internal/listingkit/phase5a_process_boundary_test.go`
- Modify: `internal/listingkit/service_process_status_test.go`
- Modify: `internal/listingkit/processor_state_machine_test.go`

- [ ] **Step 1: Add boundary guardrails**

Lock two things:

1. service-side process terminalization stays behind the explicit process flow seam
2. worker-side retry/skip decisions stay behind the ListingKit-owned processor state helper

Suggested checks:

- `service_process.go` should not regrow inline terminal persistence branches
- `processor.go` should not regrow inline pending/retry decision logic
- `service_process_flow.go` and `processor_state_machine.go` remain the ownership homes of those decisions

- [ ] **Step 2: Run full ListingKit verification**

Run:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/phase5a_process_boundary_test.go internal/listingkit/service_process_status_test.go internal/listingkit/processor_state_machine_test.go
git commit -m "test: lock listingkit process phase model boundary"
```

---

## Self-Review

### Spec coverage

This plan intentionally covers one bounded hotspot:

- service-side process phase orchestration
- terminal-state persistence
- worker-side retry/skip gating
- process boundary tests

It does not mix in workflow internals, submit/runtime context, or HTTP/bootstrap work.

### Reuse check

This slice explicitly reuses a mature local pattern already present in:

- `internal/productenrich/pipeline/state_machine.go`
- `internal/productimage/pipeline/state_machine.go`

The plan does not invent a new generic process framework.

### Root-cause check

The problem being addressed is crossed ownership between:

- process orchestration
- terminal persistence
- worker retry gating

The plan therefore focuses on:

- extracting deterministic outcome rules
- introducing one explicit service-side process flow seam
- reusing a small local state-machine idea for worker-side gating
- locking both boundaries with narrow tests

### Scope discipline

This is a bounded slice:

- no workflow redesign
- no new repo-wide abstraction
- no retry-policy change
- no return to submit/runtime context work

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-29-task-processor-framework-phase5a-listingkit-process-phase-model.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
