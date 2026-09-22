# Commercial Wallet / Billing Owner Contract

**Status:** IMPLEMENTATION_READY / DB1 owner contract  
**Date:** 2026-09-22  
**Issue:** #457  
**UI consumer:** #455  
**Figma authority:** `tg48P46SSXl6TBy9lZwg63`, page `31:463`

This contract fixes the backend ownership required by the visible Figma
`套餐与权益 / 充值中心` (`431:5166`) and `套餐与权益 / 账单与订单`
(`1839:766`). Figma remains the UI/IA authority; it is not a source of
prices, balances, payment outcomes, invoice facts, or resource grants.

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

## 5. Resource acquisition contract

`internal/ledger/orgresource` remains the only resource balance owner.

Commercial billing receives a dedicated purchased-resource grant use case. The
browser never receives a generic positive-credit API.

The grant identity is fixed to:

```text
source_type = commercial_order_item
source_identity = <canonical order/item identity>
```

The source claim is unique and replayable. Same source + same fingerprint returns
the immutable previous result. Same source + different fingerprint is a
conflict.

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
source_identity
commercial_order_id?
payment_id?
occurred_at
```

The initial entry kinds remain bounded to top-up credit, purchase reservation,
purchase commit/release, refund reversal, chargeback reversal, and debt
repayment. Tenant/browser callers never receive a generic money adjustment
endpoint.

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
