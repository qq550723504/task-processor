# Member resources and official store connection v1

Execution: #561. Parent: #551 / #469. Design Basis: Independent Architecture.
Baseline: main `6918c0276f286e51105dfb217d5eb4a2d184e64b`.
Admission: NOT_READY; first independent boundary review pending. This document
precedes production implementation. One Writer and one main Delivery Batch PR.

## 1. Product authority and deliverable

An enterprise administrator assigns concrete stores, renewal periods and data
rows to members. A member sees their actual allocations, opens an officially
connected store's service and renews its period, and acquires a product with an
actual data-row charge. An administrator can renew a member's store on their
behalf. All balances, assignments, receipts and service dates survive reload
and restart. The current Account Center must stop displaying fixed missing
values for these facts when the corresponding current owners are configured.

User decisions, recorded in #561, take precedence over historical scope:

- 2026-09-28: this batch includes full consumption, not allocation-only UI.
- 2026-09-28: first service activation requires a real platform connection.
- 2026-09-29: SHEIN official merchant/application authorization is the first
  connection. Do not substitute a source-account login or seller cookies.
- Data: reserve one row before acquiring/publishing; charge one row for one
  acquisition operation's successfully stored product result; definite failure
  releases it; UNKNOWN retains it; replaying an operation charges once, while
  a new operation for the same product charges another row.
- Previously approved unified AI points and member monthly spending limits
  remain the AI-point policy. AI Token entitlement is a separate fact.
- 2026-09-29: a SHEIN developer application has not been applied for. Code and
  bounded development tests are in scope. Real merchant authorization and
  real first-activation acceptance remain BLOCKED/NOT_RUN until the actual
  developer configuration is ready. This does not remove the connection or
  consumption implementation from the batch.

Figma authority: [final UI/IA](../product/final-ui-ia-authority.md), file
`tg48P46SSXl6TBy9lZwg63`, actually read nodes 1627:359 (resources), 1627:893
(allocation drawer), 1766:362 (member renewal), 1788:359 (administrator
renewal), and 1548:402 (binding dialog's API-unavailable state). Renewal is
30 days per period. Allocation uses existing resources; an entered data amount
is a quantity conversion at the configured current Commercial price, not a
wallet purchase or a cash transfer. The user's official-connection decision
replaces the old API-unavailable/subaccount-login copy in 1548:402; retain its
binding-dialog structure and existing component/style conventions. Do not
open subaccount-password/SMS login as a parallel connection mechanism.

Out of scope: new payment/retail prices, commissions, AI-point balance redesign,
other platforms, publication/fulfillment permissions, platform product writes,
general vault/Saga/scheduler platforms, old-data compatibility or migration,
production deployment, merge, Issue closure and unapproved real provider/data
operations. Tests and fixtures are not product acceptance.

## 2. Owners and actual composition

| Fact / action | Canonical owner | Path |
| --- | --- | --- |
| User, organization, active member ID and roles | ZITADEL / current WorkbenchContext | verified token → live organization grant → `EffectiveMemberID` |
| Store record, member assignment, service operation/date | StoreCenter / dedicated Store DB | current Store module → Store repository / local transaction |
| Official SHEIN connection and encrypted merchant credential | StoreCenter connection owner / Store DB | connection application → SHEIN OpenAPI adapter → connection repository |
| Enterprise unallocated and member-held periods/data; reservations and settlement | existing Organization Resource ledger / Commercial owner DB | resource application → bounded member/consumer contracts → orgresource adapter |
| Current price and immutable conversion quote | existing Commercial OfferCatalog / QuoteEngine | resource drawer → server quote → existing configured catalog |
| Acquisition operation and terminal publication proof | Product Sourcing / dedicated Product DB | existing acquisition/capture applications → acquisition repository + SRC-1/Catalog transaction |
| Display | Account/Store UI, strict client and BFF | current user/org-bound requests → current owner projections |

`internal/app/httpapi/current_store_center.go` is the active Store builder;
`storecenter_module.go`'s old default/shared-DB builder is not an integration
point. Store and Commercial remain separate databases/pools. The prepared
same-DB `orgresourceadapter.StoreServiceExecutor` and
`TransactionalReservationOwnerStore` cannot be wired to a different database
and presented as atomic. Preserve existing generic/same-DB contracts for their
current consumers; do not extend their meaning implicitly.

Assess existing shared transaction, Store quota reserve/resume, Commercial
recovery-loop and Temporal facilities before adding orchestration. A shared
transaction is unavailable across the admitted databases. Use the existing
local transaction/CAS/receipt patterns and the existing bounded Commercial
recovery-loop host for these two registered consumers. A new general workflow
engine, event bus or recovery platform is unnecessary. No direct remote-owner
table updates from Resource persistence; each owner exposes its own narrow port.

## 3. Store assignments and authorization

StoreCenter owns a durable `(organization_id, store_id, member_id)` assignment
with active/revoked status, version, actor and timestamps, and immutable
idempotent assignment operations/audit in the Store DB. Membership IDs are
ZITADEL authorization IDs, not user IDs or local synthetic IDs. Store deletion
revokes assignments in its own transaction; history remains. Rejoining the
same enterprise with a new membership ID does not inherit old assignments.

Authorization is existing role permission AND current organization grant AND
store assignment for ordinary members. Current enterprise administrators retain
enterprise-wide Store access. Platform administrators need a current explicit
organization grant; a global role is not a cross-organization bypass. Viewer
assignment permits reads, never writes. Assignment mutation is administrator
only and verifies the target's live active membership. Reclaiming a departed
member's remaining resources is administrator-only and does not require the
departed identity to authenticate.

List filtering and pagination totals are computed in the Store owner query,
not by filtering an already paginated full enterprise list in the browser.
Detail, update, enable/disable, service commands, connection commands and all
current StoreReference consumers use the same scoped authorizer. Inaccessible
stores return the existing non-disclosing not-found behavior. Recheck current
assignment at write admission under the Store transaction's assignment lock;
revocation fences subsequent commands. Already committed operations remain
recoverable, without admitting a new business mutation after revocation.

Preserve the current operator's Store-create permission: successful creation
explicitly records an active grant for its verified current member in the
same Store-local transaction as the new record. This is an actual assignment
event, not a later `CreatedBy` inference. Administrator-created records rely
on administrator access unless the administrator explicitly assigns a member.
Provisioning/resume must not expose an unassigned completed store through a
creator-based fallback. Sibling-path checks cover current Listing record and
StoreReference access; unopened legacy routes are not re-enabled.

## 4. One resource ledger, partitioned availability

For `store_renewal_period` and `data_row`, keep enterprise `available` as the
unallocated/admin-spendable pool; add `allocated` as the sum of members' free
holdings. Existing `reserved` and `consumed` remain enterprise-wide totals.
Member positions hold free/reserved/consumed integer units and a CAS version.
Positions and operations are in the same Resource DB as the enterprise bucket.
They partition resources; they do not mint a second enterprise balance.

| Command | Enterprise delta | Member delta |
| --- | --- | --- |
| Allocate q | available -q, allocated +q | free +q |
| Reclaim q | allocated -q, available +q | free -q |
| Member reserve q | allocated -q, reserved +q | free -q, reserved +q |
| Admin reserve q | available -q, reserved +q | none |
| Commit member reservation q | reserved -q, consumed +q | reserved -q, consumed +q |
| Release member reservation q | reserved -q, allocated +net | reserved -q, free +net |

All allocation/reservation/settlement transactions lock bucket, then member
position (when applicable), then operation/reservation in a fixed order. The
unique operation scope is `(organization_id, operation_id)`; fingerprint binds
type, member/funding position, resource, exact quantity and expected version.
Same key/different payload conflicts; same key/same payload returns its durable
snapshot after current authorization, before volatile price/version checks.
Checked arithmetic and nonnegative SQL constraints prevent overflows and
overspending. AI-point and Token paths retain their existing policies.

Reclaim is limited to free units; reserved/consumed/UNKNOWN units cannot be
reclaimed. Inactive members' positions remain visible to the administrator for
reclaim; no GET-driven deletion or silent transfer to a new member ID. Reclaim
and release must apply the existing debt-first credit rule in the same bucket
transaction: `net = gross - debt_repaid`. Only net units return to unallocated
or member free holdings. New allocations/reserves cannot spend through a
positive resource debt. Do not change existing payment/refund policies.

Read contracts explicitly distinguish unallocated, member free, member
reserved, allocated total and consumed. Enterprise usable total is unallocated
+ member free; administrator usable balance is unallocated. Do not relabel
unallocated as all enterprise resources or sum reserved twice. Unavailable
schema/source is unavailable, not a fabricated zero. All wire quantities use
bounded decimal strings and source timestamps.

Data amount conversion reuses the configured Commercial data-row offer and
immutable quote. Compute whole rows using integer minor-unit arithmetic,
return the quoted quantity/unit price/version/expiry and any remainder; explain
that no cash is charged. Confirm allocation by the quote's exact quantity and
current position version. Missing/disabled prices close the amount-conversion
action with a useful message; never use the localhost synthetic price in
production. Period allocation/reclaim is directly in integer periods. Existing
AI monthly-limit controls remain separately named; do not turn limits into a
member AI balance to imitate a Figma example number.

## 5. Cross-database consumption contract

Introduce a narrow Resource consumer-charge port registered only for Store
service operations and Product acquisitions. It does not accept browser-chosen
success/failed proofs, arbitrary owners, resource types or trusted principals.
It reserves/reads/settles canonical existing resource reservations, recording
the funding position and immutable owner binding. Any added ledger evidence
is a settlement receipt, not a second owner of Store dates or Product results.

Protocol, with each numbered write atomic only in its actual owner database:

1. Owner persists immutable authorized intent `(org, actor, member, owner type,
   owner operation ID, request fingerprint, resource, quantity, funding source)`.
   No acquisition dispatch/publication or Store date change is permitted yet.
2. Resource reserves from the bound position once and returns a durable receipt.
   Lost acknowledgement is recovered by the same identity, never a new key.
3. Owner binds that exact reservation using its immutable intent/CAS. The owner
   cannot execute without binding; the binding cannot change on replay.
4. Owner performs the actual Store-local transition or Product-local guarded
   publication and writes immutable terminal success/failure proof locally.
5. Resource retrieves the exact original terminal proof through a registered
   server-owned port and atomically commits/releases the original reservation.
   No owner result, timeout or HTTP error supplied by a caller substitutes for
   this proof. Commit/release is terminal and cannot be reversed by late input.

If step 1 acknowledgement is lost, read the original intent before step 2.
If step 2 is unknown, repeat only its idempotent reservation/read. If binding
is missing, recover the original receipt and bind; no timeout-based release.
If owner commit/terminal proof is unknown, hold the reservation and inspect the
original owner. Definite pre-dispatch cancellation/failure is persisted behind
the owner's dispatch/publication fence before release. A failed bind is not
itself proof that the owner will never execute. A read miss is not terminal
failure. Once an owner intent is failed, no older worker may commit its effect.

Recovery host: reuse `startCommercialRecoveryLoop`, startup plus 30-second
ticks, at most 25 pending reservations per pass and bounded per-proof deadlines.
The registered consumer coordinator is the sole settlement responsibility;
requests may call the same idempotent method for immediate completion. This
loop reads proof and settles only; it does not start a new acquisition, renew a
store, authorize a provider or release on elapsed time. Original intent/receipt
bindings allow settlement after member departure or role revocation without
creating a fresh business permission. Unknowns remain reserved and visible.

## 6. Store service activation and renewal

StoreCenter owns new durable service intents/receipts in its separate DB.
Reuse the existing pure Activate/Renew/Reactivate transitions, 30-day period,
effective status, quantity policy and optimistic Store version. Activation
uses one period; renewal/reactivation quantity follows the current contract.
Member self-service draws their assigned periods. Administrator renewal,
including renewal on a member's behalf, draws the administrator's unallocated
pool as specified by Figma 1788:359. The dialog names that funding source before
confirmation; it is frozen in the intent. Do not add an unrequested selector
that lets an administrator spend a member's assigned balance.

After reservation binding, lock Store + assignment + service intent locally;
verify original version/connection reference and current authorized admission,
apply the pure transition, and atomically save Store version/dates and terminal
operation proof. Concurrent renewals on an old Store version fail once and
release only from the immutable no-effect proof. Lost Store COMMIT response
reads the original receipt, not dates that may have changed again. UNKNOWN
returns a pending result, not a second renewal or an optimistic success.

First activation requires a fresh successful official SHEIN credential/store
query for the current connection version before its Store-local commit.
Capture that version/observation in the intent; a local disconnect or
reauthorization fences the activation transaction. Provider/network failures
remain unavailable. Renewal only extends service dates under the existing
transition rules; it never creates a platform connection, reauthorizes SHEIN
or grants publishing permissions. Service status and connection status stay
separate facts. Existing record/quota/create behavior and paid plan ownership
remain unchanged.

## 7. SHEIN official connection

Read official primary sources in this session:

- [Application authorization manual](https://open.sheincorp.com/documents/system/2169474d-1d4a-41a9-b9fd-427f63f54a63),
  including visible browser body and official Java/JavaScript/Python examples.
- [Token exchange](https://open.sheincorp.com/documents/apidoc/detail/3001235)
  and [official English API/FAQ](https://open.sheincorp.com/documents/apidoc/detail/3001520-1000012).
- [Signing rules](https://open.sheincorp.com/zh/documents/system/passwdrule).
- [Store information](https://open.sheincorp.com/documents/apidoc/detail/3001499),
  actual visible browser request/response tables read.

Authorization URL is the documented `https://openapi-sem.sheincorp.com/#/empower`
with `appid`, Base64 of a fixed configured HTTPS `redirectUrl`, and opaque
`state`. API host is separately configured from the documented official host
allowlist; application business mode must match its domain. No arbitrary URL,
redirect following, cookie scraping or alternate unofficial login adapter.

Begin creates a Store-local connection attempt bound to current org, Store,
actor, membership, connection version and a cryptographically random one-time
state whose hash is stored. TTL is at most five minutes, safely within both the
manual's ten-minute description and token API's stricter five-minute error
guidance. SHEIN requires merchant main-account consent. The callback landing
page removes token/state from the displayed URL promptly, has no third-party
assets and uses no-store/no-referrer. It posts them through the authenticated
subject/org-bound BFF; it does not perform an external mutation on a GET.
Completion rechecks current identity, assignment and state/attempt/app binding.
No anonymous callback can attach a merchant to a caller-chosen enterprise.

Persist the attempt's exchange dispatch claim before sending
`POST /open-api/auth/get-by-token`. Sign with developer AppId/AppSecretKey;
verify returned AppId and echoed state. Merchant credential decryption is the
documented AES-128-CBC, Base64 ciphertext, first 16 UTF-8 AppSecretKey bytes,
IV first 16 bytes of `space-station-default-iv`, PKCS padding. Reuse a mature
existing crypto helper where its contract matches; otherwise use Go standard
crypto/cipher plus validated padding. This provider wire protocol is not the
encryption-at-rest protocol.

Immediately encrypt a received merchant secret at rest using standard
AES-GCM with random nonce, a separately configured 32-byte private deployment
key and associated data binding org/Store/attempt/application/credential
version. Store key ID with ciphertext; refuse operation if its configured key
is unavailable. Never put credentials, tempToken, state or signature into
normal logs, audit payloads, HTTP response DTOs, source control or test reports.
No new general secret-management platform; reuse private runtime config.

With the stored credential, sign the documented store-info POST. Its actual
response is `info.storeInfo` plus a separate `storeProductQuota`; verify
supplier identity against token exchange. `storeStatus` and `storeName` are
optional (returned only for self-operated/semi-managed stores). A successful
authenticated query proves credential connectivity; it does not prove a
missing store-status value is enabled or grant product publishing. Keep those
fields unavailable when absent. Global uniqueness of `(AppId, openKeyId)`
prevents attaching one external Store/app relationship to two local Stores or
enterprises. User-entered external IDs are record metadata, not identity proof.

Connection attempt states: awaiting-consent → exchange-dispatched →
credential-received → verified/failed; uncertain exchange is UNKNOWN and
remains so until explicit reauthorization supersedes it. A received encrypted
credential can retry only the safe store-info query. A dispatched exchange
without its response is not automatically reissued: official docs do not
provide an authorization-record lookup API. Show reauthorization required;
explicit user consent starts a new attempt and fences old completions. New
reauthorization may rotate the merchant secret; never restore an old secret as
valid. Connection record states are disconnected, connected, expired and
unavailable, with version and observation time; timeout is unavailable.

Local disconnect fences use and removes locally usable credentials; it must
not say SHEIN authorization was revoked. Official remote deauthorization is
merchant Seller Hub → App Store → My Authorizations; no deauthorize API is
documented. Activation always performs the fresh query, so an old local
connected projection cannot open service after provider-side revocation.
No broader webhook framework is required for this batch.

If configuration is missing, preserve Account/Store reads and show connection
setup unavailable. Do not construct dummy credentials, pass fixture status as
runtime status or remove the activation connection gate.

## 8. Data-row acquisition charge and publication fence

Reuse current acquisition operation identity `(org, actor, idempotency key)`
and deterministic operation ID/fingerprint; add immutable member, funding and
reservation binding to the current operation owner. Browser capture also binds
the existing capture digest. Public HTTP, public browser and browser-capture
composition all require the same charge port. Reads, verification, AI reuse
and downstream tools do not create a second charge. Current capacity/body/
deadline bounds and operation lease/fence behavior remain applicable.

Reserve one row before provider dispatch or capture publication. Missing member
allocation fails before dispatch. An enterprise administrator uses unallocated
data rows; ordinary members use only their own free position. Reauthorization
before new business work uses the existing current live access contract.

The SRC-1/Catalog transaction must lock and validate the acquisition's original
operation + reservation binding + publishing fence before writing evidence and
Catalog. Inject a narrow transaction-bound acquisition guard through current
composition; do not teach Resource persistence to access Product tables. On
success, write the original immutable acquisition terminal publication proof
inside the same Product transaction as SRC-1/Catalog. Lock order starts with
the acquisition row, then SRC publication slot, then Catalog. A failed/cancelled
operation cannot publish later. A separate checked HTTP result followed by a
ledger release is insufficient.

Success proof binds original org/actor/member/operation/reservation,
publication ID/input hash/product key and exact Catalog version/snapshot hash.
It is read from immutable source receipt and exact Catalog facts, not merely
an operation state named `published`. Failure proof is immutable no-effect
terminal state behind the same publication fence. Conflict or validation failure
before publication can release only after this proof is durable. Network error,
COMMIT loss, lease expiry, context cancellation, missing readback or publisher
unavailability retains the row reservation. Recovery checks the original proof
and settles once; it cannot manufacture publication or refetch under a new key.

The existing generic/controlled-snapshot source publication capability is not
an uncharged alternate acquisition producer. Only acquisition-kind publications
require this charge-bound guard; other approved Product operations retain their
existing unrelated contracts. Identify active composition and sibling entry
points in implementation tests, without reviving retired source-import routes.

## 9. HTTP, UI and runtime delivery

Add exact allowlisted current routes for resource allocation/reclaim/detail,
Store assignment, connection begin/complete/disconnect, service commands and
original operation recovery. Reuse current identity/live organization access,
permission middleware, strict body parsing/limits, If-Match, Idempotency-Key
and expected user/org BFF headers. Settlement/proof/principal ports are
in-process only, never generic tenant HTTP endpoints. Bounded statuses distinguish
completed, pending/unknown, conflict, insufficient resources and unavailable.

Account resource summary/member tables and drawer read the actual owners;
member Store link opens the actual assigned directory. Renew dialog previews
current exact dates, periods and applicable funding source; successful dates
appear only after Store receipt/resource settlement readback. Pending results
offer original-operation recovery. Enterprise switch clears pending UI context;
old responses cannot overwrite the new enterprise's state. Do not display
fixture member amounts, statuses, prices or store names in runtime UI.

Initializers install only current fresh schemas, owner-specific constraints and
narrow runtime grants. Construction verifies schema/permissions and performs
no DDL/backfill. Store runtime cannot mutate Resource tables; Resource runtime
cannot mutate Product/Store tables. Assembly borrows explicit pools and uses
registered ports, with bounded timeouts. Compose secrets/configuration stay
private; missing SHEIN setup is a capability-specific unavailability, not an
invented connected state or a reason to break existing account reads.

The current 22744 acceptance checkout/volumes remain untouched. This batch does
not authorize upgrading/resetting them or deploying an unreviewed candidate.
After implementation, provide normal startup/configuration instructions and
known limitations; user/independent verifier determines actual acceptance.

## 10. Verification and admission

Use existing focused TDD and PostgreSQL integration tests, not a new runner or
fault-injection platform. First failing tests cover actual invariant violations:

- assignment list/detail and active Store consumers, role AND assignment,
  create grant transaction, revocation and cross-org/member-ID confusion;
- allocation/reclaim/consume conservation, member/admin funding, overflow,
  CAS/concurrency, same key conflicts, debt-first release, inactive members;
- two-owner COMMIT loss and restart readback; immutable terminal proof/fence;
  no release from absence/timeout; no duplicate service dates or acquisition;
- all acquisition channels charge one row on exact persistence and no row on
  definite fenced failure; repeats/read/AI reuse do not recharge;
- SHEIN official signature/decryption fixtures, host/header contract, one-time
  state/scope/credential association, unknown exchange, missing setup and
  local/remote disconnect wording. Fixtures prove adapter behavior only;
- strict BFF identity/org binding, Figma resource/renewal UI, selected quote
  quantity and pending-state behavior; relevant types/build/boundary checks.

Independent first review checks these actual high-risk changes against current
Must and the project finding classification. At most two normal architecture
rounds; freeze at IMPLEMENTATION_READY when no current BLOCKER remains. New
nonblocker hardening stays implementation tests/backlog. Final delivery review
checks the completed path/diff. Actual provider connection, exact-main runtime
and user acceptance stay separate from fixtures/CI.

Legacy decision: EXTRACT current pure service transitions and current
transaction/receipt behavior into their correct current owners where needed;
RETIRE any default shared-DB/creator-inferred/legacy cookie composition from the
new path. No compatibility, dual writes, identity mapping or migration.
