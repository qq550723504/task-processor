# ListingKit SDS OSS upload recovery

## Problem

When the signed SDS OSS multipart upload times out, `UploadFile` returns a transport
error. The remote SDS workflow records the `sds_design_sync` child task as failed,
then persists the parent ListingKit task as `needs_review`. The generic retryable-task
recovery path is bypassed, so a temporary upstream outage turns an entire Studio batch
into manual work.

This was observed in Studio batch `6d01173d-8366-48d4-a44c-24c192effab9`: all 26
created ListingKit tasks had an SDS upload response-header timeout. The Studio item
retry path cannot repair them because task links have already been created; the
existing task-scoped `sds_design_sync` retry is the correct recovery unit.

## Goals

- Treat transient SDS OSS upload failures as retryable child-task work.
- Recover an affected Studio batch without regenerating designs or creating new
  ListingKit tasks.
- Bound SDS upload pressure per store across application replicas.
- Preserve non-transient failures for review and keep retry state observable.

## Non-goals

- Retrying authentication, invalid signature, validation, or other permanent errors.
- Re-running the whole ListingKit generation workflow for an SDS-only failure.
- Changing the Studio image-generation retry semantics.

## Design

### 1. Typed SDS transient-failure classification

Add an SDS-client helper that unwraps `client.Error` and classifies only timeout,
temporary network, HTTP 429, and HTTP 5xx responses as transient. It must not use a
free-form message match as the domain contract. The helper exposes a stable reason
code, such as `sds_oss_upload_timeout`, for persistence and metrics.

`UploadFile` retains its existing short HTTP retry behavior. The new classification
is for retries that must survive a process restart and must be spaced farther apart.

### 2. Durable retry for only `sds_design_sync`

Persist an eligible child-task retry with the parent task id, child-task kind, store
scope, retry count, next-at time, reason code, and last error. A worker claims due
records and invokes the existing `RetryTaskChildTask(taskID, {kind:
"sds_design_sync"})` behavior. Back off at 1, 5, and 15 minutes, then leave the
task in `needs_review` if the third retry fails.

The first retryable failure remains visible in the task result, but does not require
a user to manually find and retry every task. A successful retry removes the failed
child-task state and lets the existing persistence logic return the parent task to
`completed` when no other review reason remains.

### 3. Store-scoped SDS upload gate

Wrap the actual signed OSS upload in a store-scoped gate with a default capacity of
two. Use the project's existing Redis distributed-lock infrastructure for cross-pod
coordination; use `golang.org/x/sync/semaphore` only for in-process waiting. The
gate acquisition obeys the request context, so work is not held after cancellation.

The limit is configurable and metrics report queued time, in-flight uploads, and
timeouts by store and OSS host.

### 4. Batch recovery action

Add a Studio batch action that selects task links from the batch and schedules only
failed `sds_design_sync` child tasks. It is idempotent, skips tasks already pending
or processing, and uses the same durable retry records rather than issuing 26
synchronous HTTP calls. It reports selected, skipped, and scheduled counts.

Do not reuse Studio `RetryItems`: it is for pre-task generation items and correctly
rejects items that already have ListingKit task links.

## Error handling and observability

- Include task id, batch id, store id, attempt, reason code, OSS host, and elapsed
  time in structured logs.
- Emit counters for upload failures and child-task retry outcomes, plus a histogram
  for upload duration and gate wait time.
- Treat auth errors, bad signatures, 4xx except 429, image validation errors, and
  SDS business errors as non-retryable and preserve the current review flow.

## Verification

- Unit-test typed classification for timeout, temporary network, 429/5xx, and
  non-retryable 4xx/auth errors.
- Add a local delayed-response multipart test for the exact response-header timeout.
- Test durable backoff, max-attempt exhaustion, idempotent claim, and successful
  child-task recovery without re-running unrelated workflow stages.
- Test the batch action for linked-task selection, duplicate suppression, and
  scheduled/skipped reporting.
- Run the affected Go packages, targeted integration tests with a local HTTP server,
  and a manual staging check using one affected task before scheduling a whole batch.
