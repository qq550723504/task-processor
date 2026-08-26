# Zitadel Authentication Runtime Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move ZITADEL discovery, token introspection, verified identity projection, and global authentication allowlist checks into `internal/authruntime/zitadel` without changing ListingKit compatibility APIs or route authorization behavior.

**Architecture:** `internal/authruntime/zitadel` owns provider-facing verification and neutral `authidentity` projection. `internal/listingkit/httpapi` retains a thin adapter for `ConfigureListingKitZitadelAuth` and `NewZitadelAuthMiddlewareFromEnv`, plus ListingKit route and permission authorization. The neutral package must not import ListingKit packages.

**Tech Stack:** Go, Gin, `net/http`, `httptest`, `go/ast` boundary scans, `testify`, and the existing ZITADEL introspection protocol.

**Spec:** `docs/superpowers/specs/2026-08-25-zitadel-authruntime-design.md`

## Global Constraints

- Preserve `503 zitadel_auth_not_configured`, `401 zitadel_token_missing`, `401 zitadel_token_invalid`, `403 zitadel_tenant_missing`, and `403 zitadel_user_missing` response contracts.
- Preserve five-second default HTTP timeout, request context cancellation, per-middleware discovery caching, header cleanup, and verified `authidentity` context projection.
- Keep `RouteRequiresZitadelAuth`, `NewRouteRoleMiddleware`, ListingKit permission mapping, and `ConfigureListingKitAuthorization` in `internal/listingkit/httpapi`.
- Keep `ConfigureListingKitZitadelAuth` and `NewZitadelAuthMiddlewareFromEnv` source-compatible as adapters.
- Do not modify SHEIN Login, Local Agent, 1688, tenant bridge, database Builder, deployment manifests, or provider protocol behavior.
- Do not log bearer tokens, client secrets, or raw introspection payloads.
- Run focused tests after each task; run `go test ./...` before delivery and report the known unrelated Tencent SMS baseline failures separately if they remain.

---

### Task 1: Define and test the neutral ZITADEL runtime contract

**Files:**
- Create: `internal/authruntime/zitadel/config.go`
- Create: `internal/authruntime/zitadel/parsing.go`
- Create: `internal/authruntime/zitadel/middleware.go`
- Create: `internal/authruntime/zitadel/middleware_test.go`

**Interfaces:**
- Produces `zitadel.Config`, `zitadel.AuthorizationConfig`, `zitadel.IntrospectionResponse`, `zitadel.NewMiddleware`, and `zitadel.ParseRoles` for the compatibility adapter and later tests.
- `NewMiddleware` accepts provider configuration and global authentication allowlist configuration; it must not accept `httproute.Descriptor`, `authz.ListingKitAuthorizer`, or any ListingKit package type.

- [ ] **Step 1: Write the failing neutral-package contract test**

Add tests that compile against this exact public contract:

```go
func TestNewMiddlewareRejectsMissingConfiguration(t *testing.T) {
    middleware := NewMiddleware(Config{}, AuthorizationConfig{})
    router := gin.New()
    router.Use(middleware)
    router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

    response := httptest.NewRecorder()
    router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

    require.Equal(t, http.StatusServiceUnavailable, response.Code)
    require.Contains(t, response.Body.String(), `"error":"zitadel_auth_not_configured"`)
}

func TestParseRolesDeduplicatesAllSupportedClaimShapes(t *testing.T) {
    roles := ParseRoles([]byte(`{"roles":["operator"],"role":"operator,admin","urn:zitadel:iam:org:project:roles":{"admin":{}}}`))
    require.Equal(t, []string{"operator", "admin"}, roles)
}
```

Also add red tests for missing Bearer token, discovery failure, inactive introspection, missing resource owner/subject, global allowlist rejection, and verified context/header projection. Use an `httptest.Server` for discovery and introspection; never use a real credential or endpoint.

- [ ] **Step 2: Run the new package tests and verify the expected red state**

Run:

```powershell
go test ./internal/authruntime/zitadel -count=1
```

Expected: FAIL because the new package and exported contract do not exist yet.

- [ ] **Step 3: Define the exact neutral types**

Create `config.go` with:

```go
type Config struct {
    IssuerURL    string
    ClientID     string
    ClientSecret string
    HTTPClient   *http.Client
}

type AuthorizationConfig struct {
    Required                          bool
    LegacyUsernameAllowlistConfigured bool
    AllowedTenantIDs                  map[string]struct{}
    AllowedUserIDs                    map[string]struct{}
    AllowedRoles                      map[string]struct{}
}

type IntrospectionResponse struct {
    Active     bool
    Subject    string
    Username   string
    UserID     string
    ResourceID string
    Roles      []string
    Extra      json.RawMessage
}
```

Keep these types independent of ListingKit route descriptors and ListingKit authorizer types. The five-second default client is created by `NewMiddleware` when `Config.HTTPClient` is nil.

- [ ] **Step 4: Implement parsing and set helpers**

Move the behavior currently in `zitadel_auth_parsing_helpers.go` into `parsing.go` with these exact exported entry points:

```go
func ParseRoles(data []byte) []string
func StringSliceToSet(values []string) map[string]struct{}
```

Keep `firstNonEmpty`, `valueInSet`, and role-map parsing helpers private to `internal/authruntime/zitadel`. Preserve trim, empty-value, claim-order, and deduplication behavior.

- [ ] **Step 5: Implement the neutral middleware**

Create `NewMiddleware(cfg Config, authzCfg AuthorizationConfig) gin.HandlerFunc` and move the current behavior from `zitadel_auth_middleware.go`:

```go
func NewMiddleware(cfg Config, authzCfg AuthorizationConfig) gin.HandlerFunc {
    if cfg.HTTPClient == nil {
        cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
    }
    cfg.IssuerURL = strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
    cfg.ClientID = strings.TrimSpace(cfg.ClientID)
    return (&middleware{cfg: cfg, authz: authzCfg}).Handle
}
```

The handler must call `authidentity.WithAuthenticatedIdentity`, clear existing identity headers before writing verified values, preserve current JSON errors/statuses, and use request context for discovery and introspection requests. Keep discovery cache and mutex state on the middleware instance.

- [ ] **Step 6: Run the neutral package tests and commit the standalone runtime**

Run:

```powershell
gofmt -w internal/authruntime/zitadel/*.go
go test ./internal/authruntime/zitadel -count=1
```

Expected: PASS. Commit only the new package:

```powershell
git add -- internal/authruntime/zitadel
git commit -m "refactor: add neutral Zitadel auth runtime"
```

### Task 2: Replace ListingKit implementation with compatibility adapters

**Files:**
- Modify: `internal/listingkit/httpapi/zitadel_auth.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_runtime.go`
- Delete: `internal/listingkit/httpapi/zitadel_auth_middleware.go`
- Delete: `internal/listingkit/httpapi/zitadel_auth_parsing_helpers.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_route_authorization.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_test.go`
- Modify: `internal/listingkit/httpapi/phase5_zitadel_auth_boundary_test.go`

**Interfaces:**
- Consumes `zitadel.Config`, `zitadel.AuthorizationConfig`, and `zitadel.NewMiddleware` from Task 1.
- Produces unchanged `ConfigureListingKitZitadelAuth`, `SetListingKitZitadelAuthConfigForTesting`, `NewZitadelAuthMiddlewareFromEnv`, `ConfigureListingKitAuthorization`, `RouteRequiresZitadelAuth`, and `NewRouteRoleMiddleware` behavior.

- [ ] **Step 1: Add a failing adapter ownership test**

Extend `phase5_zitadel_auth_boundary_test.go` so the ListingKit runtime file must contain only compatibility functions and must not contain a provider middleware method. Add a neutral package ownership assertion that `internal/authruntime/zitadel/middleware.go` contains `NewMiddleware` and `Handle`, while `internal/listingkit/httpapi/zitadel_auth_middleware.go` does not exist.

Run:

```powershell
go test ./internal/listingkit/httpapi -run 'TestZitadelAuth(File|Runtime|Middleware|RouteAuthorization)' -count=1
```

Expected: FAIL because the old implementation still exists in ListingKit HTTPAPI.

- [ ] **Step 2: Convert ListingKit private auth types to neutral aliases and retain only adapter state**

In `zitadel_auth.go`, import `task-processor/internal/authruntime/zitadel` under alias `zitadelruntime` and replace provider implementation structs with compatibility aliases:

```go
type zitadelAuthConfig = zitadelruntime.Config
type zitadelAuthorizationConfig = zitadelruntime.AuthorizationConfig
```

Keep `listingKitZitadelRuntimeConfig` only for the adapter's current provider config and ListingKit `Authorizer`. Do not keep `zitadelAuthMiddleware`, discovery, or introspection implementation types in this package.

- [ ] **Step 3: Delegate configuration and middleware construction**

In `zitadel_auth_runtime.go`, preserve existing `ConfigureListingKitZitadelAuth` input translation, but store neutral `Config` and `AuthorizationConfig` values. Replace the old constructor call with:

```go
return zitadelruntime.NewMiddleware(runtimeCfg.AuthConfig, runtimeCfg.AuthzConfig)
```

Keep `ConfigureListingKitAuthorization` unchanged except for type alias adjustments. Keep `SetListingKitZitadelAuthConfigForTesting` as the compatibility test seam so current ListingKit tests can isolate global state.

- [ ] **Step 4: Remove provider implementation and keep route authorization local**

Delete `zitadel_auth_middleware.go` and `zitadel_auth_parsing_helpers.go`. Remove only `authorizeZitadelIdentity` and its provider-response dependency from `zitadel_auth_route_authorization.go`; retain route matching, permission mapping, identity-context lookup, and `NewRouteRoleMiddleware` unchanged.

Update the parsing test in `zitadel_auth_test.go` to call `zitadelruntime.ParseRoles` directly. Keep existing end-to-end ListingKit tests using `NewZitadelAuthMiddlewareFromEnv`; they verify the compatibility adapter preserves external behavior.

- [ ] **Step 5: Update static ownership tests and run compatibility tests**

Rewrite `phase5_zitadel_auth_boundary_test.go` to assert:

- `zitadel_auth_runtime.go` owns `ConfigureListingKitZitadelAuth` and `NewZitadelAuthMiddlewareFromEnv` only;
- `zitadel_auth_route_authorization.go` owns route policy and `NewRouteRoleMiddleware`;
- `internal/authruntime/zitadel` owns middleware, discovery, introspection, and parsing;
- the retired ListingKit implementation files are absent.

Run:

```powershell
go test ./internal/authruntime/zitadel ./internal/listingkit/httpapi -run 'Test(ZitadelAuth|RouteRequiresZitadelAuth|ConfigureListingKitAuthorization)' -count=1
```

Expected: PASS with the same response and identity behavior as the baseline.

- [ ] **Step 6: Commit the compatibility migration**

```powershell
git add -- internal/authruntime/zitadel internal/listingkit/httpapi
git commit -m "refactor: route ListingKit Zitadel auth through neutral runtime"
```

### Task 3: Add the dependency-direction guard and update architecture checklist

**Files:**
- Modify: `tests/import_boundaries_test.go`
- Modify: `docs/architecture/architecture-review-checklist.md`
- Modify: `internal/listingkit/httpapi/phase5_zitadel_auth_boundary_test.go`

**Interfaces:**
- Consumes the AST/index helpers already used by `tests/import_boundaries_test.go`.
- Produces a regression guard for the neutral runtime package and the compatibility-only ListingKit bridge.

- [ ] **Step 1: Write the dependency-direction boundary test**

Add `TestZitadelAuthRuntimeDoesNotImportListingKit` that scans non-test Go files below `internal/authruntime/zitadel` and fails if any file imports:

```text
task-processor/internal/listingkit
task-processor/internal/listingkit/httpapi
```

Run:

```powershell
go test ./tests -run 'TestZitadelAuthRuntimeDoesNotImportListingKit' -count=1
```

Expected: PASS with the current neutral package; if any later change adds a reverse dependency, the test must fail with the source path and banned import.

- [ ] **Step 2: Make the guard path- and alias-independent**

Use the existing package-index/import-path matching helpers rather than matching a local identifier. Inspect every non-test Go file in the new package and report the source path and banned import.

- [ ] **Step 3: Update the review checklist**

Add this exact guard to `docs/architecture/architecture-review-checklist.md` near the authenticated identity guards:

```text
TestZitadelAuthRuntimeDoesNotImportListingKit
```

- [ ] **Step 4: Run boundary and architecture tests, then commit**

```powershell
go test ./tests -run 'TestZitadelAuthRuntimeDoesNotImportListingKit|TestAuthenticatedIdentityRootImportsStayRestricted|TestArchitectureReviewChecklistTracksEveryImportBoundaryGuard' -count=1
git diff --check
git add -- tests/import_boundaries_test.go docs/architecture/architecture-review-checklist.md internal/listingkit/httpapi/phase5_zitadel_auth_boundary_test.go
git commit -m "test: guard neutral Zitadel auth dependency direction"
```

### Task 4: Full verification and delivery checkpoint

**Files:**
- No new production files; inspect all changes from Tasks 1–3.

**Interfaces:**
- Verifies the compatibility adapter, neutral runtime, route authorization, and boundary guards as one integrated slice.

- [ ] **Step 1: Run focused behavior and boundary tests**

```powershell
go test ./internal/authruntime/zitadel ./internal/listingkit/httpapi ./internal/app/httpapi ./tests -run 'Test(ZitadelAuth|RouteRequiresZitadelAuth|ConfigureListingKitAuthorization|ZitadelAuthRuntimeDoesNotImportListingKit|AuthenticatedIdentityRoot|ArchitectureReviewChecklistTracksEveryImportBoundaryGuard)' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run formatting and static checks**

```powershell
gofmt -w internal/authruntime/zitadel internal/listingkit/httpapi tests/import_boundaries_test.go
git diff --check
go test ./internal/authruntime/zitadel ./internal/listingkit/httpapi ./internal/app/httpapi ./tests -run 'TestZitadelAuth' -count=1
```

Expected: PASS with no diff-check errors.

- [ ] **Step 3: Run the repository suite and record unrelated baseline failures**

```powershell
go test ./...
```

Expected: all touched packages pass. If the existing six Tencent SMS secret assertions in `tests/commercial_readiness_workflow_test.go` remain, record them as unrelated baseline failures and do not change them in this slice.

- [ ] **Step 4: Review the final diff and commit state**

```powershell
git status --short --branch
git diff HEAD~3..HEAD --stat
git diff HEAD~3..HEAD --check
```

Confirm that no SHEIN Login, Local Agent, 1688, tenant bridge, database Builder, deployment, or unrelated configuration files changed.

- [ ] **Step 5: Request independent review before PR delivery**

Provide the task commits and focused/full test results to a read-only reviewer. Fix Critical or Important findings before creating a PR. Keep the worktree for review iteration; do not merge or deploy as part of this plan.
