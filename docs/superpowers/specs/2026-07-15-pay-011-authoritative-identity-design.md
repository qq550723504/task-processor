# PAY-011 Authoritative Identity Design

**Status:** approved design for implementation planning

## Goal

Make the ZITADEL-verified tenant, user, and roles the sole identity source for every protected customer API. A caller must not be able to select a different tenant or user through a body field, query parameter, or forwarded header.

## Scope

- Protected ListingKit, Product Sourcing, SDS, SDS-login, SHEIN-login, and ListingKit admin customer routes.
- The Next.js ListingKit proxy and the Gin/ZITADEL middleware that it reaches.
- Request-context helpers used by task creation, list, detail, submission, and settings handlers.
- Focused route and handler tests for forged identity inputs.

This task does not add a new role model, change store ownership rules, or implement platform-admin tenancy management. Those remain PAY-012 and later work.

## Decision

Use one in-process `AuthenticatedIdentity` value, installed by the successfully completed ZITADEL middleware in Gin request context. Protected handlers must derive tenant, user, and roles from that value only.

The existing header bridge remains transport compatibility between the Next.js proxy and API, but it is not authoritative after a request crosses the API boundary. The API verifies the bearer token itself and overwrites its identity context from ZITADEL introspection. No protected route falls back to `listingkit.DefaultTenantID`.

## Request flow

1. The Next.js proxy obtains an access token and optional session identity from its verified session.
2. The proxy forwards the bearer token and only identity values obtained from that session. It never copies a client-supplied `tenant-id` into its upstream identity headers when no verified identity exists.
3. The Gin ZITADEL middleware introspects the bearer token. On success it creates and stores `AuthenticatedIdentity{TenantID, UserID, Roles}` in Gin context before role authorization runs.
4. The route-role middleware reads roles from the trusted identity context, not from request headers.
5. Protected ListingKit API helpers construct the Go request context from `AuthenticatedIdentity`. They ignore tenant/user values supplied by JSON, form input, query parameters, or headers.
6. The handler invokes its existing service using that context. Persisted tasks, queries, and external submissions therefore retain the verified tenant and user.

## Failure semantics

| Condition | Response | Side effect |
| --- | --- | --- |
| Missing or invalid bearer token on a protected route | `401` with the existing ZITADEL error code | No handler or service call. |
| Valid token without a tenant/resource claim where a tenant is required | stable `403` identity-context error | No default tenant fallback and no service call. |
| Valid authenticated tenant with insufficient route permission | `403` with `listingkit_permission_denied` | No handler or service call. |
| Body/query/header tenant or user differs from authenticated identity | Request is processed under the authenticated identity; override is ignored | No cross-tenant read or write. |
| Platform-admin cross-tenant action | Out of scope for ordinary customer routes | Must use its separate route and permission model in later work. |

## Compatibility boundary

Production external routes fail closed immediately. There is no automatic header/query fallback feature flag for them.

Unit tests and intentionally unprotected internal routes may keep explicit test-context helpers, but these helpers must not be registered as production route middleware. Any legacy direct caller that lacks a verified identity receives the same protected-route failure rather than being assigned to the default tenant.

## Tests

Add focused tests that prove:

1. A successful ZITADEL middleware invocation stores the tenant, user, and roles in Gin context.
2. Role authorization reads the stored roles, not a forged `X-User-Roles` header.
3. Protected request-context construction uses the stored tenant and user even when body/query/header values name a different tenant/user.
4. Missing authoritative identity fails before the target handler/service is called and never uses `listingkit.DefaultTenantID`.
5. Product Sourcing task creation, ListingKit task creation/query/detail, submission, and settings share the same trusted-context helper.
6. The Next.js proxy preserves a verified session identity but does not turn an arbitrary incoming `tenant-id` header into an upstream identity.

## Rollout and rollback

The change is additive at middleware/context boundaries and does not require a database migration. Deployment must first run the focused auth and handler tests, then the existing ListingKit API package tests. Rollback is a normal code rollback; it restores the current header-based behavior, so it must not be used to work around an authentication outage. Authentication configuration failures remain fail-closed.

## Non-goals and follow-ups

- PAY-012 validates that requested stores belong to the authenticated tenant.
- PAY-013 scopes uploaded assets and object keys to the authenticated tenant.
- PAY-014 adds the broader cross-tenant commercial security suite.
- Platform-admin target-tenant audit fields are not introduced by ordinary customer-route changes.
