# Auth and Tenancy Context

> Status: supporting architecture context.
>
> CURRENT STATE: inspected at `main @ cae67730c5c0e645d708cb2f6814f14781962bb1` (2026-09-05); no production acceptance claimed.
>
> Scope: ZITADEL-backed authentication, tenant context propagation, authorization boundaries, and data isolation expectations for ListingKit.

## 1. Purpose

ListingKit is a multi-tenant system. Authentication, tenant identification, route authorization, and tenant-scoped data access must stay explicit because ListingKit tasks, workbench state, uploaded assets, store configuration, and source facts can belong to different business owners.

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

CURRENT STATE also includes routes without Organization resolution. Where auth
is required they use the legacy identity middleware, followed by existing role
authorization. Public/OPTIONS behavior remains descriptor-specific. This is
not evidence that all old routes have adopted the new Organization contract;
do not add a second IAM or change route policy as a documentation fix.

## 3. Identity and tenant concepts

### User identity

User identity answers: who is calling the system?

Typical fields:

- subject / user id,
- preferred username or display name,
- email,
- roles or groups,
- token/session metadata.

### Tenant identity

Tenant identity answers: which business boundary owns this request?

`resourceowner:id` / HomeOrganizationID identifies account ownership. It is
not the current business tenant in the multi-Organization model. Business scope
comes from the server-verified EffectiveOrganizationID, checked against project
grants, organization roles and the route's access policy. A user can have access
to multiple Organizations with different roles; home ownership is not a grant
to access another Organization. ZITADEL remains the identity/authorization source.

Tenant identity must be explicit when accessing:

- ListingKit tasks,
- Studio sessions and batches,
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
No new consumer is allowed; owning domains must cut over to current identity.
The #307 clean-slate decision does not cancel source-account ownership or
profile preservation; see [protected scope](../product/issue30-clean-slate-cutover.md).

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
