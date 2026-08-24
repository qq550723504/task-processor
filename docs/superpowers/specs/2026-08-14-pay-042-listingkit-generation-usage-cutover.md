# PAY-042 ListingKit generation usage cutover

## Status

Proposed design. This document is the review gate before implementation.

## Context

PAY-041 introduced the local transactional usage ledger and its pending OpenMeter
outbox. Existing ListingKit generation still uses the legacy subscription guard
and aggregate usage counter. The first PAY-042 slice will migrate only
`POST /api/v1/listing-kits/generate` and its asynchronous task execution.

The HTTP handler creates and queues a task. It does not prove that billable
generation happened. The authoritative billable boundary is the task processor:
the task is claimed, the workflow runs, and the final result is durably written
as `completed` or `needs_review`.

## Goals

- Enforce the PAY-041 durable quota reservation before provider or generation
  work begins.
- Charge exactly once for a successful ListingKit generation task, including
  successful results that require human review.
- Release reservations for local validation, queue, workflow, or cancellation
  failures.
- Keep retries and duplicate worker delivery idempotent by task identity.
- Make ledger settlement failures recoverable without rerunning generation.
- Preserve legacy usage behavior for endpoints outside this first slice.
- Keep OpenMeter delivery asynchronous through the existing ledger outbox.

## Non-goals

- No payment provider, subscription registry, or checkout integration; PAY-043
  owns that boundary.
- No SHEIN submit/publish metering, 1688 task-create metering, storage upload or
  delete metering, Studio design endpoint migration, or batch/recovery cutover.
- No direct OpenMeter call from a request handler or task processor.
- No global removal of `AuthorizeUsage` or `RecordUsage`.

## Proposed call flow

```text
HTTP GenerateListingKit
  -> legacy active-subscription access check
  -> CreateGenerateTask (persist + dispatch)

worker ProcessListingKit
  -> claim pending task
  -> ReserveUsage (task idempotency key)
  -> run workflow/providers
  -> local failure: ReleaseUsage, persist failed/retryable state
  -> success: persist completed/needs_review result
  -> CommitUsage
  -> commit failure: persist usage-settlement retryable block; do not rerun workflow
```

The route may return an accepted task before the worker discovers a quota
exhaustion. This is intentional for an asynchronous API: the worker must reserve
before any billable provider work, and the task result is the source of truth.

## Ledger fact contract

The generation reservation uses the following immutable fact:

| Field | Value |
| --- | --- |
| module | `studio` |
| metric | `studio_design_jobs_succeeded` |
| quantity | `1` |
| source type | `listingkit_generation` |
| source id | task ID |
| idempotency key | `listingkit:generation:<task ID>` |
| period | UTC `YYYY-MM` at reservation time |
| metadata | empty (provider-safe outbox boundary) |

`studio_design_jobs_succeeded` is the canonical ledger metric. PAY-041 already
maps the legacy `design_jobs` plan limit to this metric, so no plan migration is
needed in this slice.

The task ID is created and persisted before dispatch, remains stable across queue
redelivery, and therefore provides a safer idempotency key than hashing a mutable
JSON request body.

## Dependency boundary

ListingKit will receive an optional generation usage-settlement dependency through
its service configuration. The dependency exposes only the operations needed by
this flow (reserve, commit, release, and settlement-state lookup); it does not
expose provider credentials or OpenMeter transport.

The HTTP/bootstrap layer adapts the existing `listingsubscription.Service` to
this boundary. When the dependency is absent, the current legacy behavior remains
unchanged. Enabling the dependency is an explicit rollout/configuration step.

## Lifecycle rules

1. **Claim and reserve**: after `MarkProcessing` succeeds and before
   `runWorkflow`, reserve one unit. An existing reservation for the same task is
   replayed; a committed event is treated as already settled and must not cause
   the workflow to run again.
2. **Quota rejection**: `ErrUsageQuotaExceeded` prevents provider work and is
   persisted as a non-retryable task failure with a safe quota-facing message.
3. **Local/queue/workflow failure**: if the event is still reserved, release it
   with a bounded reason code. If release cannot be proven safe, leave it
   reserved for reconciliation rather than guessing.
4. **Successful result**: persist the terminal task result first. Both
   `completed` and `needs_review` are billable success outcomes.
5. **Commit**: commit the same event idempotently. A repeated commit is a no-op.
6. **Commit uncertainty/failure**: do not rerun providers. Persist a
   `usage_commit_pending` retryable block containing only the task ID and safe
   reason code. A settlement-only recovery path resolves the reservation by
   idempotency key and retries `CommitUsage`; it must not enqueue the generation
   workflow again.

## Recovery and reconciliation

The existing generic task recovery path must distinguish a
`usage_commit_pending` block from provider/queue retry blocks. Settlement-only
recovery will:

1. load the task and derive its stable usage idempotency key;
2. look up the ledger event for the task tenant and key;
3. commit a reserved event, or accept an already committed event;
4. clear the retryable block and retain the already persisted task result.

If the event is missing or has an ambiguous delivery state, recovery fails closed
and leaves the block for PAY-044 reconciliation. It must never release or reverse
an event solely because OpenMeter delivery is unavailable.

## Compatibility boundary

`GenerateListingKit` keeps its current active-subscription check for immediate
feedback. The first slice removes no legacy counter calls outside this route's
task execution. Studio design/product-image async endpoints and reference
analysis remain on their current `AuthorizeUsage`/`RecordUsage` path until their
own result boundaries are modeled.

## Test plan

- API contract test: active subscription still permits task creation; no usage is
  recorded by the HTTP handler itself.
- Processor tests: reserve occurs before workflow; quota rejection skips provider
  calls; failures release; completed and needs-review results commit exactly once.
- Replay tests: duplicate worker delivery and duplicate reservation/commit calls
  produce one event and one outbox item.
- Settlement tests: commit failure creates a settlement-only retryable block;
  recovery commits without invoking the workflow.
- Durable ledger integration test: concurrent generation tasks respect the quota
  and preserve one event/outbox row per task.
- Legacy compatibility tests: when the optional dependency is absent, existing
  generation tests and unrelated endpoints retain their current behavior.

## Rollout

1. Land implementation and tests with the dependency disabled by default.
2. Enable it for a controlled tenant cohort and inspect ledger reconciliation and
   pending outbox health.
3. Compare durable ledger totals with legacy `design_jobs` counters before
   widening the cohort.
4. Only after this slice is stable, plan the next entrypoint (1688 task create or
   Studio design async) separately.

## Open review decisions

- Confirm the `studio_design_jobs_succeeded` metric is the intended first billing
  unit for the whole ListingKit generation task.
- Confirm that a quota rejection discovered asynchronously should leave the task
  as failed rather than introduce a new user-facing task status.
- Confirm the settlement-only recovery behavior for commit failures.
