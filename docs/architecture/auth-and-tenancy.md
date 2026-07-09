# Auth and Tenancy Context

> Status: supporting architecture context.
>
> Last reviewed: 2026-07-09.
>
> Scope: ZITADEL-backed authentication, tenant context propagation, authorization boundaries, and data isolation expectations for ListingKit.

## 1. Purpose

ListingKit is a multi-tenant system. Authentication, tenant identification, route authorization, and tenant-scoped data access must stay explicit because ListingKit tasks, workbench state, uploaded assets, store configuration, and source facts can belong to different business owners.

This document records the current expected auth/tenancy shape. It is supporting architecture context, not yet a stable boundary guard document. Package ownership rules still come from `project-boundaries.md` and specialized boundary documents.

## 2. Current high-level model

The current model is:

```text
frontend / proxy authenticates with ZITADEL
  -> trusted identity and tenant claims reach Go API
  -> ListingKit HTTPAPI middleware parses identity and authorization context
  -> tenant/user context is injected into request scope
  -> domain/application code uses tenant-aware repositories and services
```

The key boundary is:

```text
auth runtime parses and verifies identity;
tenant context identifies the resource owner;
domain services enforce tenant-aware access through repository/service contracts;
ListingKit must not silently fall back to global data when tenant context is required.
```

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

The current tenant boundary is expected to come from the trusted ZITADEL resource owner or equivalent tenant claim passed through the authenticated request path.

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
internal/listingkit/httpapi
```

Auth-related files in this area are expected to own:

- middleware construction,
- trusted header or bearer-token parsing,
- route authorization wiring,
- role/allowlist parsing helpers,
- auth runtime configuration,
- request context injection.

They should not own:

- marketplace business policy,
- product source normalization,
- ListingKit task persistence ordering,
- platform publish rules.

### Tenant context packages

Current relevant areas include:

```text
internal/shared/tenantctx
internal/tenantbridge
```

Tenant context utilities should own:

- typed tenant/user context propagation,
- compatibility bridges for legacy tenant fields,
- narrow helpers for extracting tenant state from request context.

They should not own:

- route authorization policy,
- product or marketplace rules,
- external auth provider runtime construction.

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
Tenant/resource-owner identity
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
- use legacy tenant bridge helpers as a reason to skip tenant-aware repository contracts;
- add broad auth behavior to root `internal/listingkit` when it belongs in HTTPAPI/auth runtime or tenant context utilities.

## 8. Review checklist

Before merging auth or tenant-sensitive changes, check:

```text
[ ] The route has an explicit authentication and authorization posture.
[ ] Tenant identity comes from trusted request context or trusted upstream claims.
[ ] Tenant-owned reads/writes include tenant criteria or an explicitly reviewed system-scope reason.
[ ] Marketplace/product/source packages do not parse HTTP auth details directly.
[ ] Runtime assembly does not own marketplace or product authorization policy.
[ ] Legacy tenant compatibility is narrow and does not become the default model for new code.
[ ] Tests cover denied/missing/wrong-tenant access when the path mutates or reveals tenant-owned state.
```

## 9. Upgrade path to stable boundary document

This document can be promoted from supporting context to stable boundary document when:

- active guard tests cover the main tenant-boundary rules;
- `docs/architecture/README.md` moves it from Supporting Context to Stable Boundary Documents;
- `docs/architecture/architecture-review-checklist.md` lists it as a formal review reference;
- package-specific auth/tenant tests are named in the document.

Until then, use it as shared context and keep formal package/dependency authority in the existing stable boundary documents.
