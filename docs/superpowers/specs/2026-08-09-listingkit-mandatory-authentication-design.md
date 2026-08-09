# ListingKit mandatory authentication design

## Goal

Remove every ListingKit mechanism that suggests or enables disabling identity
authentication. Protected ListingKit routes must require a verified ZITADEL
identity in every environment.

## Scope

- Remove `listingkit.zitadel.authRequired` from the runtime configuration type,
  YAML examples, defaults, and environment-variable bindings.
- Remove the local API script's `ZitadelAuthMode Disabled` behavior and its
  request-header identity bypass.
- Always construct the ListingKit ZITADEL middleware. Missing issuer or client
  configuration remains a fail-closed `503 zitadel_auth_not_configured` for a
  protected route.
- Keep `authorizationRequired` only as the optional allowlist gate after a
  caller identity has been verified; it never disables identity verification.
- Keep application health and readiness endpoints public: `/health`, `/readyz`,
  and standalone crawler `/health` and `/ready`.

## Explicitly excluded

- Public self-registration, anonymous ListingKit APIs, and trusted client
  identity headers are not introduced.
- The API-only ZITADEL member-invitation token is unchanged.

## Verification

- Configuration tests prove the old field and environment variable are absent.
- Route tests prove a protected route without a bearer token is rejected and a
  missing ZITADEL configuration fails closed.
- Script tests prove the Disabled mode is unavailable.
- Probe tests prove the four public health/readiness endpoints remain
  unauthenticated.
