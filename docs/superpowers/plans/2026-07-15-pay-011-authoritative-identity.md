# PAY-011 Authoritative Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ZITADEL-verified tenant, user, and roles the only authoritative identity inputs for protected ListingKit and Product Sourcing APIs.

**Architecture:** Keep identity as a package-neutral `listingkit.AuthenticatedIdentity` stored in `context.Context`; the Gin ZITADEL middleware installs it after introspection and the role middleware consumes it. API helpers and the 1688 source handoff read it from the request context, while the Next.js proxy forwards tenant data only when it came from a verified session.

**Tech Stack:** Go, Gin, existing ZITADEL introspection middleware, Next.js, Vitest.

## Global Constraints

- Reuse the existing ZITADEL middleware, `authz.ListingKitAuthorizer`, and `listingkit.WithTenantID`; do not create a second authentication or role model.
- Protected customer routes fail closed when the verified token has no tenant/resource claim.
- Body, query, and caller-supplied identity headers cannot override a verified identity.
- Platform-admin cross-tenant operations, store ownership, and storage ownership are out of scope for PAY-011.
- No database migration, subscription change, entitlement change, or billing change.

---

### Task 1: Add the package-neutral authenticated identity context

**Files:**
- Create: `internal/listingkit/authenticated_identity.go`
- Create: `internal/listingkit/authenticated_identity_test.go`

**Interfaces:**
- Produces `AuthenticatedIdentity`, `WithAuthenticatedIdentity(context.Context, AuthenticatedIdentity) context.Context`, and `AuthenticatedIdentityFromContext(context.Context) (AuthenticatedIdentity, bool)`.
- Consumed by the HTTP middleware, ListingKit API helpers, and Product Sourcing handoff handler.

- [x] **Step 1: Write failing context tests**

```go
func TestAuthenticatedIdentityRoundTripsThroughContext(t *testing.T) {
    want := AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a", Roles: []string{"listingkit_operator"}}
    got, ok := AuthenticatedIdentityFromContext(WithAuthenticatedIdentity(context.Background(), want))
    require.True(t, ok)
    require.Equal(t, want, got)
}

func TestAuthenticatedIdentityFromContextRejectsMissingOrBlankTenant(t *testing.T) {
    _, ok := AuthenticatedIdentityFromContext(context.Background())
    require.False(t, ok)
    _, ok = AuthenticatedIdentityFromContext(WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{UserID: "user-a"}))
    require.False(t, ok)
}
```

- [x] **Step 2: Run the focused test to verify it fails**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run TestAuthenticatedIdentity -count=1`

Expected: compile failure because the identity helpers do not exist.

- [x] **Step 3: Implement normalized identity storage**

```go
type AuthenticatedIdentity struct {
    TenantID string
    UserID   string
    Roles    []string
}

func WithAuthenticatedIdentity(ctx context.Context, identity AuthenticatedIdentity) context.Context
func AuthenticatedIdentityFromContext(ctx context.Context) (AuthenticatedIdentity, bool)
```

Trim tenant and user IDs, copy the role slice, and return `false` unless the tenant ID is non-empty. Use an unexported context-key type.

- [x] **Step 4: Run the focused test to verify it passes**

Run: `$env:GOWORK='off'; go test ./internal/listingkit -run TestAuthenticatedIdentity -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add internal/listingkit/authenticated_identity.go internal/listingkit/authenticated_identity_test.go
git commit -m "feat: add authenticated listingkit identity context"
```

### Task 2: Install and consume trusted identity in ZITADEL middleware

**Files:**
- Modify: `internal/listingkit/httpapi/zitadel_auth_middleware.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_route_authorization.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_test.go`

**Interfaces:**
- Consumes `listingkit.WithAuthenticatedIdentity` after successful token introspection.
- Produces a request context available to downstream handlers and role authorization.

- [x] **Step 1: Write failing route tests**

Extend the mock-introspection route test to assert this handler body:

```go
identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
if !ok { c.JSON(http.StatusForbidden, gin.H{"error": "missing_identity"}); return }
c.JSON(http.StatusOK, identity)
```

Add a route with `PermissionProductSourcingWrite`, send a verified token with `listingkit_operator`, and a forged `X-User-Roles: viewer`. It must return 200. The same verified token with no permitted role must return `403 listingkit_permission_denied` even when the incoming header claims `listingkit_operator`.

- [x] **Step 2: Run the middleware tests to verify they fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit/httpapi -run 'TestListingKitZitadelAuth(MapsVerifiedIdentityToContext|RoleMiddlewareIgnoresForgedHeaders)' -count=1`

Expected: the context assertion fails because no identity is stored.

- [x] **Step 3: Store identity before `c.Next()` and authorize from it**

After token verification, construct:

```go
trusted := listingkit.AuthenticatedIdentity{
    TenantID: identity.ResourceID,
    UserID: firstNonEmptyZitadelValue(identity.UserID, identity.Subject, identity.Username),
    Roles: identity.Roles,
}
if strings.TrimSpace(trusted.TenantID) == "" {
    c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "zitadel_tenant_missing", "message": "ZITADEL identity has no tenant"})
    return
}
c.Request = c.Request.WithContext(listingkit.WithAuthenticatedIdentity(c.Request.Context(), trusted))
```

Retain header replacement only for compatibility, but change `NewRouteRoleMiddleware` to load `AuthenticatedIdentityFromContext(c.Request.Context())`. If it is absent, deny with the existing authorization-unavailable response. Pass `trusted.UserID` and `trusted.Roles` to `authorizer.Authorize`.

- [x] **Step 4: Run focused middleware and package tests**

```powershell
$env:GOWORK='off'; go test ./internal/listingkit/httpapi -run 'TestListingKitZitadelAuth' -count=1
$env:GOWORK='off'; go test ./internal/listingkit/httpapi -count=1
```

Expected: PASS; missing token remains 401, missing tenant claim is 403, and forged roles do not grant access.

- [x] **Step 5: Commit**

```powershell
git add internal/listingkit/httpapi/zitadel_auth_middleware.go internal/listingkit/httpapi/zitadel_auth_route_authorization.go internal/listingkit/httpapi/zitadel_auth_test.go
git commit -m "security: bind route authorization to verified identity"
```

### Task 3: Derive ListingKit and 1688 handoff requests from the trusted context

**Files:**
- Modify: `internal/listingkit/api/tenant_context.go`
- Modify: `internal/listingkit/api/tenant_context_test.go`
- Modify: `internal/productenrich/httpapi/sourcea1688/handler.go`
- Modify: `internal/productenrich/httpapi/sourcea1688/handler_test.go`

**Interfaces:**
- `requestContext` prefers `listingkit.AuthenticatedIdentityFromContext(c.Request.Context())` before candidates, headers, or query values.
- `verifiedRequestContext` returns `listingkit.RequestIdentity` derived only from `AuthenticatedIdentity`.

- [x] **Step 1: Write failing forged-override tests**

Add an API helper test with a request context containing `{TenantID: "tenant-a", UserID: "user-a"}` and request values `tenant-b`/`user-b` in every supported candidate/header/query location. Assert `requestContext` has tenant-a and OpenAI identity user-a.

Change the Product Sourcing handler test setup to attach a trusted request context. Add a test that supplies trusted tenant-a/user-a plus forged `X-Tenant-ID: tenant-b` and `X-User-ID: user-b`; assert `CreateTaskCommand` contains tenant-a/user-a. Add a missing-trusted-context test that returns bad request and leaves the fake service command empty.

- [x] **Step 2: Run focused tests to verify they fail**

```powershell
$env:GOWORK='off'; go test ./internal/listingkit/api -run 'TestRequestContext.*Authenticated' -count=1
$env:GOWORK='off'; go test ./internal/productenrich/httpapi/sourcea1688 -run 'TestCreateListingKitTask.*(Trusted|Verified)' -count=1
```

Expected: current helpers use header/candidate values and the forged override test fails.

- [x] **Step 3: Implement trusted-first context selection**

At the top of `requestTenantID`, `requestUserID`, and `requestRoles`, read `listingkit.AuthenticatedIdentityFromContext(c.Request.Context())`. When present, return it and do not inspect candidates, request headers, or query parameters. Preserve legacy fallback only when no authenticated context exists so unprotected internal/unit routes are not silently reclassified.

Replace the Product Sourcing handler's header reads with:

```go
identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
if !ok || strings.TrimSpace(identity.UserID) == "" {
    return nil, listingkit.RequestIdentity{}, errors.New("verified request identity is required")
}
```

Then build the existing `listingkit.RequestIdentity` and tenant context from `identity`.

- [x] **Step 4: Run focused package verification**

```powershell
$env:GOWORK='off'; go test ./internal/listingkit/api -count=1
$env:GOWORK='off'; go test ./internal/productenrich/httpapi/sourcea1688 -count=1
$env:GOWORK='off'; go test ./internal/product/sourcehandoff/a1688/httpapi -count=1
```

Expected: PASS; all trusted paths retain the authenticated tenant, and direct 1688 handlers refuse header-only identity.

- [x] **Step 5: Commit**

```powershell
git add internal/listingkit/api/tenant_context.go internal/listingkit/api/tenant_context_test.go internal/productenrich/httpapi/sourcea1688/handler.go internal/productenrich/httpapi/sourcea1688/handler_test.go
git commit -m "security: use verified tenant identity in listingkit APIs"
```

### Task 4: Stop the Next.js proxy from accepting caller tenant headers

**Files:**
- Modify: `web/listingkit-ui/src/app/api/listing-kits/proxy-auth.ts`
- Modify: `web/listingkit-ui/src/app/api/listing-kits/route.test.ts`

**Interfaces:**
- `buildListingKitUpstreamHeaders` emits `tenant-id` and `X-Tenant-ID` only when `verifiedIdentity.tenantId` is present.

- [x] **Step 1: Add the failing proxy-header test**

```ts
const headers = buildListingKitUpstreamHeaders(
  new Headers({ "tenant-id": "forged-tenant", "x-tenant-id": "forged-tenant" }),
  undefined,
);
expect(headers.get("tenant-id")).toBeNull();
expect(headers.get("X-Tenant-ID")).toBeNull();
```

Keep the existing verified-session test asserting that its tenant ID is forwarded.

- [x] **Step 2: Run the focused test to verify it fails**

Run: `& .\node_modules\.bin\vitest.cmd run src/app/api/listing-kits/route.test.ts --maxWorkers=1`

Expected: FAIL because the proxy currently falls back to the incoming `tenant-id` header.

- [x] **Step 3: Remove the caller-header fallback**

Replace the tenant selection with:

```ts
const tenantID = stringifyIdentityValue(verifiedIdentity?.tenantId);
```

Do not alter authorization-token forwarding; direct bearer requests still undergo API-side introspection.

- [x] **Step 4: Run frontend verification**

```powershell
Set-Location web/listingkit-ui
& .\node_modules\.bin\vitest.cmd run src/app/api/listing-kits/route.test.ts --maxWorkers=1
npm run typecheck
```

Expected: PASS.

- [x] **Step 5: Commit**

```powershell
git add web/listingkit-ui/src/app/api/listing-kits/proxy-auth.ts web/listingkit-ui/src/app/api/listing-kits/route.test.ts
git commit -m "security: prevent proxy tenant header override"
```

## Final verification

- [x] Run `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/productenrich/httpapi/sourcea1688 ./internal/product/sourcehandoff/a1688/httpapi -count=1`.
- [x] Run `Set-Location web/listingkit-ui; npm run lint; npm run typecheck; npm test`.
- [x] Run `git diff origin/master...HEAD --check` and confirm no migration, subscription, billing, or store-ownership change is present.
- [x] Update the PAY-011 checkbox and dated validation evidence only after the production-route tests pass.

### Validation evidence (2026-07-15)

- `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/productenrich/httpapi/sourcea1688 ./internal/product/sourcehandoff/a1688/httpapi -count=1` passed.
- `npm run lint`, `npm run typecheck`, and `npm test` in `web/listingkit-ui` completed with exit code 0. Lint reported 14 pre-existing warnings and no errors.
- `git diff origin/master...HEAD --check` passed. The reviewed diff contains no database migration, subscription, billing, or store-ownership change.
