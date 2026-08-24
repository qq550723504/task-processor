# ListingKit ZITADEL SMS OTP Acceptance Procedure

Staging acceptance for delivering ZITADEL SMS OTP messages through the signed
ListingKit Tencent SMS webhook. Requires ZITADEL v4.17.1 staging with an active
HTTP SMS provider pointing at the ListingKit webhook, and a non-production test
phone. This procedure does not enable any production policy.

## Expected events and negative checks

For factor enrollment expect `user.human.mfa.otp.sms.code.added`; for a session
challenge expect `session.otp.sms.challenged`. Each accepted signed webhook
returns 204. A signed `user.human.mfa.otp.sms.code.sent` returns 400 without a
Tencent call; an invalid signature returns 401. Do not replay production
payloads.

## Device acceptance assertions

Use a staging phone to verify an existing user first enrolls a verified phone,
then adds SMS OTP, then completes one challenge. Logs and the evidence record
may contain event type and HTTP result only, never the code or full number.
