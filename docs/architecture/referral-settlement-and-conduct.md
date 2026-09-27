# Referral settlement period and promotion conduct

Status: IMPLEMENTATION_READY. Execution: #469. Product decision: 2026-09-27.
Design Basis: Independent Architecture (changes the timing of money availability).

Independent boundary review completed on 2026-09-27 by
`referral_boundary_review` against base `4742ad00f2f21ab0c99878cc804e112bf2061d05`.
No BLOCKER. The bounded period/replay/contract tests below are
IMPLEMENTATION_TEST requirements before the final delivery checkpoint.

## Product authority and scope

The user requested a 30-day settlement period and selected the bounded conduct
scope: prohibit misleading promotion, official impersonation, fabricated
registrations/orders, harassment and infringement; explain existing refund and
chargeback adjustments without adding freezing, fines or suspension machinery.
This supersedes only the previous 14-day referral period and unavailable conduct
copy in #469 and final UI authority §3.2. Commission remains 10%, the withdrawal
minimum remains CNY 100, and manual review and personal KYC admission are unchanged.

UI authority: Figma `tg48P46SSXl6TBy9lZwg63`, page `31:463`, node `1821:827`:
retain the current-rule notice and six cards, with a single column on narrow
screens. Product text may not turn Figma example sanctions into runtime facts.
Related authority: [final UI](../product/final-ui-ia-authority.md),
[KYC rollout §8](referral-withdrawal-personal-kyc-contract.md#8-referral-rules-contract-and-staged-rollout),
and [greenfield baseline](../product/greenfield-no-legacy-migration.md).

Must: backend availability and the displayed current period are 30 days; replay
does not duplicate or reschedule an existing claim; conduct copy is explicit and
truthful. Out of scope: new sanctions, moderation/appeal workflows, new tables,
policy engines, commission or refund changes, rules-v2 rollout, production data,
deployment, merge and Issue closure. Automated abuse enforcement is not claimed.

## Owners and current path

- Canonical payment/refund/chargeback facts: existing money owner.
- Commission claims, immutable ledger, projections and withdrawals: existing
  `referraleconomics` domain and referral persistence repository.
- Settlement command: canonical payment observer → `RecordSettledPayment` → one
  existing database transaction creating the claim, ledger, projection and audit.
- Maturity: existing application startup/ticker → `Repository.Mature(at)` → the
  existing transaction moves due remaining commission from pending to available.
- Display: rules page → `getReferralRules` → `/api/account/referral-rules` →
  authenticated account API → economics constants. No new injection or consumer.
- Conduct: the approved product copy in this document, presented within the sixth
  rules card. It defines permissible promotion, not persisted enforcement facts.

## Time and persistence contract

For a newly recorded claim, `available_at = canonical SettledAt.UTC() + 30 days`.
This is an elapsed 30-day period from the confirmed payment settlement timestamp,
not registration, claim processing time, next month or month end. The existing UTC
`AddDate(0, 0, 30)` calculation is retained. Before that timestamp the claim remains
pending; at or after it, the existing sweep may mature the remaining commission.
Actual processing can follow the deadline by the normal sweep interval.

`available_at` is assigned once at initial claim creation. A successful replay
reuses that durable claim and must not recompute or overwrite its maturity time.
Currently the replay comparison re-derives the timestamp from the mutable current
period, incorrectly coupling replay identity to policy. Remove only that derived
timestamp comparison. Preserve the canonical payment validation and comparisons
of referrer, issuer, payer subject, currency, commission and commissionable cash.
Canonical payment matching already validates settlement time and payment fields.
Changed payment payloads still conflict or fail canonical validation.

No backfill, migration, dual-period branch, legacy read, customer-data access or
rewriting of existing financial facts is introduced. This is the current owner's
ordinary durable replay invariant, not support for a retired model. Existing
claims keep their stored deadline; the current period applies at new claim creation.

Transactions, keys, concurrency and rollback remain unchanged. The payment ID
deduplicates claims; response loss/restart/retry reads the same committed record.
The maturity state predicate and existing row/projection locks prevent a second
pending-to-available movement. Partial/full refunds and chargebacks keep the
existing adjustment behavior before and after maturity. No new external effects.

## UI and API contract

Keep strict `referral-rules-v1` and its existing fields; only its numeric
`settlementPeriodDays` value changes to 30. Do not add conduct fields or activate
v2: the strict-reader rollout agreement in #519 remains in force. The frontend
continues to render the period supplied by the API, rather than hardcoding 30.
The rule notice distinguishes dynamic economics values from product conduct text.

The settlement card explains the start point for newly generated claims, that
existing claims keep their recorded deadline, and refund/chargeback adjustments.
The conduct card uses the following bounded content:

> 请真实、准确地介绍平台服务，不得虚构功能、价格、优惠或收益承诺，也不得冒充平台官方。
> 禁止批量虚假注册、刷单或伪造交易以获取推广收益；不得发送垃圾信息、反复骚扰他人，或未经授权使用他人品牌、内容及个人信息。
> 退款或拒付产生的收益按现有规则调整；提现申请仍需通过现有资格校验和人工审核。本页不承诺自动冻结、罚款或封禁。

Do not infer an executable penalty from these prohibitions, retroactively deduct
unrelated earnings, auto-reject withdrawals for a new reason, or invent a support
or appeals channel. KYC v1 generic eligibility wording remains unchanged.

## Security, failure and verification

The relevant threats are early/duplicate commission availability, conflicting
payment replay and misleading financial/punishment claims. Existing subject and
tenant checks, money/referral database owners, size/deadline limits and access to
review/withdrawal commands are unchanged. No new permissions or accepted security
risks are added. Abuse detection itself is outside this change.

TDD must first demonstrate failures for the target period and replay coupling.
Reuse the existing GORM/SQLite test pattern and dependencies in a test-local
referral fixture, plus applicable existing tests:

- API returns literal 30 days without changing the strict v1 shape.
- A new canonical claim is pending at day 14 and just before day 30; it matures at
  day 30, and repeated maturity/replayed payment does not credit twice.
- A committed claim with a stored deadline is replayed without recalculation,
  ledger/projection/audit duplication or rewriting that deadline; changed payment
  or attribution remains rejected.
- Existing refund/chargeback and withdrawal tests retain their applicable behavior.
- UI shows the API-supplied period, six cards and the approved conduct text without
  unsupported sanction promises. Existing typecheck/lint/build/CI are sufficient;
  no new acceptance runner or full failure platform.

Legacy decision: N/A; only current owners are changed. Independent review must
classify findings against the Must requirements above before implementation.
