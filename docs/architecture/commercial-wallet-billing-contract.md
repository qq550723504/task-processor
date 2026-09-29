# Commercial Wallet / Billing Owner Contract

> 2026-09-29 用户采用 [统一基础方案与预付资源 V1](unified-base-prepaid-resources-v1.md)。
> 本文的报价、RESOURCE_PURCHASE、钱包/资源 owner 与安全合同继续复用；
> subscription/entitlement 产品选择、旧订阅商业 overview 与 Token/store_count 额度由新决定替代。
> 新设计准入和实际切换状态见 #478，不把历史订阅证据当作新模型验收。

**Status:** IMPLEMENTATION_READY / DB1 owner contract  
**Date:** 2026-09-22  
**Issue:** #457  
**UI consumer:** #455  
**Figma authority:** `tg48P46SSXl6TBy9lZwg63`, page `31:463`

This contract fixes the backend ownership required by the visible Figma
`套餐与权益 / 充值中心` (`431:5166`) and `套餐与权益 / 账单与订单`
(`1839:766`). Figma remains the UI/IA authority; it is not a source of
prices, balances, payment outcomes, invoice facts, or resource grants.

Safety boundaries: Figma example prices never seed production; the browser
never receives a generic positive-credit API; Invoice creation is deliberately
not authorized by this contract.

## 1. Current facts

The repository already has three relevant fact families:

1. `internal/listingsubscription` is the current plan / entitlement / usage
   owner used by `GET /api/v1/workbench/commercial/overview`. It is an
   EXTRACT target for `internal/commercial/*`, not a wallet landing zone.
2. `internal/ledger/orgresource` owns organization resources:
   `store_renewal_period`, `ai_point`, and `data_row`. It explicitly does
   not own RMB.
3. `internal/ledger/money` is already the canonical money owner for accepted
   payment/refund/chargeback facts and payout methods. The account/referral
   delivery batch introduced this owner after the original #347 commercial
   read contract was written. It does not yet own an organization wallet.

Therefore the missing capability is not solved by extending
`CommercialOverview` with invented money fields. It requires an
organization-money projection plus a commercial purchase owner.

## 2. Ownership decision

```text
internal/ledger/money
  owns money
  ├─ accepted settlement facts
  ├─ organization wallet bucket
  ├─ immutable wallet entries
  ├─ wallet reservation / commit / release
  └─ refund / chargeback monetary reversal

internal/commercial/billing
  owns commercial purchase meaning
  ├─ sellable offers
  ├─ authoritative quotes
  ├─ commercial orders / order items
  ├─ order finality / recovery
  └─ orchestration between money and resource owners

internal/ledger/orgresource
  owns platform resource value
  ├─ store_renewal_period
  ├─ ai_point
  └─ data_row

internal/listingsubscription
  retains current plan / entitlement / usage facts during extraction
  and MUST NOT become a wallet/payment/order owner.
```

No table, DTO, BFF, or UI page may become a second fact owner.

## 3. Money contract

The approved #481 [dual-channel top-up design](alipay-wallet-topup-design.md)
extends this contract for provider wallet top-ups. The frozen policy is exact
CNY principal credit (payment gross equals wallet credit), debt repayment first,
platform-paid channel fees, no bonus/discount and no referral commission.
Unattributed external payers are permitted; the original order fixes the
beneficiary Organization. Existing non-top-up commission semantics are unchanged.

Provider top-ups require atomic accepted facts, channel-scoped claims and an
immutable exact posting receipt. The mutable wallet snapshot is not completion
proof. Refund/chargeback facts retain their full amount R; the shared wallet
principal effect W is bounded by the original credit, and excess E is recorded
for reconciliation without adding wallet debt (R = W + E). Typed reversal
identity is (payment_id, reversal_kind, reversal_id), including hold and readback.
Refund reservations are distinct from purchase reservations. Confirmed refunds
atomically settle their hold and receipt, even when intervening reversals exhaust
principal; unused hold value repays debt before becoming available. New refund
admission remains bounded by confirmed reversals plus outstanding refund holds.

Only a currently verified platform administrator can approve or first dispatch a
refund. Its organization-neutral route obtains the beneficiary from the original
order. Recovery can reconcile the original admitted effect after revocation.
Verified late payments are posted to the original Organization through the
dedicated top-up correction; cancelled subscription orders are never revived.
These additions do not authorize real channel operations or enable a channel
without required merchant, amount, credential and callback configuration.

### 3.1 Currency and precision

The first wallet capability is CNY only.

```text
domain amount: signed/unsigned int64 minor units as appropriate
wire amount: decimal string
currency: CNY
```

JavaScript `Number`, Go `float32/float64`, formatted Figma strings, estimated
usage cost, and implicit FX conversion are forbidden money authorities.

### 3.2 Wallet identity

A wallet bucket is uniquely identified by:

```text
(organization_id, currency)
```

The authoritative snapshot contains at least:

```text
available_minor >= 0
reserved_minor  >= 0
debt_minor      >= 0
version
updated_at
```

`available_minor` is spendable value. `reserved_minor` is value fenced for a
durable commercial attempt. `debt_minor` represents an accepted external
reversal that exceeded current available value; it is not a negative browser
balance.

Every change has an immutable wallet entry and an immutable operation result.
A mutable bucket alone is never sufficient evidence.

### 3.3 Credit and reversal

A browser cannot mint wallet value.

A top-up credit requires a trusted organization top-up settlement binding:

```text
payment_id
commercial_order_id
organization_id
currency
amount_minor
settled_at
provider_reference/version
```

The binding cannot be inferred from the payer's current Organization after the
payment. The commercial order establishes the beneficiary Organization and the
money owner establishes the accepted settlement fact.

A refund or chargeback is applied exactly once against the original settlement
binding. If spend has already consumed the credited value, the reversal consumes
available first and records the remainder as wallet debt. Later credits repay
wallet debt before increasing available. An external reversal is never rejected
merely because the browser-visible balance is too small.

The wallet invariant is fail-closed: a snapshot with positive debt has zero
available balance. New credits repay debt before restoring spendable funds.

### 3.4 Wallet purchase reservation

Commercial purchase uses a durable money reservation:

```text
reserve(order) -> reserved
commit(order)  -> spent
release(order) -> available
```

The money owner does not decide whether a resource grant succeeded. It accepts a
commit/release command only from the registered commercial owner contract,
bound to the same reservation/order identity.
The fulfilled order must retain both the reservation identity and the committed
reservation state; a reservation that is merely `RESERVED` or `RELEASED` is not
money-commit proof.

Timeout is not a business result. A lost acknowledgement must be resolved by
authoritative replay/readback before the caller decides whether another attempt
is safe.

## 4. Commercial billing contract

### 4.1 Product mapping

The first sellable resource products are:

| Product kind | Canonical resource owner value |
| --- | --- |
| `STORE_RENEWAL_PERIOD` | `orgresource.store_renewal_period` |
| `AI_POINT` | `orgresource.ai_point` |
| `DATA_ROW` | `orgresource.data_row` |

The table defines semantic mapping only. It does **not** define price.

### 4.2 Offer

A sellable offer is server-owned and contains:

```text
offer_id
product_kind
resource_type
currency
pricing_version
min_quantity
max_quantity
status
validity window
```

There is no default RMB price in this contract. No active configured offer
means `OFFER_UNAVAILABLE`; the UI remains capability-gated. Figma example
prices never seed production.

### 4.3 Quote

A quote is a server-authoritative immutable price snapshot:

```text
quote_id
organization_id
offer_id
product_kind
resource_type
resource_quantity
currency
total_minor
pricing_version
expires_at
fingerprint
```

The browser may request a quantity but cannot submit its own authoritative
`total_minor`. Order creation references an unexpired quote.

### 4.4 Order

Initial kinds:

```text
WALLET_TOP_UP
RESOURCE_PURCHASE
```

Initial resource purchase states:

```text
PENDING
  -> FUNDS_RESERVED
  -> FULFILLING
  -> FULFILLED

PENDING / FUNDS_RESERVED
  -> CANCELLED

FUNDS_RESERVED / FULFILLING
  -> RECONCILIATION_REQUIRED
```

`RECONCILIATION_REQUIRED` means an effect outcome is unknown. It does not mean
failure and does not authorize automatic re-charge.

A fulfilled resource order must retain proof of both the canonical money
reservation commit and the canonical source-bound resource grant.
`RECONCILIATION_REQUIRED` may retain a reserved wallet reservation while the
grant outcome is uncertain, but once its reservation is `COMMITTED`, the order
must also retain the complete canonical source-bound grant proof. A committed
charge without that proof is invalid and cannot be accepted as a recoverable
order state.
The order's durable grant proof carries the grant operation ID, canonical
`commercial_order_item` source type, and a source identity that exactly matches
the deterministic identity for the order's organization, order, and order item
as computed by the orgresource owner.
Cancellation is pre-grant only: a cancelled resource order must not carry any
grant or wallet-reservation evidence, because releasing its reservation after a
successful commit or grant would desynchronize wallet and resource accounting.
The `PENDING` and `FUNDS_RESERVED` states are also pre-grant: they must not carry
grant evidence, so cancellation or reservation release cannot race with an
already-recorded resource mint. Grant evidence is retained only in lifecycle
states that can represent an in-progress or completed/uncertain grant.

Wallet top-up orders do not use the resource-purchase lifecycle. A fulfilled
top-up must carry an accepted provider payment reference; without that proof it
is invalid. Top-up intent remains pending or explicitly cancelled until the
provider settlement binding is accepted. Top-up orders never carry a resource
quote, purchase items, wallet reservation evidence, or resource grant evidence.
Resource purchases never carry provider payment evidence. A
cancelled top-up must not carry payment evidence, because a settled payment
cannot be released as if it were never credited. Non-fulfilled top-up states
must not carry payment evidence.

## 5. Resource acquisition contract

`internal/ledger/orgresource` remains the only resource balance owner.

Commercial billing receives a dedicated purchased-resource grant use case. The
browser never receives a generic positive-credit API.

The grant identity is fixed to:

```text
source_type = commercial_order_item
source_identity = <canonical organization/order/item identity>
```

The source claim is unique and replayable. Same source + same fingerprint returns
the immutable previous result. Same source + different fingerprint is a
conflict.

The durable source-claim key is `(source_type, source_identity)` across all
resource types. `resource_type` is part of the immutable result and request
fingerprint, but never widens the source deduplication scope; one commercial
order item cannot mint two different resource types.

The persisted source identity is a fixed-size SHA-256 digest (with the
commercial source type prefix) over an unambiguous length-prefixed encoding of
the canonical Organization ID, order ID, and order-item ID. Delimiter
concatenation is not a uniqueness contract, and the resulting identity remains
within the resource owner's 192-byte persistence limit. Leading or trailing
whitespace in these source-binding identifiers is rejected before billing
derives or persists the identity.

The purchased grant supports only the three approved resource types and a
positive bounded quantity. Authorization is supplied by runtime assembly using
the trusted commercial principal; a tenant role string supplied by HTTP is not
proof of grant authority.

## 6. Purchase protocol

The first implementation uses a recoverable reservation protocol instead of
"debit first, hope grant succeeds":

```text
1. create/replay canonical commercial order
2. reserve wallet funds in money owner
3. mark order FUNDS_RESERVED
4. request source-bound purchased grant from orgresource owner
5. on terminal grant success, commit wallet reservation
6. mark order FULFILLED
```

Terminal pre-effect business rejection releases the money reservation and
cancels the order. Any unknown resource or money commit outcome keeps durable
state for owner reconciliation.

The order lifecycle fences the reservation state: `FUNDS_RESERVED` and
`FULFILLING` require `RESERVED`, `FULFILLED` requires `COMMITTED`, and
`RECONCILIATION_REQUIRED` permits only `RESERVED` or `COMMITTED`. `PENDING`
and `CANCELLED` carry no reservation evidence. Resource-binding identifiers
(Organization, order, and order item) are canonical and at most 128 bytes,
matching the purchased-grant owner contract.

Every step is idempotent and uses a stable operation identity. The UI does not
invent a new idempotency key after a timeout.

## 7. Organization and permission contract

All Workbench routes derive the target from verified
`EffectiveOrganizationID`. The browser cannot select an arbitrary organization
through body/query headers beyond the existing verified Organization selector.

New permissions:

```text
workbench.commercial.read
workbench.commercial.purchase
workbench.commercial.wallet_topup
```

Initial policy:

| Role | read | purchase | wallet top-up |
| --- | ---: | ---: | ---: |
| `listingkit_viewer` | deny | deny | deny |
| `listingkit_operator` | allow | deny | deny |
| `listingkit_admin` | allow | allow | allow |
| `platform_admin` | allow | allow | allow |

A process/global `admin` role does not by itself establish an effective
Organization. Financial reads and writes require fresh/live Organization grant
validation. Authorization and Organization binding are independent gates.

## 8. HTTP projection

The current plan/entitlement/usage route remains unchanged:

```http
GET /api/v1/workbench/commercial/overview
```

New target routes:

```http
GET  /api/v1/workbench/commercial/wallet
GET  /api/v1/workbench/commercial/wallet/entries
POST /api/v1/workbench/commercial/wallet/top-up-intents

POST /api/v1/workbench/commercial/quotes
POST /api/v1/workbench/commercial/orders
GET  /api/v1/workbench/commercial/orders
GET  /api/v1/workbench/commercial/orders/:order_id
```

Invoice creation is deliberately not authorized by this contract. Figma's
invoice button stays unavailable until a tax/invoice owner exists. Export may be
added as a read projection without creating a new financial fact.

### 8.1 Wallet read DTO

```json
{
  "organization_id": "org-id",
  "currency": "CNY",
  "available_minor": "0",
  "reserved_minor": "0",
  "debt_minor": "0",
  "lifetime_topup_minor": "0",
  "lifetime_spend_minor": "0",
  "version": "1",
  "observed_at": "2026-09-22T00:00:00Z"
}
```

Wallet entries use cursor pagination because an immutable ledger must not
pretend offset pagination is a stable history cursor.

### 8.2 Order list DTO

At minimum:

```text
order_id
kind
product_kind
description
amount_minor
currency
status
created_at
updated_at
```

The Figma 30-day summary is aggregated from canonical commercial orders. Usage
metering or estimated model cost never creates a bill.

### 8.3 Enterprise resource balances (2026-09-28 increment)

**Design Basis: Independent Architecture; admission: IMPLEMENTATION_READY.**

Bounded independent review on 2026-09-28 by `payment_architecture_review`
admitted this increment against main `e0d73b5cd9aad35c43f09c534d1516e04dea81db`:
no BLOCKER. Independent commercial-owner injection, one-statement missing/zero/
debt semantics, strict BFF validation and scope isolation are IMPLEMENTATION_TEST
items to satisfy before merge. This admission is not product acceptance.
This increment is limited to the new read contract below. The existing wallet,
purchase, subscription, member-limit and recovery contracts remain frozen.

Product authority is the user's 2026-09-28 decision to finish Account Center
and Plans / Entitlements first (Issue #478, acceptance #438/#473). Figma
`tg48P46SSXl6TBy9lZwg63 / 431:4158` (read live on 2026-09-28) separates the
enterprise resource pool from member allocation. Its sample balances are not
business facts. Member AI-point monthly limits retain the approved contract
in `docs/engineering/2026-09-25-issue487-main-image-execution-design.md` §9.5.

#### Outcome, scope and owner

An authorized user can read the same actual enterprise AI-point / data-row
balances on My Entitlements and Account / Organization / Resources. The cards
also identify renewal-period resources separately from actual active shops.
The existing `internal/ledger/orgresource` owns these facts; the existing
bucket/debt tables in the commercial database remain the only balance source.
Subscription limits, member monthly caps and money-wallet CNY balances are
different facts and must never be relabeled or added together.

Scope is a bounded GET projection, its explicit app injection, strict BFF/client
validation, shared cards and correction of contradicted static capability/unit
text in those consumers. Existing purchase/order/activation paths are reused.
No schema, command, grant, price, resource purchase flow, callback, provider
call, new balance, shop counter, data-service consumer, migration, retry or
recovery mechanism is added. Missing prices/capabilities remain unavailable.
Legacy decision: N/A; no retired owner is consumed.

#### Contract and call path

```http
GET /api/v1/workbench/commercial/resources
```

App-owned HTTP composition delegates to an orgresource read port implemented
by `internal/integration/orgresource`. It uses the already-installed commercial
owner connection and existing privileges. It does not use the subscription
reader connection or extend that reader's privilege allowlist.

```text
current user + live Effective Organization + commercial read permission
→ fixed-origin same-origin BFF /api/workbench/commercial/resources
→ app HTTP module / current application explicit registration
→ orgresource balance read port → existing bucket/debt tables
→ typed shared cards on Plans/Entitlements and Account/Resources
```

The route uses the same `PermissionWorkbenchCommercialRead`,
`AuthPolicyCurrentIdentity` and `OrganizationAccessPolicyLiveWrite` admission
as existing wallet/order reads. Roles and identity come only from the current
trusted context. The handler requires a bound actor, and TenantID must equal
EffectiveOrganizationID. The organization is not supplied in body/path/query.
Reject query strings and unread GET bodies. A viewer gains no new permission;
operator/admin permissions retain the existing commercial matrix. Revoked,
unavailable or suspended live grants fail closed before repository access.

The response is a strict envelope with `schema_version` equal to
`organization-resource-balances-v1`, `organization_id`, `observed_at` and
`resources`. The array contains exactly one entry for each of
`store_renewal_period`, `ai_point`, and `data_row`; no unknown/duplicate/missing
type is accepted. Each entry has `resource_type`, `unit` (`period`, `point`,
`row` respectively), `state` (`recorded` or `not_recorded`), `available`,
`reserved`, `consumed`, `debt` and `updated_at`.

For a recorded bucket, all quantities are canonical nonnegative int64 decimal
strings; an absent debt row means recorded debt `0`. Debt > 0 requires
available = 0, as in the existing debt-first owner. An absent bucket is
`not_recorded` with all quantities and updated_at null; it is not an observed
zero balance. A debt without its bucket, negative/inconsistent facts, invalid
timestamps, missing tables or a database error produce dependency unavailable,
not a successful empty response. A recorded zero remains visibly zero.

#### Read consistency and failure boundaries

Read all three buckets and their debts in one bounded SQL statement, with
every table join bound to the exact organization. This observes one database
statement snapshot without taking write locks or repairing/creating records.
Concurrent grants/reservation/finalization can produce the before or after
snapshot, never a mixed bucket/debt snapshot. No snapshot is claimed across
the independent subscription, money and resource reads.

GET has no business side effects. Failed/cancelled/time-limited reads neither
change balances nor create resource business operations/events/audits; existing
authentication/authorization denial auditing remains enabled. Retry is only another
read. Resource UNKNOWN reservations remain represented by the canonical
reserved amount and are not released by reading or changing month.

The existing 15-second bounded route/client deadlines and no-store transport
apply. Successful resource JSON is capped at 16 KiB; errors at 8 KiB. The BFF
checks exactly one expected-user/organization header against the actual session
and current organization cookie, uses only the fixed backend origin, rejects
redirects and duplicate/malformed JSON, and validates the returned organization
and the complete resource schema. Identifiers remain bounded to current rules.

React queries are scoped to actor, organization and roles; selection/loading/
switching invalidates the visible prior scope. Late A replies cannot render in
B. Resource-read errors/forbidden/missing state do not become zero and do not
hide unrelated member/entitlement sections. Refresh obtains new owner facts.
The shared resource cards label AI points and data rows explicitly; renewal
periods never become a shop-use count. Existing Token allocation is labeled
Token, never AI points. A read page links to the existing wallet and member
management pages without promising that payment/resource purchase is enabled.

#### Verification and delivery

Use focused TDD for exact int64 including > JS safe integers, recorded zero vs
absent bucket, debts, A/B isolation, current grant/role denial, strict DTO/BFF
binding, late replies and the shared consumers. An owner write followed by
this reader must reflect the resulting persisted snapshot without adding read
operations/audit/events. Reuse existing PostgreSQL facilities for the new
statement and commercial role where available; no new runner/fault platform.
Existing purchase/activation, member limits and wallet regressions are reused
and run only where the changed consumption creates combination risk.

One writer, branch and main PR under #478; a bounded independent design review
is required before production implementation, followed by focused development
checks, required CI and final independent diff/call-path review. Developer
checks are not product acceptance. Runtime deployment, real provider/payment,
shared-data changes, merge and Issue closure remain separately authorized.

## 9. BFF contract

Next BFF follows the existing Workbench transport:

- fixed server-side upstream origin;
- `Cache-Control: no-store`;
- bounded strict JSON;
- expected Organization validation;
- no arbitrary upstream URL/header forwarding;
- cancellation and late-response isolation across Organization/user/role
  changes;
- no previous-Organization wallet/order placeholder or persistent browser cache.

## 10. Persistence ownership

Greenfield only. No old billing/quota migration, compatibility table, dual
write, fallback, or backfill is authorized.

Preferred homes:

```text
internal/integration/persistence/money
  organization wallet bucket
  immutable wallet entry
  wallet operation/reservation/source claim

internal/integration/persistence/commercial
  offer
  quote snapshot where persistence is required
  order / order item
  order operation / recovery state
```

Commercial persistence must not calculate wallet balance. Money persistence
must not become the commercial catalog/order owner.

## 11. Idempotency, concurrency, and unknown outcomes

All money/order writes require `Idempotency-Key`.

The server calculates a canonical request fingerprint. Same key + same
fingerprint replays. Same key + different fingerprint is conflict.

Implementations must use bounded transactions/locks and overflow checks.
A transport deadline is not a rollback proof. After any possibly-committed
error, the owner reads the durable operation before classifying the result.

The caller may retry only the same operation identity when the owner contract
allows replay.

### 11.1 Durable wallet and order identity

Every immutable wallet entry must retain enough identity to explain the money
change and prove its Organization binding:

```text
entry_id
organization_id
currency
entry_kind
amount_delta_minor
available_after_minor
reserved_after_minor
debt_after_minor
source_identity (nonempty and canonical)
commercial_order_id?
payment_id?
occurred_at
```

The initial entry kinds remain bounded to top-up credit, purchase reservation,
purchase commit/release, refund reversal, chargeback reversal, and debt
repayment; unknown kinds and entries missing common identity/balance fields are
invalid. The after snapshot must preserve the debt-first invariant: a positive
`debt_after_minor` requires zero `available_after_minor`. Top-up,
refund/chargeback reversal, and debt-repayment entries must carry `payment_id`.
A debt-repayment entry must also carry the originating top-up's canonical
`commercial_order_id`, including when the accepted settlement is fully absorbed
by existing debt and no available-balance credit remains. Top-up credits and
refund/chargeback reversals must also carry the original canonical
`commercial_order_id` so every settlement-backed balance change remains bound to
its beneficiary order. Purchase reservation, commit, and release entries must
not carry `payment_id`; provider settlement evidence belongs only to top-up and
reversal ledger entries.
Entry deltas are kind-specific: top-up credits increase available balance;
refund and chargeback reversals decrease available balance and may increase
debt for the portion not covered by available funds; purchase reservation
moves an equal amount from available to reserved; purchase commit decreases
reserved; purchase release decreases reserved and applies the released amount
to outstanding debt first, exposing only any remainder as available; and
debt-repayment entries decrease debt against an accepted top-up. Reversed or
zero-effect deltas are invalid.
Purchase reservation, commit, and release entries must also carry the
canonical `commercial_order_id` binding.
Each after-balance minus its corresponding delta must yield a nonnegative prior
balance without integer overflow. The derived prior wallet snapshot must also
preserve the debt-first invariant: positive prior debt requires zero prior
available balance.
Tenant/browser callers never receive a generic money adjustment endpoint.

The durable commercial order and item identity must include the order's
Organization, kind, status, currency, immutable amount, quote reference,
idempotency key, request fingerprint, version, and timestamps. A
`commercial_order_item` binds a resource quantity and amount to the canonical
order; it is the only source identity accepted by the purchased-resource grant
contract.

### 11.2 Provider settlement binding

Provider integration is a separate adapter capability. When it exists, the
adapter must first verify external payment finality, submit the accepted
settlement to `internal/ledger/money`, and bind it to the canonical top-up
order. Wallet credit requires both the accepted settlement and that
Organization-scoped order binding. The Organization must never be inferred
from the payer's current membership after payment.

Refund and chargeback use the original payment/order/Organization binding and
are applied exactly once. With no provider adapter, top-up intent remains
`FEATURE_UNAVAILABLE`; no fake provider success is permitted.

### 11.3 Query and error semantics

Order and wallet lists use stable cursor pagination. Order filters are bounded
to cursor, limit, date range, kind, product kind, status, and server-defined
identifier/description search; they are not arbitrary SQL-like filters.

The public contract distinguishes at least:

```text
FORBIDDEN
FEATURE_UNAVAILABLE
OFFER_UNAVAILABLE
QUOTE_EXPIRED
INSUFFICIENT_FUNDS
IDEMPOTENCY_CONFLICT
NOT_FOUND
CONFLICT
RECONCILIATION_REQUIRED
DEPENDENCY_UNAVAILABLE
INVALID_REQUEST
```

Unavailable capability, empty balance, insufficient funds, and unknown write
outcome must not collapse into one generic error.

### 11.4 Implementation admission gate

Before #455 removes a wallet or order capability gate, the implementation must
demonstrate exact Organization isolation, minor-unit string money end to end,
reservation exact-once behavior, quote expiry, insufficient funds,
source-bound resource fulfillment, payment/refund/chargeback reversal,
idempotent replay/conflict, and unknown-outcome reconciliation. A fulfilled
order must prove resource fulfillment, and no irreversible debit may lack a
recovery path. Provider absence keeps top-up gated, and invoice remains gated
until a separate invoice owner exists.

## 12. Figma acceptance mapping

### Wallet / 充值中心 `431:5166`

- 钱包余额 -> money wallet snapshot.
- 累计充值 -> immutable accepted top-up credits.
- 累计支出 -> committed wallet purchase debits.
- 钱包流水 -> immutable wallet entries.
- 店铺服务 / AI 点数 / 数据资源 -> commercial offer + quote + order.
- 使用钱包余额 -> money reservation + source-bound orgresource grant.
- 充值钱包 -> top-up capability; unavailable until an approved provider
  adapter exists.

### 账单与订单 `1839:766`

- 30-day spend cards -> canonical order aggregation.
- search/date/type/status -> commercial order query.
- amount/status -> order immutable monetary snapshot and lifecycle.
- 查看 -> organization-scoped order detail.
- 导出 -> read-only projection if implemented.
- 发票 -> unavailable until a separate invoice owner is approved.

## 13. Non-goals

This contract does not:

- choose a payment provider;
- define actual CNY prices;
- implement tax/invoice rules;
- move current subscription/usage data during DB1;
- make referral economics a wallet owner;
- expose a generic resource mint;
- allow operators to spend Organization money;
- convert Figma examples into business data.

In particular, Figma example prices never seed production. Invoice creation is
deliberately not authorized by this contract.

## 14. Delivery sequence

```text
DB1 owner + code contracts
DB2 wallet persistence/read/immutable ledger
DB3 offers/quotes/orders + wallet reservation
DB4 purchased-resource grant + recovery
DB5 Workbench HTTP/BFF + #455 integration
Provider top-up adapter: separate capability when approved
```

DB1 is complete when the architecture owner decision, code-level contract types,
and permission names are present and CI accepts their package/dependency
placement. DB2–DB5 must preserve this ownership split rather than introducing a
facade owner.
