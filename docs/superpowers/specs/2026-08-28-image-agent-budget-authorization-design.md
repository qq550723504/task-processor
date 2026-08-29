# Image Agent Budget Authorization Design

## Context

PR #239 introduces a recoverable v3 image-generation workflow. The run model
already exposes configured limits and usage, but those values are currently
descriptive only: the Temporal start payload drops the budget, the workflow
does not authorize external effects, and persisted usage remains zero.

This is unsafe to fix with counters derived from slot count. A main-image slot
can invoke subject extraction and white-background rendering, while a scene
renderer can return more than one image. The current provider contracts also
do not expose a trustworthy cost upper bound or receipt. A counter updated
after the provider call cannot prevent spend that has already happened.

The design therefore treats a budget as an authorization policy for external
effects, not as display metadata.

## Goals

- Prevent every governed ProductImage effect whose worst-case usage would
  exceed a configured run limit.
- Keep authorization correct under concurrent slots, Temporal activity retry,
  worker loss, database failure, and ambiguous provider outcomes.
- Publish committed run usage to the existing projection without using a
  remote metering service as the correctness lock.
- Preserve frozen v2 Temporal wire contracts and old v3 histories.
- Close the other active PR #239 review findings at their owning boundaries:
  shared AI identity, durable identifier grammar, retry command acceptance,
  and healthy SSE connection retention.

## Non-goals

- Charging a customer, changing subscription entitlements, or enabling
  OpenMeter in production.
- Estimating provider prices inside the workflow.
- Automatically releasing an ambiguous provider reservation.
- Broad ProductImage or ListingKit refactoring unrelated to governed
  invocation and recovery.

## Limit semantics

Transport inputs use presence-aware optional limits. Omitted means that the
dimension is not capped. Zero is an enabled hard limit of zero. Positive
values are enabled hard limits. Negative values are invalid.

The normalized domain policy uses an explicit enabled flag and non-negative
value; it does not use zero or a negative value as a sentinel.

- Images: maximum generated outputs authorized from provider effects.
- Agent steps: maximum governed ProductImage capability invocations.
- Model calls: maximum outbound model requests.
- Repair attempts per slot: attempts after the first execution.
- Cost micros: maximum conservative provider cost in micros.
- Elapsed: absolute run deadline derived once from the persisted start time.

Usage is a vector. Authorization fails if any enabled dimension would be
exceeded. Limits are never traded against one another.

## Provider-neutral quote and receipt contracts

Each budget-capable slot executor supplies a side-effect-free quote before
generation. The quote contains:

- a worst-case usage vector;
- the finite ordered set of allowed operations;
- a maximum output count for each image-producing operation;
- provider/model and pricing-version identities;
- a stable fingerprint bound to the slot execution input.

Each governed invocation must consume an operation from that quote before
making a network call. An invocation absent from the quote fails locally.
Provider adapters must pass the quoted output maximum to providers that can
return multiple images and reject responses that exceed it.

The execution result supplies a receipt containing the actual usage vector,
provider request identities, and either an actual-cost or upper-bound cost
basis. If the adapter cannot obtain a trustworthy actual cost, it settles the
reserved upper bound. If it cannot quote a trustworthy upper bound, a
cost-capped run is blocked before the provider call with
budget_quote_unavailable.

The initial ProductImage adapter quotes the existing operation graph:

- main: subject extraction plus white-background rendering;
- scene, detail, selling point, and size: scene rendering;
- future repair/model stages must add explicit quoted operations before they
  can run.

## Durable reservation model

The existing local usage-ledger reserve/commit/release pattern is generalized
into a provider-neutral internal usage authorization component. OpenMeter or
another remote meter may consume committed events later, but is not the lock
that protects provider spend.

Each reservation is uniquely identified by:

tenant, owner user, run, plan revision, slot, attempt, and effect kind.

It stores the quote fingerprint, reserved vector, settled vector, status,
provider request identities, pricing version, timestamps, and failure
classification. Status is one of reserved, committed, released, or unknown.

The run row keeps separate reserved and committed aggregate vectors. The
public Run.Usage projection exposes committed usage and elapsed time; reserved
usage remains internal but participates in admission.

Reservation is a database transaction:

1. Lock the owner-scoped run row.
2. Return an existing matching reservation idempotently.
3. Reject a mismatched reuse of the business key.
4. Evaluate committed plus reserved plus the new quote against every limit.
5. Insert the reservation and increment reserved aggregates.
6. Create the v3 provider-claimed effect in the same transaction.

This transaction is implemented by the repository that already owns the v3
slot effect. It prevents concurrent slots from oversubscribing the last unit
of budget and avoids a second independent ledger beside the effect record.

Settlement is also atomic and idempotent:

- commit moves the reserved vector to the settled vector, moves aggregate
  amounts from reserved to committed, and persists the provider receipt;
- release removes reserved aggregate amounts only when the provider is known
  not to have produced a billable effect;
- unknown retains the reservation and records the provider identities needed
  for reconciliation.

The durable run projection is updated from the same committed aggregate. It
must never derive authoritative usage by counting UI candidates.

## Temporal and effect flow

StartManual passes the normalized policy, business task identity, and
persisted run start/deadline into the additive workflow input. Child and v3
activity inputs receive the same immutable policy and identity envelope.

The v3 activity flow is:

1. Build a side-effect-free quote.
2. Atomically reserve budget and claim the provider effect.
3. If the claim already exists, follow the persisted phase and never generate
   again merely because the activity retried.
4. Invoke only operations authorized by the quote.
5. Persist the provider receipt before staging/publication work.
6. Commit or retain the reservation according to the provider outcome.
7. Persist slot and run projections with committed usage.

The workflow enforces repair-attempt and elapsed limits before starting a
child. The activity additionally derives a context deadline from the persisted
absolute deadline. Reaching a deadline cannot prove an in-flight remote call
did not execute, so a timeout after dispatch becomes unknown rather than
released.

New workflow fields are additive and presence-aware. Histories without a
policy version follow the legacy uncapped interpretation. Frozen v2 payloads,
activity names, and behavior are unchanged. New budget behavior is selected
through a new Temporal version marker before it affects workflow decisions.

## Failure and recovery matrix

| Failure point | Durable outcome | Provider retry |
| --- | --- | --- |
| Quote unavailable or over budget | No reservation; deterministic blocked projection | No |
| Database fails before reservation commit | No authorization | No |
| Reservation committed, provider not dispatched | Release only when non-dispatch is proven | No automatic retry until reconciled |
| Provider explicitly rejects before execution | Released | User command may create the next stable attempt |
| Provider returns success and receipt persists | Committed | No |
| Provider dispatched, response or receipt persistence is ambiguous | Unknown and still reserved | No |
| Worker restarts after provider claim | Resume persisted phase; never regenerate an unowned claim | No duplicate |
| Projection write fails after budget commit | Settlement-only projection recovery | No |

Unknown reservations require a controlled reconciliation path. Reconciliation
uses provider request identity and never guesses from elapsed time alone.

## Execution identity

The immutable execution envelope contains tenant ID, owner user ID, business
task ID, and trace identity. Activity restoration writes both the authenticated
identity context and the shared ProductImage AI identity context from this
single envelope. ProductImage allowlists and governed invocation records
therefore see the same tenant, user, and business task that started the run.

## Durable identifiers

The artifact-key identifier grammar becomes one exported value-object
validator shared by:

- run start;
- initial and replacement plan slot validation;
- retry-slot commands;
- artifact preparation and publication validation.

The accepted grammar remains an ASCII alphanumeric first character followed
by at most 127 ASCII alphanumeric, dot, underscore, or hyphen characters.
Inputs that cannot become durable object keys are rejected before run
initialization or provider execution.

## Command and projection transport

RetrySlot waits only for Temporal update acceptance and returns HTTP 202 with
the action ID. It does not wait for a child activity that can run for minutes.
Business completion and failure remain visible through the persisted pending
command and projection stream. Other short commands keep their existing
completed-stage semantics unless separately reviewed.

The React hook treats snapshot refresh and stream recovery as separate
operations. A healthy projection event refreshes the snapshot while keeping
the current EventSource open. Only stream error, authentication failure, run
change, or component disposal closes it. Reconnects include the last committed
cursor in the event URL so the ListingKit proxy does not depend on forwarding
the browser-managed Last-Event-ID header.

## Implementation slices

1. Shared identifier validation, identity-envelope propagation, accepted retry
   transport, and SSE connection retention. These are independently testable.
2. Usage policy types, optional transport semantics, quote/receipt contracts,
   and ProductImage adapter quotes.
3. Atomic v3 effect plus budget reservation for memory and GORM repositories,
   including concurrency contract tests.
4. Temporal v3 authorization, settlement, deadline, repair, replay, and
   crash-recovery behavior.
5. Projection exposure, integration verification, and inline review closure.

Each slice must be independently green and compatibility preserving. No
production rollout, deployment, migration execution, or billing-provider
enablement is authorized by this design.

## Verification

- Domain tests for optional-limit normalization and vector arithmetic.
- ProductImage tests proving quotes cover every actual governed invocation and
  enforce maximum output counts.
- Repository contract tests for memory and GORM implementations, including
  concurrent last-unit reservation and idempotent commit/release/unknown.
- Temporal tests for replay markers, repair and elapsed admission, activity
  retry, worker loss, unknown outcome, and settlement-only recovery.
- Identity tests at the real tenant-gated ProductImage boundary.
- Service tests for invalid run and slot identifiers before initialization.
- Client tests proving retry waits for accepted rather than completed.
- Vitest coverage proving projection refresh does not replace a healthy
  EventSource and reconnect resumes from the stored cursor.
- Focused Go race tests, Go vet, frontend type/lint checks, and existing
  image-agent backend/frontend suites.
