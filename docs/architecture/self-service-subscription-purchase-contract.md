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
pending_effect              # empty | RESERVE | ACTIVATE
pending_effect_admitted_at
terminal_intent             # empty | CANCEL
terminal_intent_reason      # e.g. AUTHORIZATION_REVOKED
terminal_intent_at
idempotency_key
request_fingerprint
version
created_at
updated_at
```

A fulfilled subscription order must retain an `ACTIVATED` decision proof. A durable `REJECTED` decision can only back a CANCELLED order and must never satisfy fulfillment validation. A WALLET fulfilled order must also retain committed wallet proof.

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

Billing-side activation decision:

```go
type SubscriptionActivationOutcome string

const (
    SubscriptionActivationActivated SubscriptionActivationOutcome = "ACTIVATED"
    SubscriptionActivationRejected  SubscriptionActivationOutcome = "REJECTED"
)

type SubscriptionActivationFailureCode string

const (
    SubscriptionActivationActiveSubscriptionExists SubscriptionActivationFailureCode = "ACTIVE_SUBSCRIPTION_EXISTS"
    SubscriptionActivationPlanChanged              SubscriptionActivationFailureCode = "PLAN_CHANGED"
)

type SubscriptionActivationResult struct {
    OperationID               string
    OrganizationID            string
    CommercialOrderID         string
    PlanCode                  string
    PlanFingerprint           string
    Outcome                   SubscriptionActivationOutcome
    FailureCode               SubscriptionActivationFailureCode
    SubscriptionID            string
    StartsAt                  *time.Time
    ExpiresAt                 *time.Time
    EntitlementSetFingerprint string
    DecidedAt                 time.Time
    Existing                  bool
}
```

Result invariants:

- `ACTIVATED`: `FailureCode == ""`; subscription ID/window and entitlement
  fingerprint are present.
- `REJECTED`: `FailureCode` is one of the bounded terminal business
  rejections above; subscription ID/window/entitlement fingerprint are empty.
- `Existing=true` means the exact same source-bound activation **decision**
  already committed and is being replayed. A rejected decision is just as
  durable as an activated decision.
- dependency/timeouts/internal errors are not persisted as `REJECTED`; they
  remain unknown/retryable with the same source identity.

## 7. Subscription-owner command

The existing subscription owner exposes a narrow purchased-activation use case; it does not expose `ApplyPlan` directly to billing.

Recommended owner API shape:

```go
type PurchasedPlanActivationOutcome string

const (
    PurchasedPlanActivationActivated PurchasedPlanActivationOutcome = "ACTIVATED"
    PurchasedPlanActivationRejected  PurchasedPlanActivationOutcome = "REJECTED"
)

type PurchasedPlanActivationFailureCode string

const (
    PurchasedPlanActivationActiveSubscriptionExists PurchasedPlanActivationFailureCode = "ACTIVE_SUBSCRIPTION_EXISTS"
    PurchasedPlanActivationPlanChanged              PurchasedPlanActivationFailureCode = "PLAN_CHANGED"
)

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
    Outcome                   PurchasedPlanActivationOutcome
    FailureCode               PurchasedPlanActivationFailureCode
    SubscriptionID            int64
    StartsAt                  *time.Time
    ExpiresAt                 *time.Time
    EntitlementSetFingerprint string
    DecidedAt                 time.Time
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
outcome                  # ACTIVATED | REJECTED
failure_code             # nullable; bounded terminal code
subscription_id          # nullable for REJECTED
starts_at                # nullable for REJECTED
expires_at               # nullable for REJECTED
entitlement_set_fingerprint # empty/null for REJECTED
decided_at
```

Required uniqueness:

```text
UNIQUE(source_type, source_id)
UNIQUE(operation_id)
```

The row is a durable **activation decision**, not merely a success receipt.

For `ACTIVATED`, the subscription owner atomically commits in one database
transaction:

1. the source-bound activation decision;
2. the tenant subscription;
3. the exact entitlement set for the purchased plan;
4. canonical subscription activation audit.

For a deterministic terminal pre-effect rejection, the subscription owner
atomically commits in one database transaction:

1. the source-bound activation decision with `outcome=REJECTED`;
2. its bounded `failure_code`;
3. a rejection audit/result record as required by the owner contract;
4. **no** subscription or entitlement mutation.

The first-slice durable rejection codes are:

```text
ACTIVE_SUBSCRIPTION_EXISTS
PLAN_CHANGED
```

A terminal rejection is persisted while the Organization activation fence is
held and **before** billing may release wallet funds or cancel the order.

Same source + same request fingerprint returns the previously committed
decision, whether `ACTIVATED` or `REJECTED`.

Same source + different fingerprint returns `ACTIVATION_CONFLICT`.

A lost decision acknowledgement is resolved by
`ReadPurchasedPlanActivation`; it never authorizes a second source identity.

Transient dependency failure, statement timeout, context cancellation, or an
otherwise unknown outcome must not be persisted as a terminal rejection.

### 8.1 Exact entitlement set

Purchased activation must project the purchased plan as one coherent snapshot.

Inside the same transaction:

- each module in the purchased plan gets an ACTIVE entitlement with the activation window and the plan-owned limits;
- modules not present in the purchased plan must not retain an effective entitlement from an older plan;
- historical audit remains, but current effective entitlement state must equal the purchased plan.

The current `ApplyPlan` loop is not the purchased-activation transaction contract and must not be called as a sequence of externally visible writes.

### 8.2 Existing subscription rules and terminal decisions

Using the activation-owner clock while holding the Organization activation
fence:

- if the same source already has a durable activation decision, replay that
  decision first; no current-time policy re-evaluation may override it;
- an effective ACTIVE or TRIALING subscription from another source produces a
  durable `REJECTED / ACTIVE_SUBSCRIPTION_EXISTS` decision for this source;
- a future-effective ACTIVE/TRIALING subscription produces the same durable
  rejection;
- a quoted plan fingerprint mismatch produces a durable
  `REJECTED / PLAN_CHANGED` decision for this source;
- an expired or disabled previous subscription may be replaced only when this
  source has no prior durable rejection;
- upgrade, downgrade, renewal, and overlap are not inferred.

This means a commercial order rejected at time T remains rejected even if the
blocking subscription expires at T+1. A later purchase must be a new canonical
quote/order/source; the rejected order can never become an activation attempt
again.

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
3. re-read the source-bound activation decision after acquiring the lock;
4. replay it when the same source/fingerprint already has `ACTIVATED` or
   `REJECTED`;
5. re-read current/future-effective subscription state;
6. if another active/future-active subscription blocks purchase, atomically
   persist `REJECTED / ACTIVE_SUBSCRIPTION_EXISTS` for this source and return
   that durable decision;
7. re-read and fingerprint the immutable runtime plan snapshot;
8. on fingerprint mismatch, atomically persist `REJECTED / PLAN_CHANGED` for
   this source and return that durable decision;
9. otherwise commit `ACTIVATED`, subscription, exact entitlement set and
   audit atomically.

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

### Durable admission of the next external effect

Live authorization and the commercial order version must jointly gate every
**new** reserve or activation effect. A request/recovery worker must never
perform a fresh external effect merely because it observed permission before
another replica changed the order.

Add a billing-owned internal coordination field:

```go
type PendingSubscriptionOrderEffect string

const (
    PendingSubscriptionOrderEffectNone     PendingSubscriptionOrderEffect = ""
    PendingSubscriptionOrderEffectReserve  PendingSubscriptionOrderEffect = "RESERVE"
    PendingSubscriptionOrderEffectActivate PendingSubscriptionOrderEffect = "ACTIVATE"
)
```

Before calling a not-yet-started external effect:

1. read all current durable owner decisions/proofs for the order;
2. if the target effect is already proven or already admitted, reconcile/replay
   it instead of starting a new admission;
3. reject new RESERVE/ACTIVATE admission if `terminal_intent=CANCEL` is already
   durably present on the order;
4. perform a fresh service-side provider-backed live reauthorization for the
   persisted actor and exact Organization; this is required for every new
   RESERVE or ACTIVATE admission even when both occur in one HTTP request;
5. if denied, do not admit the effect; use the revocation path below;
6. if allowed, atomically CAS the commercial order's expected version/state and
   persist `pending_effect` plus `pending_effect_admitted_at`; the CAS predicate
   must also require `terminal_intent` to still be empty;
7. only the attempt that wins this CAS may invoke that newly admitted external
   effect;
8. a losing attempt must re-read the order and owner decisions before doing
   anything else.

The durable `pending_effect` marker is the effect-admission boundary. Once it
is committed, recovery may replay that exact source-bound effect without
reauthorizing, even if the actor is later revoked, because the effect was
already admitted while authorization was live. Recovery may not change the
effect kind.

After the external owner returns or readback proves a durable decision, billing
persists that proof and clears `pending_effect` in the same order update.

If the process dies after admission but before sending the external call,
recovery replays the same admitted effect identity. This does not create a new
business attempt.

This is an order-local CAS protocol, not a generic distributed lock, scheduler,
or cross-owner transaction.

#### Revocation before effect admission

Authorization revocation is itself a durable admission decision. Before any
wallet release or terminal cancellation caused by revocation, billing must CAS
the order to an irrevocable cancellation intent:

```text
terminal_intent        = CANCEL
terminal_intent_reason = AUTHORIZATION_REVOKED
terminal_intent_at     = now
```

The CAS predicate requires:

- expected order version/state still match;
- `pending_effect` is empty; neither `RESERVE` nor `ACTIVATE` may already be
  admitted;
- no durable `ACTIVATED` decision exists;
- `terminal_intent` is empty or already the exact same CANCEL reason.

Once `terminal_intent=CANCEL` is committed:

- no new RESERVE or ACTIVATE admission is legal, even if the actor later regains
  `workbench.commercial.purchase`;
- recovery may only finish the already-selected cancellation path;
- a concurrent activation-admission CAS must fail because it requires
  `terminal_intent` to be empty.

Revocation handling then becomes:

```text
no reserve decision / no activation decision
  -> CAS terminal_intent=CANCEL / AUTHORIZATION_REVOKED
  -> CANCELLED / AUTHORIZATION_REVOKED

RESERVED wallet funds, no activation decision
  -> CAS terminal_intent=CANCEL / AUTHORIZATION_REVOKED
  -> release original reservation using release:<order_id>
  -> CANCELLED only after authoritative RELEASED

pending_effect=RESERVE
  -> revocation CAS must not override it
  -> reconcile the already-admitted reserve effect first
  -> only after reserve resolves may a later revocation decision choose release
     versus no-reserve cancellation

pending_effect=ACTIVATE or durable ACTIVATED decision already exists
  -> revocation CAS must not override it
  -> reconcile the already-admitted/proven activation path instead
```

If the revocation-intent CAS loses to another replica that already admitted
`ACTIVATE`, the denying attempt must re-read and reconcile; it must not release
funds or cancel the order.

A later role restoration never revives an order that durably reached
`CANCELLED / AUTHORIZATION_REVOKED`.

### ZERO_PRICE execution

1. If activation is not already admitted/proven, complete the live-reauth + order-CAS admission protocol for `pending_effect=ACTIVATE`; only the CAS winner calls the source-bound activation command. If ACTIVATE is already admitted, replay/read back that exact source identity without a new authorization decision.
2. On any ambiguous error, immediately read the activation decision by
   organization + order ID.
3. If no durable decision can be proven, mark
   `RECONCILIATION_REQUIRED`; do not infer success or failure from transport
   outcome alone.
4. If the durable decision is `REJECTED`:
   - persist the rejected-decision proof on the commercial order;
   - copy the bounded activation failure code to the order failure reason;
   - mark the same order `CANCELLED`;
   - do not write `FULFILLING` or `FULFILLED`;
   - do not create any money effect.
5. If the durable decision is `ACTIVATED`:
   - validate the decision matches this organization/order/plan/fingerprint;
   - persist activated-decision proof;
   - mark `FULFILLING`;
   - then mark `FULFILLED`.
6. Any mismatched, malformed, or otherwise unclassifiable decision remains
   `RECONCILIATION_REQUIRED`.

A durable activation **decision** merely proves that the activation owner
reached a terminal conclusion. Only `Outcome=ACTIVATED` proves fulfillment.
`Outcome=REJECTED` is terminal cancellation evidence, never activation proof.

### WALLET reserve decision

The money owner must persist a source-bound reserve decision for every
`ReserveCommercialPurchase` operation. Returning
`ErrWalletInsufficientBalance` without a durable decision is not sufficient,
because a later top-up could otherwise make the same order reserve funds after
another replica already cancelled it.

Logical money-owned decision:

```go
type WalletReserveDecisionOutcome string

const (
    WalletReserveDecisionReserved WalletReserveDecisionOutcome = "RESERVED"
    WalletReserveDecisionRejectedInsufficientFunds WalletReserveDecisionOutcome = "REJECTED_INSUFFICIENT_FUNDS"
)

type WalletReserveDecision struct {
    OperationID       string
    OrganizationID    string
    CommercialOrderID string
    Currency          string
    AmountMinor       int64
    Outcome           WalletReserveDecisionOutcome
    ReservationID     string // present only for RESERVED
    DecidedAt         time.Time
}
```

Required identity/fingerprint:

```text
organization_id
operation_id = order_id
commercial_order_id = order_id
currency
amount_minor
```

Money persistence must enforce one immutable decision for that identity.

The money owner transaction must:

1. check for an existing reserve decision for the exact source identity;
2. same identity + same amount/currency replays the existing decision;
3. same identity + different amount/currency returns reservation conflict;
4. otherwise lock the Organization wallet;
5. re-check the decision after the wallet lock;
6. if funds are sufficient, atomically persist:
   - wallet balance movement available -> reserved;
   - wallet reservation row;
   - `RESERVED` decision pointing to the reservation;
   - immutable wallet entry;
7. if funds are insufficient, atomically persist:
   - `REJECTED_INSUFFICIENT_FUNDS` decision;
   - no reservation;
   - no wallet balance mutation;
   - no purchase reserve wallet entry.

A later wallet top-up does not change or delete the reserve decision. Replaying
the same order after a durable insufficient-funds decision returns the same
insufficient-funds result and can never create a reservation.

Expose authoritative readback owned by money:

```go
ReadCommercialPurchaseReserveDecision(
    context.Context,
    organizationID string,
    operationID string,
    commercialOrderID string,
) (WalletReserveDecision, error)
```

The existing reservation readback remains authoritative for reservation state;
the new reserve-decision readback answers whether the original reserve operation
ever terminally decided RESERVED or REJECTED_INSUFFICIENT_FUNDS.

### WALLET execution

The wallet reserve identity is frozen to the durable commercial order:

```text
ReserveWalletFundsInput.OperationID       = order_id
ReserveWalletFundsInput.CommercialOrderID = order_id
```

The same identity and immutable amount/currency must be used by foreground
execution and recovery.

1. If reserve is not already admitted/proven, complete live reauth + order CAS
   admission for `pending_effect=RESERVE`; only the CAS winner calls
   `ReserveCommercialPurchase` using the durable order ID. If RESERVE is
   already admitted, replay/read back that same reserve identity.
2. If the reserve response is lost or ambiguous, replay
   `ReserveCommercialPurchase` with the **same** order-derived input; do not
   generate another reserve identity.
3. Resolve the money-owned durable reserve decision for this exact source:
   - `RESERVED`: validate the reservation ID/amount/currency binding, persist
     reservation proof and `FUNDS_RESERVED`, then clear `pending_effect=RESERVE`;
   - `REJECTED_INSUFFICIENT_FUNDS`: prove no activation decision exists,
     persist `CANCELLED / INSUFFICIENT_FUNDS`, clear `pending_effect=RESERVE`,
     and stop; no later top-up can revive this order;
   - missing/mismatched/unknown: keep or move the order to
     `RECONCILIATION_REQUIRED` and retain the admitted RESERVE effect identity
     until readback/replay resolves it.
4. After RESERVED proof is persisted, if activation is not already admitted/proven, perform a **new provider-backed exact actor grant query** through the service-side live authorizer, then CAS `pending_effect=ACTIVATE`; the earlier RESERVE admission or request-start LiveWrite identity cannot satisfy this check. Only the CAS winner calls activation. If ACTIVATE was already durably admitted, replay/read back that exact activation identity without a new authorization decision.
6. Resolve ambiguous activation result through source-bound readback.
7. A terminal pre-effect rejection is actionable only when the returned/read
   activation decision is durably `REJECTED` for this exact source and request
   fingerprint.
8. Persist the rejected activation-decision proof on the commercial order before
   initiating wallet release.
9. Release the original reservation with deterministic finish operation ID
   `release:<order_id>`; replay the same finish operation if its acknowledgement
   is lost.
10. Only after authoritative RELEASED readback may the order become CANCELLED
    with the same activation failure code.
11. Unknown activation result -> keep reservation ->
    `RECONCILIATION_REQUIRED`.
12. `ACTIVATED` -> persist activation proof -> `FULFILLING`.
13. Commit wallet reservation using deterministic finish operation ID
    `commit:<order_id>`.
14. Resolve ambiguous commit using the existing wallet reservation readback /
    same finish-operation replay.
15. Committed wallet + valid `ACTIVATED` proof -> `FULFILLED`.
16. Any unresolved result -> `RECONCILIATION_REQUIRED`.

A transport error from reserve is never proof that no reservation exists.
Foreground execution and recovery both resolve it by replaying the same
source-bound reserve operation.

The order owner never creates a second order or a second wallet reserve identity
during reconciliation.

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
- for a WALLET order whose durable order does not yet contain a
  `reservation_id`, replay `ReserveCommercialPurchase` with the original
  order-derived reserve input;
- recover the original reservation ID from that idempotent replay;
- read wallet reservation state once the reservation ID is known;
- read the source-bound subscription activation decision, including durable
  `REJECTED` decisions;
- replay the same activation operation identity;
- release the original reservation only after a matching durable `REJECTED`
  decision is proven and persisted on the order;
- commit the original reservation only after a matching durable `ACTIVATED`
  decision is proven and persisted on the order;
- move the same order to FULFILLED only from a matching durable `ACTIVATED` decision;
- move the same order to CANCELLED from either:
  - a matching durable `REJECTED` activation decision (plus authoritative RELEASED wallet proof for WALLET), or
  - a money-owned durable `REJECTED_INSUFFICIENT_FUNDS` reserve decision for
    this exact order identity together with source-bound activation readback
    proving no activation decision exists;
- once an insufficient-funds reserve decision exists, both reserve replay and
  order recovery must remain terminal for that order even if the wallet is
  topped up later;
- otherwise remain RECONCILIATION_REQUIRED.

It may not:

- issue a new quote;
- generate a new order idempotency key;
- generate a new wallet reserve operation identity;
- create a replacement order;
- apply a different plan;
- choose a new Organization.

"Replay reserve" is recovery of an already-defined effect, not creation of a
new business attempt. The immutable replay input is reconstructed only from the
durable order:

```text
operation_id        = order_id
commercial_order_id = order_id
organization_id     = order.organization_id
currency            = order.currency
amount_minor        = order.amount_minor
```

No field is taken from the browser during recovery.

### 12.1 Recovery authorization before the first unreconciled effect

Automatic recovery has no browser bearer and must not retain one. It therefore
cannot reuse request-time `workbenchcontext.Resolver` by replaying old request
credentials.

Billing owns a narrow service-side authorization port:

```go
type SubscriptionPurchaseRecoveryAuthorizer interface {
    ReauthorizeCommercialPurchase(
        context.Context,
        string, // organization id
        string, // actor id
    ) (CommercialPurchaseAuthorization, error)
}

type CommercialPurchaseAuthorization struct {
    OrganizationID string
    ActorID        string
    Roles          []string
    Allowed        bool
    ObservedAt     time.Time
}
```

The app-layer adapter must use the existing current membership/grant authority,
with its dedicated server-side read credential, to resolve the actor's live
project assignment for the exact Organization. It must then apply the existing
`authz.PermissionWorkbenchCommercialPurchase` policy.

Foreground HTTP routes still require
`AuthPolicyCurrentIdentity + OrganizationAccessPolicyLiveWrite +
workbench.commercial.purchase` to create/replay the commercial order and bind
the persisted actor/Effective Organization.

That request-time authorization is **not** reusable as authorization for any
external effect admission. Every not-yet-admitted `RESERVE` and every
not-yet-admitted `ACTIVATE`, including a second effect later in the same HTTP
request, must call `SubscriptionPurchaseRecoveryAuthorizer` (the service-side
live authorizer) immediately before the order-CAS admission.

Therefore the WALLET sequence deliberately performs two distinct live checks:

```text
before RESERVE admission  -> exact provider-backed actor grant query
after RESERVED proof, before ACTIVATE admission
                          -> a new exact provider-backed actor grant query
```

A still-open HTTP request, an unexpired browser token, the roles attached to
the request-start identity, or a previous successful effect admission are not
proof of current permission for the next effect.

Browser bearer tokens are never persisted or replayed for effect admission.

The adapter must not:
- use roles persisted on the order as authority;
- synthesize `listingkit_admin` from historical state;
- use a cached browser grant;
- retain/replay bearer tokens;
- accept Organization or actor from an HTTP request during recovery.

For the current repository, the intended authority is the provider-backed
membership/grant source used by the current ZITADEL integration.

Recovery authorization MUST use the provider-side exact actor query supported
by `ListAuthorizations` with all three filters:

```text
inUserIds     = [actor_id]
projectId     = configured project id
organizationId = order.organization_id
```

The repository already uses this query shape in current ZITADEL authorization
code and acceptance utilities. Recovery must not fall back to offset-based
Organization-wide scans to prove actor absence.

The exact query result must be validated as follows:

- zero matching assignments -> authorization denied/revoked;
- exactly one matching assignment with provider state `STATE_ACTIVE` -> use only
  its live role keys and apply `authz.PermissionWorkbenchCommercialPurchase`;
- exactly one matching assignment with known provider state `STATE_INACTIVE`
  (or another explicitly documented deactivated/revoked terminal state) ->
  authorization denied/revoked;
- more than one matching assignment -> invalid/unavailable, not merged;
- wrong Organization/project/user, unknown/unsupported state, malformed roles,
  oversized response, timeout, non-2xx, malformed JSON, or provider failure ->
  dependency unavailable / `RECONCILIATION_REQUIRED`;
- provider unavailability is never interpreted as revocation.

A known inactive/deactivated exact authorization is authoritative revocation
evidence. For an order with RESERVED funds and no admitted activation, that
denial must enter the same irreversible `terminal_intent=CANCEL /
AUTHORIZATION_REVOKED` path and release the original reservation. Restoring the
grant later cannot revive that order.

The service-side provider credential remains read-only and must not be exposed
to billing/domain code or browser callers.

Required implementation evidence:

- exact query sends `inUserIds`, `projectId`, and `organizationId` together;
- authorized actor is admitted regardless of how many other assignments exist
  in the Organization;
- successful exact query with no assignment or one known inactive/deactivated
  assignment is denied/revoked;
- deactivate-then-restore does not revive an order once revocation cancellation
  intent was persisted;
- duplicate/malformed/unknown-state exact actor results fail unavailable;
- no offset-pagination fallback exists in the recovery authorizer.

#### When reauthorization is required

Reauthorization is required immediately before billing attempts to **admit**
a new external effect into the durable order.

```text
PENDING WALLET, no reserve decision and pending_effect != RESERVE
  -> reauthorize, then CAS-admit RESERVE, then call/replay reserve

PENDING ZERO_PRICE, no activation decision and pending_effect != ACTIVATE
  -> reauthorize, then CAS-admit ACTIVATE, then call/replay activation

FUNDS_RESERVED / RESERVED decision, no activation decision,
pending_effect != ACTIVATE, and terminal_intent is empty
  -> reauthorize, then CAS-admit ACTIVATE, then call/replay activation

terminal_intent=CANCEL
  -> never admit RESERVE/ACTIVATE again
  -> continue only the cancellation/release path

pending_effect already equals the required effect
  -> effect was already live-authorized and durably admitted
  -> do not reauthorize; resolve/replay that exact effect identity
```

If live authorization is denied before an effect is admitted, use the
order-CAS revocation path defined in `Durable admission of the next external
effect`. Never release/cancel if that CAS loses to an already-admitted effect.

The denial/cancellation is durable for that order. A later role restoration
does not revive it; a new purchase requires a new quote/order/idempotency
identity.

#### When reauthorization must not block convergence

Once an effect is already authoritatively committed or its outcome is unknown,
revocation must not strand money or entitlements in a half-finished state.

```text
durable ACTIVATED decision exists
  -> do not reauthorize to decide whether activation should have happened
  -> WALLET must converge the original reservation to COMMITTED
  -> ZERO_PRICE must converge the order to FULFILLED

durable REJECTED activation decision exists
  -> converge cancellation / wallet release

reserve or activation outcome is UNKNOWN, or pending_effect proves admission
  -> continue readback/replay with the same source identities
  -> do not reauthorize to reverse the already-admitted effect
  -> do not create a different business effect solely because authorization changed

COMMITTED wallet reservation without matching ACTIVATED decision
  -> never invent a fresh activation
  -> RECONCILIATION_REQUIRED / operator investigation
```

Authorization is therefore an admission check for the **next not-yet-started
effect**, not permission to abandon reconciliation of an effect that may already
exist.

### 12.2 Automatic recovery trigger

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
5. never create a quote, order, order idempotency key, activation source, or
   **new** wallet reservation identity from the runner;
6. a PENDING WALLET order may replay the already-defined reserve operation using
   the durable order ID as both reserve operation ID and commercial order ID;
7. an unresolved attempt updates the durable order timestamp/state so the next
   bounded sweep can retry without a busy loop.

The recovery loop is discovery/trigger only. Correctness remains in the durable
owners:

- commercial order updates retain version/row-lock conflict checks;
- wallet reserve/commit/release operations remain source-bound and idempotent;
- replaying `ReserveCommercialPurchase` with the same
  `(organization_id, operation_id=order_id, commercial_order_id=order_id,
  currency, amount_minor)` returns the original matching reservation rather
  than reserving funds twice;
- the money owner re-checks an existing reserve after acquiring the wallet lock,
  so a concurrent same-order replay cannot incorrectly classify the order as
  insufficient funds merely because the first transaction already moved funds
  from available to reserved;
- before each not-yet-admitted reserve/activation effect, recovery reauthorizes
  the persisted actor against the live Organization membership/grant authority
  and `workbench.commercial.purchase`, then must win the order-version CAS that
  persists the matching `pending_effect` before invoking it;
- an already persisted `pending_effect` is reconciled/replayed without a new
  authorization decision and can never be changed into an opposite effect;
- a durable `terminal_intent=CANCEL` is an irreversible admission fence for the
  order: all later RESERVE/ACTIVATE CAS operations require it to be empty;
- subscription activation remains protected by the Organization activation
  fence and source-bound durable-decision replay contract;
- deterministic rejection is a durable terminal decision for that order/source,
  so a later wall-clock change (including the blocking subscription expiring)
  cannot turn the same rejected order into an activation.

Therefore multiple application replicas or a foreground request racing the
recovery runner must be safe. An order-version conflict causes the losing
attempt to re-read/defer; it never authorizes a duplicate charge or activation.

Required restart evidence:

```text
A. reserve commits, reserve ACK/order proof is lost
   order remains PENDING without reservation_id
   restart current-application
     -> startup recovery selects the original order
     -> replays ReserveCommercialPurchase with operation_id = order_id
     -> receives the original reservation_id
     -> persists FUNDS_RESERVED and continues the same order
     -> no second reservation / no second debit

B. activation commits
   process stops before order activation proof is persisted
   restart current-application
     -> startup recovery selects the original order
     -> reads the existing activation by original source ID
     -> completes/recovers original wallet state
     -> converges the same order to FULFILLED
     -> no second activation/order/charge
```

For PENDING WALLET recovery, authoritative reserve outcomes are classified as:

```text
matching RESERVED reservation
  -> persist reservation proof and continue

matching COMMITTED reservation
  -> read activation proof
  -> matching activation may converge the same order to FULFILLED
  -> missing/mismatched activation remains RECONCILIATION_REQUIRED

matching RELEASED reservation
  -> if no activation exists, converge the same order to CANCELLED
  -> if activation exists, remain RECONCILIATION_REQUIRED

ErrWalletInsufficientBalance
  -> not terminal from the transient error alone
  -> read the money-owned reserve decision for the original order identity
  -> only terminal when that durable decision is
     REJECTED_INSUFFICIENT_FUNDS and source-bound activation readback proves no
     activation decision
  -> persist CANCELLED / INSUFFICIENT_FUNDS
  -> the same order can never reserve later, even after wallet top-up

reservation conflict / dependency unavailable / ambiguous transport error
  -> RECONCILIATION_REQUIRED
  -> retry the same reserve identity later
```

For a WALLET order with a durable activation decision:

```text
REJECTED / ACTIVE_SUBSCRIPTION_EXISTS or PLAN_CHANGED
  -> persist decision proof on order
  -> release with release:<order_id>
  -> CANCELLED only after RELEASED is authoritative
  -> same order/source can never activate later

ACTIVATED
  -> never release
  -> commit with commit:<order_id>
  -> FULFILLED only after COMMITTED is authoritative
```

Two replicas processing the same order may both replay the same deterministic
release/commit operation, but they may not choose opposite terminal money
effects because the source-bound activation decision is already immutable.

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
- ZERO_PRICE creates a canonical order and no money mutation; `ACTIVATED` converges to FULFILLED while durable `REJECTED` converges to CANCELLED and can never be reported as success;
- WALLET reserves, activates, then commits exactly once;
- insufficient wallet balance cancels without activation only from a money-owned durable `REJECTED_INSUFFICIENT_FUNDS` reserve decision plus proof that no activation decision exists; the same order never becomes chargeable after a later top-up;
- request-time LiveWrite authorization never substitutes for effect-time authorization; RESERVE and ACTIVATE each require their own fresh provider-backed exact actor grant check before admission, including when both occur in one HTTP request;
- recovery reauthorizes the persisted actor against live Organization membership/grants before each not-yet-admitted reserve/activation effect;
- only the order-version CAS winner may persist `pending_effect` and invoke that new effect;
- revoked/downgraded actors cannot cause a new reserve or activation during recovery;
- recovery authorization uses provider-side exact actor query (`inUserIds + projectId + organizationId`) and never uses offset-pagination absence as revocation evidence;
- authorization-denial cancellation first persists `terminal_intent=CANCEL`; role restoration cannot revive that order;
- revocation cancellation CAS requires `pending_effect` to be completely empty, so it cannot abandon an admitted RESERVE or ACTIVATE effect;
- activation admission CAS requires empty terminal_intent, so release-in-progress and activation cannot both be newly admitted;
- authorization-denial cancellation cannot race an already-admitted RESERVE/ACTIVATE effect; a losing denial CAS must reconcile the admitted effect;
- crash after effect admission but before external call replays the same admitted source identity without creating a new attempt;
- crash after revocation intent but before wallet release resumes the same release:<order_id> path and never reopens activation;
- already-admitted/proven/unknown external effects still converge safely after revocation and are not abandoned;
- active-subscription conflict commits a durable rejected activation decision before wallet release and does not charge;
- the same rejected order cannot activate after the blocking subscription expires;
- two replicas racing the same order across an `ExpiresAt` boundary cannot produce both ACTIVATED and RELEASED outcomes;
- concurrent distinct first-purchase orders for one Organization serialize so exactly one activation and at most one final charge succeeds;
- old ListingKit platform/admin subscription mutation UI/routes are absent from the admitted current runtime;
- current-application does not mutate the plan catalog during service construction or while serving purchase traffic;
- quote and activation use the same immutable runtime catalog semantics, and any controlled catalog rollout occurs outside the serving runtime;
- a quoted plan fingerprint mismatch still fails with `PLAN_CHANGED` and no mutation;
- same idempotency key/same fingerprint replays the same order;
- same key/different fingerprint conflicts;
- lost wallet reserve acknowledgement is recovered by replaying the original order-derived reserve identity and recovering the original reservation ID;
- lost activation acknowledgement is recovered by source-bound readback;
- lost wallet commit acknowledgement is recovered through the existing wallet reservation readback;
- startup and periodic automatic recovery select the original nonterminal subscription order and restart can reconcile it without creating a new order, new reserve identity, or new activation;
- fulfilled order proves both activation and, for WALLET, committed money;
- resulting commercial overview and Account resource allocation read the same canonical entitlement;
- external-payment-required offers remain unavailable until an approved provider exists.

## 19. Implementation placement

Expected primary changes:

```text
internal/commercial/billing/*
internal/commercial/billing/httpapi/*
internal/integration/persistence/commercialbilling/*
internal/integration/persistence/money/*             # durable reserve decision
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
