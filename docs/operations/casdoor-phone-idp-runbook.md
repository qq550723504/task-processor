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

## Rotating the database password

`POSTGRES_PASSWORD` only seeds the `casdoor` role when its volume is empty, so
an hourly ExternalSecret refresh never updates the persisted role. Never rotate
`CASDOOR_POSTGRES_PASSWORD` by changing the secret manager value alone; use
this coordinated procedure:

1. Scale the `casdoor` deployment to zero so no pod renders with mismatched
   credentials: `kubectl -n casdoor scale deployment/casdoor --replicas=0`.
2. Update the persisted role using the still-valid current password:
   `kubectl -n casdoor exec casdoor-postgres-0 -- psql -U casdoor -c "ALTER ROLE casdoor WITH PASSWORD '<new>'"`.
3. Change `CASDOOR_POSTGRES_PASSWORD` in the secret manager and force an
   ExternalSecret sync; verify the Kubernetes Secret carries the new value.
4. Restore serving: `kubectl -n casdoor scale deployment/casdoor --replicas=1`
   and confirm readiness plus one successful phone login.

## Production gated acceptance

Apply only after explicit production authorization, with ZITADEL core and
Login V2 on v4.17.1:

- [ ] ZITADEL core and Login V2 are v4.17.1 and healthy.
- [ ] Casdoor backup restore, phone-code limits, OIDC claims, no-link and no-grant tests are recorded.
- [ ] The prod overlay pins `casbin/casdoor` to the exact digest accepted in staging before rendering; the applied manifests reference the digest, never a mutable tag.
- [ ] DNS, TLS, ExternalSecret and Ingress readiness are verified without Secret values.
- [ ] Generic OIDC linking/update are disabled; OTP SMS is permitted but not globally required.

Pin the production image to the staging-verified immutable digest, then render
and apply. Stop if the digest was never recorded from staging acceptance:

```bash
cd deployments/kubernetes/casdoor/overlays/prod
kustomize edit set image casbin/casdoor=casbin/casdoor@sha256:<staging-verified-digest>
kustomize build . | grep 'image: casbin/casdoor@'   # confirm no mutable tag remains
kustomize build . | kubectl apply -f -
kubectl -n casdoor rollout status deployment/casdoor --timeout=10m
cd - >/dev/null
```

Stop on failed readiness, ExternalSecret or preflight. Then execute a
disposable real-device acceptance: register one disposable phone identity,
verify its final ZITADEL token is denied ListingKit access with no role, grant
one existing allowed role through member management, verify only the intended
tenant becomes accessible, record redacted evidence, then remove the
disposable user through normal identity administration.
