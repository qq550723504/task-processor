# Referral Withdrawal Personal KYC Contract

Status: **CANDIDATE — Independent Architecture review required**  
Issue: #519  
Dependencies: #510, merged PR #520, #469 / PR #517  
Baseline: `c66f63bdbb007a770e4be667b6d8dafb9c3ce111`

## 1. User outcome and product decision

A user may create a new referral-earnings withdrawal only when the **same current
personal subject has an authoritative personal KYC result in `VERIFIED` state**.

Personal KYC is a **necessary** withdrawal condition, never a sufficient one.
All existing requirements remain: authenticated current subject, verified email
and phone, an active payout method owned by that subject, minimum amount,
available earnings, expected version/idempotency and the current withdrawal
state machine. A successful KYC check never creates a payout method, reserves
money, creates a withdrawal, approves it, or pays it.

This contract applies to **new withdrawal requests only**. It must not make KYC
a precondition for canceling an already-created withdrawal or for an authorized
platform reviewer to process an existing request.

Enterprise verification is not a substitute for personal KYC. Referral earnings
and withdrawals are personal-subject owned.

## 2. Design basis

Classification: **Independent Architecture**.

Reason: this adds a fail-closed cross-domain dependency on the money-adjacent
withdrawal admission path. The personal KYC fact remains owned by
`internal/subjectverification`; referral economics remains owner of earnings
and withdrawals; canonical money remains owner of payout methods.

The dependency that originally blocked #519 is now resolved. PR #520 merged the
Aliyun personal verification implementation and froze #510 design §14 as
`IMPLEMENTATION_READY`. The personal implementation persists a subject-bound
application with an active `scope`, and only a successful server-side provider
query can transition that application to `VERIFIED`.

Real Aliyun calls and production rollout remain separately gated; this contract
does not authorize provider calls, deployment or real withdrawals.

## 3. Existing owners and facts

| Fact / behavior | Authoritative owner | Consumer behavior |
| --- | --- | --- |
| current authenticated user | existing auth identity boundary | withdrawal derives the subject server-side |
| email / phone verified | existing self-profile projection | existing `verifiedPayoutIdentity` remains unchanged |
| personal KYC application and `VERIFIED` state | `internal/subjectverification` + its existing PostgreSQL store | consume through a narrow read port only |
| active payout method | canonical money owner | existing payout-method lookup remains unchanged |
| referral earnings / withdrawal lifecycle | `internal/referraleconomics` | existing `RequestWithdrawal` remains the mutation owner |
| current referral rule projection | referral HTTP/rules contract | publishes KYC requirement only after enforcement exists |

No KYC boolean, identity document, name, ID number, provider response or KYC
state mirror may be written to the referrals database.

## 4. Narrow personal-KYC read contract

The consumer-facing capability is deliberately smaller than the account KYC UI:

```go
type personalKYCReader interface {
    IsPersonalVerified(context.Context, string) (bool, error)
}
```

The subject argument is the already-authenticated current UserID. Browser input
must never select another subject.

The implementation belongs to the existing subject-verification owner and reads
the existing `personal_verification_applications` fact. It must:

1. perform **no provider/network call**;
2. perform **no write, refresh, reconciliation or state transition**;
3. require a valid active personal-verification scope;
4. read the latest application for the same UserID through the existing store;
5. return `true` only when the persisted application is for the same UserID,
   the application scope equals the active scope, and state is exactly
   `subjectverification.Verified`;
6. return `false, nil` for no application, pending, rejected, expired/unknown
   or stale-scope facts;
7. return an error when the authoritative store/configuration is unavailable.

The read must not require the current phone value and must not call
`PersonalService.Read`, because that UI projection also handles retry URLs,
phone presentation and quota state that are irrelevant to withdrawal
eligibility.

The read may be implemented on the existing `PersonalService` as a dedicated
read-only method, but its eligibility path must depend only on the store, active
scope and bounded subject—not on provider reachability. Existing persisted
`VERIFIED` facts remain readable during a transient Aliyun outage.

If personal-verification configuration/service is not assembled, the referral
consumer receives no reader and new withdrawal requests fail closed as a
dependency outage. Other referral reads, earnings and payout-method operations
must continue to start and operate.

## 5. Composition and dependency direction

The current application already builds subject verification before referrals.
Reuse that composition order:

```text
source_accounts PostgreSQL
        |
subjectverification PersonalService
        |
        | narrow IsPersonalVerified(subject) read port
        v
app/httpapi composition
        |
        v
referralHTTPModule requestWithdrawal
        |
        +--> existing self-profile verification
        +--> existing payout-method reader
        +--> referraleconomics.RequestWithdrawal
```

The referral module must not import the persistence implementation and must not
open/use the source-accounts database directly. The app composition layer
extracts a narrow reader from the already-built subject-verification module and
injects it into the referral HTTP module.

No new database, queue, workflow engine, cross-database transaction or provider
adapter is introduced.

## 6. Exact withdrawal admission semantics

For `POST /api/v1/account/referrals/withdrawals`:

1. derive the authenticated current subject;
2. preserve the existing verified-email + verified-phone check;
3. validate the request envelope/body/idempotency as today;
4. call `IsPersonalVerified(ctx, currentUserID)` **before any withdrawal
   mutation**;
5. reader missing or dependency error → `503 DEPENDENCY_UNAVAILABLE`;
6. reader returns false → `409 PAYOUT_ELIGIBILITY_UNMET`;
7. continue with the existing active payout-method lookup;
8. call the unchanged `referraleconomics.RequestWithdrawal` for
   amount/balance/version/idempotency/state validation.

The gate is not added to:

- payout-method creation/read;
- withdrawal list/read;
- cancel of an already-created withdrawal;
- platform review/reject/pay transitions.

That asymmetry is intentional: KYC is an **admission** requirement for creating
new withdrawals. A later KYC dependency outage must not trap a user in an
existing request by removing the cancel path.

KYC denial/unavailability must occur before `RequestWithdrawal`, so no
withdrawal, reservation or audit fact is created by a rejected request.

## 7. Failure, race and replay rules

- `PENDING`, `OUTCOME_UNKNOWN`, `REJECTED`, no application and stale
  verification scope all fail eligibility.
- Store/configuration failures fail closed as dependency unavailable and do not
  degrade to "not verified".
- The eligibility read never refreshes provider state. The user completes or
  refreshes KYC through the existing #510 account-verification flow, then retries
  the withdrawal.
- A same-key retry that is rejected before `RequestWithdrawal` creates no
  idempotency/withdrawal fact. Once KYC is verified, the normal economics
  idempotency contract applies unchanged.
- Current v1 personal KYC has no revocation transition. If a future contract
  introduces revocation/expiry, this admission contract must be reviewed rather
  than silently inferring revocation semantics.
- No cross-database atomicity is claimed. This is an admission read followed by
  the existing referral mutation; KYC is not copied into the withdrawal record.

## 8. Referral rules contract and UI truth

The rules page must not hardcode a future KYC requirement before server
enforcement exists.

When the server-side gate is implemented in the same delivery, the rules
endpoint changes atomically from strict `referral-rules-v1` to
`referral-rules-v2` and adds:

```json
{
  "schemaVersion": "referral-rules-v2",
  "personalKycRequired": true
}
```

All existing rule fields remain unchanged. The strict frontend schema is updated
in the same PR, and the withdrawal rules card may then state that completed
personal KYC is required in addition to the displayed amount and other current
eligibility checks.

This avoids two invalid states:

- UI says KYC is required while runtime does not enforce it;
- runtime enforces KYC while the authoritative rules projection omits the new
  requirement.

PR #517 may merge independently because its current wording only says the
minimum is not sufficient and that current withdrawal eligibility is checked.
The #519 UI update must be rebased/synced after #517 to avoid competing edits to
the same referral component.

## 9. TDD and verification matrix

Implementation starts with failing tests for the actual admission boundary.

Required tests:

- verified email/phone + active payout + enough funds + personal KYC VERIFIED →
  reaches existing `RequestWithdrawal`;
- KYC absent / pending / rejected / unknown / stale scope → 409 and economics
  mutation spy is not called;
- KYC store/config unavailable → 503 and economics mutation spy is not called;
- KYC for another subject cannot satisfy current subject;
- provider adapter is never invoked by the KYC eligibility read;
- cancellation of an existing withdrawal remains possible without a KYC read;
- payout-method creation remains unchanged;
- same-key retries cannot duplicate withdrawals or bypass KYC;
- rules v2 strict frontend contract requires `personalKycRequired: true`;
- rules UI names personal KYC only with the v2 server contract;
- existing referral economics, payout-method, withdrawal-state, architecture and
  browser/accessibility tests remain green.

Use the existing PostgreSQL personal-verification repository integration style
to prove a persisted VERIFIED fact survives process reconstruction and is read
by subject. No real Aliyun call or real payout is part of this test matrix.

## 10. Security and privacy invariants

- no KYC name, ID number, phone, face data, provider token or provider payload
  enters referral logs/database/API;
- only the current authenticated UserID crosses the KYC read port;
- failure responses do not reveal whether another subject is verified;
- provider unavailability is not consulted for an already-persisted VERIFIED
  fact;
- browser cannot send `verified=true`, a KYC state, a provider ID or another
  subject to bypass the gate;
- enterprise KYC is never consumed for personal referral withdrawals.

## 11. Scope and legacy decision

In scope: narrow personal-KYC read capability, app composition, new-withdrawal
gate, focused tests, rules contract v2 and truthful rules/withdrawal copy after
the gate exists.

Out of scope: KYC provider changes, enterprise substitution, KYC revocation,
automatic withdrawal approval/payment, new payout channels, commission or
settlement changes, real provider verification, production deployment, real
transfer, or a generic policy engine.

Legacy decision: **N/A**. No legacy path is extended, migrated or preserved.

## 12. Admission gate

This document is a candidate until an independent architecture review confirms:

- owner and dependency direction;
- read-only authoritative KYC semantics;
- new-withdrawal-only placement;
- fail-closed behavior;
- no second KYC fact source;
- no hidden provider call or cross-database transaction;
- rules contract/UI ordering.

Only after that review has no unresolved blocker may #519 be marked
`IMPLEMENTATION_READY` and production code be added to the same delivery PR.
