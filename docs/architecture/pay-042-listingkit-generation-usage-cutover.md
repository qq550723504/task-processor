# PAY-042 ListingKit generation usage cutover

## Decision

The first PAY-042 slice meters only the asynchronous ListingKit generation
task created by `POST /api/v1/listing-kits/generate`. The HTTP handler retains
the active-subscription access check and does not write usage. The worker
claims the task, reserves one durable ledger unit before provider/workflow
execution, persists a successful `completed` or `needs_review` result, and
commits that same event exactly once.

The ledger fact is fixed: module `studio`, metric
`studio_design_jobs_succeeded`, quantity `1`, source type
`listingkit_generation`, source ID equal to the task ID, UTC `YYYY-MM` period,
empty metadata, and idempotency key `listingkit:generation:<task ID>`.

## Failure and recovery boundary

Quota rejection skips provider work and leaves a safe terminal task failure.
Workflow/local failure releases a still-reserved event. A commit failure is
persisted as the retryable `usage_commit_pending` block after the task result
is already durable. `RecoverTaskNow` handles that block through a
settlement-only path: it retries commit and clears the block without invoking
the generation workflow or task submitter again. Ambiguous or missing ledger
state fails closed for reconciliation.

## Wiring and rollout

ListingKit receives an optional settlement port. The bootstrap adapter is
constructed only when `TASK_PROCESSOR_LISTINGKIT_GENERATION_USAGE_LEDGER_ENABLED`
is `true`; the flag defaults to `false`. Enabling the flag requires the
existing Mem or GORM subscription repository, otherwise bootstrap fails closed.
OpenMeter remains behind the PAY-041 outbox; no provider, payment, or direct
OpenMeter call is introduced here.

The implementation deliberately does not meter Studio design/product-image
jobs, SHEIN submit/publish, 1688 task creation, storage deltas, batches, or
legacy counter paths. Those require separate result-boundary reviews. The
rollout sequence is controlled tenant cohort, reconciliation comparison with
legacy counters, then an independently approved widening decision.

## Verification boundary

The focused processor, recovery, configuration, bootstrap, API compatibility,
race, vet, and architecture-document tests are the evidence for this slice.
Passing these tests does not authorize enabling the production flag or imply
payment-provider integration.
