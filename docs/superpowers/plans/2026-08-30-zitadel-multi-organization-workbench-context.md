# ZITADEL Multi-Organization Workbench Context Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a production-safe workbench identity foundation in which one ZITADEL subject can select among multiple authorized Organizations and receives only the roles scoped to the selected Organization.

**Architecture:** Keep Auth.js as the browser OIDC session owner and the Go API as the authorization authority. The Go authentication middleware verifies the bearer token, a ZITADEL v2 authorization client enumerates the subject's project authorizations, and an effective-organization resolver validates the untrusted requested Organization before role middleware or handlers run. A Next.js BFF stores only the last selected Organization in an HttpOnly cookie and forwards it as untrusted input.

**Tech Stack:** Go, Gin, GORM, ZITADEL OIDC introspection, ZITADEL Authorization API v2, Next.js 16 App Router, Auth.js, React 19, TanStack Query 5, TypeScript, Vitest, Testing Library.

**Spec:** `docs/superpowers/specs/2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md`

## Global Constraints

- Do not create, mutate, or delete ZITADEL Organizations, project grants, users, or role assignments from this implementation.
- Do not accept `resourceowner:id`, a cookie, a header, URL state, or a browser-parsed claim as proof of effective-organization access.
- Use ZITADEL Organization IDs as opaque non-empty strings; never parse them as integers and never call `tenantbridge` from a workbench route.
- The official v2 `AuthorizationService/ListAuthorizations` response is the grant enumeration contract. Filter every response by the introspected subject and configured ListingKit project ID.
- Organization switch and all writes use a live grant lookup. Reads may use a verified grant cache for at most 60 seconds and never beyond token expiry.
- A ZITADEL failure without a still-valid read cache fails closed. The cache never stores access tokens.
- Existing `/listing-kits` routes may continue to compile during the slice, but new `/workbench` routes and `/api/v1/workbench/*` APIs must not depend on their tenant fallback, navigation, or data.
- Do not delete legacy tables or data in this plan.
- Use `apply_patch` for source edits. Preserve unrelated working-tree changes and stage only paths named by the active task.

---

## File Map

### Evidence and configuration

- Create: `docs/verification/zitadel-multi-organization-authorization.md`
- Create: `internal/authruntime/zitadel/testdata/list_authorizations_two_orgs.json`
- Modify: `internal/core/config/type_listingkit.go`
- Create: `internal/core/config/type_workbench.go`
- Modify: `internal/core/config/defaults.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config_env_test.go`

### Go identity and ZITADEL adapter

- Modify: `internal/authidentity/authenticated_identity.go`
- Modify: `internal/authidentity/authenticated_identity_test.go`
- Modify: `internal/authruntime/zitadel/config.go`
- Modify: `internal/authruntime/zitadel/parsing.go`
- Modify: `internal/authruntime/zitadel/middleware.go`
- Modify: `internal/authruntime/zitadel/middleware_test.go`
- Create: `internal/authruntime/zitadel/authorization_client.go`
- Create: `internal/authruntime/zitadel/authorization_client_test.go`
- Create: `internal/workbenchcontext/context.go`
- Create: `internal/workbenchcontext/context_test.go`
- Create: `internal/workbenchcontext/grant_cache.go`
- Create: `internal/workbenchcontext/grant_cache_test.go`
- Create: `internal/workbenchcontext/resolver.go`
- Create: `internal/workbenchcontext/resolver_test.go`
- Create: `internal/workbenchcontext/httpapi/module.go`
- Create: `internal/workbenchcontext/httpapi/handler.go`
- Create: `internal/workbenchcontext/httpapi/handler_test.go`

### Route composition

- Modify: `internal/httproute/descriptor.go`
- Modify: `internal/httproute/descriptor_test.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Modify: `internal/app/httpapi/server_auth.go`
- Modify: `internal/app/httpapi/server.go`
- Modify: `internal/app/httpapi/server_test.go`
- Create: `internal/app/httpapi/workbench_context_module.go`
- Create: `internal/app/httpapi/workbench_context_module_test.go`

### Next.js BFF and workbench shell

- Create: `web/listingkit-ui/src/lib/server/workbench-proxy.ts`
- Create: `web/listingkit-ui/src/lib/server/workbench-proxy.test.ts`
- Create: `web/listingkit-ui/src/app/api/workbench/[...path]/route.ts`
- Create: `web/listingkit-ui/src/app/api/workbench/[...path]/route.test.ts`
- Create: `web/listingkit-ui/src/lib/api/workbench-context.ts`
- Create: `web/listingkit-ui/src/lib/api/workbench-context.test.ts`
- Create: `web/listingkit-ui/src/components/providers/workbench-context-provider.tsx`
- Create: `web/listingkit-ui/src/components/providers/workbench-context-provider.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/workspace-app-shell.tsx`
- Create: `web/listingkit-ui/src/components/workbench/workspace-app-shell.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/organization-switcher.tsx`
- Create: `web/listingkit-ui/src/components/workbench/organization-switcher.test.tsx`
- Create: `web/listingkit-ui/src/app/workbench/layout.tsx`
- Create: `web/listingkit-ui/src/app/workbench/page.tsx`
- Create: `web/listingkit-ui/src/app/workbench/no-organization/page.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.test.tsx`
- Modify: `web/listingkit-ui/src/proxy.ts`
- Modify: `web/listingkit-ui/src/proxy.test.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.test.ts`

---

## API and Type Contract

The implementation uses these normalized Go types:

```go
type OrganizationGrant struct {
    OrganizationID   string
    OrganizationName string
    ProjectID        string
    Roles            []string
}

type AuthenticatedIdentity struct {
    TenantID                 string // existing routes only; home org before resolution
    UserID                   string // canonical ZITADEL sub
    Roles                    []string // existing flattened roles before resolution; scoped roles after resolution
    HomeOrganizationID       string
    EffectiveOrganizationID  string
    OrganizationGrants       []OrganizationGrant
    TokenExpiresAt           time.Time
}
```

The public browser contract is:

```json
{
  "user": { "id": "user-1" },
  "homeOrganizationId": "org-a",
  "effectiveOrganizationId": "org-b",
  "selectionRequired": false,
  "organizations": [
    { "id": "org-a", "name": "Sumi", "roles": ["listingkit_admin"] },
    { "id": "org-b", "name": "Star", "roles": ["listingkit_viewer"] }
  ]
}
```

The Next.js cookie name is `shuomi_effective_organization`. It is `HttpOnly`, `SameSite=Lax`, `Path=/`, and `Secure` outside development. It contains only an Organization ID.

When a user has multiple grants but no valid default, context GET returns `200`, `effectiveOrganizationId: null`, `selectionRequired: true`, and the complete normalized Organization list. Business-resource routes still require a non-empty effective Organization.

---

## Task 1: Prove the Real ZITADEL Authorization Shape

**Files:**

- Create: `docs/verification/zitadel-multi-organization-authorization.md`
- Create: `internal/authruntime/zitadel/testdata/list_authorizations_two_orgs.json`

- [ ] In the existing ZITADEL administration flow, select two non-production test Organizations and one test user. Assign `listingkit_admin` in Organization A and `listingkit_viewer` in Organization B for the configured ListingKit project.
- [ ] Sign in through the existing Auth.js flow and call the official `POST /zitadel.authorization.v2.AuthorizationService/ListAuthorizations` endpoint with the user's access token and `Connect-Protocol-Version: 1`.
- [ ] Record only sanitized evidence: issuer host, project ID suffix, subject suffix, Organization ID suffixes, Organization names, role keys, HTTP status, and observation time. Never record tokens, client secrets, cookies, usernames, or full personal data.
- [ ] Verify the response contains two active authorizations whose `user.id` equals the introspected `sub` and whose `project.id` equals `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID`.
- [ ] Revoke the A authorization, repeat a live request, and record that the revoked Organization is absent or non-active. Restore the test grant after evidence collection.
- [ ] Save a fully synthetic two-Organization response fixture in `testdata`; do not copy production identifiers.
- [ ] Write the exact response-field mapping and revocation observation into the verification document.
- [ ] Stop implementation and request ZITADEL test-environment setup if the same subject cannot obtain two differently scoped project authorizations. Do not invent a fallback grant model.

**Verification:**

```powershell
rg -n "access_token|refresh_token|client_secret|Authorization:" docs/verification/zitadel-multi-organization-authorization.md internal/authruntime/zitadel/testdata
```

Expected: no matches.

```powershell
go test ./internal/authruntime/zitadel -run TestAuthorizationFixture -count=1
```

Expected before the fixture test exists: test name is absent. Add the test in Task 3; this command must pass then.

**Commit:**

```powershell
git add docs/verification/zitadel-multi-organization-authorization.md internal/authruntime/zitadel/testdata/list_authorizations_two_orgs.json
git commit -m "docs: verify zitadel multi-organization grants"
```

## Task 2: Extend the Verified Identity Without Changing Tenant Semantics Globally

**Files:**

- Modify: `internal/authidentity/authenticated_identity.go`
- Modify: `internal/authidentity/authenticated_identity_test.go`
- Modify: `internal/authruntime/zitadel/config.go`
- Modify: `internal/authruntime/zitadel/parsing.go`
- Modify: `internal/authruntime/zitadel/middleware.go`
- Modify: `internal/authruntime/zitadel/middleware_test.go`

- [ ] Add failing tests that normalize and defensively copy `HomeOrganizationID`, `EffectiveOrganizationID`, `OrganizationGrants`, and `TokenExpiresAt`.
- [ ] Add a failing test proving `org-a:admin` and `org-b:viewer` are not flattened into one effective role set after resolution.
- [ ] Add `exp` parsing to `IntrospectionResponse` and reject an introspection response whose expiry is already past, even if `active=true`.
- [ ] Add the fields in the type contract above. Preserve `TenantID`, `UserID`, and `Roles` only because existing non-workbench handlers still compile; document that new workbench handlers read `EffectiveOrganizationID` and scoped `Roles` only.
- [ ] Keep authentication middleware responsible only for token verification, `UserID/sub`, `HomeOrganizationID/resourceowner:id`, expiry, and forged-header removal. It must not choose an effective Organization.
- [ ] Keep the current flattened `Roles` value for existing routes, but ensure the resolver in Task 5 overwrites it with only the selected Organization's roles before any workbench permission middleware runs.
- [ ] Leave `X-Requested-Organization-ID` explicitly untrusted. It may reach Go from the BFF or a direct client, but it can influence context only after the resolver proves a matching live/cached Organization grant.

**Red / green verification:**

```powershell
go test ./internal/authidentity ./internal/authruntime/zitadel -run "TestAuthenticatedIdentity|TestMiddleware.*Exp|Test.*Scoped" -count=1
```

Expected after implementation: pass.

**Commit:**

```powershell
git add internal/authidentity internal/authruntime/zitadel/config.go internal/authruntime/zitadel/parsing.go internal/authruntime/zitadel/middleware.go internal/authruntime/zitadel/middleware_test.go
git commit -m "feat: carry canonical zitadel identity context"
```

## Task 3: Implement the Official ZITADEL v2 Authorization Client

**Files:**

- Create: `internal/authruntime/zitadel/authorization_client.go`
- Create: `internal/authruntime/zitadel/authorization_client_test.go`
- Modify: `internal/core/config/type_listingkit.go`
- Create: `internal/core/config/type_workbench.go`
- Modify: `internal/core/config/defaults.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config_env_test.go`

- [ ] Add failing `httptest.Server` tests for request method, v2 path, bearer propagation, protocol-version header, pagination, non-2xx response, malformed JSON, wrong subject, wrong project, inactive authorization, duplicate roles, and two Organizations with different roles.
- [ ] Implement `AuthorizationClient.ListOwnProjectAuthorizations(ctx, bearerToken, subject, projectID)` using `POST /zitadel.authorization.v2.AuthorizationService/ListAuthorizations`.
- [ ] Send `Connect-Protocol-Version: 1`, `Content-Type: application/json`, and the user bearer token. Never log the bearer token or response body.
- [ ] Fetch pages with a page size of 100 until the API reports no further page. Reject more than 1,000 returned authorizations to bound memory and fail closed on an unexpected account shape.
- [ ] Normalize only active authorizations where `authorization.user.id == subject` and `authorization.project.id == projectID`; group them by `organization.id`, deduplicate/sort role keys, and retain Organization display name.
- [ ] Reject an item that has a blank Organization ID, blank project ID, or blank user ID. Ignore other projects and other users rather than granting access.
- [ ] Add `AuthorizationAPIURL` to `ListingKitZitadelConfig`, defaulting to `IssuerURL` when unset, and bind `TASK_PROCESSOR_LISTINGKIT_ZITADEL_AUTHORIZATION_API_URL` for test/proxy deployments.
- [ ] Add top-level `WorkbenchConfig{Enabled bool}` and bind `workbench.enabled` / `TASK_PROCESSOR_WORKBENCH_ENABLED`. Keep it disabled by default; do not hide a neutral workbench flag under a legacy ListingKit UI flag.
- [ ] Add config validation requiring `ProjectID` for workbench context enablement. Do not add a management token: this API call uses the user's access token.

**Verification:**

```powershell
go test ./internal/authruntime/zitadel ./internal/core/config -run "TestAuthorization|Test.*Zitadel.*Config" -count=1
```

Expected: pass, including the synthetic fixture.

**Commit:**

```powershell
git add internal/authruntime/zitadel/authorization_client.go internal/authruntime/zitadel/authorization_client_test.go internal/authruntime/zitadel/testdata internal/core/config
git commit -m "feat: enumerate scoped zitadel project authorizations"
```

## Task 4: Add the 60-Second Grant Cache and Default-Selection Policy

**Files:**

- Create: `internal/workbenchcontext/context.go`
- Create: `internal/workbenchcontext/context_test.go`
- Create: `internal/workbenchcontext/grant_cache.go`
- Create: `internal/workbenchcontext/grant_cache_test.go`

- [ ] Add failing table tests for selection precedence: valid requested Organization, authorized home Organization, sole Organization, and selection-required when multiple grants remain.
- [ ] Add failing tests for cache keys including subject, project ID, and contract version; no bearer token may appear in a key or value.
- [ ] Add fake-clock tests proving TTL is `min(60s, tokenExpiry-now)`, expired entries are rejected, a token with less than one second remaining is not cached, and invalidation removes all entries for the subject/project.
- [ ] Define `GrantSource` with explicit modes `GrantReadCached` and `GrantLive`. Implement live lookup through the Task 3 client and a concurrency-safe in-process read cache.
- [ ] Do not cache errors, empty malformed grants, or authorizations beyond token expiry. An authoritative empty grant list is a valid live result but must lead to no-organization access.
- [ ] Implement deterministic normalization: Organization IDs sorted ascending for API stability; roles sorted ascending; names trimmed.
- [ ] Return typed errors for `ErrOrganizationSelectionRequired`, `ErrOrganizationAccessDenied`, `ErrOrganizationAccessRevoked`, and `ErrAuthorizationDependencyUnavailable`.

**Verification:**

```powershell
go test ./internal/workbenchcontext -run "TestSelect|TestGrantCache" -count=1 -race
```

Expected: pass.

**Commit:**

```powershell
git add internal/workbenchcontext/context.go internal/workbenchcontext/context_test.go internal/workbenchcontext/grant_cache.go internal/workbenchcontext/grant_cache_test.go
git commit -m "feat: cache verified organization grants safely"
```

## Task 5: Resolve Effective Organization Before Workbench Authorization

**Files:**

- Create: `internal/workbenchcontext/resolver.go`
- Create: `internal/workbenchcontext/resolver_test.go`
- Modify: `internal/httproute/descriptor.go`
- Modify: `internal/httproute/descriptor_test.go`
- Modify: `internal/app/httpapi/server_auth.go`
- Modify: `internal/app/httpapi/server.go`
- Modify: `internal/app/httpapi/server_test.go`

- [ ] Add `OrganizationAccessPolicy` to route descriptors with values `none`, `context_read`, `cached_read`, `live_write`, and `live_switch`.
- [ ] Add failing server tests proving handler order is token authentication, effective-organization resolution, role authorization, handler.
- [ ] Add failing resolver tests for forged Organization headers, wrong Organization, no selection, suspended local business status, cached reads, live writes, live switches, expired token, and ZITADEL failure.
- [ ] Read the requested Organization from `X-Requested-Organization-ID`; treat it as untrusted. When absent, apply the Task 4 default-selection policy.
- [ ] For `context_read`, load the bounded grant cache and allow a successful request with no effective Organization only when selection is required or no grants exist. For `cached_read`, require a selected Organization. For `live_write` and `live_switch`, always call the live grant source. A switch invalidates the subject/project cache before and after the live lookup.
- [ ] On success, write `EffectiveOrganizationID`, normalized grants, `TenantID=EffectiveOrganizationID`, and `Roles=selectedGrant.Roles` into a fresh request context. Never mutate a shared identity value.
- [ ] Ensure role middleware uses only the post-resolution `identity.Roles` for workbench routes. Add the A-admin/B-viewer regression test at the mounted-server level.
- [ ] Map typed resolver errors to the stable error codes in the design spec without revealing whether a resource exists in another Organization.

**Verification:**

```powershell
go test ./internal/workbenchcontext ./internal/httproute ./internal/app/httpapi -run "Test.*Organization|Test.*AuthHandlerOrder|Test.*ScopedRole" -count=1
```

Expected: pass.

**Commit:**

```powershell
git add internal/workbenchcontext/resolver.go internal/workbenchcontext/resolver_test.go internal/httproute internal/app/httpapi/server_auth.go internal/app/httpapi/server.go internal/app/httpapi/server_test.go
git commit -m "feat: resolve effective organization for workbench routes"
```

## Task 6: Expose the Workbench Context Go API

**Files:**

- Create: `internal/workbenchcontext/httpapi/module.go`
- Create: `internal/workbenchcontext/httpapi/handler.go`
- Create: `internal/workbenchcontext/httpapi/handler_test.go`
- Create: `internal/app/httpapi/workbench_context_module.go`
- Create: `internal/app/httpapi/workbench_context_module_test.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_modules.go`

- [ ] Add failing handler tests for `GET /api/v1/workbench/context` and `PUT /api/v1/workbench/context/effective-organization`.
- [ ] Register GET with `context_read` and PUT with `live_switch`; both require verified identity. PUT accepts only `{ "organizationId": "..." }`, rejects unknown fields, and rejects a body/header target mismatch.
- [ ] Return the public context contract exactly; never include bearer tokens, token expiry, authorization IDs, project grant internals, or raw ZITADEL payloads.
- [ ] When multiple Organizations exist and no default is valid, return `200` with `effectiveOrganizationId: null` and `selectionRequired: true` so the UI can present the choices. Return a successful context with an empty organization list only for the explicit no-access UI state. Business-resource routes continue to return `ORGANIZATION_SELECTION_REQUIRED` until a switch succeeds.
- [ ] Build the ZITADEL authorization client, cache, resolver, handler, and HTTP module once in the app composition root using the configured shared HTTP client and project ID.
- [ ] Fail startup when workbench is enabled but issuer/project/authorization API configuration is incomplete. Keep the feature disabled by default until the verification gate is satisfied.
- [ ] Add route-registry tests proving both routes are present exactly once with the correct access policies.

**Verification:**

```powershell
go test ./internal/workbenchcontext/... ./internal/app/httpapi -run "Test.*WorkbenchContext|Test.*Workbench.*Module" -count=1
```

Expected: pass.

**Commit:**

```powershell
git add internal/workbenchcontext internal/app/httpapi/workbench_context_module.go internal/app/httpapi/workbench_context_module_test.go internal/app/httpapi/types.go internal/app/httpapi/composition_builder.go internal/app/httpapi/composition_modules.go
git commit -m "feat: expose verified workbench context"
```

## Task 7: Add the HttpOnly Organization-Selection BFF

**Files:**

- Create: `web/listingkit-ui/src/lib/server/workbench-proxy.ts`
- Create: `web/listingkit-ui/src/lib/server/workbench-proxy.test.ts`
- Create: `web/listingkit-ui/src/app/api/workbench/[...path]/route.ts`
- Create: `web/listingkit-ui/src/app/api/workbench/[...path]/route.test.ts`

- [ ] Add failing tests that the BFF reads the Auth.js access token server-side, forwards it as bearer auth, and never returns it to the browser.
- [ ] Proxy `/api/workbench/*` to `/api/v1/workbench/*` using the existing upstream base URL rules, but do not reuse legacy ListingKit tenant/user/role header forwarding.
- [ ] Forward the selection cookie as `X-Requested-Organization-ID` for ordinary requests. For the effective-organization PUT route, validate the JSON body shape and forward its `organizationId` as the requested header. Strip any browser-supplied value for that header before building the upstream request.
- [ ] On successful PUT switch, set `shuomi_effective_organization` to the Organization ID returned as effective by Go, not blindly to the request body.
- [ ] Set the cookie attributes from the contract above. Clear it when Go returns `ORGANIZATION_ACCESS_REVOKED`, `ORGANIZATION_ACCESS_DENIED`, or an empty effective Organization.
- [ ] Preserve upstream status and the stable JSON error body. Cap request bodies and upstream responses at the existing proxy limits.
- [ ] Add tests for forged header stripping, PUT body-to-header forwarding, Go body/header mismatch rejection, failed switch not changing the cookie, revoked selection clearing it, and an open-redirect-free unauthenticated response.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/lib/server/workbench-proxy.test.ts "src/app/api/workbench/[...path]/route.test.ts"
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/lib/server/workbench-proxy.ts web/listingkit-ui/src/lib/server/workbench-proxy.test.ts "web/listingkit-ui/src/app/api/workbench/[...path]"
git commit -m "feat: proxy verified workbench organization context"
```

## Task 8: Build the Workbench Context Provider and Shell

**Files:**

- Create: `web/listingkit-ui/src/lib/api/workbench-context.ts`
- Create: `web/listingkit-ui/src/lib/api/workbench-context.test.ts`
- Create: `web/listingkit-ui/src/components/providers/workbench-context-provider.tsx`
- Create: `web/listingkit-ui/src/components/providers/workbench-context-provider.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/workspace-app-shell.tsx`
- Create: `web/listingkit-ui/src/components/workbench/workspace-app-shell.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/organization-switcher.tsx`
- Create: `web/listingkit-ui/src/components/workbench/organization-switcher.test.tsx`
- Create: `web/listingkit-ui/src/app/workbench/layout.tsx`
- Create: `web/listingkit-ui/src/app/workbench/page.tsx`
- Create: `web/listingkit-ui/src/app/workbench/no-organization/page.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.tsx`
- Modify: `web/listingkit-ui/src/components/application-frame.test.tsx`

- [ ] Add failing contract tests for context fetch and switch mutations using stable error codes rather than message parsing.
- [ ] Add `WorkbenchContextProvider` with a query key rooted at `['workbench-context']`. Expose user, organizations, effective Organization, scoped roles, switch mutation, and loading/error state.
- [ ] On successful switch, call `queryClient.clear()` before installing the returned context, then re-seed `['workbench-context']`. This prevents data, selection, draft, or pagination leakage across Organizations.
- [ ] Build a new neutral `WorkspaceAppShell`; reuse the low-level sidebar primitives and visual tokens, not `ListingKitAppShell` or its tenant query-string switcher.
- [ ] Render only implemented first-slice navigation: `工作台` and `店铺中心 / 我的店铺`. Do not render placeholders for the remaining Figma menus.
- [ ] Show current Organization name globally. For one Organization, show a non-interactive label; for multiple, show an accessible switcher; for none, route to `/workbench/no-organization`.
- [ ] If a switch is rejected or revoked, clear Organization-scoped queries and show a blocking access state. Do not silently fall back to the home Organization after a rejected explicit switch.
- [ ] Make `ApplicationFrame` route `/workbench/*` through the new shell and keep public/legacy routing separate.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/lib/api/workbench-context.test.ts src/components/providers/workbench-context-provider.test.tsx src/components/workbench/workspace-app-shell.test.tsx src/components/workbench/organization-switcher.test.tsx src/components/application-frame.test.tsx
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/lib/api/workbench-context* web/listingkit-ui/src/components/providers/workbench-context-provider* web/listingkit-ui/src/components/workbench web/listingkit-ui/src/app/workbench web/listingkit-ui/src/components/application-frame*
git commit -m "feat: add multi-organization workbench shell"
```

## Task 9: Protect Workbench Routes in Next.js

**Files:**

- Modify: `web/listingkit-ui/src/proxy.ts`
- Modify: `web/listingkit-ui/src/proxy.test.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.test.ts`

- [ ] Add failing proxy tests for unauthenticated `/workbench`, `/workbench/stores`, and `/workbench/no-organization` requests.
- [ ] Extend the matcher to `/workbench/:path*` and allow `/workbench` in `normalizeReturnTo` while preserving open-redirect protections.
- [ ] For workbench pages, require a valid Auth.js access token and canonical subject only. Remove flattened-role/tenant allowlist authorization from the browser edge for these routes; scoped authorization belongs to Go.
- [ ] Keep the existing ListingKit edge policy isolated until those routes are explicitly retired. Do not let workbench behavior inherit `hasPlatformAdminRole(identity.roles)` checks.
- [ ] Add regression tests proving an A-admin/B-viewer session can reach the shell but Go/API permissions still differ after switching.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/proxy.test.ts src/lib/server/zitadel-auth.test.ts
npm.cmd run typecheck
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/proxy.ts web/listingkit-ui/src/proxy.test.ts web/listingkit-ui/src/lib/server/zitadel-auth.ts web/listingkit-ui/src/lib/server/zitadel-auth.test.ts
git commit -m "feat: protect workbench with canonical zitadel session"
```

## Task 10: Complete Cross-Layer Verification

**Files:**

- Modify only files already named above when a verification failure proves a defect.

- [ ] Run all focused Go tests with the race detector.
- [ ] Run frontend unit tests, type checking, and lint.
- [ ] Start the API and UI with non-secret environment variables present and verify the process/port/page title before manual acceptance.
- [ ] With the real two-Organization test subject, verify A shows admin roles, B shows viewer roles, a forged cookie/header cannot select an unauthorized Organization, and switching back restores A only after a live grant check.
- [ ] Revoke A and verify: a write and a switch fail immediately; a cached read fails within 60 seconds; the cookie is cleared; no A data is rendered after the failure.
- [ ] Inspect logs to confirm no bearer token, cookie value, raw authorization response, or credential is logged.
- [ ] Record local checks separately from real-environment ZITADEL acceptance. Do not call the slice production-ready if only mocks passed.

**Verification:**

```powershell
go test ./internal/authidentity ./internal/authruntime/zitadel ./internal/workbenchcontext/... ./internal/httproute ./internal/app/httpapi -count=1 -race
Set-Location web/listingkit-ui
npm.cmd test -- src/proxy.test.ts src/lib/server/zitadel-auth.test.ts src/lib/server/workbench-proxy.test.ts "src/app/api/workbench/[...path]/route.test.ts" src/lib/api/workbench-context.test.ts src/components/providers/workbench-context-provider.test.tsx src/components/workbench/organization-switcher.test.tsx src/components/workbench/workspace-app-shell.test.tsx
npm.cmd run typecheck
npm.cmd run lint
```

Expected: every command exits 0 and Vitest reports that the named tests actually ran.

**Commit:**

```powershell
git status --short
git add docs/verification/zitadel-multi-organization-authorization.md internal/authidentity internal/authruntime/zitadel internal/workbenchcontext internal/httproute internal/app/httpapi web/listingkit-ui/src
git diff --cached --check
git commit -m "test: verify multi-organization workbench context"
```

Do not stage unrelated paths. If there is no verification fix, do not create an empty commit.

---

## Completion Gate

This plan is complete only when:

1. Real ZITADEL evidence proves two Organizations and different scoped roles for one subject.
2. Go rejects an unauthorized requested Organization regardless of cookie/header manipulation.
3. A live grant lookup protects switches and writes; cached reads expire within 60 seconds and token expiry.
4. The mounted middleware order is authentication → Organization resolution → scoped role authorization → handler.
5. The new workbench shell clears Organization-scoped client state on every successful or rejected switch.
6. No legacy numeric tenant resolution is reachable from a workbench route.
