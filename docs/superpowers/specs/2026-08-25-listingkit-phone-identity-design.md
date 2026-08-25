# ListingKit Phone Identity Design

## Goal

Allow a person without an email mailbox to prove control of a phone number by
SMS, then sign in to ListingKit through ZITADEL.  This must not turn phone
verification into automatic ListingKit admission.

## Boundaries

Casdoor is the passwordless phone-code identity provider and is the only
component that talks to Tencent SMS.  ZITADEL remains the token issuer and
source of truth for organizations, projects, roles, and external-identity
links.  ListingKit consumes only the final ZITADEL subject and its existing
roles; it stores neither phone numbers nor verification codes.

## Staging topology

Casdoor 3.143.0 runs from its fixed manifest digest
`sha256:1284af680ddf10aa80569f1f4a46210dd9875ce70845e67047053363d0c0ba58`
in a new `casdoor` namespace and is exposed only as
`https://id.staging.shuomiai.com`.  It reuses the existing private
`platform-data/shared-postgresql` service, but has its own `casdoor` database
and least-privilege `casdoor_app` role.  No Casdoor PostgreSQL StatefulSet,
Pod, PVC, or public database endpoint is created.  Provisioning the database
and role is a one-off, redacted administrative operation; the application role
has no access to other databases or schemas.

Casdoor credentials, the application database password, Tencent SMS settings,
and the OIDC client secret are kept in a Casdoor-only Secret Manager entry and
injected into the `casdoor` namespace.  No workload reads, copies, or prints
the shared PostgreSQL administrator secret.

## Identity flow

1. The user enters a phone number and an SMS verification code at Casdoor.
2. Casdoor applies resend cooldowns, rate limits, expiry, and failed-attempt
   lockout, then issues an OIDC authorization-code flow protected by PKCE.
3. ZITADEL accepts only the fixed Casdoor issuer and a verified-phone claim.
   Its authentication action derives a technical `.invalid` email from the
   immutable Casdoor `sub`; it rejects missing or invalid claims and never
   receives the phone number.
4. ZITADEL creates or resolves only that external identity.  Automatic linking,
   automatic profile update, tenant membership, and ListingKit role grants are
   disabled.
5. ListingKit permits access only after an administrator assigns an existing
   allowed role through the normal authorization workflow.

## Existing users and safety

Existing email/password login remains unchanged.  SMS MFA remains optional:
the existing signed ListingKit SMS webhook can deliver only the two explicit
ZITADEL factor events and does not make SMS a global login requirement.

## Acceptance and rollout

The first delivery is staging manifests, static render tests, and read-only
OIDC/JWKS/provider-policy preflights.  After a separate deployment approval,
staging acceptance uses a non-production phone to verify registration, repeat
login, rate-limit and lockout behavior, PKCE, no automatic account linking,
and denial of ListingKit access before a manual role assignment.  Production
database placement, DNS/TLS, and activation remain separate approval gates.
