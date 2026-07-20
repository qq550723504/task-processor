# ListingKit SDS OSS Upload Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Recover transient SDS OSS upload failures without re-generating Studio designs or re-running unrelated ListingKit workflow stages.

**Architecture:** Retain the SDS HTTP client's short retries and introduce a typed transient-upload classifier. Persist delayed retries for only `sds_design_sync` and execute them through the existing child-task retry service. Schedule these retry jobs from the Studio batch's linked ListingKit tasks; never call Studio `RetryItems` after task links exist. Gate signed OSS uploads by SDS store across replicas with the existing Redis lock infrastructure.

**Tech Stack:** Go 1.26, GORM/PostgreSQL, Gin, Redis distributed lock, golang.org/x/sync/semaphore, Vitest.

## Global Constraints

- Retry only timeout, temporary-network, HTTP 429, and HTTP 5xx upload failures.
- Never automatically retry authentication, signature/validation, or HTTP 4xx except 429 failures.
- Retry only child task kind `sds_design_sync`; never enqueue complete ListingKit generation.
- Delays are 1 minute, 5 minutes, then 15 minutes; the third failed retry is terminal.
- Default active uploads are limited to two per SDS store across replicas.
- Do not log signed form fields, cookies, access tokens, or image bytes.

---

### Task 1: Add typed SDS upload failure classification

**Files:**

- Create: `internal/sds/client/transient_upload_error.go`
- Create: `internal/sds/client/transient_upload_error_test.go`
- Modify: `internal/sds/client/errors.go`
- Test: `internal/sds/client/transient_upload_error_test.go`

**Interfaces:**

- Produces: `func RetryableUploadFailure(error) (reasonCode string, ok bool)`
- Produces: `RetryableUploadReasonTimeout`, `RetryableUploadReasonTransientNetwork`, `RetryableUploadReasonRateLimited`, and `RetryableUploadReasonServerError`
- Consumed by: remote SDS workflow and delayed retry runner

- [ ] **Step 1: Write failing tests**

    func TestRetryableUploadFailureTimeout(t *testing.T) {
        err := &Error{Message: "multipart upload failed", Err: context.DeadlineExceeded}
        got, ok := RetryableUploadFailure(err)
        require.True(t, ok)
        require.Equal(t, RetryableUploadReasonTimeout, got)
    }

    func TestRetryableUploadFailureRejectsSignatureError(t *testing.T) {
        _, ok := RetryableUploadFailure(&Error{StatusCode: http.StatusForbidden, Message: "SignatureDoesNotMatch"})
        require.False(t, ok)
    }

- [ ] **Step 2: Verify the tests initially fail**

  Run: `go test ./internal/sds/client -run RetryableUploadFailure -count=1`

  Expected: failure because the classifier and constants do not exist.

- [ ] **Step 3: Implement the classifier**

  Use `errors.As` to unwrap `*client.Error`, `errors.Is` for `context.DeadlineExceeded`, and the underlying `net.Error` timeout/temporary contract. Classify only 429 and 5xx status codes. Return false for 400/401/403 and SDS business errors.

- [ ] **Step 4: Add table-driven coverage**

  Cover wrapped deadline errors, a timeout `net.Error`, a temporary network error, HTTP 429, HTTP 503, HTTP 400, HTTP 403, and an auth error. Assert the exact reason code for every retryable case.

- [ ] **Step 5: Run the package verification**

  Run: `go test ./internal/sds/client -count=1 -timeout 2m`

  Expected: PASS.

- [ ] **Step 6: Commit**

  Run:

    git add -- internal/sds/client/errors.go internal/sds/client/transient_upload_error.go internal/sds/client/transient_upload_error_test.go
    git commit -m "fix(sds): classify transient OSS upload failures"

### Task 2: Persist delayed retries for only SDS design sync

**Files:**

- Create: `internal/listingkit/sds_child_retry_job.go`
- Create: `internal/listingkit/sds_child_retry_repository.go`
- Create: `internal/listingkit/sds_child_retry_service.go`
- Create: `internal/listingkit/sds_child_retry_service_test.go`
- Modify: `internal/listingkit/interfaces_dependencies.go`
- Modify: `internal/listingkit/workflow_sds_sync_remote_support.go`
- Modify: the existing ListingKit GORM migration/bootstrap registration that migrates `listingkit_studio_batch_task_links`

**Interfaces:**

- Produces: `SDSChildRetryJob` with task id, batch id, store id, kind, status, attempt, next retry time, reason code, error, lease expiry, and timestamps.
- Produces: `ScheduleSDSChildRetry(ctx context.Context, task *Task, batchID string, reasonCode string, cause error) error`
- Produces: `RunDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int) (int64, error)`
- Consumes: `RetryTaskChildTask(ctx, taskID, &RetryChildTaskRequest{Kind: "sds_design_sync"})`

- [ ] **Step 1: Write failing job-service tests**

  Test these cases with a controllable clock:

  - first transient failure creates attempt 1 due at now plus 1 minute;
  - failures reschedule at 5 and 15 minutes;
  - a third retry failure marks the record exhausted and leaves the parent in `needs_review`;
  - scheduling the same active task and kind twice creates one job;
  - a successful child retry completes the job and does not rerun unrelated child tasks.

- [ ] **Step 2: Verify the tests initially fail**

  Run: `go test ./internal/listingkit -run SDSChildRetry -count=1`

  Expected: compilation failure because the retry-job service does not exist.

- [ ] **Step 3: Implement the GORM model and repository**

  Create table `listingkit_sds_child_retry_jobs`. Enforce one active record per `listingkit_task_id` and kind. Add an atomic due-job claim that writes a lease owner and expiry in the same transaction so two pods cannot run the same job.

- [ ] **Step 4: Implement scheduling and execution**

    var sdsRetryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

    func (s *SDSChildRetryService) RunDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int) (int64, error) {
        jobs, err := s.repo.ClaimDue(ctx, dueBefore, limit, s.leaseTTL)
        // Call RetryTaskChildTask for each claimed job.
        // Complete it on success; reschedule only a newly classified transient upload error.
    }

  Permanent errors and exhausted jobs must be marked terminal without changing their parent task to a new generated task.

- [ ] **Step 5: Schedule only the relevant remote-SDS failure**

  In `workflow_sds_sync_remote_support.go`, check `client.RetryableUploadFailure(err)` at the boundary that currently converts the failure to a warning. Keep the diagnostic in the result, but create the durable child retry job for classified upload errors. Do not schedule failures from catalog lookup, template selection, or auth.

- [ ] **Step 6: Register the runner**

  Register `RunDueSDSChildRetries` on the existing ListingKit recovery/scheduler cadence with a bounded page size. A no-job run must be a no-op.

- [ ] **Step 7: Verify focused ListingKit behavior**

  Run: `go test ./internal/listingkit -run 'SDSChildRetry|RetryTaskChildTask|WorkflowSDS' -count=1 -timeout 5m`

  Expected: PASS.

- [ ] **Step 8: Commit**

  Run:

    git add -- internal/listingkit
    git commit -m "fix(listingkit): retry transient SDS upload failures"

### Task 3: Limit signed OSS uploads by SDS store

**Files:**

- Create: `internal/sds/design/upload_gate.go`
- Create: `internal/sds/design/upload_gate_test.go`
- Modify: `internal/sds/design/service.go`
- Modify: the existing SDS service bootstrap/config builder
- Modify: the existing Redis lock wiring that provides `internal/infra/lock.DistributedLock`

**Interfaces:**

- Produces: `type UploadGate interface { Acquire(ctx context.Context, storeID int64) (release func(context.Context) error, err error) }`
- Produces: `NewStoreScopedUploadGate(lock lock.DistributedLock, capacity int, ttl time.Duration) UploadGate`
- Consumed by: `Service.UploadToOSS`

- [ ] **Step 1: Write failing gate tests**

  Test that two same-store uploads acquire slots, a third waits until its context expires, different stores do not block each other, and a claimed Redis lease is released after upload completion.

- [ ] **Step 2: Verify the tests initially fail**

  Run: `go test ./internal/sds/design -run UploadGate -count=1`

  Expected: compilation failure because the gate does not exist.

- [ ] **Step 3: Implement with project primitives**

  Use `semaphore.Weighted` only for local wait coordination. Acquire one of the Redis lease keys `listingkit:sds:upload:<storeID>:<slot>` through `DistributedLock`; try slots from zero through capacity minus one; release the acquired key with a defer. Return context cancellation if no slot is available.

- [ ] **Step 4: Guard only the multipart post**

  Pass store identity into the SDS upload request/context, acquire immediately before `UploadFile(ctx, signature.Host, ...)`, and release after it returns. Do not hold a slot for signature retrieval, SDS material registration, polling, or cleanup.

- [ ] **Step 5: Add configuration and safe logs**

  Add a default capacity of two and follow existing config validation conventions. Log store id, OSS host, gate wait duration, upload duration, and reason code. Never log signature or multipart form values.

- [ ] **Step 6: Verify**

  Run: `go test ./internal/sds/design -count=1 -timeout 2m`

  Expected: PASS.

- [ ] **Step 7: Commit**

  Run:

    git add -- internal/sds/design internal/infra/lock internal/core/config
    git commit -m "fix(sds): bound concurrent OSS uploads per store"

### Task 4: Schedule retries from a Studio batch

**Files:**

- Modify: `internal/listingkit/task_studio_batch_task_flow_support.go`
- Modify: `internal/listingkit/task_studio_batch_service.go`
- Modify: `internal/listingkit/api/studio_batches_handler.go`
- Modify: `internal/listingkit/api/studio_batches_handler_test.go`
- Modify: `internal/listingkit/httpapi/routes_task.go`
- Modify: the existing ListingKit Studio batch recovery UI component and its test in `web/listingkit-ui`

**Interfaces:**

- Produces: `POST /api/v1/listing-kits/studio/batches/:batch_id/sds-retries`
- Response: `{scheduled_count, skipped_count, exhausted_count}`
- Consumes: batch task links plus `ScheduleSDSChildRetry`

- [ ] **Step 1: Write failing backend tests**

  Create a batch fixture with linked tasks whose `sds_design_sync` child state is failed, processing, completed, and absent. Assert that only failed child tasks are scheduled, no call reaches `resetStudioBatchRetryItems`, and repeated requests do not create duplicate active retry jobs.

- [ ] **Step 2: Verify the tests initially fail**

  Run: `go test ./internal/listingkit/api -run StudioBatchSDSRetries -count=1`

  Expected: missing route/handler failure.

- [ ] **Step 3: Implement the action**

  Load the batch detail graph and task links, inspect each linked ListingKit task result, schedule eligible `sds_design_sync` retries, and return exact scheduled/skipped/exhausted counts. This action must not create new Studio items or ListingKit tasks.

- [ ] **Step 4: Add the focused UI action**

  Show a recovery button only if the linked tasks include retryable SDS failures. Call the new endpoint, display its counts, and refresh the batch state. Keep the existing failed-style retry control unchanged because it is a pre-task image-generation action.

- [ ] **Step 5: Verify backend and frontend**

  Run: `go test ./internal/listingkit ./internal/listingkit/api -run 'StudioBatchSDSRetries|SDSChildRetry' -count=1 -timeout 5m`

  Run from `web/listingkit-ui`: `npm run typecheck` and the targeted Studio recovery test.

  Expected: PASS.

- [ ] **Step 6: Commit**

  Run:

    git add -- internal/listingkit web/listingkit-ui
    git commit -m "feat(listingkit): recover SDS failures from Studio batches"

### Task 5: Final verification and controlled operational recovery

**Files:**

- Modify: only tests or documentation required by Tasks 1-4.

- [ ] **Step 1: Run final static and Go checks**

  Run: `gopls check` for modified Go files, then `go test ./internal/sds/client ./internal/sds/design ./internal/listingkit ./internal/listingkit/api -count=1 -timeout 10m`.

  Expected: PASS.

- [ ] **Step 2: Validate with a local delayed multipart response**

  Use a local HTTP server that delays response headers. Verify the exact timeout creates one durable retry job with reason `sds_oss_upload_timeout`; advance the test clock and assert that only `sds_design_sync` is invoked.

- [ ] **Step 3: Deploy and recover the affected batch**

  Only after the code is deployed, call the new batch recovery endpoint for `6d01173d-8366-48d4-a44c-24c192effab9`. Confirm the response schedules only eligible linked tasks, then monitor job completion before scheduling another batch.

- [ ] **Step 4: Commit final test/documentation changes**

  Run:

    git add -- <verified files>
    git commit -m "test(listingkit): cover SDS upload recovery"

