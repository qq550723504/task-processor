# ListingKit ZITADEL-native phone onboarding design

## Status

- Status: Accepted
- Date: 2026-08-25
- Scope: ListingKit passwordless phone registration, sign-in, invitation admission, tenant bootstrap, and Login V2 customization
- Implementation status: Not started
- Supersedes: `docs/superpowers/plans/2026-08-25-casdoor-phone-idp.md`
- Accepted amendment: Users provide no email or password; ZITADEL v4.17.1
  receives a generated verified technical email because its Human User schema
  requires the email field.

## Goal

Allow a person without an email address to register or accept a ListingKit
invitation using an E.164 phone number and one ZITADEL-managed SMS code. The
same SMS challenge proves phone possession and creates the first authenticated
ZITADEL session. Later sign-ins also use one SMS code and no password.

A direct self-registration creates one new ListingKit tenant. That tenant is a
ZITADEL Organization, its creator receives `ORG_OWNER` and
`listingkit_admin`, and it starts a 14-day `professional` subscription in
`trialing` status. A person registering from an invitation joins the invited
tenant and does not receive a personal tenant or a new trial.

## Decision summary

1. Extend the official open-source ZITADEL Login V2 instead of introducing
   Casdoor or implementing a second identity provider.
2. Keep ZITADEL as the identity, session, organization, project-grant, and role
   authority. ListingKit does not store or validate SMS codes.
3. Add a narrow phone-onboarding module to the existing
   `product-listing-api`; do not add a new service or database.
4. Reuse the existing ZITADEL HTTP SMS Provider, signed ListingKit webhook,
   and official Tencent Cloud SMS SDK integration.
5. Use the existing PostgreSQL database for durable, idempotent onboarding
   state and anti-abuse identity bindings.
6. Keep the current ListingKit tenant model for phase one: one phone identity
   belongs to exactly one ZITADEL Organization/ListingKit tenant.
7. Treat the one-code registration flow as a staging feasibility gate. It is a
   composition of supported ZITADEL APIs, not a documented turnkey ZITADEL
   registration screen. No rollout may proceed until the exact flow is proven
   against ZITADEL v4.17.1.

## Why this architecture

The official Login V2 already owns the ZITADEL authentication-request and
session lifecycle and supports SMS OTP as an authentication factor. Extending
it preserves that behavior and its upstream security fixes. Casdoor would add
another account store, database, OIDC federation boundary, SMS configuration,
and account-linking risk without changing ListingKit's final authority.

The existing ListingKit API already contains the trusted boundaries needed for
the remaining work:

- ZITADEL access tokens are introspected by the Go API.
- `urn:zitadel:iam:user:resourceowner:id` becomes the authoritative ListingKit
  tenant ID.
- ListingKit project roles are read from ZITADEL token claims.
- `listingsubscription.ApplyPlan` applies the existing `professional` plan and
  supports `trialing` plus an expiry timestamp.
- `internal/listingkit/zitadelsms` verifies signed ZITADEL webhook requests and
  delivers through Tencent Cloud SMS.

This design adds orchestration around those authorities instead of replacing
them.

## Product contract

### Direct registration

1. The user enters an E.164 phone number in the customized Login V2.
2. ZITADEL sends one SMS code.
3. The user enters that code once.
4. Successful verification completes tenant provisioning and signs the user
   in.
5. The technical organization name is generated as `lk-<opaque-id>` and never
   contains phone digits.
6. The backend generates `u-<opaque-id>@phone.invalid` as an internal verified
   technical email. It is not supplied by the user, displayed, mailed, or used
   for recovery, and it contains no phone digits.
7. No password credential is created.
8. The default UI label is `我的工作空间`; it may be renamed later.
9. The creator receives `ORG_OWNER` and `listingkit_admin` only after the SMS
   proof succeeds.
10. The new tenant receives `professional`, status `trialing`, starting at
   successful finalization and expiring exactly 14 days later in UTC.

### Invitation registration

1. A tenant administrator creates a phone invitation with one of the existing
   ListingKit roles: `listingkit_viewer`, `listingkit_operator`, or
   `listingkit_admin`.
2. The invitation produces an opaque, single-use onboarding URL that the
   administrator can share out of band. Sending a separate invitation SMS is
   not required for phase one.
3. The invitee proves the invited phone number with one ZITADEL SMS code.
4. The user is created in the invited ZITADEL Organization and receives only
   the invited ListingKit role.
5. The user does not receive `ORG_OWNER`, a personal tenant, or a new trial.

### Later sign-in

Known phone identities use the same Login V2 screen and one ZITADEL SMS OTP
challenge. Login does not rerun tenant provisioning or extend a trial.

### Phase-one tenant limitation

One phone identity belongs to one tenant. Direct registration creates its
tenant; invitation registration binds it to the invitation tenant. An identity
already bound to one tenant cannot join or switch to another tenant in phase
one.

Multi-tenant membership requires a separate design because the current Go
middleware fixes the tenant to the token's ZITADEL resource owner. Adding a
workspace selector without changing that authority would create ambiguous role
and tenant claims, so it is explicitly out of scope.

## Component boundaries

```text
Browser
  -> customized official ZITADEL Login V2
       -> phone-onboarding endpoints in product-listing-api
            -> ZITADEL User, Organization, Project and Session APIs
            -> existing ListingKit subscription service
       -> official ZITADEL authentication-request completion

ZITADEL HTTP SMS Provider
  -> existing signed ListingKit SMS webhook
       -> Tencent Cloud SMS official Go SDK
```

### Customized Login V2

Owns:

- phone entry, code entry, resend feedback, and generic user-facing errors;
- the existing ZITADEL authentication-request and OIDC redirect lifecycle;
- an opaque HttpOnly onboarding-flow cookie;
- choosing direct registration versus an invitation URL.

Does not own:

- Tencent Cloud credentials or SMS delivery;
- ZITADEL organization, project-grant, or role-management credentials;
- trial creation or entitlement policy;
- phone/OTP persistence or validation;
- authoritative ListingKit tenant or role decisions.

The source must be a thin fork of the official Login V2 pinned to ZITADEL
v4.17.1, with a documented upstream commit/tag and a small ListingKit-specific
patch series. Do not copy Login V2 into this repository or create a new login
framework. This repository owns its backend contract, deployment configuration,
and acceptance runbook.

### Phone-onboarding module

The module lives inside `product-listing-api` and owns:

- phone normalization, HMAC fingerprinting, rate-limit decisions, and flow
  idempotency;
- invitation validation and one-tenant binding;
- calls to ZITADEL management and session APIs;
- durable onboarding state, retry, reconciliation, and exact pending-resource
  cleanup;
- subscription application through `listingsubscription.ApplyPlan`;
- the final ordering of project grant, subscription, organization membership,
  and ListingKit user grant.

It uses a dedicated, minimal-permission ZITADEL service identity. The required
ZITADEL roles must be discovered and recorded during the staging spike. It must
not receive instance-owner privileges. If v4.17.1 cannot support the flow
without materially broader privileges, implementation stops for design review.

### Existing SMS relay

The SMS relay remains the only component with Tencent Cloud credentials. Its
signature, freshness, E.164, body-size, exact-event, and redacted-error checks
remain unchanged. The reusable OTP event slice is defined in
`docs/superpowers/plans/2026-08-25-listingkit-zitadel-sms-otp-events.md`.

No event wildcard is allowed. The expected additional events are:

- `user.human.mfa.otp.sms.code.added`
- `session.otp.sms.challenged`

## One-code registration protocol

The exact API composition must be proven before broader implementation:

1. Login V2 submits a normalized phone and optional invitation token to the
   onboarding start endpoint.
2. The backend obtains a per-phone lock and creates or resumes one durable
   attempt.
3. For self-registration, it creates an inert ZITADEL Organization with a
   generated technical name and an inert human user in that organization. For
   invitation registration, it creates the inert user in the invitation's
   existing organization.
   The Human User has a generated verified `@phone.invalid` technical email, no
   password, and a generated username; none contains the phone number.
4. The trusted backend provisions the phone in the minimum ZITADEL state needed
   to add SMS OTP and create a session challenge. This provisional state is not
   proof of possession and grants no ListingKit or organization privilege.
5. The backend creates a ZITADEL session and SMS OTP challenge. ZITADEL invokes
   the existing HTTP SMS Provider, which sends the only code through Tencent.
6. Login V2 submits the code to the backend. The backend passes it to the
   ZITADEL Session API; it never compares or stores the code itself.
7. Only a successful ZITADEL session check advances the attempt to
   `otp_verified`.
8. The backend idempotently finalizes tenant access and returns a one-time
   completion result to Login V2.
9. Login V2 uses the official ZITADEL authentication-request completion path to
   produce the OIDC response. It does not construct its own redirect or token.

The session ID and token needed between challenge and verification are kept
server-side. The token is encrypted at rest with a dedicated AEAD key from a
Kubernetes Secret, is never returned to browser JavaScript, and is erased when
the attempt completes or expires. The browser receives only an opaque,
HttpOnly, Secure, SameSite flow cookie.

If the v4.17.1 staging spike shows that a provisionally usable phone, OTP SMS
factor, and Session API challenge cannot safely yield one authenticated session
from one code, stop. Do not add a ListingKit OTP table, trust a client-supplied
`phone_verified` flag, or silently introduce a second verification code.

## Authorization and finalization ordering

Before `otp_verified`, the new user must have none of these, and a newly
created self-registration organization must not yet have its ListingKit project
grant:

- ListingKit user role;
- `ORG_OWNER` membership;
- ListingKit subscription or entitlements;
- a completed OIDC login response.

An invitation target organization can already have its normal project grant and
subscription; verification must not change either one.

After `otp_verified`, self-registration advances monotonically:

1. Ensure the tenant's grant to the central ListingKit project.
2. Apply `professional` with `StatusTrialing`, `StartsAt=verified_at`, and
   `ExpiresAt=verified_at+14 days` through `listingsubscription.ApplyPlan`.
3. Add the creator as `ORG_OWNER`.
4. Add the creator's `listingkit_admin` user grant last.
5. Mark the attempt `completed` and allow authentication-request completion.

Invitation registration skips steps 1 and 2 unless the target tenant is missing
an invariant that an administrator must repair. It adds only the invited role
and consumes the invitation.

`ApplyPlan` updates the subscription and entitlements through multiple
repository operations, so the onboarding flow must not pretend that this is a
cross-system transaction. It records each completed effect, re-reads the
result, and reconciles partial application before granting `listingkit_admin`.
Retries must use ensure/upsert semantics and never extend the original trial
window.

## Durable state

Use the existing PostgreSQL database with migrations owned by the ListingKit
schema path. Two logical records are required.

### Transient onboarding attempt

Minimum fields:

- opaque attempt ID and flow kind (`register`, `invite`, or `login`);
- phone HMAC fingerprint and key version;
- invitation ID, target organization ID, and ZITADEL user ID where applicable;
- encrypted ZITADEL session material and its expiry;
- monotonic state and completed-effect markers;
- `verified_at`, `created_at`, `updated_at`, `expires_at`;
- bounded retry count and redacted last-error code.

Suggested states are:

```text
pending
  -> resources_ready
  -> otp_challenged
  -> otp_verified
  -> project_ready
  -> subscription_ready
  -> roles_ready
  -> completed
```

`expired` and `failed` are terminal only when the flow cannot be safely
resumed. A database uniqueness constraint plus row locking prevents two active
registration attempts for the same fingerprint.

### Durable phone identity binding

Minimum fields:

- phone HMAC fingerprint and key version;
- ZITADEL organization ID and user ID;
- origin (`self_registration` or `invitation`);
- nullable `trial_claimed_at` and immutable original trial end;
- created/updated timestamps.

This record enforces one tenant per phone and one trial per phone even after a
transient attempt is cleaned up. It contains no plaintext phone number. HMAC
key rotation must support reading the previous version while rewriting to the
current version.

## Public HTTP contract

The exact route names may follow existing API conventions, but the behavior is:

- `start`: accepts phone plus optional invitation token, creates/resumes the
  flow, and sends or reuses one current challenge;
- `verify`: accepts the SMS code and verifies it through ZITADEL;
- `resend`: applies cooldown/rate limits and requests a new ZITADEL challenge;
- `status`: returns only the next UI action and generic terminal state.

These routes are intentionally unauthenticated because the caller has no
identity yet, but they are not affected by any general authentication toggle.
They use a dedicated route group with strict body limits, CSRF/origin checks,
rate limits, opaque flow cookies, and no caller-supplied tenant/user authority.

Known and unknown phone numbers receive equivalent public shapes and timing
classes. Internally, a known binding becomes a login flow and an unknown phone
becomes registration/invitation. Public responses never reveal whether the
phone, user, tenant, or trial already exists.

## Security and abuse controls

- Normalize to E.164 before any lookup; reject unsupported destinations with a
  generic response.
- ZITADEL's required email field uses `u-<opaque-id>@phone.invalid`, marked
  verified without sending mail. It is never a login choice in the customized
  UI and cannot be used for recovery.
- Omit both `password` and `hashedPassword` when creating the Human User. The
  user has no password authentication path.
- ListingKit PostgreSQL stores only a keyed HMAC fingerprint. ZITADEL stores the
  actual phone because it is the identity authority.
- Request bodies, phone numbers, SMS codes, session material, invitation
  tokens, and Tencent/ZITADEL credentials are never logged.
- Invitation tokens are random, one-time, expiry-bound, and stored only as a
  hash.
- Enforce at least a 60-second resend cooldown and five-minute code expiry.
- Lock an attempt temporarily after five incorrect codes.
- Enforce per-phone-fingerprint and per-IP hourly/daily caps at both API and
  ingress layers. Exact production thresholds are configuration, not code
  constants.
- A successful verification consumes the current challenge; replay cannot
  repeat role or trial effects.
- The service identity is isolated from Login V2 and from the Tencent Secret.
- `listingkit_admin` is the final externally useful effect.
- Security audit events contain attempt ID, event type, resource IDs, result,
  and redacted error code only.

## Failure recovery and cleanup

Each external effect is an idempotent saga step. A failed request returns a
generic retryable error while preserving enough state for a reconciler inside
`product-listing-api` to continue. Reconciliation must confirm actual ZITADEL
and subscription state instead of trusting a stale local marker.

Unverified attempts expire after 24 hours. Cleanup may remove only resources
that are still provably owned by that exact attempt and have no organization
membership, project/user grant, subscription, or successful session. Ambiguous
resources are quarantined for operator review, not deleted.

Completed tenants and durable phone bindings are never removed by pending-flow
cleanup. Rolling back Login V2 or the onboarding module does not roll back
ZITADEL organizations or subscription data.

## Compatibility

- Existing email/password/passkey users keep the current ZITADEL Login V2
  paths and are not automatically converted or linked by matching profile data.
- Existing email and phone-assisted invitation behavior remains available until
  the new phone invitation contract has separate compatibility tests.
- The Go API remains the only authoritative ListingKit API authorization layer.
- The token's ZITADEL resource owner remains the authoritative tenant ID.
- The feature does not weaken mandatory authentication or introduce an
  authentication-off switch.

## Delivery and verification strategy

### Phase 0: feasibility spike

Against a disposable ZITADEL v4.17.1 environment and mock SMS provider, prove:

1. an inert human user can be created without user-supplied email or password,
   using only the generated verified technical email required by v4.17.1;
2. one ZITADEL SMS challenge can verify possession and produce the first usable
   session;
3. the service identity has a documented minimal role set;
4. no role, project authorization, subscription, or OIDC completion exists
   before the successful code check;
5. replay, expiry, wrong-code lockout, resend, and cleanup behave as designed.

Failure of any item returns the design to review. It does not authorize a
second OTP system.

### Automated tests

- State-machine and transition-table unit tests.
- Idempotency tests for every external effect and duplicate callback.
- Concurrent start/verify tests for one phone fingerprint.
- Failure injection after every ZITADEL and subscription call.
- Rate-limit, enumeration-resistance, CSRF, cookie, encryption, redaction, and
  invitation-token negative tests.
- Real ZITADEL v4.17.1 integration tests with a mock HTTP SMS receiver.
- Regression tests for email login, Auth.js/OIDC, admin console access, current
  invitations, Go API introspection, and role authorization.

### Staging acceptance

Use a non-production Tencent SMS number and record redacted evidence that:

- direct registration receives one code and lands in the new tenant;
- the creator has exactly `ORG_OWNER` plus `listingkit_admin`;
- the tenant has `professional/trialing` with the original 14-day window;
- a repeat login receives one code and does not reprovision;
- an invitation creates no personal tenant/trial and grants only the invited
  role;
- a phone already bound to another tenant cannot join;
- no pre-verification attempt can call a protected ListingKit API.

### Deployment and rollback

Build a separate customized Login V2 image from the pinned upstream fork. First
point only a staging/test OIDC application at it; leave the current login entry
unchanged. After acceptance, switch the ListingKit login entry explicitly.

Rollback restores the official Login V2 v4.17.1 image/address. Backend endpoints
may remain disabled behind their dedicated feature gate. Do not use database
rollback to delete completed identity or subscription records.

No production activation, credential mutation, or cleanup of the old Casdoor
worktree is authorized by this design document.

## Out of scope

- Casdoor or any other federated phone identity provider.
- Password registration, password reset, or ListingKit-managed OTP validation.
- Multiple tenants per phone, workspace switching, or cross-organization role
  aggregation.
- Automatic account linking between existing email users and phone users.
- Email collection for phone-only users.
- Global activation for every ZITADEL application.
- Deleting historical Casdoor work or live identity data.

## Source references

- ZITADEL hosted/self-hosted Login V2: <https://zitadel.com/docs/guides/integrate/login/hosted-login>
- ZITADEL Login App: <https://zitadel.com/docs/guides/integrate/login-ui/login-app>
- Create user: <https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.CreateUser>
- ZITADEL v4.17.1 UserService schema: <https://github.com/zitadel/zitadel/blob/v4.17.1/proto/zitadel/user/v2/user_service.proto>
- Add organization: <https://zitadel.com/docs/reference/api/org/zitadel.org.v2.OrganizationService.AddOrganization>
- Add SMS OTP: <https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.AddOTPSMS>
- Session SMS flow: <https://zitadel.com/docs/guides/integrate/login-ui/mfa>
- Create session: <https://zitadel.com/docs/reference/api/session/zitadel.session.v2.SessionService.CreateSession>
- Set session: <https://zitadel.com/docs/reference/api/session/zitadel.session.v2.SessionService.SetSession>
- Project grant: <https://zitadel.com/docs/reference/api/project/zitadel.project.v2.ProjectService.CreateProjectGrant>
- SaaS organization/project model: <https://zitadel.com/docs/guides/solution-scenarios/saas>
- HTTP SMS providers: <https://zitadel.com/docs/guides/manage/customize/notification-providers>

## Acceptance criteria

1. One SMS code proves possession and creates the initial ZITADEL session in
   the pinned staging version.
2. No pre-verification user can obtain organization, project, role,
   subscription, or protected ListingKit access.
3. Direct registration creates exactly one tenant, one durable binding, one
   original 14-day professional trial, and the two approved creator roles.
4. Invitation registration joins exactly the target tenant, grants only the
   invited role, and creates no personal tenant or trial.
5. Every external effect is idempotent and recoverable after partial failure;
   retries never duplicate or extend access.
6. ListingKit stores no plaintext phone or OTP and exposes no phone-existence
   oracle.
7. Users provide no email or password; generated technical emails contain no
   phone data, receive no mail, are not exposed as user-facing login names, and
   provide no email recovery path.
8. Existing email login and invitation paths remain compatible.
9. The custom UI is a maintained thin fork of official Login V2 and can be
   rolled back independently of identity/subscription data.
