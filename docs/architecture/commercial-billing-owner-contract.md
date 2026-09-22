# Commercial Billing / Wallet / Resource Purchase Owner Contract

**Status:** Proposed architecture contract for Issue #457  
**Scope:** Figma `31:463`, especially `431:5166` (企业钱包 / 充值中心) and `1839:766` (账单与订单)  
**Repository baseline:** `ff1ceaee55736474dd54d00c67afa39662793f0b`

## 1. Purpose

The Console already has a real read chain for plan, subscription, entitlement, and usage through:

```http
GET /api/v1/workbench/commercial/overview
```

That endpoint remains a read projection over the current commercial owner. It is not a wallet, payment, or order authority.

The Figma wallet and billing screens require three distinct facts:

1. real monetary value;
2. commercial purchase/order semantics;
3. organization resource balances.

This contract fixes one canonical owner for each fact and defines the narrow cross-domain contracts needed to implement the Figma screens without mock balances, synthetic bills, or duplicated ledgers.

## 2. Authority and existing facts

The following existing architecture remains authoritative:

- `internal/listingsubscription` currently owns the implemented plan / entitlement / usage behavior, but target architecture requires reusable commercial behavior to move toward `internal/commercial/*`.
- `internal/ledger/orgresource` owns organization-scoped platform resources:
  - `store_renewal_period`
  - `ai_point`
  - `data_row`
- `internal/ledger/money` already owns canonical:
  - settled payment facts;
  - refund settlement facts;
  - chargeback settlement facts;
  - payout-method facts.
- `internal/ledger/orgresource` explicitly does not own money.
- usage metering is not money and must not be reused as a wallet ledger.
- Figma controls UI / IA / visible interaction semantics, not financial truth.

## 3. Canonical ownership

### 3.1 `internal/ledger/money` — monetary value owner

Owns:

- canonical payment settlement;
- canonical refund settlement;
- canonical chargeback settlement;
- organization wallet account keyed by `(organization_id, currency)`;
- immutable wallet entries;
- wallet credit, debit, reserve, commit, release, reversal;
- money-operation idempotency;
- amount precision and currency rules;
- debt caused by refund/chargeback after previously credited funds have already been spent;
- immutable operation/result snapshots used for replay/recovery.

Must not own:

- plan definitions;
- resource catalog/offer definitions;
- commercial price policy;
- commercial order lifecycle;
- AI points/data rows/store renewal balances;
- usage metering;
- invoice/tax policy.

### 3.2 `internal/commercial/billing` — commercial purchase owner

Owns:

- sellable commercial offer;
- price/version policy;
- server-authoritative quote;
- commercial order and order item;
- order lifecycle and finality;
- top-up order business meaning;
- resource-purchase order business meaning;
- query projections for order list/detail and period summaries;
- capability availability for customer-facing commercial actions;
- durable coordination between money and resource owners;
- reconciliation/recovery state when a cross-domain outcome is unknown.

Must not own:

- wallet balance;
- payment settlement truth;
- resource balance;
- usage metering;
- IAM/session truth;
- external provider SDKs.

### 3.3 `internal/ledger/orgresource` — platform resource owner

Continues to exclusively own:

- `store_renewal_period`;
- `ai_point`;
- `data_row`;
- immutable resource operations/events;
- resource reservation/consumption semantics already defined by that domain.

A commercial purchase may acquire resources only through a narrow source-bound grant contract. The source identity must bind to a canonical commercial order/item. No browser or ordinary tenant caller may access a generic positive-mint API.

## 4. Money representation

Initial supported currency:

```text
CNY
```

All canonical money values are integer minor units (fen).

Rules:

- Go domain/persistence may use bounded integer minor units.
- HTTP/JSON transmits monetary integers as decimal strings.
- Frontend must not use floating point for canonical monetary arithmetic.
- No price is inferred from Figma.
- No RMB amount is inferred from usage.
- No implicit currency conversion.
- AI model tokens are not AI points.

## 5. Organization wallet contract

Wallet key:

```text
(organization_id, currency)
```

Required projection fields:

```text
organization_id
currency
available_minor
reserved_minor
debt_minor
lifetime_topup_minor
lifetime_spend_minor
version
observed_at
```

Invariants:

- `available_minor >= 0`
- `reserved_minor >= 0`
- `debt_minor >= 0`
- a normal tenant purchase cannot overdraw available funds;
- every balance change has an immutable wallet entry;
- one external settlement/source identity cannot credit twice;
- one order reservation cannot commit twice;
- release/commit are terminal and idempotent for the same reservation identity.

### 5.1 Refund / chargeback after funds were spent

A valid external refund or chargeback cannot be rejected only because the credited funds were later spent.

Apply monetary loss as:

1. reduce available wallet funds first;
2. any remaining loss becomes `debt_minor`;
3. future top-up credit repays debt before increasing available funds.

This debt is a money-owner fact and is separate from any orgresource debt concept.

## 6. Wallet entries

Each immutable entry must contain enough identity to prove why money changed:

```text
entry_id
organization_id
currency
entry_type
amount_minor
available_after_minor
reserved_after_minor
debt_after_minor
source_type
source_id
order_id?
payment_id?
occurred_at
```

Initial entry types should be narrowly bounded, for example:

```text
TOP_UP_CREDIT
PURCHASE_RESERVE
PURCHASE_COMMIT
PURCHASE_RELEASE
REFUND_REVERSAL
CHARGEBACK_REVERSAL
DEBT_REPAYMENT
```

Do not expose a generic arbitrary adjustment endpoint to tenant/browser callers.

## 7. Offer and quote contract

Initial resource purchase targets:

```text
STORE_RENEWAL_PERIOD -> orgresource.store_renewal_period
AI_POINT             -> orgresource.ai_point
DATA_ROW             -> orgresource.data_row
```

An offer is server-owned and includes at least:

```text
offer_id
product_kind
resource_type
currency
pricing_version
minimum_quantity
maximum_quantity
status
valid_from?
valid_until?
```

This architecture contract does **not** approve concrete RMB prices.

If no approved active offer exists, quote/purchase returns an explicit unavailable state and the UI remains capability-gated.

A quote is an immutable pricing snapshot:

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

The browser supplies purchase intent (offer/quantity) but never supplies authoritative total price.

## 8. Commercial order contract

Initial order kinds:

```text
WALLET_TOP_UP
RESOURCE_PURCHASE
```

Resource-purchase lifecycle:

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

`RECONCILIATION_REQUIRED` means the authoritative outcome is unknown. A request timeout is not a business failure.

An order contains at least:

```text
order_id
organization_id
kind
status
currency
total_minor
quote_id?
created_by
idempotency_key
request_fingerprint
created_at
updated_at
version
```

An order item contains at least:

```text
order_item_id
order_id
product_kind
resource_type?
resource_quantity?
amount_minor
fulfillment_source_id?
```

## 9. Wallet-funded resource purchase

Use durable reservation; do not debit irreversibly before fulfillment is known.

Required coordination:

1. commercial owner creates/replays the order from a valid quote;
2. money owner reserves wallet funds;
3. commercial owner requests a source-bound purchased-resource grant from orgresource using canonical order/item identity;
4. confirmed resource grant success permits money reservation commit;
5. terminal resource business failure releases the reservation;
6. an unknown grant/commit result keeps funds reserved and order in `RECONCILIATION_REQUIRED`;
7. recovery reads/replays canonical owner operations; the UI must not create a second order/key automatically.

Every step is idempotent.

Same idempotency key + same canonical fingerprint => replay prior result.  
Same idempotency key + different fingerprint => conflict.

## 10. Top-up and payment settlement binding

External provider integration is an adapter concern and is not required merely to establish the wallet/order owner.

When a provider exists:

1. provider adapter verifies external payment finality;
2. verified settlement enters `internal/ledger/money`;
3. a canonical top-up order binds:
   - organization;
   - order;
   - payment settlement;
4. wallet credit requires both the accepted money settlement and the canonical top-up-order binding;
5. organization must never be inferred from a user's current browser organization at settlement time;
6. refund/chargeback uses the original payment/order/organization binding for reversal;
7. one payment settlement can satisfy only its approved binding.

The current referral-facing internal settlement ingress is not the public wallet API and must not become the commercial owner.

If no provider is configured, top-up intent returns a stable `FEATURE_UNAVAILABLE` state.

## 11. Read/API contract for the Figma screens

Keep:

```http
GET /api/v1/workbench/commercial/overview
```

It continues to serve plan / entitlement / usage projection.

Add target endpoints:

```http
GET  /api/v1/workbench/commercial/wallet
GET  /api/v1/workbench/commercial/wallet/entries

POST /api/v1/workbench/commercial/quotes
POST /api/v1/workbench/commercial/orders
GET  /api/v1/workbench/commercial/orders
GET  /api/v1/workbench/commercial/orders/:order_id

POST /api/v1/workbench/commercial/wallet/top-up-intents
```

Top-up-intent remains unavailable until a provider contract exists.

Potential later endpoints:

```http
GET  /api/v1/workbench/commercial/orders/export
POST /api/v1/workbench/commercial/invoices
```

Invoice remains capability-gated until a separate invoice/tax owner is approved.

### 11.1 Wallet screen mapping (`431:5166`)

Backend must be able to project:

- available wallet balance;
- cumulative credited/top-up amount;
- cumulative committed spend;
- immutable wallet entries;
- resource-purchase quote capability for:
  - store service renewal periods;
  - AI points;
  - data rows;
- top-up availability.

### 11.2 Billing/order screen mapping (`1839:766`)

Backend must be able to project:

- period spend summary;
- store-service purchase spend;
- AI-point purchase spend;
- data-row purchase spend;
- other explicitly defined commercial product spend;
- order search;
- date/type/status filters;
- amount/currency/status;
- order detail.

A bill/order row is produced from canonical commercial orders and money facts. It is never synthesized from estimated usage cost.

## 12. Query semantics

Order lists use stable cursor pagination.

Recommended filters:

```text
cursor
limit
created_from
created_to
kind
product_kind
status
query
```

`query` may match only bounded identifiers/descriptions allowed by the server. It must not become arbitrary SQL-like filtering.

Wallet ledger uses cursor pagination, ordered by a stable tuple such as `(occurred_at, entry_id)`.

## 13. Organization isolation and permissions

Browser APIs derive the target Organization from verified effective-organization context.

The browser cannot select another organization through request body/query arbitrary override.

Commercial money reads should use fresh/live authorization where the existing authorization model supports it; writes require live authorization.

Target permission semantics:

```text
workbench.commercial.read
workbench.commercial.purchase
workbench.commercial.wallet_topup
```

Role mappings must be explicit in the current authorization owner. A global/process role alone must not bypass organization binding.

## 14. Write and recovery semantics

For every money/order write:

- `Idempotency-Key` required;
- canonical server-side fingerprint;
- bounded quantities/amounts and overflow checks;
- row lock / serializable-equivalent invariants where required;
- timeout does not mean failure;
- committed-but-response-lost is replayable;
- an unknown cross-domain outcome retains a durable recoverable state;
- no UI retry may silently generate a new idempotency key for the same user intent.

## 15. Persistence ownership

Target adapters:

### `internal/integration/persistence/money`

Own persistence for:

- organization wallet projection/bucket;
- immutable wallet entries;
- wallet reservations;
- money operations;
- settlement/order source claims.

### `internal/integration/persistence/commercial`

Own persistence for:

- offers;
- quote snapshots where persistence is required;
- orders;
- order items;
- order operations;
- reconciliation/recovery state.

Persistence tables must not cross ownership:

- a commercial table is not a wallet balance;
- a money table is not the commercial product catalog/order model;
- an orgresource table is not a cash ledger.

Greenfield rules apply: no legacy payment/quota migration, no compatibility double-write, no fallback fact source.

## 16. BFF contract

Workbench BFF keeps the established transport constraints:

- fixed upstream origin;
- strict bounded JSON;
- `no-store`;
- expected Organization validation;
- abort and late-response isolation;
- no arbitrary upstream URL;
- no cross-organization cached wallet/order response.

Money integers remain decimal strings through the browser contract.

## 17. Error/state contract

At minimum distinguish:

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

Do not collapse unavailable, insufficient funds, empty wallet, no orders, or unknown write outcome into one generic failure.

## 18. Invoice boundary

Figma includes an invoice action. This contract does not create an invoice/tax owner.

Until a separate approved owner exists:

- invoice capability remains unavailable;
- no fake invoice number/file is created;
- no order is marked invoiced merely because the UI button was clicked.

## 19. Architecture non-goals

This contract does not:

- move all existing `listingsubscription` code;
- change existing plan/entitlement/usage semantics;
- implement a payment provider;
- approve product prices;
- introduce TigerBeetle;
- merge resource and money ledgers;
- create a generic positive-mint API;
- create a generic money-adjustment API;
- make Figma sample values production defaults.

## 20. Implementation admission / exit gate

Before #455 removes wallet/order capability gates, the implementation must prove:

- one canonical money owner;
- one canonical commercial order/quote owner;
- one canonical resource owner;
- exact Organization isolation;
- minor-unit money end-to-end;
- wallet reservation exact-once behavior;
- quote-expiry behavior;
- insufficient-funds behavior;
- purchased-resource grant bound to order/item identity;
- no fulfilled order without provable resource fulfillment;
- no irreversible debit without a recovery path;
- payment/refund/chargeback exact-once wallet effects;
- refund/chargeback debt behavior after funds are spent;
- idempotent replay and conflict tests;
- unknown-outcome reconciliation tests;
- provider absence keeps top-up capability gated;
- invoice stays gated without an invoice owner;
- PostgreSQL/integration/race/app HTTP/BFF tests as applicable;
- independent architecture/security review on final HEAD.

## 21. Recommended delivery sequence

1. owner contract + domain interfaces/types;
2. organization wallet read + immutable ledger;
3. quote/order owner + wallet reservation;
4. source-bound orgresource purchased grant + reconciliation;
5. Workbench HTTP/BFF read/write projection;
6. #455 wiring and removal of wallet/order capability gates only for capabilities proven real;
7. external payment provider/top-up integration separately when a provider is selected.
