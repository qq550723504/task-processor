# ListingKit Tencent SMS onboarding design

## Goal

Allow a platform administrator to invite a ListingKit member who has no email
address. The invited person verifies a phone number and initializes their first
ZITADEL login method from a ZITADEL-managed SMS message, while receiving only
the selected ListingKit role in the selected tenant.

## Chosen architecture

Keep identity lifecycle inside ZITADEL. ListingKit will create a human user
with an E.164 phone number and request ZITADEL to send the user initialization
message. ZITADEL's active HTTP SMS Provider will call a dedicated webhook in
the existing `product-listing-api`; that webhook verifies the ZITADEL request
signature and delivers the supplied message through Tencent Cloud SMS using
Tencent's official Go SDK.

The webhook is not a normal end-user route: it must never accept bearer-less
traffic merely because ListingKit authentication is disabled. Its sole
authentication mechanism is a required, fresh `ZITADEL-Signature` HMAC over
the untouched request body. It returns a non-2xx response for invalid
signatures or failed SMS delivery so ZITADEL can report delivery failure.

## User-facing contract

`POST /api/v1/listing-kits/platform/tenants/{tenant_id}/members/invitations`
accepts exactly one contact channel:

* Existing email invitation: `email`, no `phone` or `username`.
* SMS invitation: `phone` in E.164 format and a stable `username`, no email.

Both channels use the existing tenant-directory check and allow only
`listingkit_viewer`, `listingkit_operator`, or `listingkit_admin`. Successful
responses and audit records expose the selected delivery channel and a masked
contact value; they never expose an initialization code, URL, HMAC value, or
Tencent request identifier.

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
3. Platform administrators can submit an E.164 phone invitation; email-only
   and phone-only validation is deterministic and retains the existing role and
   tenant checks.
4. The ZITADEL provider creates the phone identity, requests its initialization
   notification, then assigns exactly one ListingKit role. Partial failures
   retain the created user ID in the durable audit record.
5. Production manifests keep all SMS credentials API-only, render successfully,
   and document a manual provider activation plus a real-device delivery test.
