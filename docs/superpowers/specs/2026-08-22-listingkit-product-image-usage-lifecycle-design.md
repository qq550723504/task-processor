# ListingKit Product Image Usage Lifecycle Design

## Context

PR #166 introduced ledger-backed product-image reservations while retaining the
legacy counter during staged rollout. Two follow-up failures exposed a missing
durable lifecycle boundary:

- batch and synchronous API reservations shared `listingkit_product_image`, so
  the API reconciler could expire an active batch reservation;
- batch settlement recalculated the rollout decision instead of using the
  accounting route chosen during authorization, so a rollout change between
  authorization and settlement could leave work unmetered or strand a ledger
  reservation.

The route and work kind are facts about a reservation lifecycle, not current
feature-flag state. They must remain stable after work starts.

## Goals

- Select a batch candidate's accounting route exactly once and persist it
  before product-image generation starts.
- Make direct synchronous, asynchronous, and batch reservations independently
  identifiable to reconciliation.
- Preserve safe behavior for links and reservations created before this change.
- Keep legacy counters and usage-ledger mirrors balanced for every terminal
  outcome.

## Non-goals

- Replace the legacy counter or end the staged ledger rollout.
- Change quota values, entitlement ownership, or provider generation behavior.
- Retrospectively release ambiguous old reservations solely from their age.

## Design

### Durable batch accounting route

Add `ProductImageUsageRoute` to `StudioBatchTaskLinkRecord`, with final values
`legacy` and `ledger`. Newly created AI links write an explicit `pending`
sentinel; empty remains reserved for links created before this change. A new
optional repository operation atomically replaces `pending` with a final route
while the caller holds its claim token. The service derives the route once from
the admission policy and reservation availability, persists it, then
authorizes on that exact route.

Settlement and release read the stored route; they never evaluate
`generationUsageAdmission` again. This makes an in-flight candidate immune to
rollout additions or removals. The existing durable link remains the authority
for retries and lease reclaims.

For pre-change links whose route is empty, the compatibility resolver checks
for the deterministic ledger reservation first. If it exists, the route is
persisted as `ledger`; otherwise it is persisted as `legacy`. A new link can
never take this compatibility path because it starts at `pending`. Choosing
legacy when no old event exists is conservative: it prevents a rollout change
from turning an already-authorized legacy candidate into an uncharged no-op.

### Reservation work kinds

Reserve events use separate source types:

- `listingkit_sync_product_image` for direct synchronous HTTP generation;
- `listingkit_async_product_image` for async jobs;
- `listingkit_batch_product_image` for batch candidate generation.

The direct API reconciliation loop only applies its 30-minute expiry lease to
the synchronous source type. Async reservations continue to use their durable
job lease. Batch reservations are owned by the batch task-link claim and are
not aged out by API reconciliation.

The batch usage adapter is the only producer of the batch source type. This
keeps source classification local to the adapter that creates the ledger event
instead of inferring it from candidate IDs in a different layer.

### Compatibility and recovery

Existing generic `listingkit_product_image` events remain readable. The
reconciler may expire them only when their idempotency key has the known direct
API prefix `listingkit:api:studio_product_image:`. Generic events without that
proof are excluded from age-based recovery, because they can be live batch
reservations created by the merged PR.

Existing released, pending-mirror, and committed-mirror reconciliation rules
remain unchanged. A release of any settled mirror must retain the existing
`legacy mirror release pending` marker until the compensating legacy operation
is complete.

## Failure semantics

| Boundary | Durable state | Retry behavior |
| --- | --- | --- |
| Route claim succeeds, authorization fails | Route is retained | Retry uses the same route and retries authorization |
| Ledger reservation succeeds, process exits | Batch route and event identity are retained | Retry commits or releases the same reservation |
| Legacy authorization succeeds, process exits | Legacy route is retained | Retry settles through idempotent legacy accounting |
| Direct synchronous process exits | Sync source event remains reserved | Direct reconciler releases it after its expiry lease |
| Batch generation exceeds 30 minutes | Batch source event remains reserved | Batch heartbeat/settlement owns it; API reconciler does not release it |

## Tests and acceptance criteria

Add Mem and GORM coverage for:

1. a legacy-authorized batch that enters the ledger rollout before settlement;
2. a ledger-authorized batch that leaves the rollout before settlement;
3. an active batch reservation older than the synchronous expiry lease;
4. an expired direct synchronous reservation and an active direct reservation;
5. old generic direct events versus ambiguous old generic batch events;
6. route-claim retry, lease reclaim, and legacy-mirror release idempotency.

Acceptance requires all affected package tests, targeted race tests, and a
full repository test run. The baseline was serially green with
`go test ./... -count=1 -p 1`; one parallel baseline run failed only in
`internal/listingsubscription`, while a standalone rerun passed. That
pre-existing intermittent parallel-test behavior is tracked separately and is
not a reason to weaken the lifecycle assertions above.

## Implementation boundaries

The expected changes are limited to the batch task-link model/repositories,
the batch product-image usage service, the subscription product-image adapter,
and the API reconciliation filter/tests. No HTTP contract, entitlement schema,
or external billing provider interface changes are required.
