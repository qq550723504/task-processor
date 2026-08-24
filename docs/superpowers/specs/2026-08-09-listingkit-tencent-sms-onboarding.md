# ListingKit Tencent SMS onboarding design

## Goal

Allow a platform administrator to invite a ListingKit member with an email
address and, when desired, an E.164 phone number. ZITADEL sends its normal
email-based first-login initialization and verifies the phone number by SMS,
while the person receives only the selected ListingKit role in the selected
tenant. ZITADEL's supported Human-user APIs require email, so ListingKit does
not claim or emulate phone-only registration.

## Chosen architecture

Keep identity lifecycle inside ZITADEL. ListingKit will create a human user
with the required email and optional E.164 phone number. For a phone-assisted
invite it requests ZITADEL phone verification by SMS; ZITADEL retains its
normal email-based first-login initialization. ZITADEL's active HTTP SMS
Provider will call a dedicated webhook in the existing `product-listing-api`;
that webhook verifies the ZITADEL request signature and delivers the supplied
message through Tencent Cloud SMS using Tencent's official Go SDK.

The webhook is not a normal end-user route: it must never accept bearer-less
traffic merely because ListingKit authentication is disabled. Its sole
authentication mechanism is a required, fresh `ZITADEL-Signature` HMAC over
the untouched request body. It returns a non-2xx response for invalid
signatures or failed SMS delivery so ZITADEL can report delivery failure.

## User-facing contract

`POST /api/v1/listing-kits/platform/tenants/{tenant_id}/members/invitations`
accepts one of these validation modes:

* Existing email invitation: `email`, no `phone` or `username`.
* Phone-assisted invitation: required `email`, E.164 `phone`, and a stable
  `username`. ZITADEL sends the phone verification SMS; first-login
  initialization remains email based.

Both modes use the existing tenant-directory check and allow only
`listingkit_viewer`, `listingkit_operator`, or `listingkit_admin`. Successful
responses and audit records expose the selected delivery mode and masked
contact value(s); they never expose an initialization code, URL, HMAC value,
or Tencent request identifier.

## Security and operations

* Reuse ZITADEL's official signature implementation and its five-minute
  freshness tolerance; do not invent a second verification-code store.
* Store Tencent `secret_id`, `secret_key`, SMS app id, sender sign, template id,
  and the ZITADEL HTTP-provider signing key in a dedicated API-only Kubernetes
  Secret. Do not place them in `listingkit-workbench-secret`, UI, workers,
  imgproxy, migration jobs, logs, or tests.
* The webhook validates body size, signature, E.164 destination, configured
  template values, and Tencent's response before returning success. It logs
  only event type and a hashed recipient correlation value.
* Existing email invitations remain behaviorally unchanged.
* ZITADEL HTTP SMS Provider creation and activation happen only after the
  deployed webhook has a public HTTPS route and its signing key is stored in
  Kubernetes. Tencent Cloud credentials are supplied via the deployment secret,
  never via source control or chat.

## Acceptance criteria

1. Missing/invalid/stale ZITADEL signatures receive a non-success response and
   never invoke the Tencent client.
2. A valid ZITADEL SMS payload maps only the approved template parameters to
   Tencent SMS; raw code, URL, phone number, and credentials are absent from
   errors and logs.
3. Platform administrators can submit an email-only invitation or a
   phone-assisted invite containing email, E.164 phone, and username;
   validation is deterministic and retains the existing role and tenant checks.
4. The ZITADEL provider creates the human identity, requests phone
   verification when supplied, retains ZITADEL's email first-login flow, then
   assigns exactly one ListingKit role. Partial failures retain the created user
   ID in the durable audit record.
5. Production manifests keep all SMS credentials API-only, render successfully,
   and document a manual provider activation plus a real-device delivery test.
