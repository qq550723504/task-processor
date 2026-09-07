# Account read-only contract (#346 → #348)

Status: implemented after first independent admission (IMPLEMENTATION_READY). This document is the single wire
contract. Implemented commit and verification evidence belong in the Issue/PR.

## Product boundary

Must: expose only the verified caller's profile and resolved Effective Organization;
keep Home separate; no business/identity writes; retain current authorization,
switch/audit and serverAuth/token ownership; bounded, cancellable reads and safe
errors. No query may select another user or arbitrary organization.

Should: explicit missing fields and source/freshness, exact reusable TS client.
Out of scope: UI/Shell, IAM/member/account changes, enterprise certification,
administrator identity, member/store/invitation counts, financial resources,
tenantbridge, persistence, production and real IAM operations.

Existing accepted risk: ordinary organization grants follow the approved
CachedRead window of at most 60 seconds, bounded by token expiry (multi-org spec
section 18). This is not an immediate role-revocation guarantee. Userinfo is an
observation at request time, not a transactional snapshot with grants or a promise
of provider-internal replication freshness. No additional risk is accepted.

Threat model: forged identity/scope, cross-user/org responses, expired/revoked
authorization, profile claims without provider disclosure, late/cancelled results,
oversized requests/responses, unsafe errors and dependency failures disguised as
missing data. No new IAM, retry/recovery state or account cache is introduced.

## Routes and types

Only GET `/api/v1/account/profile` and `/api/v1/account/organization` in Go;
browser GET `/api/account/profile` and `/api/account/organization` in dedicated
Next routes. No query or body. Other browser methods return 405 INVALID_REQUEST.
The existing Workbench catch-all remains unchanged.

Go exported DTOs in `internal/workbenchcontext/httpapi/account.go`:

```go
type AccountProfile struct {
    SchemaVersion string  `json:"schemaVersion"` // account-v1
    UserID string         `json:"userId"`
    HomeOrganizationID string `json:"homeOrganizationId"`
    DisplayName *string   `json:"displayName"`
    Email *string         `json:"email"`
    EmailVerified *bool   `json:"emailVerified"`
    PhoneNumber *string   `json:"phoneNumber"`
    PhoneNumberVerified *bool `json:"phoneNumberVerified"`
    Source string         `json:"source"` // zitadel_userinfo
    ReadAt string         `json:"readAt"` // UTC RFC3339Nano projection time
}
type AccountOrganization struct {
    SchemaVersion string  `json:"schemaVersion"` // account-v1
    UserID string         `json:"userId"`
    HomeOrganizationID string `json:"homeOrganizationId"`
    EffectiveOrganizationID string `json:"effectiveOrganizationId"`
    Name *string          `json:"name"`
    Roles []string        `json:"roles"` // actual selected project grant only
    Source string         `json:"source"` // zitadel_project_authorizations
    ReadAt string         `json:"readAt"` // projection time, NOT grant fetched-at
    AuthorizationMaxAgeSeconds int `json:"authorizationMaxAgeSeconds"` // 60
}
```

All keys are present. Optional claims are JSON null, never guessed from login
name/session. Null means the provider did not disclose a value this request;
missing scope and absent profile attribute cannot be distinguished by userinfo,
so consumers must say "not provided", not "not bound". A verification boolean
is null if its corresponding address/number is null. No avatar URL in this slice.
Technical addresses ending in `@phone.invalid` (case-insensitive) are suppressed:
email and emailVerified are both null, per account-entry native-flow spec §14.

Unique TS module: `web/listingkit-ui/src/lib/api/account.ts`.
Public exports: `AccountProfile`, `AccountOrganization`, `AccountReadError`,
`getAccountProfile`, `getAccountOrganization`.
The same module exports `parseAccountPayload` and `accountErrorCode` for BFF wire
validation; these are shared validators, not a second client/DTO or UI API.

```ts
getAccountProfile(options: {
  expectedUserId: string; signal?: AbortSignal;
}): Promise<AccountProfile>
getAccountOrganization(options: {
  expectedUserId: string; expectedOrganizationId: string; signal?: AbortSignal;
}): Promise<AccountOrganization>
// AccountReadError extends Error: readonly status: number; readonly code: string
```

Client sends identity assertions only for equality checks, never authorization.
Profile uses X-Expected-User-ID; organization additionally uses
X-Expected-Organization-ID. BFF compares user with its server session identity
and projected upstream user; compares organization with the existing HttpOnly
selection cookie and projected upstream EffectiveOrganizationID. Only that cookie
supplies the Go X-Requested-Organization-ID selector. Browser bearer/identity/org
headers are not forwarded. Profile ignores organization selection entirely.

## Field provenance and errors

| Field | Trusted authority / read permission | Actual call | Missing/revocation/switch/dependency semantics |
| --- | --- | --- | --- |
| profile.userId, homeOrganizationId | verified token subject/resource owner; current caller | existing zitadel.Verifier.Verify → introspection | invalid token 401; provider failure 503; unaffected by selected org |
| profile.displayName | current caller's granted profile claims | new minimal authruntime/zitadel UserInfoClient → GET issuer/oidc/v1/userinfo with same user bearer | omitted/blank→null; denied→403; invalid token→401; failure→503; mismatched sub→502; never session fallback |
| profile.email, emailVerified | current caller's granted email claims | same userinfo call | same; no email→both null; do not add scope |
| profile.phoneNumber, phoneNumberVerified | current caller's granted phone claims | same userinfo call | same; default login does not request phone, therefore it may be null; do not add scope |
| organization.userId, homeOrganizationId | verified current identity | same existing introspection | 401/503 as above; Home never reinterpreted as selected enterprise |
| organization.effectiveOrganizationId | server resolver validates current selected org against project grants | existing Resolver.Resolve(CachedRead) | no selection assertion→409; unknown selection→403; 0 grants→403 revoked; dependency→503; no fallback |
| organization.name | selected grant.organization.name | existing AuthorizationClient.ListOwnProjectAuthorizations | blank→null; role revoked follows <=60s cache; switch live-invalidates; unavailable uncached→503 |
| organization.roles | same selected project grant; own membership/context visibility | same resolver, exact selected grant | no global/platform-admin synthesis; missing grants denied; no membership or billing permission inferred |
| schemaVersion, source, readAt, authorizationMaxAgeSeconds | local contract metadata, not business facts | projection constructor | readAt is projection time; maxAge describes policy, not a cache hit age |

Enterprise certification, administrator name, counts and resources have no
approved reader in this slice and are absent from the contract. Existing business
status checker is nil in default assembly; this response does not claim verified
enterprise lifecycle/certification. No new role permission is invented for reading
one's own project grant/context. UI must treat roles as presentation only.

## Authentication assembly and admission

Current VerifiedIdentity + None routes use the old allowlist middleware;
ContextRead first loads grants and can reject stale selections, so neither is a
valid independent personal-profile entry. Add an explicit CurrentIdentity auth
policy in httproute, handled by the existing workbenchAuthenticationMiddleware in
app/httpapi. Profile uses it with None; organization with CachedRead. Existing
routes keep their policies. Shared assembly edits are announced to coordination.
Personal context clears legacy TenantID/roles/Effective/grants, retaining only
verified subject, Home and token expiry. #348 owns the approved minimal profile
page Shell gate exception; #346 does not edit Shell/provider.
Account methods live in the current workbenchcontext HTTP owner; profile reader
is a narrow port and adapter in current authidentity/authruntime owners. The
existing workbench module composes the two dedicated routes and reader.

Use a descriptor request timeout (15 seconds) applied before auth by server route
assembly, only opted in by these two routes. This is a request context deadline,
not a scheduler/retry framework. Userinfo uses a fixed server issuer, 5-second HTTP
timeout, no redirects, 16 KiB actual body bound and exact subject binding. No
management credential, persistence, Ensure/Create or authorization mutation.

## Error and resource contract

Every account error: `{code,message,requestId,fieldErrors:[]}`; message is a local
safe constant, requestId is empty for account projection/BFF errors. Existing Go
auth errors use the existing envelope. BFF never echoes raw upstream messages.

| HTTP | code |
| --- | --- |
| 400 | INVALID_REQUEST |
| 401 | AUTHENTICATION_REQUIRED |
| 403 | PERMISSION_DENIED, ORGANIZATION_ACCESS_DENIED, ORGANIZATION_ACCESS_REVOKED, ORGANIZATION_SUSPENDED |
| 405 | INVALID_REQUEST |
| 409 | IDENTITY_CONTEXT_CHANGED, ORGANIZATION_CONTEXT_CHANGED, ORGANIZATION_SELECTION_REQUIRED |
| 502 | INVALID_UPSTREAM_RESPONSE, DEPENDENCY_UNAVAILABLE (BFF transport failure) |
| 503 | ACCOUNT_NOT_CONFIGURED, DEPENDENCY_UNAVAILABLE |
| 504 | DEADLINE_EXCEEDED |

Missing selected cookie is ORGANIZATION_SELECTION_REQUIRED; malformed/duplicate
cookie or mismatched expected selector is ORGANIZATION_CONTEXT_CHANGED. No cookie
is written by account GETs (late responses must not erase a new selection).
When Workbench is disabled Go does not register these routes and returns 404.
For these two fixed GET paths only, BFF maps upstream 404 to 503
ACCOUNT_NOT_CONFIGURED (capability not provided), cancelling the upstream body.
It does not interpret this as an absent user or diagnose a particular flag.
No auto-retry, no persistent/browser/query cache in this client. Success and
failure use private,no-store and nosniff. Consumer clears sensitive results on
logout/user/org/role transitions, aborts prior requests and rejects late epochs;
client validates returned identity/scope and checks cancellation after body read.

Request: zero body/query bytes; user/org assertions 1..128 ASCII ID characters;
no arbitrary user/org URL input. Profile/name/email/phone strings <=512 UTF-8
bytes, roles <=32 entries each <=128 bytes; IDs <=128 bytes. Response <=16 KiB
actual serialized/read bytes at Go/BFF/client. Timestamps <=40 ASCII bytes.
BFF total 15s includes serverAuth and body read; Go total 15s includes auth/grants;
userinfo HTTP <=5s. Existing introspection/grants retain their own 1 MiB/1000
authorization limits inside that deadline. BFF fixed origin from existing
LISTINGKIT_SERVICE_API_BASE (must be an HTTP(S) origin plus `/api/v1`; no
credentials/query/fragment; absent configuration is ACCOUNT_NOT_CONFIGURED);
manual redirect/no-store and
restricted headers. No client-selected origin.

## Verification plan

TDD first missing-route/reader/client tests; then real Go server assembly with
actual verifier, AuthorizationClient, GrantResolver, selection and structured
audit, plus controlled external OIDC/authorization HTTP provider. Browser fixture
runs actual Next route/serverAuth/token helper → Go → external substitute. It
does not stub the final account DTO. No PostgreSQL required.
Cover A Home/B Effective, wrong subject, no grants, no selection, switched user,
live switch/cache invalidation, revoke cached/expired policy, userinfo scopes and
provider error/slow/oversized/redirect responses, cancellation/late results and
headers. Run related auth/context/store/BFF regressions, race, type/lint/build,
architecture/depguard and final-SHA independent review. Real IAM/login/provider
configuration and production are NOT_RUN; fixture identities/tokens only.

Authority: Issue #346/#348, root AGENTS, Issue-driven V1,
`docs/architecture/auth-and-tenancy.md`, multi-org spec sections 8/13/17/18/19,
`docs/product/final-ui-ia-authority.md`.
Provider contract: https://zitadel.com/docs/apis/openidoauth/endpoints and
https://zitadel.com/docs/apis/openidoauth/scopes (read 2026-09-07).

## Reproducible isolated acceptance and #348 handoff

From `web/listingkit-ui` after `pnpm install --frozen-lockfile`:

```sh
node scripts/account-fixture.mjs
node scripts/account-fixture.mjs --serve
# Optional: run the Next server from an isolated #348 combination checkout:
node scripts/account-fixture.mjs --serve --web-dir <absolute-ui-directory>
```

The first command compiles the actual Go HTTP package test binary, starts a
loopback external-provider substitute, assembles the actual current verifier,
authorization HTTP client, grants cache/resolver, audit and account routes, starts
Next, then compiles and calls the real TypeScript client. Only its browser
origin/cookie transport is supplied by the Node harness; responses are never
replaced. The 12 groups verify reads, Home A/Effective B, live switch to C, wrong
user/organization, userinfo subject mismatch, no membership, grant outage,
profile outage, token revocation, cached/expired grant revocation, cancellation.
Go and Vitest tests additionally exercise the error and byte/deadline boundaries.

`--serve` prints a task-specific Temp manifest path for browser acceptance. It
contains synthetic encrypted Auth.js cookies (real auth helper decodes them),
loopback origins and test controls. Treat the manifest as private; never commit
or paste session values. No real login, production/IAM data or database is used.

| Session | Meaning |
| --- | --- |
| u1, u2 | Different subjects, Home A, project roles in B/C; no Home grant |
| no-org | Valid userinfo with zero memberships |
| grant-down | Valid profile, authorization upstream unavailable |
| down | Profile upstream unavailable |
| mismatch | Provider userinfo returns a different subject; rejected |
| expired | Introspection reports inactive token; rejected |
| slow | Userinfo deliberately delayed for cancellation/deadline checks |

The manifest's `controlOrigin` is a separate test-only loopback listener, never a
production route. POST `/revoke` removes all fixture grants; `/restore` restores
them; `/expire` advances only the injected grant-cache clock 61 seconds;
`/unavailable` makes the authorization substitute unavailable. Switch uses the
real BFF `PUT /api/workbench/context/effective-organization` with JSON
`{ "organizationId": "B" }` or C. Account GET never changes the selection cookie.

Stop normally by writing a `stop-fixture` file in the printed `controlDirectory`,
or send SIGINT to the launcher. It stops only its spawned Next PID tree, asks its
own Go test to return PASS, checks the port is released, and writes cleanup.json.
There is a 27-minute serve limit. It never deletes worktrees, branches or data.
The regular run writes report.json and cleanup.json into its private Temp folder.
The Go fixture also executes an actual self read in ordinary CI without external
fixture environment; it does not claim the Next/client acceptance ran there.

NOT_RUN: real IAM, real login/provider issuance, production deployment and UI
rendering/design acceptance (#348). Local source, CI and final independent review
evidence are distinct and recorded against the pushed SHA in the PR.

## Review dispositions

First independent admission: IMPLEMENTATION_READY, no remaining BLOCKER.
The rejected alternative of coupling profile to ContextRead was excluded before
coding. The five admission IMPLEMENTATION_TEST items were addressed with explicit
selection, before-auth timeout and cancellable discovery gate, subject assertions,
technical email null mapping and cache/switch/cancellation tests.

Working-tree review: two IMPLEMENTATION_TEST findings were reproduced and fixed:
oversized auth-stage requestId is bounded before authentication; malformed JSON,
wrong content type and oversized client responses map to INVALID_UPSTREAM_RESPONSE.
Neither finding expands the threat model or reopens architecture design.
A final configuration-path check additionally classified unregistered account
routes as IMPLEMENTATION_TEST: two failing plain-text 404 tests now verify the
fixed capability-not-configured mapping before JSON parsing.
