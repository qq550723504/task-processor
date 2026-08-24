# ListingKit ZITADEL SMS OTP Events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver ZITADEL SMS OTP messages through the existing signed ListingKit Tencent SMS webhook without weakening its trust boundary.

**Architecture:** Extend only `approvedEventType`. Signature verification, timestamp tolerance, E.164 validation, body cap and redacted failures remain unchanged; tests prove the two new events send and near matches fail closed.

**Tech Stack:** Go, `testify/require`, ZITADEL HTTP SMS provider, Tencent Cloud SMS.

**Spec:** `docs/superpowers/specs/2026-08-25-listingkit-phone-identity-design.md`

## Global Constraints

- This slice does not enable a ZITADEL OTP SMS policy.
- Retain the two existing events and add only `user.human.mfa.otp.sms.code.added` and `session.otp.sms.challenged`.
- Never use an event wildcard or log a code/full phone number.
- Invalid signatures, invalid payloads and Tencent failures must continue to fail closed.

---

### Task 1: Add exact event contracts with TDD

**Files:**
- Modify: `internal/listingkit/zitadelsms/service.go:114-117`
- Modify: `internal/listingkit/zitadelsms/service_test.go:47-116`

**Interfaces:**
- Consumes: `Service.Deliver(context.Context, []byte, string)` and `validZitadelSMSPayload`.
- Produces: a four-value explicit event allowlist used by `parseZitadelSMSPayload`.

- [ ] **Step 1: Add a failing test for every approved event.**

```go
func TestDeliverMapsEveryApprovedEventToTencent(t *testing.T) {
  for _, eventType := range []string{
    "user.human.phone.code.added",
    "user.human.initialization.code.added",
    "user.human.mfa.otp.sms.code.added",
    "session.otp.sms.challenged",
  } {
    t.Run(eventType, func(t *testing.T) {
      sender := &senderStub{}; service := newTestSMSService(t, sender)
      body := validZitadelSMSPayload(t, "+8613800138000", "123456", eventType)
      require.NoError(t, service.Deliver(context.Background(), body, signedHeader(t, body, time.Now())))
      require.Len(t, sender.messages, 1)
    })
  }
}
```

- [ ] **Step 2: Add near-match rejection cases and run them.**

```go
for _, eventType := range []string{
  "user.human.mfa.otp.sms.code.sent",
  "session.otp.sms.checked",
  "user.human.mfa.otp.sms.code.added.extra",
} {
  // A signed payload must return ErrInvalidPayload and leave sender.messages empty.
}
```

Run: `go test ./internal/listingkit/zitadelsms -run 'TestDeliverMapsEveryApprovedEventToTencent|TestDeliverRejectsNearMatchOTPSMSEvents' -count=1`

Expected: FAIL because the two exact OTP events are not yet allowed.

- [ ] **Step 3: Implement a named fixed allowlist.**

```go
var approvedZitadelSMSEvents = map[string]struct{}{
  "user.human.phone.code.added": {},
  "user.human.initialization.code.added": {},
  "user.human.mfa.otp.sms.code.added": {},
  "session.otp.sms.challenged": {},
}

func approvedEventType(eventType string) bool {
  _, ok := approvedZitadelSMSEvents[eventType]
  return ok
}
```

- [ ] **Step 4: Run focused plus deployment-boundary regression checks.**

Run: `go test ./internal/listingkit/zitadelsms -count=1; go test ./tests -run 'TestListingKit.*(Secret|Deploy)' -count=1`

Expected: PASS; no manifest, route or Secret boundary changes.

- [ ] **Step 5: Commit only service code and tests.**

```powershell
git add internal/listingkit/zitadelsms/service.go internal/listingkit/zitadelsms/service_test.go
git commit -m "feat: allow ZITADEL SMS OTP webhook events"
```

### Task 2: Add a version-pinned staging acceptance procedure

**Files:**
- Create: `docs/operations/listingkit-zitadel-sms-otp-acceptance.md`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`

**Interfaces:**
- Consumes: ZITADEL v4.17.1 staging, active HTTP SMS provider, and a non-production test phone.
- Produces: redacted evidence for factor-enrollment and login-challenge delivery; it does not change production policy.

- [ ] **Step 1: Document expected events and negative checks.**

```markdown
For factor enrollment expect `user.human.mfa.otp.sms.code.added`; for a session challenge expect `session.otp.sms.challenged`. Each accepted signed webhook returns 204. A signed `user.human.mfa.otp.sms.code.sent` returns 400 without a Tencent call; an invalid signature returns 401. Do not replay production payloads.
```

- [ ] **Step 2: Document the device acceptance assertions.**

```markdown
Use a staging phone to verify an existing user first enrolls a verified phone, then adds SMS OTP, then completes one challenge. Logs and the evidence record may contain event type and HTTP result only, never the code or full number.
```

- [ ] **Step 3: Verify docs and code.**

Run: `rg -n 'code.added|session.otp.sms.challenged|code.sent|204|401' docs/operations/listingkit-zitadel-sms-otp-acceptance.md; go test ./internal/listingkit/zitadelsms -count=1`

Expected: PASS.

- [ ] **Step 4: Commit the acceptance procedure.**

```powershell
git add docs/operations/listingkit-zitadel-sms-otp-acceptance.md deployments/kubernetes/listingkit-workbench/README.md
git commit -m "docs: add ZITADEL SMS OTP acceptance procedure"
```
