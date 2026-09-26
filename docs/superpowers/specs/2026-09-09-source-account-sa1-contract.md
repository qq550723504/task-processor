# Source Account SA1 contract

Issue: #368
Product decision: #365 / `PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08`
Status: IMPLEMENTATION_READY — independent R1/R2 admission completed 2026-09-09

## Delivery boundary

SA1 delivers one current business path:

```text
NewSourceAccountApplication (isolated current application)
  -> existing authentication
  -> existing Effective Organization resolver
  -> existing permission authorizer
  -> sourceaccountregistry/httpapi
  -> sourceaccountregistry Service
  -> sourceaccountregistry Store port
  -> integration/persistence/sourceaccountregistry PostgreSQL adapter
```

It is an architecture-sensitive but bounded slice: one HTTP/authorization
boundary and one PostgreSQL transaction boundary, with a small two-state
management state machine. The completed pre-review implementation inventory is
30 related files and approximately 1,878 added production lines. That exceeds
the 1,500-line architecture-sensitive threshold, so implementation proceeded
only after this contract and independent architecture admission were complete.
Splitting the schema/UoW from its sole HTTP business chain would produce either
unused persistence or an unverifiable API and is therefore not an independently
mergeable slice. Exceeding 30 related files or 2,500 changed production lines
still stops implementation for rescoping.

The new owner is `internal/sourceaccountregistry`. Persistence is implemented
by `internal/integration/persistence/sourceaccountregistry`; HTTP transport is
implemented by `internal/sourceaccountregistry/httpapi`; only
`internal/app/httpapi` assembles concrete adapters.

`NewSourceAccountApplication(db, verifier, resolver, authorizer)` follows the
existing isolated-application pattern. It accepts a caller-owned PostgreSQL
handle and current auth dependencies, validates the SA1 schema read-only, and
returns an `http.Server` containing only the SA1 route module and the existing
auth/Organization middleware. It performs no config loading, database opening,
schema mutation, provider construction or legacy/default feature assembly.

SA1 is deliberately absent from `httpFeatureComposition`,
`newHTTPFeatureCompositionBuilder` and the normal `product-listing-api` startup
in this slice. That default composition still constructs legacy ListingKit and
old Source Account repositories and has two independent auto-migration paths;
using it would make the clean-room acceptance false. Binding this admitted
module into shared/production composition is a later explicit rollout decision,
not a hidden feature flag or part of #368.

The new call graph must not import or invoke:

- legacy `internal/sourceaccount` or its GORM repository/service;
- `internal/tenantbridge`, root `internal/listingkit`, or
  `internal/compatibility/*`;
- `internal/integration/persistence/sourceaccount/ownershipmigration`;
- #364/#366 rehearsal code, migration receipts, old IDs, profiles or old tables.

The existing Store Center is a pattern reference, not a Source Account owner.
No Source Account fact is stored in a Store table.

## Resource and state

`SourceAccount` is an Organization-owned internal resource with these fields:

| Field | Contract |
| --- | --- |
| `id` | Server-generated canonical UUIDv7; never supplied or derived from an old ID |
| `organization_id` | Non-empty opaque Effective Organization ID; never accepted from body/query |
| `platform` | Closed enum containing only `1688` in SA1 |
| `display_name` | Trimmed valid UTF-8, 1–120 bytes, no control characters |
| `management_status` | `enabled` or `disabled`; created as `enabled` |
| `connection_status` | Always `pending_connection` in SA1 |
| `version` | Positive signed-bigint-compatible integer; created at 1 and incremented per state change |
| `created_by`, `updated_by` | Current authenticated subject, stored but not exposed in public DTOs |
| `created_at`, `updated_at` | Server UTC timestamps canonicalized to PostgreSQL microsecond precision before persistence and response |

`enabled` means only that this internal resource is administratively enabled.
It does not mean the 1688 account is authenticated, connected, verified,
usable, or allowed to crawl. Registration never writes `connected_at`,
`verified_at`, credentials, cookies, tokens, profile paths, login URLs, proxy
URLs or any client-claimed connection state.

SA1 has no external-platform account identifier because no provider ownership
or connection has been established. `display_name` is presentation metadata,
not an identity or deduplication key, so different idempotency keys may create
resources with the same display name. A future connection slice may bind a
provider-owned identity only after real provider proof; it must not infer one
from the display name or a legacy record.

State transitions are:

```text
create -> enabled(version 1), pending_connection
enabled(N)  --disable with If-Match N--> disabled(N+1)
disabled(N) --enable  with If-Match N--> enabled(N+1)
```

A fresh request to enable an enabled account or disable a disabled account is
`409 INVALID_TRANSITION`. An exact idempotent replay performs no transition and
returns the current projection with `replayed=true`. Therefore replay of an old
enable cannot overwrite a later disable.

There is no delete, transfer, batch mutation, member grant, platform login or
connection transition in SA1.

## HTTP contract

The isolated application registers these exact descriptors through the existing
HTTP server and route-auth pipeline:

| Method and path | Purpose | Organization policy | Permission |
| --- | --- | --- | --- |
| `POST /api/v1/workbench/source-accounts` | Register | `live_write` | `workbench.source_account.manage` |
| `GET /api/v1/workbench/source-accounts` | Stable organization page | `cached_read` | `workbench.source_account.read` |
| `GET /api/v1/workbench/source-accounts/:source_account_id` | Detail | `cached_read` | `workbench.source_account.read` |
| `POST /api/v1/workbench/source-accounts/:source_account_id/disable` | Disable exact version | `live_write` | `workbench.source_account.manage` |
| `POST /api/v1/workbench/source-accounts/:source_account_id/enable` | Enable exact version | `live_write` | `workbench.source_account.manage` |

Authentication is `verified_identity`. Middleware order is the repository's
existing order: reject prohibited body where descriptor-owned, authenticate,
resolve Effective Organization, then authorize the route permission, then call
the handler. The service independently requires a non-expired authenticated
identity where `TenantID == EffectiveOrganizationID`, and asks the existing
authorizer for the explicit SourceAccount read/manage permission using the
verified `UserID` and resolved Effective Organization roles.

**Current product decision B (2026-09-12, #411 Gate A):** the user explicitly
selected the existing `authz` SourceAccount permissions as the sole capability
authority. This replaces this contract's former service-level role-only rule
that passed an empty global subject. Configured platform-admin users and roles
may therefore receive SourceAccount read/manage after normal Organization
admission. The former rule was intentional; this is a new product decision,
not a correction of the historical contract. No UI/service fixed-role check
may further restrict a grant from that authority. Verified identity, live grant
resolution on every write/replay, suspension checks and exact Organization
scope remain mandatory. This decision does not change membership permissions,
mutation APIs, idempotency, versions, management state or unknown outcomes.

Role mapping extends the existing authorizer, not a new RBAC system:

| Resolved Organization role | Read | Manage |
| --- | --- | --- |
| `listingkit_viewer` | yes | no |
| `listingkit_operator` | yes | yes |
| `listingkit_admin` | yes | yes |
| `platform_admin` | yes | yes, only after normal Effective Organization admission |

Configured platform-admin identity does not bypass Organization selection or
grant resolution. `live_write` performs a live grant read on every write,
including idempotent replay. Reads use the current approved cached-read
contract. Revoked/suspended/cross-Organization access and authorization
dependency failure remain fail-closed.

### Inputs and projections

Register requires exactly one canonical UUID `Idempotency-Key` and a strict JSON
body, maximum 8 KiB:

```json
{"displayName":"Primary 1688","platform":"1688"}
```

Unknown/duplicate fields, malformed Unicode, query parameters, client identity,
status, connection claims or secret-like fields are rejected.

Enable/disable require exactly one canonical UUID `Idempotency-Key`, one strong
decimal `If-Match` such as `"3"`, no query and a zero-length body. Detail
requires a canonical UUID path ID, no query and no body.

The exported Go wire contract is equivalent to:

```text
SourceAccountDTO {
  id, platform, displayName, managementStatus, connectionStatus,
  version, createdAt, updatedAt
}
MutationResponse { schemaVersion: 1, account, replayed }
DetailResponse   { schemaVersion: 1, account }
PageResponse     { schemaVersion: 1, items: [], nextCursor: string|null }
```

Versions are decimal strings on the wire and strong ETags on single-resource
success responses. New registration returns `201`; registration replay and
state changes/replays return `200`; reads return `200`. DTOs omit Organization
ID, actors, operation fingerprints, database rows and secrets.

List accepts only `limit` and `cursor`. Default limit is 20, maximum 100; query
text is at most 1 KiB and cursor at most 512 bytes. Ordering is immutable
`(created_at ASC, id ASC)`. The opaque canonical base64url cursor contains
version, SHA-256 of the current Organization ID, timestamp and ID; a cursor
from another Organization is rejected before SQL. SQL still scopes by
`organization_id` before the keyset predicate. Empty list is `items: []` with
`nextCursor: null` and never falls back to legacy data.

## Errors and limits

Errors use the existing Workbench envelope with stable uppercase codes and no
raw dependency text:

| Status | Code | Meaning |
| --- | --- | --- |
| 400 | `INVALID_REQUEST` | Invalid method input, headers, ID, cursor or strict JSON |
| 401 | `AUTHENTICATION_REQUIRED` | Missing/invalid/expired current identity |
| 403 | `PERMISSION_DENIED` / existing Organization denial code | Missing permission, revoked, suspended or cross-Organization |
| 404 | `SOURCE_ACCOUNT_NOT_FOUND` | Missing and foreign-Organization detail/mutation are indistinguishable |
| 409 | `IDEMPOTENCY_CONFLICT` | Same operation key with different canonical request |
| 409 | `VERSION_CONFLICT` | `If-Match` is not the current version |
| 409 | `INVALID_TRANSITION` | Fresh request targets the current state |
| 409 | `RESOURCE_LIMIT_REACHED` | The Organization already owns 100 current resources |
| 413 | `INPUT_TOO_LARGE` | Request/body bound exceeded |
| 503 | `DEPENDENCY_UNAVAILABLE` | Database or authorization dependency unavailable before a known commit |
| 503 | `OUTCOME_UNKNOWN` | PostgreSQL commit result cannot be confirmed |
| 504 | `DEADLINE_EXCEEDED` | Request cancellation/deadline wins error projection |

The complete request, including middleware and SQL, is bounded to 10 seconds.
Responses are bounded to 128 KiB. All successful and error responses use
`Cache-Control: no-store` and `X-Content-Type-Options: nosniff`.

## Persistence and initialization

The current schema owner creates exactly two new tables with names that do not
collide with the abandoned B1 target:

- `source_account_resources`;
- `source_account_operations`.

Both use Organization ID in primary/scoped keys. Database checks constrain the
platform, states, positive version, bounded identifiers/fingerprint and
timestamps. The operation table stores Organization, actor, idempotency key,
kind, request fingerprint, account ID, resulting version/status and creation
time. It is a current request receipt, not a migration receipt or second account
fact.

`public.organization_source_accounts` belongs to the cancelled ownership-
migration preparation and has an incompatible numeric/profile schema. It is
neither renamed nor altered, is never read or written by SA1, and may be absent.
Its existence does not satisfy SA1 schema validation and its absence does not
block initialization or runtime.

Schema installation is an explicit Goose history owned by
`internal/app/schema/sourceaccountregistry` and exposed only by the bounded
`source-account-registry-schema-init` operational command. It does not call the
existing product-listing or ListingKit schema owners. The independent history
uses `goose_source_account_registry_version` so other migration histories do not
interpret its versions. The initializer creates exactly the two SA1 business
tables plus its Goose metadata on an empty PostgreSQL database, is repeatable,
and rejects schema drift; it never creates, reads, renames or alters a legacy
Source Account table.

SA1 assembly verifies the exact SA1 schema read-only, constructs the adapter,
service and HTTP module from caller-supplied dependencies, and fails safely if
the database/schema is absent or invalid. The caller owns the database and
server lifecycle; SA1 registers no database closer. No `AutoMigrate`, in-memory
fallback, legacy fallback or business-request DDL is allowed.

The clean initialization and runtime commands are:

```text
go run ./cmd/source-account-registry-schema-init -config <task-owned-config>
pwsh -File ./scripts/source-account-registry-local-acceptance.ps1
```

The first command performs only explicit SA1 schema initialization. The second
command owns a disposable loopback PostgreSQL instance, starts the isolated
application through `http.Server.Serve`, issues register/list/detail/disable/
enable requests, reconstructs the application from the same database, then
calls `http.Server.Shutdown` and removes only task-owned resources. The script
is a bounded acceptance owner for the schema command, not a product launcher or
production route switch. Its server never assembles legacy Source Account or
ListingKit features. Deployment and a real environment are outside SA1
authority.

## Transaction and idempotency

Operation identity is `(organization, actor, Idempotency-Key)`. Fingerprints
use canonical length-framed JSON over operation kind plus exact public input;
state operations include account ID, expected version and target state. Same
key/same fingerprint is replay; same key/different fingerprint is conflict.

Every write uses one PostgreSQL transaction:

1. acquire a transaction-scoped advisory lock derived from the operation key;
2. read the Organization/actor-scoped operation receipt;
3. if replay, load the account by Organization and ID and return its current
   projection without writing;
4. for create, acquire an Organization-scoped capacity advisory lock, count all
   enabled and disabled resources for that Organization, reject count 100, then
   insert the server-owned account; for state change, select the Organization-
   scoped account `FOR UPDATE`, validate exact version/transition, then update
   with an Organization and version predicate;
5. insert the operation receipt;
6. commit account and receipt atomically.

The fixed `MaxAccountsPerOrganization = 100` is a resource-exhaustion bound, not
a paid-plan quota or entitlement system. Disabled resources count because SA1
has no deletion. Replays are resolved before the capacity check, so a committed
create remains replayable at the limit; the Organization lock prevents
different-key concurrent creates from exceeding it.

Concurrent same-key create yields one account. Concurrent different-key state
changes serialize on the account row; only the exact version succeeds. A
failed pre-commit transaction leaves neither an account change nor a receipt.
Commit error is `OUTCOME_UNKNOWN`; no automatic second write, alternate key or
background recovery is started. The client retries the same request/key after
fresh authentication; a durable receipt resolves a committed result. A lost
HTTP response is resolved the same way after service reconstruction.

No Saga/outbox is required because account state and receipt share one database
transaction and SA1 has no external side effect.

GET paths issue only scoped `SELECT` statements and never create operations,
touch timestamps or perform DDL.

## Required evidence

Implementation starts with failing tests and must produce:

- domain tests for validation, state/version transitions, permission and
  idempotency fingerprints;
- HTTP contract tests for exact routes/DTO/ETag/statuses, strict input and size
  bounds, empty/not-found/denied/unavailable separation;
- real PostgreSQL tests for schema on an empty database, scoped SQL,
  same/different-key concurrency, different-payload conflict, state races,
  exact Organization capacity under concurrent creates, rollback, response
  loss/replay and reconstruction;
- a mounted HTTP integration using the existing verifier, Organization resolver
  and authorizer with controlled provider fixtures, proving Home A / Effective B
  writes B, forged/cross-Organization/permission/revocation/dependency failures
  are rejected, and replay reauthorizes;
- before/after table evidence that GET performs zero business writes;
- assembly tests proving explicit schema requirement, one mounted module, no
  extra database closer and service reconstruction from the same PostgreSQL
  state, plus a controlled isolated start that performs no DDL and never calls
  default/legacy feature builders;
- guards proving the new owner does not import or call old Source Account,
  tenantbridge, root ListingKit, ownershipmigration or #366 code;
- focused/race/lint, affected shared regressions, full repository tests, final
  HEAD CI and independent full-slice review.

Real ZITADEL, real 1688 login/connection, production deployment and real data
are `NOT_RUN` / `NOT_AUTHORIZED`.

## Legacy and out of scope

```text
Legacy decision: RETIRE
Reusable behavior: only framework-neutral validation and current PostgreSQL,
authorization, HTTP and UoW patterns; no old Source Account owner behavior.
Current owner: internal/sourceaccountregistry.
Cutover/deletion condition: #301-owned bounded retirement tasks remove old
callers; SA1 adds no old caller and does not claim repository-wide retirement.
```

Out of scope: migration/backfill/rehearsal, old ID mapping, profile/cookie/token
handling, platform registration/authentication, browser/crawler/import, UI/BFF,
hard delete, transfer, member grants, proxy management, batch operations,
payment/quota systems, C/D, #30 production routing, deployment and real data.
