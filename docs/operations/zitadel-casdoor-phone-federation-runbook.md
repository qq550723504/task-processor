# ZITADEL Casdoor Phone Federation Staging Runbook

This runbook configures a staging-only external OIDC provider. It does not
grant ListingKit access automatically and does not activate a production IdP.

## Casdoor application contract

Create the Casdoor OIDC application `listingkit-phone-idp` with:

- authorization-code grant and PKCE enabled;
- the exact ZITADEL Generic OIDC callback copied from the ZITADEL console;
- scopes `openid profile email`;
- verification-code sign-in/sign-up only;
- password sign-in, password reset, email recovery, and unused providers off.

Keep the Casdoor client secret only in the staging Secret Manager entry. Never
place it in this repository, a shell argument, or an evidence record.

## ZITADEL provider policy

In the ZITADEL staging instance, create a Generic OIDC provider with:

- issuer `https://id.staging.shuomiai.com`;
- scopes `openid profile email`;
- PKCE enabled;
- automatic account creation enabled;
- account linking disabled;
- automatic profile update disabled;
- external login enabled for the staging organization.

Attach `deployments/kubernetes/casdoor/zitadel-actions/map-casdoor-phone-identity.js`
as the external-authentication/post-authentication action. The action accepts
only the fixed staging issuer, a stable Casdoor `sub`, and the verified-phone
claim. It derives a technical `.invalid` email and does not copy the phone
number, grant roles, or link an existing user by display data.

Run the read-only policy preflight with the administrator token supplied only
through the process environment:

```powershell
if ([string]::IsNullOrWhiteSpace($env:ZITADEL_ADMIN_TOKEN)) { throw 'inject ZITADEL_ADMIN_TOKEN out-of-band' }
if ([string]::IsNullOrWhiteSpace($env:ZITADEL_CASDOOR_PROVIDER_ID)) { throw 'inject ZITADEL_CASDOOR_PROVIDER_ID out-of-band' }
pwsh -NoProfile -File scripts/zitadel-casdoor-federation-preflight.ps1 `
  -ZitadelURL https://auth.shuomiai.com `
  -ProviderID $env:ZITADEL_CASDOOR_PROVIDER_ID `
  -ExpectedProviderIssuer https://id.staging.shuomiai.com
Remove-Item Env:ZITADEL_ADMIN_TOKEN
Remove-Item Env:ZITADEL_CASDOOR_PROVIDER_ID
```

The output contains provider policy metadata only. Do not paste the token or
the raw API response into logs, tickets, or chat.

## Acceptance matrix

With a disposable staging phone:

1. A verified Casdoor phone user completes the OIDC callback and receives a
   new ZITADEL subject.
2. The subject has no ListingKit project or tenant role and is denied the
   workbench.
3. Assign one existing allowed role manually, then confirm only its intended
   tenant is reachable.
4. A second login resolves the same external subject.
5. An existing email user with matching display data is not linked.
6. Wrong issuer, wrong audience, expired token, and missing
   `phone_verified=true` fail before user creation.

Record only pass/fail, event type, and timestamps. Never record the phone
number, SMS code, access token, refresh token, or client secret.
