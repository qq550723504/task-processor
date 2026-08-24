# ListingKit Child Retry Error Reporting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make SDS child-task retries durable and make the actual SHEIN login or SDS failure visible to the user instead of returning a 180-second timeout with stale task text.

**Architecture:** Reuse `listingkit_sds_child_retry_jobs` and the existing recovery sweep. The HTTP retry endpoint will validate the task and enqueue one idempotent `sds_design_sync` job, then return `202 Accepted`; the worker will keep using the existing synchronous domain retry internally and persist the terminal task result. The UI will render queued state and mutation errors, while the task panel will show the latest blocking error even when review reasons exist.

**Tech Stack:** Go, Gin, GORM, PostgreSQL, React, TypeScript, TanStack Query, Vitest, Go tests.

## Global Constraints

- Do not change production database data or deployment configuration as part of this code change.
- Reuse `SDSChildRetryJobRepository`, `RunDueSDSChildRetries`, and the existing `sds_design_sync` domain retry; do not add a second queue or worker.
- Preserve tenant scoping and the existing retry idempotency key `(listingkit_task_id, kind)`.
- Keep the synchronous domain method for the worker and SDS repair service; only the public child-retry HTTP boundary becomes asynchronous.
- Preserve the existing unrelated working-tree changes and do not commit files without explicit authorization.

---

### Task 1: Define the accepted retry response and enqueue service

**Files:**
- Modify: `internal/listingkit/sds_child_retry_job.go`
- Modify: `internal/listingkit/interfaces_services.go`
- Modify: `internal/listingkit/service_sds_child_retry.go`
- Modify: `internal/listingkit/api/handler.go`
- Test: `internal/listingkit/service_sds_child_retry_test.go`

**Interfaces:**
- Consumes: `Repository.GetTask`, `SDSChildRetryJobRepository.ScheduleSDSChildRetry`, and the current child-task state.
- Produces: `ScheduleTaskChildRetry(ctx, taskID, req) (*TaskChildRetryAccepted, error)` with task ID, kind, and `queued` status.

- [x] **Step 1: Write the failing service test**

  Add a test named `TestScheduleTaskChildRetryQueuesSDSDesignSyncWithoutRunningRemoteWork`. Build an in-memory task in `needs_review` with a failed `sds_design_sync` child and a repository implementing the existing retry-job interface. Call the new scheduling method and assert the response has `Status == "queued"`, the job has `NextRetryAt` at or before the injected current time, and exactly one job was created.

- [x] **Step 2: Run the focused test and verify it fails**

  Run: `go test ./internal/listingkit -run TestScheduleTaskChildRetryQueuesSDSDesignSyncWithoutRunningRemoteWork -count=1`

  Expected: FAIL because the scheduling response and service method do not exist.

- [x] **Step 3: Implement the minimal scheduling boundary**

  Add `TaskChildRetryAccepted` with JSON fields `task_id`, `kind`, and `status`. Add a service interface method that loads the task, validates the same parent and child states as `RetryTaskChildTask`, requires `sds_design_sync`, and calls `ScheduleSDSChildRetry` with `NextRetryAt: time.Now().UTC()`, reason code `manual_child_task_retry`, and a non-sensitive initial message. Let the repository's unique conflict handling return the existing active job so repeated clicks remain idempotent.

- [x] **Step 4: Run the focused test and verify it passes**

  Run: `go test ./internal/listingkit -run TestScheduleTaskChildRetryQueuesSDSDesignSyncWithoutRunningRemoteWork -count=1`

  Expected: PASS.

### Task 2: Return HTTP 202 and preserve domain retry behavior

**Files:**
- Modify: `internal/listingkit/api/generation_tasks_handler.go`
- Modify: `internal/listingkit/api/handler.go`
- Test: `internal/listingkit/api/child_task_retry_handler_test.go`
- Test: `internal/listingkit/service_sds_child_retry_test.go`

**Interfaces:**
- Consumes: `TaskChildRetryAccepted` from Task 1.
- Produces: `POST /api/v1/listing-kits/tasks/:task_id/child-tasks/retry` returning `202` and a JSON queued acknowledgement; `RunDueSDSChildRetries` remains the only caller of the long-running domain retry from this path.

- [x] **Step 1: Write failing handler and worker regression tests**

  Add a handler test that posts `{"kind":"sds_design_sync"}` and asserts HTTP `202`, `status: "queued"`, and the task ID. Add a worker test that runs one due job and asserts the existing `RetryTaskChildTask` path is invoked and the job becomes `completed` for a completed child result. Keep the existing synchronous service tests unchanged to protect SDS repair behavior.

- [x] **Step 2: Run the focused tests and verify the new handler test fails**

  Run: `go test ./internal/listingkit/api ./internal/listingkit -run 'TestScheduleTaskChildRetry|TestRunDueSDSChildRetries' -count=1`

  Expected: FAIL because the handler still calls the long-running synchronous method and returns `200` task data.

- [x] **Step 3: Implement the HTTP and worker wiring**

  Extend the handler dependency interface with the scheduling method, call it from `RetryTaskChildTask`, map validation and retry-conflict errors to the existing `400`/`409` responses, and return `http.StatusAccepted` for a queued job. Keep `RetryTaskChildTask` callable by `runSDSChildRetry` and `RepairAndRetryTaskSDS`; do not move remote execution into the HTTP handler.

- [x] **Step 4: Run focused backend tests**

  Run: `go test ./internal/listingkit/api ./internal/listingkit -run 'TestScheduleTaskChildRetry|TestRunDueSDSChildRetries|TestRetryTaskChildTask' -count=1`

  Expected: PASS with zero failures.

### Task 3: Preserve terminal SHEIN errors from the worker

**Files:**
- Inspect: `internal/listingkit/service_sds_child_retry.go`
- Inspect: `internal/listingkit/service_child_task_retry_helpers.go`
- Test: `internal/listingkit/service_child_task_retry_test.go`

**Interfaces:**
- Consumes: remote SDS and SHEIN adaptation failures already captured by `ListingKitResult` and `Task.Error`.
- Produces: persisted `needs_review` task data whose `error`, failed child error, or blocking workflow issue contains the latest failure, including the SHEIN cookie or verification-code timeout, while retaining historical workflow stages.

- [x] **Step 1: Review the existing persistence test coverage**

  Review the existing retry and process-status tests. They already assert that workflow issues, child-task errors, and review reasons are persisted from the current result. The production defect was that the synchronous request timed out before this persistence path completed; no persistence rewrite is required after making the HTTP boundary asynchronous.

- [x] **Step 2: Run the existing persistence tests**

  Run: `go test ./internal/listingkit -run 'TestRetryTaskChildTask|TestGetTaskResultPrefersWorkflowIssuesForReviewReasons' -count=1`

  Expected: PASS, confirming the current domain result retains the latest structured failure.

- [x] **Step 3: Keep domain persistence unchanged**

  Keep the existing domain persistence order: the retry worker persists the current `ListingKitResult` and its workflow history. The UI renders the current task error and retry error instead of hiding them behind review reasons.

- [x] **Step 4: Run backend regression tests**

  Run: `go test ./internal/listingkit -run 'TestRetryPersistsLatestPlatformAuthenticationError|TestRetryTaskChildTask' -count=1`

  Expected: PASS.

### Task 4: Show queued and failed retry states in the UI

**Files:**
- Modify: `web/listingkit-ui/src/lib/api/child-task-retry.ts`
- Modify: `web/listingkit-ui/src/lib/query/use-child-task-retry.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-screen.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.tsx`
- Test: `web/listingkit-ui/src/lib/query/use-child-task-retry.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/tasks/task-status-panel.test.tsx`

**Interfaces:**
- Consumes: `TaskChildRetryAccepted` and `ApiError` payloads from the backend.
- Produces: immediate “已加入重试队列” state, visible request failures, and a task panel that shows both review reasons and the newest blocking error.

- [x] **Step 1: Write failing UI tests**

  Add a mutation test asserting a successful retry resolves to `{status: "queued"}` and invalidates task queries. Add a panel test with `needs_review`, a review reason, and `error: "SHEIN 验证码等待超时"`; assert both strings render. Add an error test with an `ApiError` payload message and assert the mutation error is rendered as user-facing text.

- [x] **Step 2: Run focused UI tests and verify they fail**

  Run: `pnpm --dir web/listingkit-ui exec vitest run src/lib/query/use-child-task-retry.test.tsx src/components/listingkit/tasks/task-status-panel.test.tsx`

  Expected: FAIL because the hook has no queued-result contract or visible error prop, and the panel hides the primary error whenever review reasons exist.

- [x] **Step 3: Implement minimal UI behavior**

  Update the response type/parser to accept the queued acknowledgement. Pass the mutation error and queued acknowledgement through the status/workspace screens. Use the existing `ApiError` payload message extraction pattern. Render a warning block for a queued retry and a destructive block for a retry request error. Change the status panel to render review reasons and the primary error independently, while retaining the existing error precedence.

- [x] **Step 4: Run focused UI tests**

  Run: `pnpm --dir web/listingkit-ui exec vitest run src/lib/query/use-child-task-retry.test.tsx src/components/listingkit/tasks/task-status-panel.test.tsx`

  Expected: PASS with zero failures.

### Task 5: Verify integration boundaries and diff hygiene

**Files:**
- Inspect: all files changed in Tasks 1–4
- Test: existing related Go and UI suites

- [x] **Step 1: Run backend package tests**

  Run: `go test ./internal/listingkit/... -count=1`

  Expected: PASS with zero failures.

- [x] **Step 2: Run frontend related tests and type checks**

  Run: `pnpm --dir web/listingkit-ui exec vitest run src/lib/query/use-child-task-retry.test.tsx src/components/listingkit/tasks/task-status-panel.test.tsx src/components/listingkit/tasks/task-status-screen.test.tsx` and `pnpm --dir web/listingkit-ui exec tsc --noEmit`.

  Expected: PASS with zero failures and no TypeScript errors.

- [x] **Step 3: Check formatting and diff scope**

  Run: `gofmt -l internal/listingkit internal/listingkit/api` and `git diff --check`; inspect `git status --short`.

  Expected: no Go formatting output, no whitespace errors, and only the retry/error-reporting implementation, tests, and this plan are changed; the pre-existing observability documents remain untouched.
