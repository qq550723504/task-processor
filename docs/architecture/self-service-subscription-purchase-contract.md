# Self-Service Subscription Purchase Contract

Status: IMPLEMENTATION_READY candidate  
Refs: #478, #479, #473, #438, #457  
Baseline: `bdabab425a441634b4452d9c519ecc9b6209d62f`

## 1. Product outcome

The current Organization administrator can open **套餐与权益 → 套餐方案**, choose a server-owned sellable subscription offer, confirm a server-authoritative quote, create one canonical commercial order, and have the existing subscription/entitlement owner activate that plan for the same verified Effective Organization.

The browser never edits entitlement facts directly and never calls the platform-admin cross-tenant entitlement endpoint as the self-service path.

The target chain is:

```text
verified user + Effective Organization
  -> sellable subscription offer
  -> server quote
  -> canonical commercial subscription order
  -> ZERO_PRICE or WALLET settlement
  -> subscription activation owner
  -> subscription + exact entitlement set
  -> commercial overview / Account resources readback
```

External payment is not implemented by this slice. An offer requiring an external provider remains non-executable until a provider adapter is separately approved.

## 2. Ownership

### `internal/commercial/billing`

Owns:

- sellable subscription offer meaning;
- quote and immutable commercial price snapshot;
- commercial subscription order;
- settlement mode and order lifecycle;
- wallet reserve / commit / release coordination;
- activation invocation and durable recovery coordination;
- order-side activation proof.

It does not own:

- plan module definitions;
- entitlement limits;
- subscription status policy;
- subscription/entitlement persistence;
- AI token limit values;
- payment settlement truth;
- wallet balances.

### Existing subscription owner

`internal/listingsubscription` remains the current plan / subscription / entitlement authority for this slice.

It owns:

- active plan definition;
- canonical plan semantic fingerprint;
- activation window;
- subscription row;
- the exact entitlement set derived from the plan;
- purchase activation idempotency;
- canonical activation audit.

This slice does not move those facts to `internal/commercial/billing` and does not create a second entitlement owner.

### `internal/ledger/money`

Remains the only wallet / monetary value owner.

### App assembly

The application layer is the only place allowed to adapt the billing subscription port to the existing subscription service. Billing must not import HTTP handlers or write subscription tables directly.

## 3. First-slice scope

Executable self-service settlement modes:

```text
ZERO_PRICE
WALLET
```

Recognized but non-executable in this slice:

```text
EXTERNAL_PAYMENT
```

Supported purchase:

```text
first activation when no effective active/trialing subscription exists
```

Not implemented in this slice:

```text
upgrade
downgrade
renewal
automatic renewal
proration
coupon
tax
invoice
external checkout
```

An existing active or future-active subscription owned by a different activation source fails closed with `ACTIVE_SUBSCRIPTION_EXISTS`. An expired or disabled previous subscription may be replaced according to the activation owner rules below.

## 4. Commercial contract extensions

The existing commercial billing model stays the single offer / quote / order owner. Add a discriminated subscription product instead of creating a second billing subsystem.

### 4.1 Product and order kind

Add:

```go
const ProductSubscriptionPlan ProductKind = "SUBSCRIPTION_PLAN"

const OrderSubscriptionPurchase OrderKind = "SUBSCRIPTION_PURCHASE"
```

Existing resource product kinds and order kinds retain their current semantics.

### 4.2 Settlement mode

Add:

```go
type SettlementMode string

const (
    SettlementZeroPrice       SettlementMode = "ZERO_PRICE"
    SettlementWallet          SettlementMode = "WALLET"
    SettlementExternalPayment SettlementMode = "EXTERNAL_PAYMENT"
)
```

Invariants:

- `ZERO_PRICE`: amount is exactly 0 and no wallet reservation exists.
- `WALLET`: amount is greater than 0 and fulfillment requires a committed wallet reservation.
- `EXTERNAL_PAYMENT`: recognized catalog state only; order creation returns `PAYMENT_METHOD_UNAVAILABLE` until a provider contract exists.

No browser field can override settlement mode.

### 4.3 Subscription term

The first slice uses calendar-month terms:

```go
type SubscriptionTerm struct {
    Months int
}
```

Rules:

- the term is server-owned offer data;
- `Months` is positive and bounded;
- the browser never submits a term;
- activation computes `StartsAt` from the subscription-owner clock and `ExpiresAt = StartsAt.AddDate(0, Months, 0)`;
- no caller supplies status, starts_at, expires_at, or limits.

A future non-monthly product requires a new explicit product decision, not an overloaded integer convention.

### 4.4 Existing Offer / Quote / Order

Extend the existing discriminated records rather than introducing another offer/order owner.

For a subscription offer, persist at least:

```text
offer_id
product_kind = SUBSCRIPTION_PLAN
plan_code
term_months
settlement_mode
currency
unit_price_minor
pricing_version
status
offer validity window
```

Subscription-offer invariants:

- `ResourceType == ""`;
- resource quantity fields are unused;
- `PlanCode` is non-empty;
- `TermMonths > 0`;
- quantity is semantically exactly one;
- `ZERO_PRICE -> UnitPriceMinor == 0`;
- `WALLET / EXTERNAL_PAYMENT -> UnitPriceMinor > 0`;
- currency remains CNY for this contract.

For a subscription quote, freeze at least:

```text
quote_id
organization_id
offer_id
product_kind
plan_code
plan_fingerprint
term_months
settlement_mode
currency
total_minor
pricing_version
created_at
expires_at
fingerprint
```

The quote must include the semantic plan fingerprint observed at quote creation. It is not enough to bind only `plan_code`.

For the subscription order, retain at least:

```text
order_id
organization_id
actor_id
kind = SUBSCRIPTION_PURCHASE
quote_id
plan_code
plan_fingerprint
term_months
settlement_mode
currency
amount_minor
status
failure_code
wallet_reservation_id/state when applicable
activation_operation_id
activation_result_fingerprint
idempotency_key
request_fingerprint
version
created_at
updated_at
```

A fulfilled subscription order must retain activation proof. A WALLET order must also retain committed wallet proof.

## 5. Plan semantic snapshot

Billing may price a plan, but it cannot define its entitlements.

Add a narrow billing port:

```go
type SubscriptionPlanSnapshot struct {
    PlanCode     string
    DisplayName  string
    Fingerprint  string
}

type SubscriptionPurchasePort interface {
    ResolvePurchasablePlan(
        context.Context,
        string, // plan code
    ) (SubscriptionPlanSnapshot, error)

    ActivatePurchasedSubscription(
        context.Context,
        SubscriptionActivationRequest,
    ) (SubscriptionActivationResult, error)

    ReadPurchasedSubscriptionActivation(
        context.Context,
        string, // organization id
        string, // commercial order id
    ) (SubscriptionActivationResult, error)
}
```

The billing package owns this port and its DTOs. The app adapter maps it to the existing subscription owner; the subscription owner does not import billing.

### 5.1 Plan fingerprint and runtime catalog immutability

The subscription owner computes the canonical semantic plan fingerprint from:

```text
plan_code
plan active flag
ordered module_code set
for each module:
  ordered canonical limit key/value pairs
```

Exclude non-semantic display text and timestamps so a copy edit does not
invalidate a quote.

For this first self-service slice, the plan catalog is **immutable while the
current application is serving traffic**.

The old runtime catalog-management product is retired. The current application
must not expose runtime commands that edit plan active state, module membership,
or module limits.

At quote creation:

1. billing reads the sellable offer;
2. billing resolves a transactionally consistent current plan snapshot through
   the subscription port;
3. billing stores the returned plan fingerprint in the quote.

At activation:

1. subscription owner acquires the Organization activation fence;
2. it re-reads a transactionally consistent plan snapshot;
3. it recomputes the fingerprint;
4. mismatch with the quoted fingerprint returns `PLAN_CHANGED`;
5. no subscription or entitlement mutation occurs;
6. matching fingerprint is used to atomically derive the exact entitlement set.

No online plan writer may race this sequence in the admitted runtime.

### 5.2 Hard-cut of the old subscription-management runtime

The following old ListingKit product surfaces are RETIRED from the current
application instead of being made compatible with self-service purchase:

```text
UI:
  /listing-kits/platform/subscriptions
  /listing-kits/platform/subscription-plans

runtime API:
  /api/v1/listing-kits/platform/subscriptions...
  /api/v1/listing-kits/platform/subscription-plans...
  PUT /api/v1/listing-kits/admin/subscription/entitlements/:module_code
```

The implementation must remove these mutation routes from current-application
route registration and remove their old navigation/page entrypoints. Tests that
assert those routes are present must be rewritten as retirement guards.

Read-only legacy subscription projections may be removed when they have no
current consumer; they are not an architecture prerequisite for #478/#479.

The following historical service commands are not admitted runtime mutation
entrypoints for the new product:

```text
ApplyPlan
UpsertEntitlement / UpsertEntitlementWithAudit
UpsertPlan
UpsertPlanModule
DeletePlanModule
SetPlanActive
```

If some of these functions remain temporarily for package tests or bounded
fixture setup, no current application / HTTP / worker code may call them as a
production mutation path. New production code must not wrap them to preserve
the old platform-management behavior.

The current runtime must also stop mutating the plan catalog during service
construction. In particular, the current application's subscription service
construction must not execute `SyncDefaultCatalog`,
`UpsertDefaultPlans`, or equivalent plan/module writes after another replica
could already be serving purchase traffic.

The admitted model is:

```text
deployment / controlled provisioning
  -> durable server-owned plan catalog + commercial offers
  -> current-application starts
  -> runtime catalog is read-only
  -> users self-service purchase against that catalog
```

Catalog provisioning is not a browser API and is not part of tenant
self-service. It must finish before the current application begins serving
purchase requests. A catalog change requires a controlled catalog rollout /
application restart for this slice.

If online plan administration is reintroduced later, that is a new product and
architecture decision. It must add an explicit plan-version/fence/CAS contract
before runtime editing is admitted; the retired ListingKit routes must not be
restored as a compatibility shortcut.

## 6. Activation port DTO

Billing-side request:

```go
type SubscriptionActivationRequest struct {
    OperationID       string
    OrganizationID    string
    ActorID           string
    CommercialOrderID string
    PlanCode          string
    PlanFingerprint   string
    TermMonths        int
}
```

Rules:

- `OperationID` is deterministic from the durable order, for example `subscription-activate:<order_id>`;
- `CommercialOrderID` is the immutable source identity;
- all identifiers are bounded and normalized;
- `ActorID` is the authenticated actor persisted on the order;
- no status, window, module, limit, or amount field is accepted.

Billing-side result:

```go
type SubscriptionActivationResult struct {
    OperationID              string
    OrganizationID           string
    CommercialOrderID        string
    PlanCode                 string
    PlanFingerprint          string
    SubscriptionID           string
    StartsAt                 time.Time
    ExpiresAt                time.Time
    EntitlementSetFingerprint string
    ActivatedAt              time.Time
    Existing                 bool
}
```

`Existing=true` means the exact same source-bound operation already committed. It is replay evidence, not a new activation.

## 7. Subscription-owner command

The existing subscription owner exposes a narrow purchased-activation use case; it does not expose `ApplyPlan` directly to billing.

Recommended owner API shape:

```go
type PurchasedPlanActivationInput struct {
    OperationID       string
    OrganizationID    string
    ActorID           string
    SourceType        string
    SourceID          string
    PlanCode          string
    PlanFingerprint   string
    TermMonths        int
}

type PurchasedPlanActivationResult struct {
    OperationID               string
    OrganizationID            string
    SourceType                string
    SourceID                  string
    PlanCode                  string
    PlanFingerprint           string
    SubscriptionID            int64
    StartsAt                  time.Time
    ExpiresAt                 time.Time
    EntitlementSetFingerprint string
    ActivatedAt               time.Time
    Existing                  bool
}

func (s *Service) ResolvePurchasablePlan(
    context.Context,
    string,
) (PurchasedPlanSnapshot, error)

func (s *Service) ActivatePurchasedPlan(
    context.Context,
    PurchasedPlanActivationInput,
) (PurchasedPlanActivationResult, error)

func (s *Service) ReadPurchasedPlanActivation(
    context.Context,
    string, // organization
    string, // source id
) (PurchasedPlanActivationResult, error)
```

The source type is fixed to:

```text
commercial_subscription_order
```

The browser and HTTP layer never choose this value.

## 8. Subscription activation persistence and atomicity

Add a durable source-bound activation operation owned by the subscription domain.

Logical fields:

```text
operation_id
organization_id
source_type
source_id
request_fingerprint
plan_code
plan_fingerprint
term_months
subscription_id
starts_at
expires_at
entitlement_set_fingerprint
activated_at
```

Required uniqueness:

```text
UNIQUE(source_type, source_id)
UNIQUE(operation_id)
```

The subscription owner must atomically commit, in one database transaction:

1. the source-bound activation operation result;
2. the tenant subscription;
3. the exact entitlement set for the purchased plan;
4. canonical subscription activation audit.

No partial state is accepted.

Same source + same request fingerprint returns the previously committed result.

Same source + different fingerprint returns `ACTIVATION_CONFLICT`.

A lost commit acknowledgement is resolved by `ReadPurchasedPlanActivation`; it never authorizes a second source identity.

### 8.1 Exact entitlement set

Purchased activation must project the purchased plan as one coherent snapshot.

Inside the same transaction:

- each module in the purchased plan gets an ACTIVE entitlement with the activation window and the plan-owned limits;
- modules not present in the purchased plan must not retain an effective entitlement from an older plan;
- historical audit remains, but current effective entitlement state must equal the purchased plan.

The current `ApplyPlan` loop is not the purchased-activation transaction contract and must not be called as a sequence of externally visible writes.

### 8.2 Existing subscription rules

Before mutation, using the activation-owner clock:

- an effective ACTIVE or TRIALING subscription from another source blocks purchase with `ACTIVE_SUBSCRIPTION_EXISTS`;
- an already committed activation for the same source replays;
- an expired or disabled previous subscription may be replaced;
- a future-effective ACTIVE/TRIALING subscription also blocks;
- upgrade, downgrade, renewal, and overlap are not inferred.

### 8.3 Per-Organization first-activation fence

Even with the old admin writers retired, two distinct self-service orders have
different source identities and can race while the Organization has no
subscription row. Source idempotency alone is therefore insufficient.

The subscription owner owns a transaction-only serialization record:

```text
saas_subscription_activation_fences
  organization_id PRIMARY KEY
```

The fence is an internal concurrency primitive, not a business fact and not a
browser-visible resource.

Every `ActivatePurchasedPlan` transaction must:

1. ensure the fence row exists using idempotent insert-on-conflict;
2. lock the Organization fence for update;
3. re-read the source-bound activation operation after acquiring the lock;
4. replay it when the same source/fingerprint is already committed;
5. re-read current/future-effective subscription state;
6. reject another active/future-active subscription with
   `ACTIVE_SUBSCRIPTION_EXISTS`;
7. re-read and fingerprint the immutable runtime plan snapshot;
8. commit activation operation, subscription, exact entitlement set and audit
   atomically.

Locking only `saas_tenant_subscriptions` is not sufficient because the first
purchase has no row to lock.

Two distinct WALLET orders may temporarily reserve funds before activation, but
at most one may commit activation. A losing order with a proven
`ACTIVE_SUBSCRIPTION_EXISTS` result must release its own reservation before
becoming CANCELLED and must never commit wallet funds.

Required PostgreSQL concurrency evidence:

```text
two distinct SUBSCRIPTION_PURCHASE orders
same Organization
different idempotency keys / source IDs
start concurrently
  -> exactly one activation commits
  -> exactly one subscription/entitlement snapshot is current
  -> at most one wallet reservation commits
  -> losing reservation is released
  -> losing order is CANCELLED with ACTIVE_SUBSCRIPTION_EXISTS
```

There is no purchase-vs-platform-mutation compatibility matrix in this slice,
because the old platform/admin subscription mutation runtime is deliberately
removed.

## 9. Entitlement-set fingerprint

The activation result includes a canonical fingerprint over the resulting current subscription projection:

```text
organization_id
plan_code
starts_at
expires_at
ordered entitlements:
  module_code
  status
  starts_at
  expires_at
  ordered limit key/value pairs
```

Billing stores this proof but does not interpret the limits.

On recovery, a mismatched activation result is `RECONCILIATION_REQUIRED`, not success.

## 10. Commercial order lifecycle

Reuse the existing generic order states; do not add another state machine when the current states are sufficient.

### ZERO_PRICE

```text
PENDING
  -> FULFILLING
  -> FULFILLED
```

No wallet reservation exists.

### WALLET

```text
PENDING
  -> FUNDS_RESERVED
  -> FULFILLING
  -> FULFILLED
```

`FUNDS_RESERVED` requires a RESERVED wallet reservation.  
`FULFILLED` requires a COMMITTED wallet reservation.

### Failure / unknown

```text
PENDING / FUNDS_RESERVED
  -> CANCELLED                 # terminal pre-effect rejection

PENDING / FUNDS_RESERVED / FULFILLING
  -> RECONCILIATION_REQUIRED   # any possibly committed unknown
```

For WALLET, a terminal pre-activation rejection releases the reservation before the order becomes CANCELLED.

After activation has committed, cancellation is forbidden.

## 11. Order execution algorithm

### Create / replay

1. Resolve current identity and Effective Organization in HTTP.
2. Require `workbench.commercial.purchase`.
3. Persist actor ID from verified identity in the order request.
4. Find existing order by `(organization_id, idempotency_key)`.
5. Same key + same fingerprint replays.
6. Same key + different fingerprint returns `IDEMPOTENCY_CONFLICT`.
7. Read the quote and verify organization, expiry, product kind, and immutable fingerprint.
8. Create one PENDING subscription order.

### ZERO_PRICE execution

1. Call the source-bound activation command.
2. On any ambiguous error, immediately read activation by organization + order ID.
3. If matching activation exists, persist activation proof and continue.
4. If terminal pre-effect error is proven and no activation exists, cancel.
5. If outcome remains unknown, mark `RECONCILIATION_REQUIRED`.
6. Persist activation proof and `FULFILLING`.
7. Mark `FULFILLED`.

### WALLET execution

1. Reserve wallet funds using the order ID.
2. Persist reservation proof and `FUNDS_RESERVED`.
3. Call activation.
4. Resolve ambiguous activation result through readback.
5. Terminal pre-effect rejection -> release reservation -> CANCELLED.
6. Unknown -> keep reservation -> `RECONCILIATION_REQUIRED`.
7. Activation success -> persist activation proof -> `FULFILLING`.
8. Commit wallet reservation.
9. Resolve ambiguous commit using the existing wallet reservation readback.
10. Committed wallet + valid activation proof -> `FULFILLED`.
11. Any unresolved result -> `RECONCILIATION_REQUIRED`.

The order owner never creates a second order during reconciliation.

## 12. Reconciliation ownership

`internal/commercial/billing` is the only reconciliation owner for subscription purchase orders.

Expose an internal service operation:

```go
func (s *Service) ReconcileSubscriptionOrder(
    context.Context,
    string, // organization
    string, // order id
) (Order, error)
```

Reconciliation may:

- read the durable order;
- read wallet reservation state;
- read source-bound subscription activation;
- replay the same activation operation identity;
- commit or release the existing reservation only when the authoritative state permits it;
- move the same order to FULFILLED, CANCELLED, or remain RECONCILIATION_REQUIRED.

It may not:

- issue a new quote;
- generate a new idempotency key;
- create a replacement order;
- apply a different plan;
- choose a new Organization.

### 12.1 Automatic recovery trigger

Recovery cannot depend on a user resubmitting the original POST. The
current-application composition must start one dedicated
subscription-order recovery loop after the application has assembled
successfully and all required commercial, money, and subscription dependencies
are available.

This is a bounded commercial-billing recovery component, not a generic
scheduler/reconciliation framework and not a `kernel/module.Registry`
lifecycle extension.

The current `internal/app/runtime/currentapplication` code passes a
15-second `startupContext` into application assembly. The recovery runner must
**not** inherit that startup context, or it will terminate immediately after
startup. Implementation must explicitly bind the runner to the long-lived
runtime parent context and start it only after application assembly succeeds.
A minimal constructor/signature split between startup context and runtime
context is acceptable; adding a generic kernel scheduler or global lifecycle
framework is not.

The runner contract is:

```go
type SubscriptionOrderRecoveryStore interface {
    ListRecoverableSubscriptionOrders(
        context.Context,
        int, // limit
    ) ([]Order, error)
}

type SubscriptionOrderRecoveryRunner interface {
    Run(context.Context)
}
```

Persistence selection is restricted to:

```text
kind = SUBSCRIPTION_PURCHASE
status IN (
  PENDING,
  FUNDS_RESERVED,
  FULFILLING,
  RECONCILIATION_REQUIRED
)
```

The query is bounded to at most 50 orders per sweep and ordered by
`updated_at, order_id`. Terminal `FULFILLED` and `CANCELLED` orders are
never selected.

Runtime behavior:

1. run one recovery sweep immediately after successful application assembly,
   before relying on user traffic for recovery;
2. while the application context remains alive, repeat a bounded sweep every
   30 seconds;
3. cancel promptly on the parent application context;
4. for every selected order, invoke only
   `ReconcileSubscriptionOrder(organization_id, order_id)`;
5. never create a quote, order, idempotency key, activation source, or wallet
   reservation identity from the runner;
6. an unresolved attempt updates the durable order timestamp/state so the next
   bounded sweep can retry without a busy loop.

The recovery loop is discovery/trigger only. Correctness remains in the durable
owners:

- commercial order updates retain version/row-lock conflict checks;
- wallet operations remain source-bound and idempotent;
- subscription activation remains protected by the Organization activation
  fence and source-bound replay contract.

Therefore multiple application replicas or a foreground request racing the
recovery runner must be safe. An order-version conflict causes the losing
attempt to re-read/defer; it never authorizes a duplicate charge or activation.

Required restart evidence:

```text
activation commits
process stops before order activation proof is persisted
restart current-application
  -> startup recovery selects the original order
  -> reads the existing activation by original source ID
  -> completes/recovers original wallet state
  -> converges the same order to FULFILLED
  -> no second activation/order/charge
```

A persistent dependency outage may leave the order
`RECONCILIATION_REQUIRED`; the automatic loop must keep using the same durable
identities after the dependency recovers.

## 13. Public Workbench HTTP contract

Use dedicated self-service routes for the first slice so resource-purchase request schemas do not become ambiguous.

```http
GET  /api/v1/workbench/commercial/subscription-offers
POST /api/v1/workbench/commercial/subscription-quotes
POST /api/v1/workbench/commercial/subscription-orders
GET  /api/v1/workbench/commercial/subscription-orders/:order_id
```

All routes use the verified Effective Organization. No body/query/header can select another Organization.

### GET subscription-offers

No body. No arbitrary query.

Response item contains at least:

```json
{
  "offer_id": "pro-monthly",
  "plan_code": "professional",
  "plan_name": "专业版",
  "term_months": "1",
  "settlement_mode": "ZERO_PRICE",
  "currency": "CNY",
  "total_minor": "0",
  "pricing_version": "pricing-v1",
  "availability": "available"
}
```

The example values are shape examples only, not seed data or approved price.

`total_minor` is a decimal string.

Availability is server-derived. At minimum distinguish:

```text
available
current_plan
active_subscription_conflict
payment_unavailable
offer_unavailable
```

### POST subscription-quotes

Request:

```json
{"offer_id":"..."}
```

No organization, quantity, plan code, amount, term, status, window, or limits.

Response includes the immutable quote fields from section 4.

### POST subscription-orders

Requires exactly one `Idempotency-Key`.

Request:

```json
{"quote_id":"..."}
```

No organization, actor, amount, plan, entitlement, or settlement override.

### GET subscription-orders/:id

Returns the current durable order and activation proof summary. It never synthesizes entitlement success from order status alone.

## 14. HTTP error contract

At minimum:

```text
FORBIDDEN
ORGANIZATION_SELECTION_REQUIRED
OFFER_UNAVAILABLE
QUOTE_EXPIRED
PAYMENT_METHOD_UNAVAILABLE
INSUFFICIENT_FUNDS
ACTIVE_SUBSCRIPTION_EXISTS
PLAN_CHANGED
IDEMPOTENCY_CONFLICT
NOT_FOUND
CONFLICT
RECONCILIATION_REQUIRED
DEPENDENCY_UNAVAILABLE
INVALID_REQUEST
```

Transport timeout is never mapped to a terminal business failure without authoritative readback.

## 15. Authorization and tenancy

- offer reads use current commercial-read permission policy;
- quote/order writes require `workbench.commercial.purchase`;
- `listingkit_admin` is the first tenant self-service buyer role;
- viewer/operator cannot purchase;
- `platform_admin` only uses this tenant flow after a verified Effective Organization exists;
- browser-provided roles, actor, tenant, organization, or target IDs are ignored/rejected;
- grant revocation before each write fails closed;
- a quote from Organization A cannot create an order in B after an Organization switch.

## 16. Persistence boundary

Commercial tables may gain the fields required for subscription product discrimination and activation proof. They remain billing-owned.

The subscription activation-operation table is subscription-owned even when it physically resides in the same PostgreSQL database.

Do not create one cross-owner transaction that writes both commercial order tables and subscription tables. The durable order + source-bound activation protocol is the consistency boundary.

No direct SQL business-fact seeding is part of the product path.

## 17. Audit

Billing records commercial order lifecycle audit/evidence according to current project conventions.

The subscription owner atomically records a canonical activation audit with:

```text
organization
actor
source_type = commercial_subscription_order
source_id = commercial order id
plan_code
window
activation operation id
result fingerprint
```

Do not store credentials, tokens, payment secrets, or request bodies in audit payloads.

## 18. First implementation acceptance

The implementation is ready for #478 only when it proves:

- a server-owned subscription offer resolves to an active plan snapshot;
- quote freezes the plan semantic fingerprint;
- browser cannot submit price, plan internals, window, limits, actor, or organization;
- `listingkit_admin` can purchase only the current Effective Organization;
- viewer/operator are denied;
- ZERO_PRICE creates a canonical order and source-bound activation without money mutation;
- WALLET reserves, activates, then commits exactly once;
- insufficient wallet balance cancels without activation;
- active-subscription conflict does not charge;
- concurrent distinct first-purchase orders for one Organization serialize so exactly one activation and at most one final charge succeeds;
- old ListingKit platform/admin subscription mutation UI/routes are absent from the admitted current runtime;
- current-application does not mutate the plan catalog during service construction or while serving purchase traffic;
- quote and activation use the same immutable runtime catalog semantics, and any controlled catalog rollout occurs outside the serving runtime;
- a quoted plan fingerprint mismatch still fails with `PLAN_CHANGED` and no mutation;
- same idempotency key/same fingerprint replays the same order;
- same key/different fingerprint conflicts;
- lost activation acknowledgement is recovered by source-bound readback;
- lost wallet commit acknowledgement is recovered through the existing wallet reservation readback;
- startup and periodic automatic recovery select the original nonterminal subscription order and restart can reconcile it without creating a new order or activation;
- fulfilled order proves both activation and, for WALLET, committed money;
- resulting commercial overview and Account resource allocation read the same canonical entitlement;
- external-payment-required offers remain unavailable until an approved provider exists.

## 19. Implementation placement

Expected primary changes:

```text
internal/commercial/billing/*
internal/commercial/billing/httpapi/*
internal/integration/persistence/commercialbilling/*
internal/listingsubscription/*                  # narrow purchased-activation command
internal/app/httpapi/commercial_billing_module.go  # adapter/assembly only
internal/app/httpapi/current_application.go          # retire old subscription-management routes
internal/listingkit/httpapi/*                        # remove retired subscription management descriptors/interfaces
web/listingkit-ui/*                                  # remove old platform subscription management pages/nav
schema migration owned by the corresponding persistence owner
tests for domain, PostgreSQL, authorization, replay, recovery and hard-cut guards
```

The app adapter may import both billing and listingsubscription. Neither domain package imports app/httpapi.

Do not expand platform-admin subscription handlers into the tenant self-service implementation.

## 20. Deliberate non-goals

This contract does not authorize:

- actual production prices;
- a default free plan;
- production offer seeding;
- external payment provider integration;
- upgrade/downgrade;
- renewal/auto-renew;
- proration;
- coupons;
- invoice/tax;
- production deployment;
- historical subscription migration;
- a second subscription owner;
- a second commercial order owner;
- compatibility for the retired ListingKit platform/admin subscription-management UI or mutation APIs.

A concrete executable offer is product/catalog data and must be explicitly configured; the implementation must not invent one from Figma or tests.
