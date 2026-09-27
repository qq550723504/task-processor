# Immutable settlement reversal locking

Status: IMPLEMENTATION_READY. Execution: #413. Design Basis: Independent Architecture
(bounded repair of the money owner's payment-level concurrency mechanism).

Independent boundary review: `reversal_boundary_review`, 2026-09-27, against
base `fa38859ed351267e5ec86de50ebefd9efb27e773`. No BLOCKER. Least-privilege,
mixed concurrency, isolation override, cancellation and observer replay remain
IMPLEMENTATION_TEST checks for this slice before merge.

## User outcome and authority

An invited user's settled purchase can be partially or fully refunded, and its
commission is reversed through the existing money and referral owners. The user
authorized this repair after the isolated #413 run failed with PostgreSQL
`permission denied for table ledger_payment_settlements` at `SELECT FOR UPDATE`.

Authority: [greenfield product baseline](../product/greenfield-no-legacy-migration.md),
[commercial money contract](commercial-wallet-billing-contract.md),
[current referral rules](referral-settlement-and-conduct.md), and the runtime
immutable-table contract in `money.VerifyProviderTopUpRuntime`. No UI change;
the current [UI authority](../product/final-ui-ia-authority.md) remains applicable.

Must: preserve immutable canonical settlements, exact replay/conflict semantics,
and combined refund + chargeback amounts no greater than commissionable cash.
Both ordinary reversal paths must work with the current least-privilege role.
Scope: ordinary (non-wallet-top-up) refund and chargeback persistence only.
Out of scope: payment policy, wallet-top-up reversal semantics, new tables,
permissions, routes, KYC, payout, scheduling/reconciliation, migration, real
provider calls, deployment/merge authority, and the old #475 UNKNOWN request.
No new accepted financial or authorization risk is introduced.

## Owner and execution path

Existing controlled settlement ingress / internal caller → money Repository
`RecordRefundSettlementAndNotify` / `RecordChargebackSettlementAndNotify` →
canonical money transaction → existing referral observer transaction → earnings
projection → authenticated BFF and current page. Injection and consumers stay
unchanged. The money database owns the payment/refund/chargeback facts; referral
owns commission claims, its ledger and projection. No second fact owner.

## Lock and transaction contract

PostgreSQL `FOR UPDATE` requires UPDATE authority, which this immutable table
deliberately denies. Replace that row lock in both ordinary reversal transactions
with the PostgreSQL transaction-scoped advisory lock already used elsewhere in
this repository, keyed by `money:ordinary-reversal:` + exact payment ID using
`hashtextextended(..., 0)`. Refund and chargeback MUST use the same namespace.
A hash collision only serializes unrelated payments, never admits excess value.
Do not create a shared lock framework or add privileges to any role.

Use READ COMMITTED for these PostgreSQL transactions: acquire the advisory lock
as a separate statement, then read the immutable payment, existing reversal,
both aggregate totals, and insert the new reversal in that order. The separate
post-lock reads must see a predecessor's committed refund/chargeback after a
wait. The lock remains held until commit/rollback; no network work is performed
under it. No ordering cycle is introduced: one payment lock, no nested payment
locks. Existing SQLite unit fixtures retain their transaction behavior; they
are not evidence for PostgreSQL concurrency. Unsupported dialects must fail
closed rather than silently omit the lock.

Wallet top-ups keep their existing provider claim lock/receipts and dispatch
before this ordinary path; their immutable payment purpose does not change.
Ordinary payment insertion/replay remains unchanged. Payment disappearance or
mutation is excluded by the current schema/runtime authority contract.

## Failure, identity and side effects

The existing refund ID / chargeback ID and payload comparisons remain unchanged.
Same identity and same payload returns the committed result even at the cap;
changed payload conflicts. Distinct identities compete under the payment lock
and re-read both totals before inserting. Confirmed rollback or cancellation
before commit leaves no reversal and releases the lock. Lost commit responses
remain UNKNOWN; use only the original identity/payload on the existing recovery
path rather than inferring rollback. Request cancellation and configured
PostgreSQL statement timeout bound waits. Restart/connection loss releases
transaction locks; there is no lease, new durable state or automatic retry.

Canonical settlement commits before the existing observer transaction. An
observer failure can still leave a committed canonical fact; recovery uses the
same original reversal ID and payload to notify the idempotent observer again.
This repair neither changes that existing boundary nor infers success from a
transport error. No provider payment/refund is dispatched by these methods.
Current authenticated/loopback service boundaries remain unchanged. No identity,
tenant scope or client-supplied ownership changes are introduced.

Legacy decision: N/A; this repairs the current canonical money owner and does
not revive a retired abstraction or add compatibility/backfill behavior.

## Bounded verification and delivery

Before production edits, obtain the AGENTS.md high-risk boundary review and mark
IMPLEMENTATION_READY. Reuse the existing PostgreSQL testcontainer and exact
money-owner grants; RED must reproduce both ordinary reversal failures under
the runtime role. GREEN must cover partial/full reversal and same-ID replay,
payload conflicts, mixed refund/chargeback contention at the cap, and canceled
lock waits followed by a successful request. Assert immutable UPDATE remains
denied and no excess reversal facts are inserted. Existing wallet-top-up tests
must remain green. Exercise the canonical-to-referral observer path with the
existing fixture, including its commission projection.

Keep this independently blocking main repair separate from the already-reviewed
registration fixture PR #537. One repair branch/PR; one final independent diff
check. The retained user instance remains on verified merged main until an
authorized merge and matching main CI permit runtime re-verification. Candidate
integration results are developer evidence, not product acceptance.
