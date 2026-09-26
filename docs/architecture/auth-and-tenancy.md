# Auth and Tenancy Context

> Status: supporting architecture context.
>
> CURRENT STATE: the bounded account/context/member interface map below was checked against `main @ 7dc9a19a5043169cb4e014909da0549aa2a921cc` (2026-09-26). Earlier context was inspected at `cae67730c5c0e645d708cb2f6814f14781962bb1` (2026-09-05). Neither observation is production acceptance or a repository-wide authorization audit.
>
> Scope: Shuomi authentication, authorization and Organization context; remaining ListingKit paths are implementation debt, not target product authority.

Quick entry: [current interface and permission map](#auth-interface-map). This is a reading map of existing contracts, not a new wire specification or implementation gate.

## 1. Purpose

Shuomi reuses its current identity and Organization contracts. Authentication, Organization selection, action authorization and resource ownership must stay separate. User-owned identity data is not automatically Organization-owned; store, source and other business resources keep their own verified ownership boundaries.

This supporting document explains the current
[Identity / Organization contract](../superpowers/specs/2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md)
and [server auth assembly](../../internal/app/httpapi/server_auth.go). It does not
introduce IAM policy. Package rules remain in `project-boundaries.md` and the
[Legacy Register](../refactoring/legacy-register.md).

## 2. Current high-level model

For routes with an OrganizationAccessPolicy requiring resolution:

```text
authenticated UI/BFF forwards bearer token
  -> Go ZITADEL verifier verifies token and constructs identity
  -> route policy / target resolver selects and validates effective Organization
  -> organization-scoped role authorization
  -> domain access checks and Organization-scoped repositories
```

The Go API discards supplied identity/tenant/role headers before Organization
resolution. `X-Requested-Organization-ID` is a selector, not authority; a route
target resolver can select the target instead. Successful resolution writes
`EffectiveOrganizationID` into verified context and downstream tenant headers.
Missing identity, denied/revoked access or unavailable required dependencies
fail closed under the route policy; no global tenant fallback is allowed.

Routes without Organization resolution do not all use legacy authentication.
`CurrentIdentity` uses the current verifier and clears business Organization/role
context for personal operations; `CurrentIdentityWithVerifiedRoles` retains the
verified roles for explicitly admitted identity-scoped routes. Other retained
routes may still use the legacy allowlist/middleware path. Read the actual
AuthPolicy, OrganizationAccessPolicy and injected middleware together; do not
replace one policy with another as a documentation fix. Public/OPTIONS behavior
also remains descriptor-specific.

## 3. Identity and tenant concepts

### User identity

User identity answers: who is calling the system?

The internal [AuthenticatedIdentity](../../internal/authidentity/authenticated_identity.go)
contains verified subject, Home/Effective Organization, effective member/grant
identity, scoped roles/grants and token expiry. Display name, email and phone are
separate UserInfo/self-service projections, not extra authentication authority.
Do not serialize the internal principal as the public Workbench Context DTO.

### Tenant identity

Tenant identity answers: which business boundary owns this request?

`resourceowner:id` / HomeOrganizationID identifies account ownership. It is
not the current business tenant in the multi-Organization model. Business scope
comes from the server-verified EffectiveOrganizationID, checked against project
grants, organization roles and the route's access policy. A user can have access
to multiple Organizations with different roles; home ownership is not a grant
to access another Organization. ZITADEL remains the identity/authorization source.

Tenant identity must be explicit when accessing:

- remaining legacy ListingKit tasks and Studio state (not new-feature owners),
- uploaded files and image assets,
- store and subscription configuration,
- source import records,
- marketplace credentials or store context,
- generated listing packages,
- submission and recovery state.

### Authorization

Authorization answers: what can this caller do in this tenant?

Authorization should be evaluated at the route/module/service boundary before mutating tenant-owned state. Route authorization belongs in HTTP/API assembly and auth middleware; business services should still avoid trusting unauthenticated or missing tenant context.

## 4. Package ownership expectations

### HTTPAPI auth/runtime packages

Current relevant area:

```text
internal/app/httpapi/server_auth.go
internal/authruntime/zitadel
internal/workbenchcontext
internal/authidentity
```

Auth-related files in this area are expected to own:

- middleware construction,
- bearer-token verification and verified identity propagation (untrusted identity headers are not authority),
- route authorization wiring,
- role/allowlist parsing helpers,
- auth runtime configuration,
- request context injection.

They should not own:

- marketplace business policy,
- product source normalization,
- ListingKit task persistence ordering,
- platform publish rules.

`internal/listingkit/httpapi` still supplies ListingKit-specific route/role
helpers to assembly. That retained code is CURRENT STATE, not a permanent
new-feature facade; extraction follows the Legacy Register.

### Tenant context packages

Current relevant areas include:

```text
internal/authidentity
internal/shared/tenantctx
```

Tenant context utilities should own:

- typed tenant/user context propagation,
- propagation of the already verified effective Organization into established tenant-aware contracts,
- narrow helpers for extracting tenant state from request context.

They should not own:

- route authorization policy,
- product or marketplace rules,
- external auth provider runtime construction.

`internal/tenantbridge` is separately classified as drain-only debt: it maps
current Organizations to legacy numeric tenant identifiers for remaining callers.
It is not an Organization membership authority or a utility for new code.
No new consumer is allowed; owning domains use current identity directly and
RETIRE remaining legacy callers. Under **PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08**,
legacy profile preservation is not a current requirement; current source-account
ownership and authorization remain with their current owners.

### Domain and repository layers

Domain services and repositories should treat tenant identity as part of their contract when the underlying data is tenant-owned.

They should not:

- query tenant-owned data without tenant criteria unless the operation is explicitly system-scoped;
- infer tenant from mutable user input when a trusted context value exists;
- fall back to a default tenant silently;
- mix tenant bridging with marketplace publish policy.

## 5. Request propagation rule

A tenant-aware request should preserve these values through the call chain:

```text
Authenticated user identity
Verified effective Organization (Home Organization retained separately)
Authorization decision or role context
Correlation/request id when available
Source/store/task identifiers scoped to that tenant
```

If a handler receives a task, batch, store, source, or uploaded asset id, it should assume the id is not globally safe by itself. The request must still be evaluated through tenant-aware access control.

## 6. Data isolation expectations

Tenant-owned data includes:

- tasks and submission state,
- Studio batches, items, attempts, and designs,
- uploaded files and generated assets,
- store credentials and store configuration,
- subscription/customer state,
- source import state,
- marketplace publish records,
- operator review and repair state.

System-scoped data may exist, but it should be explicitly named and reviewed. Examples might include shared platform descriptors, static route descriptors, or global health metadata.

Do not make data system-scoped merely because it is convenient for a runtime adapter.

## 7. Stop lines

Do not:

- accept tenant id from arbitrary request body fields when a trusted auth context should supply it;
- silently default to a global tenant for tenant-owned operations;
- let marketplace packages parse HTTP auth/session details;
- let product source normalization own auth provider concerns;
- let app/runtime packages own business authorization policy;
- add tenantbridge callers, infer Organization from numeric IDs, or bypass tenant-aware repository contracts;
- add broad auth behavior to root `internal/listingkit` when it belongs in HTTPAPI/auth runtime or tenant context utilities.

## 8. Review checklist

Before merging auth or tenant-sensitive changes, check:

```text
[ ] The route has an explicit authentication and authorization posture.
[ ] Effective Organization comes from server verification/resolution, not home/resource-owner or an unvalidated selector.
[ ] Tenant-owned reads/writes include tenant criteria or an explicitly reviewed system-scope reason.
[ ] Marketplace/product/source packages do not parse HTTP auth details directly.
[ ] Runtime assembly does not own marketplace or product authorization policy.
[ ] Existing legacy paths are identified as drain debt; new code adds no tenantbridge consumer.
[ ] Tests cover denied/missing/wrong-tenant access when the path mutates or reveals tenant-owned state.
```

Current code evidence includes
[server auth tests](../../internal/app/httpapi/server_test.go),
[Organization resolver](../../internal/workbenchcontext/resolver.go) and
[dated multi-Organization verification](../verification/zitadel-multi-organization-authorization.md).
The dated verification is evidence for its own baseline, not approval of today's HEAD.

## 9. Upgrade path to stable boundary document

This document can be promoted from supporting context to stable boundary document when:

- active guard tests cover the main tenant-boundary rules;
- `docs/architecture/README.md` moves it from Supporting Context to Stable Boundary Documents;
- `docs/architecture/architecture-review-checklist.md` lists it as a formal review reference;
- package-specific auth/tenant tests are named in the document.

Until then, use it as shared context and keep formal package/dependency authority in the existing stable boundary documents.


<a id="auth-interface-map"></a>

## 10. Current authentication and authorization interface map

This section is a bounded navigation aid for the inspected baseline, covering
account identity, Workbench context and member-management examples. Route existence
in source does not prove that a module is enabled in a deployed application.
See [current application assembly](../../internal/app/httpapi/current_application.go)
and [Repository Structure](../development/repository-structure.md#current-entrypoint-map).
Existing wire contracts, route descriptors and domain checks remain authoritative;
this map neither merges endpoints nor changes DTOs, permissions or error semantics.

### 10.1 Existing owners and approved contract entrypoints

| Concern | Existing owner / entrypoint | Consumer rule |
| --- | --- | --- |
| Login and browser session | [approved Login Phase1 decision](../superpowers/specs/2026-09-05-shuomi-login-phase1-zitadel-native-simplification.md), official ZITADEL Login V2 and existing Auth.js | Continue the approved generic `/login` flow; this task adds no Go login/refresh/logout or OTP-primary API. |
| Request authentication | [ZITADEL verifier](../../internal/authruntime/zitadel/verifier.go), [server auth assembly](../../internal/app/httpapi/server_auth.go) | Verify the request bearer; supplied user/tenant/role headers are not identity proof. |
| Verified principal | [authidentity](../../internal/authidentity/authenticated_identity.go) | `UserID` is the caller, Home is identity ownership, Effective is verified business scope, `EffectiveMemberID` is the selected authorization/grant identity, not a second user ID. |
| Organization resolution | [Resolver](../../internal/workbenchcontext/resolver.go), [selection policy](../../internal/workbenchcontext/context.go), [Identity / Organization contract](../superpowers/specs/2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md) | A selector chooses a candidate, not a permission grant; apply the route's freshness policy. |
| Personal and Organization reads | [account read-only contract](../engineering/account-readonly-contract.md) | Reuse the existing `account-v1` fields, BFF/client assertions, cancellation, null values and errors; do not publish a second wire contract here. |
| Personal identity mutations | [account identity HTTP module](../../internal/app/httpapi/account_identity.go), [self-service adapter](../../internal/authruntime/zitadel/self_service.go) | Only the authenticated user's official provider self-service; no administrator credential or client-selected target user. Current HTTP placement is recorded, not moved. |
| Member actions | [membership contract](../engineering/issue410-membership-contract.md), [route module](../../internal/organization/membership/httpapi/module.go) | Existing `authz` checks and membership service enforce project/Organization/actor/target/operation scope; no second RBAC. |

### 10.2 Account and Workbench routes

| User operation | Browser/BFF entry | Go API | Actual AuthPolicy / OrganizationAccessPolicy | Additional boundary |
| --- | --- | --- | --- | --- |
| Read own identity summary | `GET /api/account/profile` | `GET /api/v1/account/profile` | `CurrentIdentity` / `None` | No selected enterprise needed. UserInfo subject must match caller; missing claims stay null, not invented. |
| Read selected enterprise | `GET /api/account/organization` | `GET /api/v1/account/organization` | `CurrentIdentity` / `CachedRead` | Explicit bounded selector required; own selected grant is not member-management permission. |
| Read choices/current context | `GET /api/workbench/context` | `GET /api/v1/workbench/context` | `VerifiedIdentity` / `ContextRead` | Returns existing context projection; not the internal principal. |
| Select enterprise | `PUT /api/workbench/context/effective-organization` | `PUT /api/v1/workbench/context/effective-organization` | `VerifiedIdentity` / `LiveSwitch` | Body `organizationId` is a candidate. Body/header mismatch is rejected; a switch grants no new membership. |

Evidence: [Go descriptors](../../internal/workbenchcontext/httpapi/module.go),
[Go context handler](../../internal/workbenchcontext/httpapi/handler.go),
[Workbench BFF](../../web/listingkit-ui/src/app/api/workbench/%5B...path%5D/route.ts),
and the account read-only contract above. `VerifiedIdentity` and `CurrentIdentity`
are actual different enum choices, not interchangeable labels.

For account reads, the browser's `X-Expected-User-ID` and, when applicable,
`X-Expected-Organization-ID` are equality assertions. The BFF checks them against
its server session/selection and the response; they do not confer authority.
The existing selection cookie supplies the Go selector, and Go revalidates the
grant. Browser bearer/user/role headers are not forwarded as trusted context.

For generic ContextRead, an absent selector may select Home **only if Home has
an actual matching grant**, otherwise a sole grant may be selected. An explicitly
requested unauthorized Organization is rejected, never silently replaced.
Account Organization GET is stricter: it requires the explicit selector and does
not invoke that defaulting behavior. With ContextRead and no selection, zero
grants can yield an empty context; `selectionRequired` is true only when no
Effective Organization is selected and there are multiple choices. Do not turn
that into an invented membership or deny independent personal-profile access.

The public Context DTO remains `user.id`, `homeOrganizationId`, nullable
`effectiveOrganizationId`, `selectionRequired` and `organizations` with existing
roles/capabilities. It does not gain `memberId`, token expiry, raw grants or a new
schema just because the internal principal has those fields. UI roles and
capabilities are projections; backend permissions are checked again per action.

### 10.3 Current-user self-service routes

All rows use browser prefix `/api/account/identity` and Go prefix
`/api/v1/account/identity`, with `CurrentIdentity` and no Organization resolution.
Neither selected-enterprise membership nor enterprise-admin status is a new
precondition or a way to edit another person's identity.

| Method and suffix (same on both prefixes) | Existing provider operation |
| --- | --- |
| `GET /profile` | Read the editable provider profile, distinct from the account summary. |
| `PUT /profile` | Update own provider profile. Display name is not a login credential. |
| `PUT /email` | Set own email; verification remains a separate provider fact. |
| `POST /email/resend` | Request an email verification message. |
| `POST /email/verify` | Submit the email code to the provider. |
| `PUT /phone` | Set own phone. |
| `POST /phone/resend` | Request a phone verification message. |
| `POST /phone/verify` | Submit the phone code to the provider, not OTP-primary login. |
| `PUT /password` | Provider self-service password update, not local password authentication. |

[BFF entry](../../web/listingkit-ui/src/lib/server/account-identity-route.ts)
uses serverAuth/server-held token and delegates to
[proxyAccountIdentity](../../web/listingkit-ui/src/lib/server/account-proxy.ts).
The Go module and provider adapter linked in 10.1 own exact input/output and
validation details. This map does not introduce new request fields or permissions.
Passwords/codes may pass transiently through this self-service transport; they
must not become locally stored/validated credentials or audit payloads.

Keep unknown-result handling distinct from definite rejection. Go maps the
adapter's unknown mutation outcome to `502 RESULT_UNVERIFIED`; the BFF can also
return `502/504 RESULT_UNVERIFIED` after forwarding/transport failure. This is
not a single global status-code rewrite. Keep the current safe errors, bounded
requests/responses and no-store handling; do not add automatic mutation retries
or treat a refresh/timeout as proof that the original operation failed. Contact
verification is not personal real-name verification or enterprise certification.

### 10.4 Member operations: permission and freshness are independent of method

Browser `/api/account/members...` and `/api/account/member-operations...` use the
existing [members proxy](../../web/listingkit-ui/src/lib/server/members-proxy.ts)
to reach the corresponding `/api/v1/account/...` routes. All rows below use
`CurrentIdentity` plus `LiveWrite` in the current membership descriptors.
`LiveWrite` is the existing live-grant freshness policy name, **not a claim that
every GET writes data** and not permission to change ordinary GET policies.

| Go route group | Existing permission |
| --- | --- |
| `GET /api/v1/account/members` and `GET /api/v1/account/members/:member_id` | `workbench.organization_member.read` |
| `GET /api/v1/account/member-operations` and `GET /api/v1/account/member-operations/:operation_id` | `workbench.organization_member.manage` |
| `POST /api/v1/account/members/invitations`, `POST /api/v1/account/members/:member_id/role`, `POST /api/v1/account/members/:member_id/remove` | `workbench.organization_member.manage` |
| `POST /api/v1/account/member-operations/:operation_id/verify` | `workbench.organization_member.manage`; explicit recovery may dispatch provider actions, not a harmless status read. |

Under the membership contract, viewer/operator have read only; organization
admin has read/manage, and configured platform users/roles follow the existing
policy owner. Do not replace this with `IsTenantAdmin`, source-account permission,
UI visibility or Organization equality. The service/provider adapter also checks
project, Organization, actor and target/receipt ownership. Replay does not bypass
fresh authorization. Pending-receipt listing is actor-scoped and does not settle
an UNKNOWN invitation.

### 10.5 How an Agent should reuse this map

For the specific new user operation, cite the existing contract and trace:

```text
browser/BFF -> Go route descriptor
  -> verified identity -> required Organization resolution
  -> action permission -> domain target/ownership check
  -> existing provider/repository/operation receipt
```

Do not choose policies solely from the URL or HTTP method. Ordinary account
Organization reads allow the approved cache window (at most 60 seconds and
bounded by token expiry); this is not immediate revocation. LiveWrite/LiveSwitch
use current grants, but freshness is not an atomic transaction with a later
external mutation. An optional suspension checker only applies where actually
injected; do not claim all assemblies enforce a global enterprise lifecycle gate.

Shared server assembly still references `internal/listingkit/httpapi` route/role
helpers and `authz.ListingKitAuthorizer`. Policy and injected middleware determine
which path is used. Those dependencies/names are current code observations,
not a new-feature dependency allowance, a new authorization vulnerability finding,
or proof that physical retirement is complete. Any later extraction needs a
specific consumer/benefit and its own bounded scope; this document does not
require moving `account_identity.go` or creating a giant AuthService.

### 10.6 Evidence, limits and subject verification

Existing evidence locations include the [account read contract](../engineering/account-readonly-contract.md),
[membership contract](../engineering/issue410-membership-contract.md),
[server auth tests](../../internal/app/httpapi/server_test.go),
[account read tests](../../internal/app/httpapi/account_read_test.go) and
[account BFF tests](../../web/listingkit-ui/src/lib/server/account-route.test.ts).
These are references for the applicable path, not a new full-suite acceptance
matrix or a claim that this documentation task reran them. Preserve original
SHA/command/fixture/real-provider distinctions; a source reading is not runtime
or security acceptance. Per-route wire contracts retain their own error/size/
deadline rules rather than inheriting a new universal protocol from this map.

Personal/enterprise subject verification remains #510 / #511, separate from
login, contact verification and Organization authorization. Its new application,
permissions and provider evidence must be designed there; none is enabled here.
This documentation maintenance is not a prerequisite for that work, and does not
promote the discarded AUTH-1 rewrite into a new implementation plan.
