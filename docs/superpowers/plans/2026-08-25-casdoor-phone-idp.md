# Casdoor Phone Identity Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a staging-only, locked-down Casdoor phone-code IdP and its ZITADEL federation contract without phone leakage, account takeover, or default ListingKit grants.

**Architecture:** Native Kustomize manifests run Casdoor 3.143.0 (manifest digest `sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58`) in `casdoor`, using a logically isolated `casdoor` database and `casdoor_app` role on the existing private `platform-data/shared-postgresql` service. Casdoor authenticates phone users upstream; ZITADEL Generic OIDC plus one External Authentication action creates the final ZITADEL user and remains the only ListingKit issuer/role authority.

**Tech Stack:** Casdoor 3.143.0, existing shared PostgreSQL 18.4 for staging, Kubernetes/Kustomize, Traefik, External Secrets, Tencent Cloud SMS, ZITADEL Generic OIDC and Actions.

**Spec:** `docs/superpowers/specs/2026-08-25-listingkit-phone-identity-design.md`

## Global Constraints

- Use native manifests, not the current Casdoor Helm chart: PostgreSQL rendering has a documented regression in recent chart versions.
- Pin the staging-verified Casdoor 3.143.0 image digest `sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58`; never use `latest`.
- Use only the staging Secret Manager key `task-processor/staging/casdoor-phone-idp`; do not reuse `listingkit-tencent-sms-secret` or its signing key.
- Casdoor gives ListingKit no token, tenant membership, or role. ListingKit consumes the final ZITADEL `sub` only.
- Password sign-in, password reset, email recovery, unused IdPs, automatic account linking and automatic profile update are disabled.
- This plan creates no PostgreSQL workload, PVC, database administrator Job, production Ingress, or production DNS record.
- Creating the `casdoor` database and `casdoor_app` role on `shared-postgresql` is a separate redacted DBA operation and is not automated by these manifests.
- No Tencent SMS credential or ZITADEL client secret may appear in Git, command output, test fixtures, or evidence.

---

### Task 1: Create isolated and render-tested Casdoor manifests

**Files:**
- Create: `deployments/kubernetes/casdoor/base/namespace.yaml`
- Create: `deployments/kubernetes/casdoor/base/{configmap,deployment,service,ingress,kustomization}.yaml`
- Create: `deployments/kubernetes/casdoor/overlays/staging/{kustomization,patch-config,patch-ingress,external-secret}.yaml`
- Create: `scripts/tests/casdoor-kustomize-test.ps1`

**Interfaces:**
- Consumes: Secret keys `CASDOOR_POSTGRES_PASSWORD`, `CASDOOR_TENCENT_SECRET_ID`, `CASDOOR_TENCENT_SECRET_KEY`, `CASDOOR_TENCENT_SMS_APP_ID`, `CASDOOR_TENCENT_SMS_SIGN_NAME`, `CASDOOR_TENCENT_SMS_TEMPLATE_ID`, and `CASDOOR_OIDC_CLIENT_SECRET`.
- Produces: public HTTPS staging Casdoor service on port 8000 and one Casdoor-only Kubernetes Secret. It does not create a PostgreSQL workload, PVC, database administrator Job, or public database service.

- [ ] **Step 1: Write the failing render test.**

```bash
#!/usr/bin/env bash
set -euo pipefail
rendered="$(kustomize build deployments/kubernetes/casdoor/overlays/staging)"
grep -F 'namespace: casdoor' <<<"$rendered"
grep -F 'image: casbin/casdoor:3.143.0@sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58' <<<"$rendered"
grep -F 'host: id.staging.shuomiai.com' <<<"$rendered"
grep -F 'driverName = postgres' <<<"$rendered"
grep -F 'shared-postgresql.platform-data.svc.cluster.local' <<<"$rendered"
! grep -Eqi 'image: .*:latest|listingkit-tencent-sms-secret|TASK_PROCESSOR_LISTINGKIT_ZITADEL_SMS_SIGNING_KEY' <<<"$rendered"
```

- [ ] **Step 2: Run the test.**

Run: `pwsh -File scripts/tests/casdoor-kustomize-test.ps1`

Expected: FAIL because no Casdoor overlay exists.

- [ ] **Step 3: Implement the base workload and explicit Secret reference.**

```yaml
containers:
  - name: casdoor
    image: casbin/casdoor:3.143.0@sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58
    ports: [{name: http, containerPort: 8000}]
    envFrom: [{secretRef: {name: casdoor-phone-idp-secret}}]
    volumeMounts: [{name: config, mountPath: /conf/app.conf, subPath: app.conf}]
volumes: [{name: config, configMap: {name: casdoor-config}}]
```

Set `driverName = postgres`, use only `shared-postgresql.platform-data.svc.cluster.local`, and make base ingress non-routable. A separately authorized, redacted administrative operation creates the `casdoor` database and least-privilege `casdoor_app` role; the manifests never read or copy the shared PostgreSQL administrator secret. Each overlay sets origin, TLS, ExternalSecret remote key and the digest after staging verification.

- [ ] **Step 4: Add and attach the public endpoint rate limiter.**

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata: {name: casdoor-auth-rate-limit, namespace: casdoor}
spec: {rateLimit: {average: 5, burst: 10}}
```

Attach `casdoor-auth-rate-limit@kubernetescrd` to the ingress. Casdoor's separately configured per-phone retry/lockout behavior remains mandatory.

- [ ] **Step 5: Run render tests and commit the infrastructure slice.**

Run: `pwsh -File scripts/tests/casdoor-kustomize-test.ps1`

Expected: PASS; no latest tag, no SQLite, no ListingKit secret, no PostgreSQL workload, and no production host.

```powershell
git add deployments/kubernetes/casdoor scripts/tests/casdoor-kustomize-test.ps1
git commit -m "feat: add isolated Casdoor phone IdP manifests"
```

### Task 2: Prove passwordless phone OIDC and abuse controls in staging

**Files:**
- Create: `scripts/casdoor-phone-idp-preflight.ps1`
- Test: `scripts/casdoor-phone-idp-preflight.Tests.ps1`
- Create: `docs/operations/casdoor-phone-idp-runbook.md`

**Interfaces:**
- Consumes: staging issuer URL, its discovery document, Casdoor-only Tencent credentials and a non-production phone.
- Produces: a verified issuer, JWKS, PKCE code flow and bounded phone-code behavior without Secret output.

- [ ] **Step 1: Write the failing read-only discovery test.**

```powershell
Describe "casdoor-phone-idp-preflight" {
  It "reads discovery without a credential" {
    $c = Get-Content -Raw scripts/casdoor-phone-idp-preflight.ps1
    $c | Should Match '\.well-known/openid-configuration'
    $c | Should Not Match 'SecretKey|Authorization:|Bearer |kubectl.+get secret'
  }
}
```

- [ ] **Step 2: Implement and test the discovery/JWKS preflight.**

```powershell
param([Parameter(Mandatory)][uri]$IssuerURL)
$issuer = $IssuerURL.ToString().TrimEnd('/')
$d = Invoke-RestMethod -Uri ($issuer + '/.well-known/openid-configuration')
if ($d.issuer -ne $issuer -or [string]::IsNullOrWhiteSpace($d.authorization_endpoint) -or [string]::IsNullOrWhiteSpace($d.jwks_uri)) { throw 'invalid Casdoor OIDC discovery' }
[pscustomobject]@{issuer=$d.issuer; authorizationEndpoint=$d.authorization_endpoint; jwksUri=$d.jwks_uri} | ConvertTo-Json -Compress
```

Run: `Invoke-Pester -Path scripts/casdoor-phone-idp-preflight.Tests.ps1 -Verbose`

Expected: PASS; output has endpoints only.

- [ ] **Step 3: Add the staging console configuration and black-box matrix to the runbook.**

```markdown
Application name is `listingkit-phone-idp`; it has exactly the ZITADEL callback URI, authorization-code grant and required PKCE. Enable only phone verification-code signup/signin. Disable password, password reset and email recovery. Emit `https://shuomiai.com/claims/phone_verified=true` only after phone verification.

Using a non-production phone, prove new registration, repeat login, resend cooldown, five incorrect codes causing temporary lockout, expired-code rejection, and equivalent error responses for known and unknown numbers. Record no number or code.
```

- [ ] **Step 4: Run staging acceptance and commit.**

Run: `./scripts/casdoor-phone-idp-preflight.ps1 -IssuerURL https://id.staging.shuomiai.com`

Expected: issuer, authorization endpoint and JWKS URI are HTTPS and match the staging domain; the manual matrix passes.

```powershell
git add scripts/casdoor-phone-idp-preflight.ps1 scripts/casdoor-phone-idp-preflight.Tests.ps1 docs/operations/casdoor-phone-idp-runbook.md
git commit -m "docs: add Casdoor phone IdP acceptance runbook"
```

### Task 3: Federate verified Casdoor identities to ZITADEL

**Files:**
- Create: `deployments/kubernetes/casdoor/zitadel-actions/map-casdoor-phone-identity.js`
- Create: `scripts/zitadel-casdoor-federation-preflight.ps1`
- Test: `scripts/zitadel-casdoor-federation-preflight.Tests.ps1`
- Create: `docs/operations/zitadel-casdoor-phone-federation-runbook.md`

**Interfaces:**
- Consumes: Task 2 issuer and ZITADEL Generic OIDC settings.
- Produces: a ZITADEL external identity linked by Casdoor `sub`, technical email `casdoor-<sub>@phone.id.shuomiai.invalid`, and no ListingKit role or tenant grant.

- [ ] **Step 1: Write the action and its static safety tests.**

```javascript
function mapCasdoorPhoneIdentity(ctx, api) {
  const claims = ctx.claimsJSON();
  if (claims.iss !== "https://id.shuomiai.com") return;
  if (typeof claims.sub !== "string" || !/^[A-Za-z0-9_-]{8,128}$/.test(claims.sub) || claims["https://shuomiai.com/claims/phone_verified"] !== true) throw new Error("verified Casdoor phone identity required");
  const id = claims.sub, alias = `casdoor-${id}@phone.id.shuomiai.invalid`;
  api.setFirstName(typeof claims.given_name === "string" && claims.given_name ? claims.given_name : "Phone");
  api.setLastName(typeof claims.family_name === "string" && claims.family_name ? claims.family_name : "User");
  api.setDisplayName(typeof claims.name === "string" && claims.name ? claims.name : "Phone user");
  api.setUsername(`casdoor-${id}`); api.setEmail(alias); api.setEmailVerified(true);
}
```

Static Pester checks must require the fixed issuer, phone-verification gate and technical domain while forbidding `phone_number`, `api.userGrants`, `fetch`, and logging.

- [ ] **Step 2: Run the failing static test, then add the action and preflight.**

Run: `Invoke-Pester -Path scripts/zitadel-casdoor-federation-preflight.Tests.ps1 -Verbose`

Expected: FAIL until action and preflight exist.

- [ ] **Step 3: Implement the read-only ZITADEL provider policy check.**

```powershell
param([Parameter(Mandatory)][uri]$IssuerURL, [Parameter(Mandatory)][string]$ProviderID)
$token = [string]$env:ZITADEL_ADMIN_TOKEN
if ([string]::IsNullOrWhiteSpace($token)) { throw "ZITADEL_ADMIN_TOKEN is required" }
$headers = @{ Authorization = "Bearer $token"; "Content-Type" = "application/json" }
$providers = Invoke-RestMethod -Method Post -Uri ($IssuerURL.ToString().TrimEnd('/') + "/admin/v1/idps/_search") -Headers $headers -Body "{}"
$provider = @($providers.result | Where-Object id -eq $ProviderID)[0]
$policy = Invoke-RestMethod -Uri ($IssuerURL.ToString().TrimEnd('/') + "/admin/v1/policies/login") -Headers $headers
if ($null -eq $provider -or $provider.config.issuer -ne "https://id.shuomiai.com" -or $provider.config.scopes -notcontains "openid" -or -not $provider.config.isCreationAllowed -or $provider.config.isLinkingAllowed -or $provider.config.isAutoUpdate -or -not $policy.externalLogin) { throw "unsafe Casdoor IdP policy" }
[pscustomobject]@{ providerID=$provider.id; issuer=$provider.config.issuer; scopes=@($provider.config.scopes); creationAllowed=[bool]$provider.config.isCreationAllowed; linkingAllowed=[bool]$provider.config.isLinkingAllowed; automaticUpdate=[bool]$provider.config.isAutoUpdate; externalLogin=[bool]$policy.externalLogin } | ConvertTo-Json -Compress
```

- [ ] **Step 4: Configure staging via the runbook and execute negative cases.**

```markdown
Create Generic OIDC provider `手机号登录` with staging issuer, scopes `openid profile email`, PKCE on, automatic creation on, automatic update off, account creation allowed, and account linking off. Attach the action only to External Authentication/Post Authentication; attach no Post Creation grant action.

Verify a new phone user gets a ZITADEL subject but no ListingKit role, and that an old email user with matching display data is not linked. Wrong issuer/audience, expired token, and missing phone_verified must fail before user creation.
```

- [ ] **Step 5: Run all federation checks and commit.**

Run: `Invoke-Pester -Path scripts/zitadel-casdoor-federation-preflight.Tests.ps1 -Verbose; pwsh -NoProfile -File ./scripts/casdoor-phone-idp-preflight.ps1 -IssuerURL https://id.staging.shuomiai.com`

Expected: PASS; staging evidence proves no automatic linking and no default grant.

```powershell
git add deployments/kubernetes/casdoor/zitadel-actions/map-casdoor-phone-identity.js scripts/zitadel-casdoor-federation-preflight.ps1 scripts/zitadel-casdoor-federation-preflight.Tests.ps1 docs/operations/zitadel-casdoor-phone-federation-runbook.md
git commit -m "feat: define Casdoor phone identity federation"
```

### Task 4: Staging acceptance evidence

**Files:**
- Modify: `docs/operations/casdoor-phone-idp-runbook.md`
- Modify: `docs/operations/zitadel-casdoor-phone-federation-runbook.md`

**Interfaces:**
- Consumes: green Tasks 1-3, ZITADEL v4.17.1, a staging DNS/TLS entry, a non-production phone, and explicit staging deployment authorization.
- Produces: redacted staging evidence for phone login, no automatic linking, no default ListingKit grant, and optional SMS OTP enrollment; MFA is not forced globally.

- [ ] **Step 1: Add the staging checklist.**

```markdown
- [ ] ZITADEL core and Login V2 are v4.17.1 and healthy.
- [ ] The shared PostgreSQL `casdoor` database and least-privilege `casdoor_app` role were created by the separately authorized DBA operation.
- [ ] Casdoor phone-code limits, OIDC claims, no-link and no-grant tests are recorded without a phone number or code.
- [ ] Staging DNS, TLS, ExternalSecret and Ingress readiness are verified without Secret values.
- [ ] Generic OIDC linking/update are disabled; OTP SMS is permitted but not globally required.
```

- [ ] **Step 2: Apply staging only after authorization, then validate before IdP activation.**

Run: `kustomize build deployments/kubernetes/casdoor/overlays/staging | kubectl apply -f -; kubectl -n casdoor rollout status deployment/casdoor --timeout=10m`

Expected: Casdoor is healthy; stop on failed readiness, ExternalSecret or preflight. Do not apply this command to a production overlay.

- [ ] **Step 3: Execute a disposable staging-phone acceptance.**

```markdown
Register one disposable staging phone identity. Verify its final ZITADEL token is denied ListingKit access with no role. Grant one existing allowed role through member management, verify only the intended tenant becomes accessible, record redacted evidence, then remove the disposable user through normal identity administration.
```

- [ ] **Step 4: Commit staging acceptance documentation.**

```powershell
git add docs/operations/casdoor-phone-idp-runbook.md docs/operations/zitadel-casdoor-phone-federation-runbook.md
git commit -m "docs: add phone identity staging acceptance checklist"
```
