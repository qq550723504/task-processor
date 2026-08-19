# ListingKit Device Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Let an operator acquire a short-lived interactive ZITADEL token and safely run the existing ListingKit acceptance workflow only as the expected canonical tenant.

**Architecture:** Add a read-only authenticated auth-context route that returns the already verified request identity. Add a private PowerShell helper for RFC 8628 discovery, device authorization, and in-memory polling; opt the current acceptance runner into it and require identity validation before its existing preflight or confirmed POST paths.

**Tech Stack:** Go, Gin, existing ListingKit HTTP route descriptors, PowerShell 7, Pester 3.4, OAuth 2.0/OIDC discovery, ZITADEL Device Authorization Grant.

**Spec:** docs/superpowers/specs/2026-08-19-listingkit-device-auth-design.md

## Global Constraints

- Interactive only: no service account, client credentials, refresh token, offline_access, CI credential, or unattended task creation.
- Do not read Kubernetes Secrets or use production client, directory, or invitation secrets.
- Tokens, Authorization headers, device codes, and verification-URI query parameters never reach disk, environment variables, test output, or error text.
- Production issuer/API URLs require HTTPS; HTTP is allowed only for literal loopback test endpoints.
- Discovered device/token endpoints must be same-origin with the configured issuer.
- Static environment/file token behavior is unchanged unless -UseDeviceAuthorization is explicitly supplied.
- Existing CREATE-1688-TASK confirmation and all subscription, quota, source, and store gates remain in force.
- Do not change PAY-042 identity mapping, usage ledger behavior, OpenMeter projection, or payment behavior.

---

## File Structure

| Path | Responsibility |
| --- | --- |
| internal/listingkit/api/auth_context_handler.go | Return verified canonical tenant, subject, and roles. |
| internal/listingkit/api/auth_context_handler_test.go | Prove exact response and missing-identity denial. |
| internal/listingkit/httpapi/routes_auth_context.go | Define protected GET descriptor and interface. |
| internal/listingkit/httpapi/routes_descriptor_entrypoints.go | Register the optional auth-context interface without widening existing route stubs. |
| internal/listingkit/httpapi/http_module_test.go | Guard route registration. |
| internal/app/httpapi/server_test.go | Prove real middleware protection. |
| scripts/lib/listingkit-device-auth.ps1 | OIDC validation, device request, polling, redaction. |
| scripts/1688-runtime-acceptance.ps1 | Opt-in device token and tenant gate. |
| scripts/1688-runtime-acceptance.Tests.ps1 | Mocked Pester contract coverage. |

## Task 1: Add a protected auth-context route

**Files:**
- Create: internal/listingkit/api/auth_context_handler.go
- Create: internal/listingkit/api/auth_context_handler_test.go
- Create: internal/listingkit/httpapi/routes_auth_context.go
- Modify: internal/listingkit/httpapi/routes_descriptor_entrypoints.go
- Modify: internal/listingkit/httpapi/http_module_test.go
- Modify: internal/app/httpapi/server_test.go

**Interfaces:**
- Consumes: listingkit.AuthenticatedIdentityFromContext(context.Context) (listingkit.AuthenticatedIdentity, bool).
- Produces: func (h *handler) GetAuthContext(c *gin.Context).
- Produces: type AuthContextRouteHandler interface { GetAuthContext(*gin.Context) }.
- Registration: AppendRouteDescriptors uses an optional AuthContextRouteHandler type assertion so existing RouteHandler test stubs stay source-compatible.
- HTTP: GET /api/v1/listing-kits/auth-context returns exactly tenant_id, user_id, and roles.

- [ ] **Step 1: Write failing handler tests**

~~~
func TestGetAuthContextReturnsVerifiedIdentity(t *testing.T) {
    router := gin.New()
    router.GET("/auth-context", (&handler{}).GetAuthContext)
    request := httptest.NewRequest(http.MethodGet, "/auth-context", nil).
        WithContext(listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{
            TenantID: "373211199677923496", UserID: "subject-1", Roles: []string{"listingkit_operator"},
        }))
    // Assert 200 and exact fields; assert no billing field or token value.
}
func TestGetAuthContextRejectsMissingIdentity(t *testing.T) {
    // Assert 403 and no identity payload.
}
~~~

- [ ] **Step 2: Run RED**

Run: go test ./internal/listingkit/api -run 'TestGetAuthContext' -count=1

Expected: FAIL because GetAuthContext does not exist.

- [ ] **Step 3: Implement the minimal handler**

~~~
func (h *handler) GetAuthContext(c *gin.Context) {
    identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
    if !ok || strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
        c.JSON(http.StatusForbidden, gin.H{"error": "zitadel_identity_missing", "message": "ZITADEL identity is required"})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "tenant_id": identity.TenantID,
        "user_id": identity.UserID,
        "roles": append([]string(nil), identity.Roles...),
    })
}
~~~

Do not resolve a billing tenant or expose the raw introspection response.

- [ ] **Step 4: Register the route and test actual middleware**

Define the descriptor in routes_auth_context.go. In AppendRouteDescriptors, use `handler.(AuthContextRouteHandler)` and append it only when the assertion succeeds; do not widen RouteHandler. Assert the descriptor in http_module_test.go. In server_test.go, assert no bearer token is rejected by the real middleware and the existing test bearer receives the exact three-field payload.

- [ ] **Step 5: Run GREEN and commit**

Run: go test ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/app/httpapi -run 'TestGetAuthContext|Test.*AuthContext|TestListingKit.*Route' -count=1

Expected: PASS; unauthenticated requests never reveal identity data.

~~~
git add -- internal/listingkit/api/auth_context_handler.go internal/listingkit/api/auth_context_handler_test.go internal/listingkit/httpapi/routes_auth_context.go internal/listingkit/httpapi/routes_descriptor_entrypoints.go internal/listingkit/httpapi/http_module_test.go internal/app/httpapi/server_test.go
git commit -m "feat: expose verified ListingKit auth context"
~~~

## Task 2: Add the in-memory device authorization helper

**Files:**
- Create: scripts/lib/listingkit-device-auth.ps1
- Modify: scripts/1688-runtime-acceptance.Tests.ps1

**Interfaces:**
- Produces: Resolve-ListingKitDeviceToken -IssuerURL <string> -ClientID <string> -TimeoutSec <int> -OpenBrowser <switch>.
- Produces: Assert-ListingKitDeviceURI -Uri <string> -Issuer <uri>.
- Returns: access-token string to the caller only.

- [ ] **Step 1: Write failing Pester validation tests**

~~~
It "rejects a non-HTTPS issuer before discovery" {
    { Resolve-ListingKitDeviceToken -IssuerURL "http://issuer.example" -ClientID "device-client" -TimeoutSec 30 } |
        Should Throw "*HTTPS*"
    Assert-MockCalled Invoke-RestMethod -Times 0 -Exactly
}
It "rejects a discovered token endpoint outside the issuer origin" {
    Mock Invoke-RestMethod { @{ device_authorization_endpoint = "https://issuer.example/device"; token_endpoint = "https://attacker.example/token" } }
    { Resolve-ListingKitDeviceToken -IssuerURL "https://issuer.example" -ClientID "device-client" -TimeoutSec 30 } |
        Should Throw "*same-origin*"
}
~~~

- [ ] **Step 2: Run RED**

Run: Invoke-Pester -Script ./scripts/1688-runtime-acceptance.Tests.ps1 -TestName 'rejects a non-HTTPS issuer before discovery','rejects a discovered token endpoint outside the issuer origin' -Verbose

Expected: FAIL because Resolve-ListingKitDeviceToken is undefined.

- [ ] **Step 3: Implement discovery and endpoint validation**

Use [Uri] validation. Accept HTTPS, or HTTP only for localhost, 127.0.0.1, or ::1. Fetch issuer plus /.well-known/openid-configuration. Require valid device_authorization_endpoint and token_endpoint with the issuer scheme, host, and effective port. Send client_id and scope=openid profile to the device endpoint. Require device_code, user_code, verification_uri, positive expires_in, and a positive polling interval (default 5).

Print only the validated verification URI and user code. Never print device_code.

- [ ] **Step 4: Implement polling and redaction**

~~~
switch ($oauthError) {
    "authorization_pending" { Start-Sleep -Seconds $pollInterval; continue }
    "slow_down" { $pollInterval += 5; Start-Sleep -Seconds $pollInterval; continue }
    "access_denied" { throw "Device authorization was denied" }
    "expired_token" { throw "Device authorization expired" }
    default { throw "Device authorization token exchange failed" }
}
~~~

Poll until the earlier provider expiry or TimeoutSec. Return a nonempty access_token directly without Write-Output. Sanitize provider failures so response bodies, token values, Authorization headers, device codes, and URI queries cannot appear.

- [ ] **Step 5: Add Pester success and error coverage**

Mock discovery, device authorization, and polling. Assert pending then success; slow_down changes the interval; denial, expiry, malformed responses, and timeout fail closed. Capture host/error output and assert it lacks access-token-sentinel, device-code-sentinel, and Authorization.

- [ ] **Step 6: Run GREEN and commit**

Run: Invoke-Pester -Script ./scripts/1688-runtime-acceptance.Tests.ps1 -Verbose

Expected: PASS using fake endpoints only.

~~~
git add -- scripts/lib/listingkit-device-auth.ps1 scripts/1688-runtime-acceptance.Tests.ps1
git commit -m "feat: add ListingKit device authorization helper"
~~~

## Task 3: Integrate explicit device-auth mode with the runner

**Files:**
- Modify: scripts/1688-runtime-acceptance.ps1
- Modify: scripts/1688-runtime-acceptance.Tests.ps1

**Interfaces:**
- Consumes: Resolve-ListingKitDeviceToken and auth-context JSON.
- Produces: Resolve-AcceptanceToken and Assert-AuthenticatedTenant.
- New CLI: -UseDeviceAuthorization, -IssuerURL, -ClientID, -ExpectedTenantID, -OpenBrowser.

- [ ] **Step 1: Write failing integration tests**

~~~
It "stops device-auth preflight before settings health when auth context tenant differs" {
    Mock Resolve-ListingKitDeviceToken { "access-token-sentinel" }
    Mock Invoke-AcceptanceRequest {
        param($Method, $Path)
        if ($Path -eq "/api/v1/listing-kits/auth-context") {
            return @{ tenant_id = "wrong-tenant"; user_id = "subject"; roles = @("listingkit_operator") }
        }
        throw "settings health must not run"
    }
    { Invoke-Preflight -Token "access-token-sentinel" -ExpectedTenantID "373211199677923496" } | Should Throw "*tenant*"
}
~~~

Also test an EndToEnd mismatch makes zero Post calls, and a run without -UseDeviceAuthorization still calls the old Resolve-ListingKitToken path.

- [ ] **Step 2: Run RED**

Run: Invoke-Pester -Script ./scripts/1688-runtime-acceptance.Tests.ps1 -TestName 'stops device-auth preflight before settings health when auth context tenant differs','keeps legacy token resolution when device authorization is absent' -Verbose

Expected: FAIL because the switch and tenant assertion do not exist.

- [ ] **Step 3: Implement explicit selection and tenant gate**

~~~
function Resolve-AcceptanceToken {
    if (-not $UseDeviceAuthorization) { return Resolve-ListingKitToken }
    if ([string]::IsNullOrWhiteSpace($IssuerURL) -or [string]::IsNullOrWhiteSpace($ClientID) -or [string]::IsNullOrWhiteSpace($ExpectedTenantID)) {
        throw "-IssuerURL, -ClientID, and -ExpectedTenantID are required with -UseDeviceAuthorization"
    }
    return Resolve-ListingKitDeviceToken -IssuerURL $IssuerURL -ClientID $ClientID -TimeoutSec $TimeoutSec -OpenBrowser:$OpenBrowser
}
function Assert-AuthenticatedTenant {
    param([string]$Token, [string]$ExpectedTenantID)
    $context = Get-ResponseData (Invoke-AcceptanceRequest -Method Get -Path "/api/v1/listing-kits/auth-context" -Token $Token)
    if ([string]$context.tenant_id -cne $ExpectedTenantID) { throw "authenticated tenant does not match -ExpectedTenantID" }
    if ([string]::IsNullOrWhiteSpace([string]$context.user_id)) { throw "authenticated identity is incomplete" }
}
~~~

Dot-source the helper from a path derived only from $PSScriptRoot. In device mode run Assert-AuthenticatedTenant before authenticated health, Crawl, or EndToEnd. Preserve public health checks and existing literal task confirmation.

- [ ] **Step 4: Run GREEN and commit**

Run: Invoke-Pester -Script ./scripts/1688-runtime-acceptance.Tests.ps1 -Verbose

Run: [void][scriptblock]::Create((Get-Content -Raw ./scripts/1688-runtime-acceptance.ps1)); [void][scriptblock]::Create((Get-Content -Raw ./scripts/lib/listingkit-device-auth.ps1)); 'parser=clean'

Expected: Pester passes and both scripts parse.

~~~
git add -- scripts/1688-runtime-acceptance.ps1 scripts/1688-runtime-acceptance.Tests.ps1
git commit -m "feat: support device-authorized ListingKit acceptance"
~~~

## Task 4: Publish safe operator instructions and final verification

**Files:**
- Modify: docs/superpowers/specs/2026-08-19-listingkit-device-auth-design.md
- Modify: docs/superpowers/plans/2026-08-19-listingkit-device-auth.md

- [ ] **Step 1: Add the safe preflight command template**

~~~
pwsh ./scripts/1688-runtime-acceptance.ps1 `
  -Mode Preflight `
  -UseDeviceAuthorization `
  -IssuerURL 'https://issuer.example' `
  -ClientID 'public-device-client-id' `
  -ExpectedTenantID '373211199677923496'
~~~

State that task creation still requires source/store input and -ConfirmCreateTask CREATE-1688-TASK. Do not add real tokens or product URLs.

Implemented in the design specification with the public-placeholder command:

~~~powershell
pwsh ./scripts/1688-runtime-acceptance.ps1 `
  -Mode Preflight `
  -UseDeviceAuthorization `
  -IssuerURL 'https://issuer.example' `
  -ClientID 'public-device-client-id' `
  -ExpectedTenantID '373211199677923496'
~~~

- [ ] **Step 2: Run final checks from the final commit**

Run: go test ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/app/httpapi -count=1

Run: go vet ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/app/httpapi

Run: Invoke-Pester -Script ./scripts/1688-runtime-acceptance.Tests.ps1 -Verbose

Run: git diff --check HEAD~3..HEAD

Expected: every command exits 0. Verify git status --short is clean and the diff contains no token, credential, or Kubernetes Secret change.

- [ ] **Step 3: Commit operational instructions**

~~~
git add -- docs/superpowers/specs/2026-08-19-listingkit-device-auth-design.md docs/superpowers/plans/2026-08-19-listingkit-device-auth.md
git commit -m "docs: record ListingKit device auth operation"
~~~

## Completion gate

Publish the code as a Draft PR only after Task 4 passes. CI proves code quality, not live acceptance. A device-authorized production canary remains separately authorized after a ZITADEL public device client is registered.
