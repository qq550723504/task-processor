# Referral Withdrawal Personal KYC Contract

Status: **IMPLEMENTATION_READY — architecture review round 2 completed; rollout amendment approved 2026-09-27**  
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

For `POST /api/v1/account/referrals/withdrawals`, committed-operation replay is
resolved **before mutable eligibility dependencies**. This preserves an
already-committed result after response loss even if profile, KYC or payout
dependencies are temporarily unavailable.

The referral owner exposes a narrow read-only replay capability:

```go
type withdrawalReplayReader interface {
    ReplayWithdrawal(context.Context, WithdrawalReplayRequest) (Withdrawal, bool, error)
}
```

`WithdrawalReplayRequest` contains only the authenticated referrer, CNY,
amount, payoutMethodID, expectedVersion and idempotency key. The repository:

1. looks up `referral_withdrawal_operations` by the idempotency key;
2. when absent, returns `found=false` without creating anything;
3. when present, loads the referenced authoritative withdrawal row;
4. requires that row's `referrer` exactly equals the authenticated subject;
5. reconstructs the existing withdrawal fingerprint using the committed row's
   method plus the request's referrer/currency/payoutMethodID/amount/version and
   compares it to the stored fingerprint;
6. returns the current withdrawal projection only on an exact match;
7. returns the existing idempotency conflict for a same key with different
   subject or payload.

No schema change is required: the existing operation fingerprint already binds
referrer, currency, method, payoutMethodID, amount and expectedVersion, while the
referenced withdrawal row supplies the committed method/referrer needed for safe
reconstruction without consulting the payout-method owner.

The HTTP order is:

1. derive the authenticated current subject;
2. validate/decode the request body and idempotency key sufficiently to form the
   replay request;
3. call `ReplayWithdrawal`;
4. exact committed replay → return that withdrawal immediately, with **no**
   profile, KYC, payout-method or new economics mutation;
5. replay conflict → existing `409 CONFLICT`; replay-store failure →
   `503 DEPENDENCY_UNAVAILABLE`;
6. only for `found=false`, run the existing verified-email + verified-phone
   check;
7. call `IsPersonalVerified(ctx, currentUserID)`;
8. KYC reader missing/error → `503 DEPENDENCY_UNAVAILABLE`; false →
   `409 PAYOUT_ELIGIBILITY_UNMET`;
9. continue with the existing active payout-method lookup;
10. call the unchanged `referraleconomics.RequestWithdrawal`, whose internal
    operation lookup remains the final race-safe idempotency guard.

A concurrent request may commit after the preflight reports absent. The final
transactional operation lookup still prevents duplicate withdrawal facts. If a
dependency fails during that race, the caller may retry the same key; the next
preflight returns the committed result. No KYC state is copied into referrals.

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
  verification scope all fail **new** withdrawal eligibility.
- Store/configuration failures fail closed as dependency unavailable and do not
  degrade to "not verified" for a never-seen key.
- An exact already-committed operation is replayed before those eligibility
  checks, so response-loss recovery does not depend on current KYC/profile/payout
  availability.
- A same-key/different-payload or same-key/different-subject request never
  bypasses KYC through replay; it returns the existing idempotency conflict.
- The eligibility read never refreshes provider state. The user completes or
  refreshes KYC through the existing #510 account-verification flow, then retries
  a never-seen withdrawal key.
- A never-seen key rejected before `RequestWithdrawal` creates no
  idempotency/withdrawal fact.
- The existing transactional operation lookup inside `RequestWithdrawal`
  remains required after the preflight to close concurrent first-write races.
- Current v1 personal KYC has no revocation transition. If a future contract
  introduces revocation/expiry, this admission contract must be reviewed rather
  than silently inferring revocation semantics.
- No cross-database atomicity is claimed. This is an admission read followed by
  the existing referral mutation; KYC is not copied into the withdrawal record.

## 8. Referral rules contract and staged rollout

The rules page must never claim that personal KYC is an active requirement
before the runtime enforces it. The inverse does **not** require an immediate
schema cutover: once the runtime enforces KYC, the existing strict v1 rules
response may remain temporarily authoritative because its withdrawal copy
already states that additional eligibility conditions are checked and does not
claim that the minimum amount is sufficient.

The user approved this rollout amendment on 2026-09-27. It supersedes only the
prior deployment ordering that coupled server enforcement and the v2 rules
schema. Fact owners, withdrawal admission, replay ordering, fail-closed
semantics, cancellation asymmetry and privacy invariants remain unchanged.

The rollout now has **three logical stages**.

### Stage A — compatibility client, rules v1

PR #517 is merged. Its client accepts a strict union of:

- existing `referral-rules-v1`; and
- future `referral-rules-v2` with required
  `personalKycRequired: true`.

The UI renders the personal-KYC requirement only for v2. For v1 it retains the
generic eligibility wording. Stage A merge status is sufficient for repository
compatibility work; no deployment evidence is required to implement or merge
Stage B because Stage B does not change the rules schema.

### Stage B — enforce personal KYC, keep rules v1

#519 activates personal-KYC admission unconditionally for every new withdrawal
while the authoritative rules endpoint continues to return strict
`referral-rules-v1`. The temporary rollout boolean is removed rather than left
as a silent fallback to pre-KYC withdrawal semantics.

This ordering is safe for both an old strict-v1 client and the merged dual-schema
client because the server response shape is unchanged. The runtime becomes the
source of truth for withdrawal eligibility, while the rules page remains
truthful but intentionally generic until Stage C.

Stage B must therefore:

1. enable the server-side new-withdrawal personal-KYC gate;
2. keep the rules endpoint on strict `referral-rules-v1`;
3. keep all existing v1 rule fields and wording semantics unchanged;
4. preserve the merged dual-reader client for future v2 cutover.

No deployment is authorized by this contract update. Repository merge and
production rollout remain separate decisions.

### Stage C — publish authoritative rules v2 after compatible UI deployment

Only after a compatible dual-schema UI has actually been deployed and
independently confirmed may a later release switch the rules endpoint to strict
`referral-rules-v2` and add:

```json
{
  "schemaVersion": "referral-rules-v2",
  "personalKycRequired": true
}
```

All existing rule fields remain unchanged.

This later schema cutover is not required to make the KYC gate real. It only
makes the already-enforced requirement explicit in the rules projection.

The prohibited states are:

- UI explicitly says KYC is required while runtime does not enforce it;
- rules v2 is returned to a deployed client that cannot parse/report v2;
- Stage B disables or weakens the KYC gate merely to preserve rules v1.

The intentionally permitted intermediate state is:

- runtime enforces personal KYC;
- rules endpoint remains strict v1;
- UI continues generic additional-eligibility wording.

## 9. TDD and verification matrix

Implementation starts with failing tests for the actual admission boundary.

Required tests:

- exact committed operation + same subject/fingerprint replays successfully even
  when profile/KYC/payout dependencies are unavailable;
- same idempotency key with a different subject or request payload returns
  conflict and never leaks another subject's withdrawal;
- a never-seen key still requires verified email/phone + personal KYC VERIFIED +
  active payout method before reaching `RequestWithdrawal`;
- KYC absent / pending / rejected / unknown / stale scope → 409 and economics
  mutation spy is not called for a never-seen key;
- KYC store/config unavailable → 503 and economics mutation spy is not called
  for a never-seen key;
- KYC for another subject cannot satisfy current subject;
- provider adapter is never invoked by the KYC eligibility read;
- cancellation of an existing withdrawal remains possible without a KYC read;
- payout-method creation remains unchanged;
- concurrent first requests remain duplicate-safe because
  `RequestWithdrawal` keeps its transactional operation lookup;
- Stage A frontend contract accepts strict v1 and strict v2, renders no KYC claim
  for v1, and renders the KYC requirement for v2;
- Stage B production composition makes personal-KYC admission mandatory for new
  withdrawals while the rules endpoint intentionally remains strict v1;
- Stage C rules v2 requires `personalKycRequired: true` after compatible UI
  deployment evidence exists;
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

Architecture review round 1 identified two real issues: an unsafe strict v1→v2
single-release rollout (BLOCKER) and loss of committed-withdrawal replay when KYC
is unavailable (IMPLEMENTATION_TEST). Both were incorporated into this contract
at `2074316a32bb177059dda49d88427cbfda8bece7`.

Architecture review round 2 found no new blocker. This document is now the frozen
`IMPLEMENTATION_READY` baseline for #519.

Implementation may proceed by TDD on the same delivery PR. Architecture is only
reopened by a newly demonstrated blocker in the frozen owner, identity,
transaction, replay, rollout or privacy boundaries. Stage A compatible-client deployment evidence is **not** a prerequisite for
Stage B because Stage B keeps the rules response on strict v1. Deployment
evidence remains a prerequisite only for the later Stage C v2 schema cutover.
