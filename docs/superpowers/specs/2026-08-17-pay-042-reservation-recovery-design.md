# PAY-042 reservation recovery design

## Goal

Ensure a ListingKit generation reservation cannot consume a tenant quota forever
when the worker exits between reserving usage and persisting its terminal or
retryable task state.

## Scope

This change applies only to the optional PAY-042 ListingKit generation ledger.
It does not call OpenMeter, alter payment providers, change legacy aggregate
usage, or change SHEIN, 1688, storage, or standalone Studio entrypoints.

## Design

The task stores an internal, durable generation-usage reservation intent and a
lease expiry. The worker writes the intent before asking the local ledger to
reserve, renews the lease while provider work is active, and records that the
reservation is held after a successful reserve. This pre-intent closes the
crash window between the ledger transaction and task persistence.

The recovery sweep reads expired reservation intents separately from normal
blocked-task retries. It inspects the stable task-id usage identity. A reserved
event is released, then the task is placed back into the ordinary retry flow;
an intent with no event is also retried. A committed, released, unknown, or
lookup-error event is never guessed: it stays durable for operator-visible
reconciliation rather than re-running provider work.

New ledger admission remains controlled by the explicit billing-tenant cohort.
Once a task has a durable reservation intent, retries bypass the cohort gate
and reuse the same stable identity, so narrowing or disabling the cohort does
not strand a previous reservation. Tasks created before this field existed have
no billing identity or reservation intent and remain on the legacy path; they
are not silently backfilled or charged during cutover.

Outside the cohort, retryable provider failures use the pre-existing classified
failure persistence. The PAY-042 scheduled retry path is used only when a
ledger reservation is active.

## Safety invariants

- A provider is never called before a successful reservation when the ledger is
  admitted for the task.
- A task with a durable reservation intent may resume or release its existing
  event even after cohort configuration changes.
- Stale recovery is lease-based; an active worker renews its lease and is not
  reclaimed by the sweep.
- Uncertain ledger state remains durable and is not released, committed, or
  retried by guessing.
- Missing billing identity means legacy behavior, not a fallback to the
  canonical owner tenant for a new ledger reservation.

## Verification

Tests must prove the crash-window pre-intent, expired reservation release and
retry, active-lease exclusion, cohort-narrowing replay, old-task legacy
fallback, and cohort-outside retry behavior. Run the focused ListingKit and
HTTP bootstrap suites, targeted race tests, package tests, vet, diff check, and
the repository test script before replying to review threads.
