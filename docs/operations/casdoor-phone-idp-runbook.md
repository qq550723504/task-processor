# Casdoor Phone IdP Runbook

Operate the self-hosted Casdoor phone-code identity provider at
`id.shuomiai.com` (staging: `id.staging.shuomiai.com`). Casdoor authenticates
phone users upstream; ZITADEL remains the only ListingKit issuer/role authority.

## Staging console configuration

Application name is `listingkit-phone-idp`; it has exactly the ZITADEL callback
URI, authorization-code grant and required PKCE. Enable only phone
verification-code signup/signin. Disable password, password reset and email
recovery. Emit `https://shuomiai.com/claims/phone_verified=true` only after
phone verification.

## Preflight

Read-only discovery check (no credentials, no Secret output):

```bash
./scripts/casdoor-phone-idp-preflight.ps1 -IssuerURL https://id.staging.shuomiai.com
```

Expected: issuer, authorization endpoint and JWKS URI are HTTPS and match the
staging domain.

## Black-box acceptance matrix

Using a non-production phone, prove new registration, repeat login, resend
cooldown, five incorrect codes causing temporary lockout, expired-code
rejection, and equivalent error responses for known and unknown numbers.
Record no number or code.

## Production gated acceptance

Apply only after explicit production authorization, with ZITADEL core and
Login V2 on v4.17.1:

- [ ] ZITADEL core and Login V2 are v4.17.1 and healthy.
- [ ] Casdoor backup restore, phone-code limits, OIDC claims, no-link and no-grant tests are recorded.
- [ ] DNS, TLS, ExternalSecret and Ingress readiness are verified without Secret values.
- [ ] Generic OIDC linking/update are disabled; OTP SMS is permitted but not globally required.

```bash
kustomize build deployments/kubernetes/casdoor/overlays/prod | kubectl apply -f -
kubectl -n casdoor rollout status deployment/casdoor --timeout=10m
```

Stop on failed readiness, ExternalSecret or preflight. Then execute a
disposable real-device acceptance: register one disposable phone identity,
verify its final ZITADEL token is denied ListingKit access with no role, grant
one existing allowed role through member management, verify only the intended
tenant becomes accessible, record redacted evidence, then remove the
disposable user through normal identity administration.
