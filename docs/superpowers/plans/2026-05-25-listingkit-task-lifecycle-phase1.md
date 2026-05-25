# ListingKit Task Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract ListingKit task lifecycle behavior into a focused collaborator without changing the external `listingkit.Service` contract.

**Architecture:** Keep `internal/listingkit/service.go` as the facade and dependency owner, but move task lifecycle behavior into a package-local collaborator that receives only the dependencies required for task creation, dispatch, result assembly, and task list reads. The first cut stays in the same Go package to avoid import cycles while still reducing root-service responsibility.

**Tech Stack:** Go, existing `listingkit` service tests, package-local collaborator pattern, `apply_patch`

---

### Task 1: Add a failing collaborator test

**Files:**
- Create: `internal/listingkit/task_lifecycle_service_test.go`
- Test: `internal/listingkit/task_lifecycle_service_test.go`

- [ ] **Step 1: Write the failing test**

Add a test that constructs the new task lifecycle collaborator with a stub repo and asserts `ListTasks` preserves the current query propagation and result shaping semantics.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestTaskLifecycleServiceListTasks -count=1`
Expected: FAIL because the collaborator does not exist yet.

### Task 2: Implement the focused collaborator

**Files:**
- Create: `internal/listingkit/task_lifecycle_service.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/service_task.go`
- Modify: `internal/listingkit/task_creation_support.go`
- Modify: `internal/listingkit/task_result_support.go`

- [ ] **Step 1: Add the collaborator type**

Create a package-local `taskLifecycleService` that holds only the dependencies needed by task lifecycle behavior.

- [ ] **Step 2: Move task lifecycle methods behind the collaborator**

Implement `CreateGenerateTask`, `GetTaskResult`, `ListTasks`, and the task dispatch helpers on the collaborator, then make the root `service` facade delegate to it.

- [ ] **Step 3: Wire the collaborator in `NewService`**

Store the collaborator on `service` so new task lifecycle behavior has a single focused home.

### Task 3: Run regression checks

**Files:**
- Test: `internal/listingkit/service_test.go`
- Test: `internal/listingkit/service_process_status_test.go`

- [ ] **Step 1: Run targeted tests**

Run: `go test ./internal/listingkit -run "TestTaskLifecycleServiceListTasks|TestCreateGenerateTask|TestGetTaskResult|TestListTasks" -count=1`
Expected: PASS.

- [ ] **Step 2: Run a broader package verification**

Run: `go test ./internal/listingkit -count=1`
Expected: PASS, or report exact failing tests if unrelated existing issues surface.
