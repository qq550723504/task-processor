# ZITADEL Casdoor Phone Federation Runbook

Federate verified Casdoor phone identities into ZITADEL via Generic OIDC plus
one External Authentication action. ZITADEL remains the only ListingKit
issuer/role authority; Casdoor receives no ListingKit role, tenant membership,
or token.

## Staging configuration

Create Generic OIDC provider `手机号登录` with staging issuer
(`https://id.staging.shuomiai.com`), scopes `openid profile email`, PKCE on,
automatic creation on, automatic update off, account creation allowed, and
account linking off. Attach the action
`map-casdoor-phone-identity` (see
`deployments/kubernetes/casdoor/zitadel-actions/map-casdoor-phone-identity.js`)
only to External Authentication/Post Authentication; attach no Post Creation
grant action.

## Read-only policy preflight

```bash
./scripts/zitadel-casdoor-federation-preflight.ps1 -IssuerURL https://id.staging.shuomiai.com -ProviderID <provider-id>
```

Requires `ZITADEL_ADMIN_TOKEN` in the environment. Fails closed on: wrong
issuer, missing `openid` scope, linking allowed, automatic update allowed, or
external login disabled.

## Negative cases

Verify a new phone user gets a ZITADEL subject but no ListingKit role, and that
an old email user with matching display data is not linked. Wrong issuer/
audience, expired token, and missing `phone_verified` must fail before user
creation.
