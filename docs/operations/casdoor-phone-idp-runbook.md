# Casdoor Phone IdP Staging Runbook

This runbook is for staging only. It does not authorize a cluster apply,
database administration, DNS changes, or ZITADEL policy changes.

## Preconditions

- ZITADEL core and Login V2 are healthy at v4.17.1.
- A DBA has created database `casdoor` and role `casdoor_app` on the private
  `platform-data/shared-postgresql` service. The role has access only to that
  database. Do not put the shared PostgreSQL administrator password in this
  repository or command output.
- Secret Manager key `task-processor/staging/casdoor-phone-idp` contains the
  Casdoor database password, Tencent SMS settings, and the OIDC client secret.
  The ExternalSecret must be ready before the Casdoor Pod starts.
- `id.staging.shuomiai.com` is reserved for staging and is not a production
  login domain.

## Render and apply gate

Run the local checks first:

```powershell
pwsh -NoProfile -File scripts/tests/casdoor-kustomize-test.ps1
kubectl kustomize deployments/kubernetes/casdoor/overlays/staging | Out-Null
```

Only after separate staging deployment authorization, apply the rendered
manifests and wait for readiness:

```powershell
kubectl kustomize deployments/kubernetes/casdoor/overlays/staging | kubectl apply -f -
kubectl -n casdoor rollout status deployment/casdoor --timeout=10m
kubectl -n casdoor get externalsecret,secret,service,ingress
```

The last command may show resource names and readiness only; never print Secret
data.

## Initial administration

Before adding DNS or allowing external users, use a private port-forward to
open the Casdoor console and replace the built-in demo administrator password.
Do not expose a fresh Casdoor installation with its demo credential. Confirm
the console can be reached and the password was changed, then close the
port-forward.

In the Casdoor console:

1. Add a Tencent Cloud SMS provider with the staging credentials from Secret
   Manager. Configure the approved SMS template and test only with a disposable
   staging phone. Do not record the phone number, code, SecretId, or SecretKey.
2. Create the OIDC application `listingkit-phone-idp`.
3. Use authorization code flow, enable PKCE, and set the redirect URI to the
   exact ZITADEL Generic OIDC callback URL copied from the ZITADEL console.
4. Enable only verification-code sign-in/sign-up. Disable password sign-in,
   password reset, email recovery, and unused providers.
5. Record only the application ID and non-secret endpoint metadata needed by
   the next ZITADEL configuration step. Store the client secret only in the
   staging Secret Manager entry.

Casdoor's OIDC discovery and JWKS are checked without credentials:

```powershell
pwsh -NoProfile -File scripts/casdoor-phone-idp-preflight.ps1 `
  -IssuerURL https://id.staging.shuomiai.com
```

Expected output contains only the issuer, authorization endpoint, JWKS URI,
and key count.

## Abuse-control acceptance

Using one disposable staging phone, record only pass/fail and timestamps:

- first registration and repeat login succeed after a valid code;
- resend cooldown blocks immediate repeats;
- expired codes fail;
- five incorrect codes trigger temporary lockout;
- known and unknown numbers have equivalent error responses;
- no phone number or verification code appears in logs or evidence.
