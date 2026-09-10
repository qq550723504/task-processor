# Title review admission — Issue #333

Scope authority: current #333 body/start comment, #36, #137, AGENTS and
Product Phase 3. This is the same delivery's narrow contract, not a prerequisite PR.
Status: IMPLEMENTATION_READY (round 1, independent read-only review_contract). No architecture BLOCKER; replay, transaction faults, concurrency and time budgets are IMPLEMENTATION_TEST obligations in this delivery.

## Owners and boundaries

Product review owns proposals, revisions, decisions and operation receipts.
Catalog remains the only Product writer. Enrichment remains side-effect free.
New code: product/review (model, service, binding, ports), dedicated
integration/persistence/product/review (schema and UoW adapter), explicit
app/httpapi/product_review application/handlers and dedicated tests.
Catalog changes: optional ExpectedBaseVersion on PublishRequest, stale error,
same head-lock algorithm and caller-transaction constructor. Exact guard
updates permit this adapter's narrow ports; no broad exclusion. Estimated
20–26 files, 1,300–1,500 production lines. Reassess before crossing AGENTS limits.
No default server registration, automatic schema migration, old data admission,
paid model, UI, image, price, SKU or other task's fixture changes.

## Input and authorization

Only server-injected exact (org, product key, version, publication ID, source
envelope) bindings from controlled upstream are eligible. Service reads the
immutable Catalog version through its 2 MiB bounded reader and verifies full
lineage against sourcing.ToSnapshot(envelope).Sources, not URL or numeric
coincidence. Bindings are immutable setup data, not HTTP inputs or a registry.
Reconstruction injects the same approved bindings. Missing binding fails closed.

Every invocation, including replay, requires nonexpired authenticated identity,
TenantID == EffectiveOrganizationID, and read permission. Writes additionally
require write permission. Use existing organization resolver with LiveWrite for
POST and CachedRead for GET, then existing authorizer with scoped roles only:
empty user subject prevents configured global user grants from becoming org
grants. Admin uses existing IsTenantAdmin with only the effective scoped roles.
Operator reads/edits own proposal; admin reads organization proposals and alone
accepts/rejects/applies. SQL constrains org and owner before reading/locking.
Home/global roles cannot replace selected-org grants; same-user review allowed.
No body org/actor/role/full snapshot/before/evidence/approval claims.

## HTTP and limits

POST /api/product/text-proposals: {product_key, base_version}.
GET /api/product/text-proposals/:proposal_id: safe diff, evidence references,
quality/unresolved diagnostics, revision/state, decision history and receipt;
no complete snapshot, raw provider response or arbitrary metadata.
POST .../:proposal_id/decisions: {action: accept|edit|reject,
expected_revision, title (edit only)}.
POST .../:proposal_id/apply: {expected_revision}.
POST requires one Idempotency-Key (canonical nonempty <=128 bytes).
Canonical UUID proposal IDs; positive signed-bigint-compatible versions.
Title: nonblank canonical UTF-8, <=4096 bytes, no controls; engineering bound,
not platform policy. Request <=32 KiB actual bytes, strict unknown/duplicate
field rejection using existing sigs.k8s.io/json. No query parameters.
10 s request/body/transaction budget; context reaches generation and SQL.
Persisted proposal <=64 KiB and <=100 revisions; bound SQL projection before
decode. Pending/accepted saves reserve the worst-case decision/receipt space;
edit cannot consume the final revision needed for acceptance. Strict wire checks
reject malformed UTF-8 and unpaired UTF-16 escapes before JSON decoding.
Limits and rejected/canceled transport map to stable 400/403/404/409/
413/504 errors; dependency/unknown outcome 503. No raw dependency errors exposed.

## State and immutable history

Create invokes real Proposer, title-only policy v1, saves original proposal and
base/evidence/policy, revision 1 pending. Each accept/reject/edit increments
revision, appends actor/action/old and new title and clears previous approval.
accept stores accepted revision; reject is terminal; edit from pending/accepted
produces pending revision, preserving original evidence and marking manual edit.
Applied is terminal. Each mutation checks exact expected revision under row lock.
Apply requires exact accepted revision, deterministic title/evidence validation,
and base Catalog version. A stale base returns conflict without writing.
Validation.Valid and ProductSnapshot.Review are never approval credentials.
Only Title is assigned on a cloned fixed base; no normalization of other facts.
Historical decisions remain durable and cannot be silently replaced.

## Idempotency and transaction

Operation scope = organization + actor + Idempotency-Key, shared across writes.
Fingerprint = operation kind + proposal ID + exact input, canonical JSON hash.
Same key/different payload conflicts; same key/same payload returns stored safe
response. Authorize and scope-check before replay. Replay precedes fresh stale
checks, so later head/revision movement cannot turn committed success into write.
An operation-only preflight takes and releases the advisory lock without writes;
stable replay/conflict returns there. A new mutation then performs source
authorization and carries a private short-lived proof into the final UoW. PG
transaction-scoped advisory lock serializes operation creation; proposal row lock
serializes decisions/applies. No provider call occurs while a PG connection or
lock is held. Generation may race before create UoW, but only one proposal/result
persists per operation.

Apply UoW: lock operation → receipt replay → scoped proposal FOR UPDATE → exact
accepted revision validation → bounded base read → real Catalog Publisher using
transaction-bound Catalog repository → save applied proposal/audit + operation
receipt → COMMIT. Existing Catalog algorithm locks the same head used by normal
Publisher, resolves publication replay first, then compares expected base and
allocates/inserts/advances version. No Catalog SQL in review adapter. Publication
ID is stable application hash of org/proposal/accepted revision, not model text.
Bound transaction adapter must not commit caller's transaction or escape DB.

Any failure before COMMIT rolls back Catalog, review and receipt. Error after
commit/cancellation is outcome-unknown; client retries same operation after fresh
auth and durable receipt resolves it. No background retry, Saga, outbox or second
recovery owner. Different operation against applied proposal conflicts. Lock
order operation→proposal→Catalog, all waits deadline-bound; normal Catalog only
locks Catalog, so no inverse cycle. Two instances share PostgreSQL, not mutex.

## Evidence plan (red then green)

Real HTTP + middleware + authorizer + Proposer + Sourcing/Catalog Publisher +
isolated empty PostgreSQL. Only external source/generator and verified identity/
grant provider transport fixtures are controlled; real resolver/middleware run.
Verify all transitions, edit invalidation, own/admin/cross-org/current grants,
revocation on replay, evidence mismatch, strict inputs and byte limits; before
and non-title equality; restart/replay; changed-payload conflict; competing
decisions/two instance applies/normal Publisher race; failure at review/receipt
writes rolls back version; lost committed response reconstructs durable receipt.
Focused/race/PG, existing Catalog/Sourcing/Enrichment, architecture/depguard and
final HEAD CI. Independent full-slice review. Real identity/source/production
opening NOT_RUN; #36/#33 remain open.
