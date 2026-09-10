# Source Account SA-2 BFF contract

Issue: #370
Product decision: #365 / `PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08`
Candidate dependency: PR #369 @ `0a86bfc7b13353be2ab05c384ec94b94b16aae7f`
Status: IMPLEMENTATION_READY — independent admission R2 and independent
implementation review passed 2026-09-09; the SA-2 implementation candidate
passes local focused verification, while final integration evidence is not final

## Delivery boundary

SA-2 adds one browser boundary to the current SA-1 owner:

```text
source-accounts.ts (the only browser DTO/client export)
  -> contracts/source-account.ts (one browser-safe internal wire-schema owner)
  -> same-origin /api/workbench/source-accounts routes
  -> existing Auth.js serverAuth and server-only ZITADEL access token
  -> existing Effective Organization selection cookie
  -> existing strict Workbench BFF transport
  -> the five SA-1 Go routes
  -> existing authentication / Effective Organization / permission middleware
  -> sourceaccountregistry Service / Store port
  -> isolated PostgreSQL adapter
```

The existing `web/listingkit-ui/src/app/api/workbench/[...path]/route.ts` and
`web/listingkit-ui/src/lib/server/workbench-proxy.ts` remain the only Workbench
BFF and upstream transport owners. SA-2 may add Source Account route descriptors
and protocol validation there. It must not add another proxy framework, token
exchange, identity database, transport fallback, durable browser receipt store,
or a second DTO schema.

`web/listingkit-ui/src/lib/contracts/source-account.ts` is the single internal
owner of the Source Account wire schemas, bounds, IDs, cursor, ETag, and text
validators. Both the browser client and BFF consume it. It exports no competing
browser client or DTO type surface; `source-accounts.ts` remains the sole public
browser DTO/client path.

The browser never supplies an upstream URL, Authorization header, authoritative
actor, Home Organization, role, or owner field. `serverAuth` supplies one
server session from which the BFF reads both the current subject and server-only
access token. A mutation carries a non-authoritative `X-Expected-User-ID`
stale-context assertion captured with the operation intent. The BFF compares it
exactly, after the Fetch/Headers layer's standard outer-OWS normalization, with
that same `request.auth` session's current subject and never
forwards it. It separately compares the bounded
`X-Expected-Organization-ID` assertion with the single selected-Organization
HTTP-only cookie, then sends only the selected value as
`X-Requested-Organization-ID`. The Go middleware independently verifies the
bearer subject, resolves Effective Organization, rechecks live grants, and
remains the actor/Organization/authorization authority.

SA-2 does not modify Go domain, authorization, state-machine, transaction,
schema, or receipt behavior. A reproducible Go/Schema defect is returned to the
#368 owner rather than fixed here.

## Exact route map

There are exactly five paths. Browser and upstream methods are identical; the
BFF adds the existing `/api/v1` service prefix only.

| Browser route | Upstream route | Purpose | Go organization policy | Go permission |
| --- | --- | --- | --- | --- |
| `POST /api/workbench/source-accounts` | `POST /api/v1/workbench/source-accounts` | register | `live_write` | `workbench.source_account.manage` |
| `GET /api/workbench/source-accounts` | `GET /api/v1/workbench/source-accounts` | list | `cached_read` | `workbench.source_account.read` |
| `GET /api/workbench/source-accounts/:source_account_id` | `GET /api/v1/workbench/source-accounts/:source_account_id` | detail | `cached_read` | `workbench.source_account.read` |
| `POST /api/workbench/source-accounts/:source_account_id/disable` | `POST /api/v1/workbench/source-accounts/:source_account_id/disable` | disable | `live_write` | `workbench.source_account.manage` |
| `POST /api/workbench/source-accounts/:source_account_id/enable` | `POST /api/v1/workbench/source-accounts/:source_account_id/enable` | enable | `live_write` | `workbench.source_account.manage` |

No sixth status, verify, retry, legacy, or compatibility route is added.
Explicit unknown-result verification calls the same mutation function again
only after the original actor and Organization context are restored. It keeps
the exact original expected actor, expected Organization, idempotency key,
method/route including account ID, public body, and strong `If-Match`. A new
token/session for the same actor is allowed; substituting a new actor or
Organization is not a replay.

## TypeScript export and wire DTOs

The sole export path is:

```text
web/listingkit-ui/src/lib/api/source-accounts.ts
```

It exports these DTOs and functions:

```text
SourceAccount {
  id: string                 // canonical UUIDv7, opaque
  platform: "1688"
  displayName: string
  managementStatus: "enabled" | "disabled"
  connectionStatus: "pending_connection"
  version: string            // canonical positive signed-bigint decimal
  createdAt: string          // canonical UTC RFC3339
  updatedAt: string          // canonical UTC RFC3339
}

SourceAccountMutationResult {
  schemaVersion: 1
  account: SourceAccount
  replayed: boolean
  etag: string               // exact validated strong upstream ETag
}

SourceAccountDetailResult {
  schemaVersion: 1
  account: SourceAccount
  etag: string
}

SourceAccountPage {
  schemaVersion: 1
  items: SourceAccount[]
  nextCursor: string | null
}
```

Functions are `createSourceAccount`, `listSourceAccounts`,
`getSourceAccount`, `disableSourceAccount`, and `enableSourceAccount`. Each
requires the captured expected Organization ID and accepts `AbortSignal`.
Mutations additionally require `expectedActorSubject`, captured as
`WorkbenchContext.user.id` from the same verified context as the Effective
Organization, plus a caller-created canonical UUID `Idempotency-Key`.
Lifecycle calls accept the exact strong ETag returned by detail or mutation;
they do not derive it from `version`, increment versions locally, generate a
replacement key, or retry automatically.

One mutation intent fixes these values before its first call:

```text
expectedActorSubject + expectedOrganizationId + Idempotency-Key
+ method + route/account ID + original body + original If-Match
```

The caller may retain this ordinary in-memory parameter snapshot. It must not
recapture actor/Organization from a later session for verification and must not
build a persistent browser ledger. Missing or unready context means no request.
Username, email, Home Organization, and a client-authored owner field are never
valid substitutes for `WorkbenchContext.user.id`.

## Request contract

Register accepts strict `application/json`, at most 8 KiB of actual UTF-8 bytes:

```json
{"displayName":"Primary 1688","platform":"1688"}
```

It rejects missing, duplicate, unknown, case-variant, secret-like, identity,
Organization, status, connection, cookie/profile, URL, or path fields.
`displayName` is already trimmed, contains no Unicode control characters or
unpaired UTF-16 surrogate code units, and is 1–120 UTF-8 bytes. Valid Unicode
scalar values, including emoji represented by a paired surrogate in JavaScript,
remain allowed. Register has no query.

List accepts only one `limit` and one `cursor`. `limit` is a canonical decimal
1–100, default 20. Raw query is at most 1 KiB and cursor at most 512 UTF-8
bytes; a present cursor is canonical unpadded base64url and otherwise remains
opaque to SA-2. Detail accepts one canonical UUIDv7 path ID. Detail and all mutations
reject query parameters. GET and lifecycle calls require a zero-length body.

Every mutation requires a single canonical, non-nil RFC-variant UUID
`Idempotency-Key`. Lifecycle also requires one exact strong decimal ETag
`If-Match`, `"1"` through `"9223372036854775807"`. Weak, wildcard, unquoted,
zero, signed, padded, comma-combined, unsafe-integer-converted, or malformed
values are rejected before forwarding.

Source Account POSTs require the configured public application Origin and
`Sec-Fetch-Site` absent or `same-origin`. Host and forwarded headers never
establish trust. SameSite cookies alone are not the write protection.

Every mutation requires one valid `X-Expected-User-ID` assertion,
bounded by the current Workbench user-ID contract. Missing, empty, unsafe,
duplicate/comma-combined, case-folded, or numerically coerced values are
rejected with `400 INVALID_REQUEST`; the browser client also rejects an
untrimmed intent value before constructing Headers. A valid assertion different
from the same server session's subject is rejected before upstream dispatch with
`409 IDENTITY_CONTEXT_CHANGED`, without revealing the actual subject. Missing
server identity or token is `401 AUTHENTICATION_REQUIRED`. The assertion is
excluded from the upstream header allowlist, Go payload/scope/fingerprint, and
all responses.

## Response, ETag, and state truth

The BFF and client both read strict UTF-8 `application/json`, reject duplicate
or unknown fields, and enforce the SA-1 128 KiB response bound. Successful
single-resource responses require one strong ETag exactly equal to quoted
`account.version`; the exact header is returned to the caller. Neither layer
converts `version` or ETag digits to JavaScript `Number`.

The only success statuses are:

- register: `201`, or `200` when `replayed=true`;
- list/detail/enable/disable: `200`.

Resource IDs are canonical UUIDv7 strings. Versions remain canonical positive
decimal strings within signed-bigint range. `schemaVersion` is exactly numeric
`1`; pages contain at most 100 items and a bounded opaque cursor.
Detail and lifecycle responses must contain the exact path account ID. The BFF
and browser client both reject a mismatched ID; Create has no pre-existing path
ID and does not apply this check.

`managementStatus` and `connectionStatus` are independent. `enabled` never
means connected, authenticated, verified, usable, or ready to crawl.
`pending_connection` remains visible even when management is enabled. A replay
returns the current account projection plus `replayed=true`; it does not
reapply the historical transition or resurrect later-disabled state.

All BFF responses are `no-store` and `nosniff`. It forwards no Set-Cookie,
Location, server, dependency text, secret, actor, Organization fact, or raw
upstream header. Existing Organization denial/revocation handling may clear the
selected Organization cookie.

## Errors and mutation outcome

The stable SA-1 status/code pairs are preserved:

| Status | Code |
| --- | --- |
| 400 | `INVALID_REQUEST` |
| 401 | `AUTHENTICATION_REQUIRED` |
| 403 | `PERMISSION_DENIED`, existing Organization denial/revocation/suspension codes |
| 404 | `SOURCE_ACCOUNT_NOT_FOUND` |
| 409 | `ORGANIZATION_SELECTION_REQUIRED`, `IDEMPOTENCY_CONFLICT`, `VERSION_CONFLICT`, `INVALID_TRANSITION`, `RESOURCE_LIMIT_REACHED`, BFF `ORGANIZATION_CONTEXT_CHANGED` / `IDENTITY_CONTEXT_CHANGED` |
| 413 | `INPUT_TOO_LARGE` |
| 503 | `DEPENDENCY_UNAVAILABLE`, `OUTCOME_UNKNOWN` |
| 504 | `DEADLINE_EXCEEDED` |

`SourceAccountAPIError` retains safe `status`, `code`, `requestId`, the parsed
error `payload`, and `outcome: not_sent | rejected | unknown`.

- local validation or an already-aborted signal is `not_sent`;
- a valid response guaranteed to have been generated before Go dispatch,
  including `IDENTITY_CONTEXT_CHANGED`, rejects only the current attempt;
- another valid pre-commit rejection is `rejected`;
- backend `OUTCOME_UNKNOWN`, mutation deadline/cancellation after dispatch,
  response loss, malformed/oversized mutation response, or BFF/client transport
  loss after dispatch is `unknown`.

The BFF never says that an unknown write rolled back or was not committed.
Cancellation stops waiting, not the PostgreSQL transaction. No mutation is
automatically retried by fetch policy, reconnect/focus handling, timeout code,
or this client. The only supported resolution is a caller-explicit call with
the complete original intent after the original actor and Organization are
restored; Go reauthorizes every call and loads the current projection when its
durable receipt exists. If no receipt exists, that original actor may complete
the write once. A mismatch rejection proves only that the current attempt was
not dispatched; it never changes the first attempt from `unknown` to rejected,
not-sent, or rolled-back.

One request-scoped 15-second BFF deadline begins before `serverAuth`, links the
incoming abort signal, and is shared through authentication, body validation,
upstream dispatch, response read, and projection. A shared dispatch marker is
set only immediately before the Go fetch. An abort before dispatch returns
`504 DEADLINE_EXCEEDED` without a Go call. An abort after mutation dispatch is
`503 OUTCOME_UNKNOWN`. Authentication finishing after deadline must recheck the
shared signal and cannot start a background POST after the BFF has replied.

Synthetic failures are fixed as follows:

| Boundary | Read result | Mutation result |
| --- | --- | --- |
| deadline/cancel before Go dispatch | `504 DEADLINE_EXCEEDED` | `504 DEADLINE_EXCEEDED`, current attempt not dispatched |
| network failure after Go dispatch | `502 DEPENDENCY_UNAVAILABLE` | `503 OUTCOME_UNKNOWN` |
| deadline/cancel after Go dispatch | `504 DEADLINE_EXCEEDED` | `503 OUTCOME_UNKNOWN` |
| malformed, wrong content-type, oversized, wrong-status, or mismatched-ID upstream success/error | `502 INVALID_UPSTREAM_RESPONSE` | `503 OUTCOME_UNKNOWN` |

Client-side loss after a mutation fetch begins is conservatively `unknown`
because the browser cannot prove whether the BFF dispatched to Go. A legitimate
backend `503 DEPENDENCY_UNAVAILABLE` remains `rejected`; a legitimate backend
`503 OUTCOME_UNKNOWN` or mutation `504 DEADLINE_EXCEEDED` remains `unknown`.

## SA-2 implementation evidence

Independent architecture admission R2 and the post-implementation review both
passed with no `BLOCKER`. The implementation review classified and closed three
`IMPLEMENTATION_TEST` findings: Source Account operation UUIDs no longer reuse
the Store v1-v5 validator; client and BFF consume one shared internal schema
owner; and unpaired UTF-16 surrogate inputs are rejected before dispatch while
valid non-BMP characters remain accepted. The Store validator was not changed.

Focused Source Account, route/deadline, and shared same-origin regression tests
pass (`4` files, `79` tests). The independent reviewer also passed a wider
`6`-file, `329`-test slice. The full frontend suite passes (`272` files,
`2032` passed, `1` skipped); TypeScript typecheck, target and full ESLint
(`0` errors, `5` pre-existing scope-external warnings), the Next production
build, and `go test ./tests -count=1` pass locally. These are unit/BFF contract
results only. They do not substitute for the final empty-PostgreSQL real-chain
matrix below.

## Upstream lifecycle and dependency gate

PR #369 is still OPEN and unmerged at this contract's candidate SHA. SA-2 unit
and BFF contract tests against its fixed contract are DEVELOPMENT evidence, not
final Go integration. Final merge admission requires all of:

1. PR #369 is actually merged after approval;
2. the merge commit is present on `main` and matching `event=push`,
   `branch=main` CI succeeds;
3. the #368 owner provides the final five routes, schema initializer, isolated
   start/check/stop commands, and final HEAD/merge handoff;
4. SA-2 consumes fresh main normally and proves the real
   `Next -> serverAuth/Effective Organization -> strict BFF -> Go auth ->
   Registry/service/UoW -> empty task-owned PostgreSQL` chain.

The final fixture must initialize an empty database only through the SA-1
initializer and start `NewSourceAccountApplication` through normal application
assembly. It must create every Source Account through HTTP, never by seeding
`source_account_resources`. One fixture owner controls PostgreSQL, loopback
ports, processes, checks, cancellation, shutdown, and cleanup. A controlled
identity/grant provider is labelled a test substitute; real ZITADEL and real
1688 remain `NOT_RUN`.

The final matrix includes actor A and actor C, both with manage permission in
Organization B: A's Create commits and loses its response; C presents A's
original intent and receives pre-dispatch `409 IDENTITY_CONTEXT_CHANGED` while
Go call count, account rows, and receipt rows do not change; a fresh session for
A then presents the unchanged intent and receives `200` with `replayed=true`,
with one account total. A second case proves that when A's original request did
not commit, restored A can complete it at most once. The same
actor/Organization assertion gate covers Create, Enable, and Disable, plus
missing/duplicate/forged assertions, Organization changes, new same-actor
tokens, revocation, path/body/key/If-Match drift, and late responses.

## Receipt-growth finding and non-goals

R-SA1 finding `3962175378` is classified `BACKLOG`, solution B, for the current
isolated slice. Successful fresh register/enable/disable operations append one
durable receipt; same-key replay and failed operations append none. The
100-account bound limits registration, not receipts. Shared/production
retention, old-key lifetime, rate/limit behavior, alerting, and ownership remain
undecided rollout work.

SA-2 must report this as a production gate and must not call it fixed,
accepted-risk, or production-safe. Neither #369 nor #370 adds arbitrary TTL,
receipt row caps, archival, rate limiting, deletion, or a general cleanup
framework. Evidence that replay itself grows rows or that current bounded
acceptance exhausts capacity is returned to #368/architecture for
reclassification.

Out of scope: page/navigation work, real 1688 login, profile/cookie/token
capture, crawler/import, historical migration or old-ID mapping, old Service
compatibility, fallback, dual read/write, production composition/deployment,
payment, shared/real data, and repository-wide legacy cleanup.

```text
Legacy decision: RETIRE
Reusable behavior: current Auth.js session, server-only token, Effective
Organization cookie/assertion, strict Workbench transport, and protocol parsers.
Current owner: sourceaccountregistry behind the existing Workbench BFF.
Cutover/deletion condition: #301 retires old Source Account callers separately;
SA-2 adds no dependency on them and claims no repository-wide cutover.
```
