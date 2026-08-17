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

Before a new reservation, the worker persists a task-side `pending` intent
with a ten-minute lease; after the ledger reserve it changes that intent to
`reserved`. A live worker renews the lease every three minutes. Terminal
commit or release clears the intent only after the ledger operation succeeds.
If a worker dies while the task remains `processing`, recovery scans an expired
intent before ordinary blocked-task recovery. It looks up the deterministic
ledger event: a known `reserved` event is released and the task becomes the
auto-retryable `generation_usage_worker_interrupted` block with a delayed next
attempt; no event means the same safe retry without a release. Lookup failure,
an already committed/released/reversed event, or a failed release produces the
non-auto-resuming `generation_usage_reconciliation_pending` block and retains
the intent for operator reconciliation. That sweep never submits the provider
task in the same pass.

## Wiring and rollout

ListingKit receives an optional settlement port. The bootstrap retains that
adapter whenever a durable subscription ledger is available so previously
persisted `usage_commit_pending` and `usage_release_pending` blocks can drain.
New reservations require both
`TASK_PROCESSOR_LISTINGKIT_GENERATION_USAGE_LEDGER_ENABLED=true` and an explicit
`TASK_PROCESSOR_LISTINGKIT_GENERATION_USAGE_LEDGER_TENANT_IDS` billing-tenant
cohort match. An empty cohort is fail-safe: it admits no new ledger events while
recovery still settles historical blocks. The task keeps its canonical owner
tenant for access scope and stores the legacy subscription tenant separately as
internal billing identity for the ledger. Enabling the flag requires the
existing Mem or GORM subscription repository, otherwise bootstrap fails closed.
OpenMeter remains behind the PAY-041 outbox; no provider, payment, or direct
OpenMeter call is introduced here.

The cohort gate applies only when creating a new intent. A task that already
has an intent resumes its reserve/commit/release path even if the flag or
cohort is later narrowed, preventing a stranded reservation. Tasks created
before this cutover have an empty `BillingTenantID`; when a cohort is configured
they deliberately remain on the legacy path rather than being charged against
their canonical owner tenant. Backfilling a durable billing identity is a
separate, explicitly approved migration and is not performed by worker retry.

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
