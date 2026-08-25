# ListingKit ZITADEL Phone Onboarding Feasibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove against ZITADEL v4.17.1 that a user who supplies only a phone number can receive one ZITADEL SMS code, verify it once, and obtain a checked passwordless session without receiving ListingKit roles, a project grant, or a subscription.

**Architecture:** Reuse the already implemented exact-event extension for the signed ListingKit Tencent SMS relay, then add a standalone Go preflight runner that calls only the version-pinned ZITADEL Organization, User, OTP SMS, and Session APIs. The runner creates an isolated temporary organization and a Human User with a generated verified technical email, no password, and a verified phone; it requests one ZITADEL OTP SMS challenge, submits the entered code to ZITADEL, verifies the returned session factors, and deletes the temporary organization on every cleanup path. It is not wired into `product-listing-api` and cannot grant ListingKit access.

**Tech Stack:** Go 1.25, standard `net/http`, ZITADEL v4.17.1 REST APIs, existing signed ListingKit HTTP SMS Provider relay, Tencent Cloud SMS, `httptest`, Kubernetes read-only preflight commands.

**Spec:** `docs/superpowers/specs/2026-08-25-listingkit-zitadel-native-phone-onboarding-design.md`

## Global Constraints

- Pin every contract assertion to ZITADEL Core and Login V2 v4.17.1.
- The user supplies no email and no password.
- Create the internal email as `u-<opaque-id>@phone.invalid`, set `isVerified: true`, send no email, and include no phone digits.
- Omit both `password` and `hashedPassword` from `POST /v2/users/new`.
- Treat `phone.isVerified=true` as provisional bootstrap state, never as proof of possession or authorization.
- ZITADEL generates, sends, and validates the only SMS code; the runner never stores or compares it.
- Use the returned ZITADEL user ID, not a phone/login-name query, when creating the session.
- Before verification, create no project grant, user authorization, organization admin membership, subscription, or entitlement.
- Never print or log the phone, technical email, code, bearer token, session token, callback URL, Tencent response, or provider response body.
- Use a temporary organization named `lk-phone-preflight-<opaque-id>` and a non-production phone; delete it by exact ID during cleanup.
- Keep provisioning and login/session credentials separate: the provisioning
  service user has `IAM_ORG_MANAGER`; the Login Client has `IAM_LOGIN_CLIENT`.
  Neither token is sent to the browser or written to disk.
- No production feature gate, Login V2 route, API route, schema migration, or
  subscription call is part of this plan. Temporary organization cleanup is
  required and uses only the exact ID created by the current attempt.
- A failed real-device proof stops the program; it does not authorize a second OTP implementation.

## Program decomposition

This plan implements only the version-pinned feasibility gate. A successful
result authorizes writing three later plans:

1. durable backend onboarding state, invitation admission, trial, and role finalization;
2. the thin official Login V2 fork and browser/OIDC integration;
3. Kubernetes staging rollout, reconciliation, acceptance, and rollback.

The later plans must use the exact API behavior and permissions recorded here.

---

### Task 1: Reuse the exact ZITADEL OTP SMS event slice

**Files:**
- Reuse commit: `26e9bdfaa` from `feature/zitadel-phone-security`
- Modify through cherry-pick: `internal/listingkit/zitadelsms/service.go:114-117`
- Modify through cherry-pick: `internal/listingkit/zitadelsms/service_test.go:55-99`

**Interfaces:**
- Consumes: `Service.Deliver(context.Context, []byte, string)` and the existing signed webhook contract.
- Produces: `approvedZitadelSMSEvents` containing the two existing phone events plus `user.human.mfa.otp.sms.code.added` and `session.otp.sms.challenged` as exact matches.

- [ ] **Step 1: Confirm the reusable slice is not already present**

```powershell
git merge-base --is-ancestor 26e9bdfaa HEAD
rg -n 'user\.human\.mfa\.otp\.sms\.code\.added|session\.otp\.sms\.challenged' internal/listingkit/zitadelsms/service.go
```

Expected: the merge-base command is nonzero and the search returns no matches.
If a newer base already has both exact events, skip Step 2 and verify that code.

- [ ] **Step 2: Reuse the existing reviewed commit**

```powershell
git cherry-pick 26e9bdfaa
```

Expected: exactly `service.go` and `service_test.go` change. Do not retype or
broaden the allowlist.

- [ ] **Step 3: Verify exact acceptance and near-match rejection**

```powershell
go test ./internal/listingkit/zitadelsms -run 'TestDeliverMapsEveryApprovedEventToTencent|TestDeliverRejectsNearMatchOTPSMSEventsWithoutSending' -count=1
go test ./tests -run 'TestListingKit.*(Secret|Deploy)' -count=1
```

Expected: PASS; four exact events send, near matches fail closed, and Secret
boundaries are unchanged. The cherry-pick is the task commit.

---

### Task 2: Add a version-pinned, redacting ZITADEL preflight client

**Files:**
- Create: `internal/listingkit/phoneonboardingpreflight/zitadel_client.go`
- Test: `internal/listingkit/phoneonboardingpreflight/zitadel_client_test.go`

**Interfaces:**
- Consumes: issuer URL, a short-lived `IAM_ORG_MANAGER` provisioning token, a
  short-lived `IAM_LOGIN_CLIENT` session token, and an `*http.Client`.
- Produces:

```go
type Client interface {
	CreateOrganization(context.Context, string) (string, error)
	CreateTechnicalUser(context.Context, TechnicalUserInput) (string, error)
	AddOTPSMS(context.Context, string) error
	CreateSMSChallenge(context.Context, string, time.Duration) (SessionMaterial, error)
	VerifySMS(context.Context, string, string) (string, error)
	GetSession(context.Context, string, string) (SessionProof, error)
	DeleteSession(context.Context, string) error
}

type TechnicalUserInput struct {
	OrganizationID string
	Username       string
	TechnicalEmail string
	Phone          string
}

type SessionMaterial struct { ID, Token string }

type SessionProof struct {
	UserID           string
	OrganizationID   string
	UserVerifiedAt   time.Time
	OTPSMSVerifiedAt time.Time
}
```

- [ ] **Step 1: Write failing request-contract tests**

Use `httptest.Server` and assert these exact JSON shapes:

```go
wantCreateUser := map[string]any{
	"organizationId": "org-preflight",
	"username": "lkp-01JTEST",
	"human": map[string]any{
		"profile": map[string]any{
			"givenName": "Phone", "familyName": "Preflight", "displayName": "Phone Preflight",
		},
		"email": map[string]any{"email": "u-01JTEST@phone.invalid", "isVerified": true},
		"phone": map[string]any{"phone": "+8613712345678", "isVerified": true},
	},
}

wantCreateSession := map[string]any{
	"checks": map[string]any{"user": map[string]any{"userId": "user-preflight"}},
	"challenges": map[string]any{"otpSms": map[string]any{"returnCode": false}},
	"lifetime": "300s",
}

wantVerifySession := map[string]any{
	"checks": map[string]any{"otpSms": map[string]any{"code": "654321"}},
}
```

Also assert paths and forbidden fields:

```go
require.Equal(t, "/v2/organizations", requests[0].URL.Path)
require.Equal(t, "/v2/users/new", requests[1].URL.Path)
require.Equal(t, "/v2/users/user-preflight/otp_sms", requests[2].URL.Path)
require.Equal(t, "/v2/sessions", requests[3].URL.Path)
require.Equal(t, "/v2/sessions/session-preflight", requests[4].URL.Path)
require.NotContains(t, string(createUserBody), "password")
require.NotContains(t, string(createUserBody), "hashedPassword")
require.NotContains(t, string(createUserBody), "13712345678@")
```

The organization request body is exactly
`{"name":"lk-phone-preflight-01JTEST"}`. The OTP factor request has an empty
JSON body and uses `POST /v2/users/user-preflight/otp_sms`.

- [ ] **Step 2: Run the request tests and observe the red state**

```powershell
go test ./internal/listingkit/phoneonboardingpreflight -run 'TestClient' -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the typed API operations**

Implement `NewClient` with trimmed issuer/token validation, HTTPS enforcement
except loopback, a 10-second default timeout, bearer authorization, JSON
content negotiation, and a 1 MiB response limit. `CreateOrganization`,
`CreateTechnicalUser`, and `AddOTPSMS` use only the provisioning token;
session create/verify/read/delete use only the Login Client token. Tests assert
the token split for every path. The essential bodies are:

```go
func (c *zitadelClient) CreateTechnicalUser(ctx context.Context, in TechnicalUserInput) (string, error) {
	body := createUserRequest{
		OrganizationID: in.OrganizationID,
		Username: in.Username,
		Human: createHuman{
			Profile: humanProfile{GivenName: "Phone", FamilyName: "Preflight", DisplayName: "Phone Preflight"},
			Email: verifiedEmail{Email: in.TechnicalEmail, IsVerified: true},
			Phone: verifiedPhone{Phone: in.Phone, IsVerified: true},
		},
	}
	var response struct{ ID string `json:"id"` }
	if err := c.doJSON(ctx, http.MethodPost, "/v2/users/new", body, &response); err != nil { return "", err }
	if strings.TrimSpace(response.ID) == "" { return "", errors.New("ZITADEL user creation returned no id") }
	return response.ID, nil
}

func (c *zitadelClient) CreateSMSChallenge(ctx context.Context, userID string, lifetime time.Duration) (SessionMaterial, error) {
	body := createSessionRequest{
		Checks: checks{User: &checkUser{UserID: userID}},
		Challenges: requestChallenges{OTPSMS: &otpSMSChallenge{ReturnCode: false}},
		Lifetime: formatProtoDuration(lifetime),
	}
	var response struct { SessionID string `json:"sessionId"`; SessionToken string `json:"sessionToken"` }
	if err := c.doJSON(ctx, http.MethodPost, "/v2/sessions", body, &response); err != nil { return SessionMaterial{}, err }
	if response.SessionID == "" || response.SessionToken == "" { return SessionMaterial{}, errors.New("ZITADEL session creation returned incomplete material") }
	return SessionMaterial{ID: response.SessionID, Token: response.SessionToken}, nil
}
```

`VerifySMS` returns the replacement session token from PATCH. `GetSession`
requests `/v2/sessions/{id}?sessionToken=<escaped-token>` and requires both
`session.factors.user` and `session.factors.otpSms`. `DeleteSession` uses
`DELETE /v2/sessions/{id}` and accepts 200 or 204.

- [ ] **Step 4: Add fail-closed and redaction tests**

Implement these tests explicitly:

```go
func TestClientErrorsDoNotExposeProviderBodyTokenPhoneOrCode(t *testing.T)
func TestClientRejectsMissingSessionMaterial(t *testing.T)
func TestClientRejectsSessionWithoutUserAndOTPSMSFactors(t *testing.T)
func TestClientRejectsNonHTTPSIssuerOutsideLoopback(t *testing.T)
func TestClientLimitsProviderResponseBody(t *testing.T)
```

Provider errors may report only a stable operation and HTTP status, never the
body or an unwrapped transport error.

- [ ] **Step 5: Verify and commit the client slice**

```powershell
go test ./internal/listingkit/phoneonboardingpreflight -count=1
go test ./internal/listingkit/memberinvite ./internal/listingkit/zitadelsms -count=1
git add -- internal/listingkit/phoneonboardingpreflight/zitadel_client.go internal/listingkit/phoneonboardingpreflight/zitadel_client_test.go
git commit -m "feat: add ZITADEL phone onboarding preflight client"
```

Expected: all tests pass and the commit contains only the client and its tests.

---

### Task 3: Add the isolated interactive preflight runner

**Files:**
- Create: `internal/listingkit/phoneonboardingpreflight/runner.go`
- Test: `internal/listingkit/phoneonboardingpreflight/runner_test.go`
- Create: `hack/debug/listingkit-phone-onboarding-preflight/main.go`
- Test: `hack/debug/listingkit-phone-onboarding-preflight/main_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: Task 2 `Client`, cryptographic randomness, current time, temporary
  phone/code line input, and a redacted output writer.
- Produces:

```go
type Attempt struct {
	OrganizationID string
	UserID         string
	SessionID      string
	sessionToken   string
}

func NewRunner(client Client, random io.Reader, now func() time.Time, output io.Writer) (*Runner, error)
func (r *Runner) Start(context.Context, string) (*Attempt, error)
func (r *Runner) Verify(context.Context, *Attempt, string) (SessionProof, error)
```

- [ ] **Step 1: Write failing orchestration tests**

Use a fake `Client` and assert the exact start order:

```go
require.Equal(t, []string{"CreateOrganization", "CreateTechnicalUser", "AddOTPSMS", "CreateSMSChallenge"}, fake.calls)
require.Regexp(t, `^lk-phone-preflight-[0-9A-HJKMNP-TV-Z]{26}$`, fake.organizationName)
require.Regexp(t, `^lkp-[0-9A-HJKMNP-TV-Z]{26}$`, fake.user.Username)
require.Regexp(t, `^u-[0-9A-HJKMNP-TV-Z]{26}@phone\.invalid$`, fake.user.TechnicalEmail)
require.NotContains(t, fake.user.Username+fake.user.TechnicalEmail, "13712345678")
```

`Verify` must call `VerifySMS`, replace the in-memory token, call `GetSession`,
require matching user/organization IDs and both factor timestamps, and then
call `DeleteSession` and `DeleteOrganization`. It also deletes both resources
after a factor mismatch.

- [ ] **Step 2: Run the runner tests and observe the red state**

```powershell
go test ./internal/listingkit/phoneonboardingpreflight -run 'TestRunner' -count=1
```

Expected: FAIL because `Runner` does not exist.

- [ ] **Step 3: Implement minimal orchestration and redacted output**

Generate one 128-bit ULID-compatible identifier per attempt. Normalize E.164
with the same strict rules as the SMS relay. Keep phone, technical email, code,
and session token in memory only.

Success output is limited to:

```text
status=challenge_sent attempt=<opaque-id> organization_id=<id> user_id=<id> session_id=<id>
status=otp_verified attempt=<opaque-id> user_factor=true otp_sms_factor=true
```

Failure output is limited to:

```text
status=failed attempt=<opaque-id> step=<stable-step-name>
```

Allowed step names are `organization_create`, `user_create`, `otp_sms_add`,
`challenge_create`, `code_verify`, `session_read`, `session_delete`, and
`organization_delete`.

- [ ] **Step 4: Write and implement CLI boundary tests**

The command loads only `ZITADEL_ISSUER_URL`,
`ZITADEL_PREFLIGHT_PROVISION_TOKEN`, and `ZITADEL_PREFLIGHT_LOGIN_TOKEN`; it
must not load full application configuration or
require database, Tencent, AI-provider, RabbitMQ, COS, or subscription values.
Capture stdout/stderr and prove they contain none of the token, phone, code, or
technical email.

This is a temporary non-production probe. It reads the phone and OTP as
ordinary line input so it works reliably in PowerShell; the values are held
only in memory and are not written to logs or files. Do not run it against a
production issuer, and do not paste the OTP into chat or issue trackers.

```go
func readSecret(prompt string, input *os.File, output io.Writer) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil { return "", err }
	var value string
	if _, err := fmt.Fscanln(input, &value); err != nil { return "", errors.New("secure input failed") }
	_, _ = fmt.Fprintln(output)
	return strings.TrimSpace(value), nil
}
```

Use a five-minute overall context timeout and a 300-second session lifetime.
Exit nonzero on the first failed step and never retry a code automatically.

- [ ] **Step 5: Verify and commit the runner slice**

```powershell
go -C hack/debug test ./listingkit-phone-onboarding-preflight -count=1
go test ./internal/listingkit/phoneonboardingpreflight -count=1
go test ./internal/listingkit/zitadelsms ./internal/listingkit/memberinvite -count=1
git add -- hack/debug/listingkit-phone-onboarding-preflight internal/listingkit/phoneonboardingpreflight/runner.go internal/listingkit/phoneonboardingpreflight/runner_test.go go.mod go.sum
git commit -m "feat: add interactive phone onboarding preflight"
```

Expected: PASS and no application runtime or deployment file changes.

---

### Task 4: Execute the v4.17.1 real-device acceptance gate

**Files:**
- Create: `docs/operations/listingkit-zitadel-phone-onboarding-feasibility.md`
- Create after execution: `docs/superpowers/verification/2026-08-25-listingkit-zitadel-phone-onboarding-feasibility.md`

**Interfaces:**
- Consumes: Core/Login V2 v4.17.1, active HTTP SMS Provider, deployed Task 1 relay, short-lived preflight token, and one non-production Tencent SMS phone.
- Produces: redacted pass/fail evidence containing only versions, opaque IDs, result classes, factor booleans, timestamps, and permission findings.

- [ ] **Step 1: Write the runbook and read-only gates**

The runbook runs these before any mutation:

```powershell
kubectl -n zitadel get deployment zitadel -o jsonpath='{.spec.template.spec.containers[0].image}'
kubectl -n zitadel get deployment zitadel-login -o jsonpath='{.spec.template.spec.containers[0].image}'
kubectl -n listingkit get deployment product-listing-api -o jsonpath='{.spec.template.spec.containers[0].image}'
```

Expected: Core and Login V2 report v4.17.1. Record the API digest without
reading a Secret. Read-only inspect that the active SMS Provider points to the
expected HTTPS webhook; never print signing/Tencent credentials.

- [ ] **Step 2: Prove the binary cannot grant access**

```powershell
rg -n 'CreateProjectGrant|CreateAuthorization|ApplyPlan|AuthorizationService/CreateAuthorization|/v2/projects|/subscriptions|/entitlements' internal/listingkit/phoneonboardingpreflight hack/debug/listingkit-phone-onboarding-preflight
```

Expected: no matches.

- [ ] **Step 3: Execute one real SMS proof**

Set both role-scoped tokens interactively in the current shell without writing
them to a file:

```powershell
$env:ZITADEL_ISSUER_URL = 'https://auth.shuomiai.com'
$provisionToken = Read-Host 'IAM_ORG_MANAGER preflight token' -AsSecureString
$loginToken = Read-Host 'IAM_LOGIN_CLIENT preflight token' -AsSecureString
$provisionTokenPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($provisionToken)
$loginTokenPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($loginToken)
try {
    $env:ZITADEL_PREFLIGHT_PROVISION_TOKEN = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($provisionTokenPointer)
    $env:ZITADEL_PREFLIGHT_LOGIN_TOKEN = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($loginTokenPointer)
    go -C hack/debug run ./listingkit-phone-onboarding-preflight --non-production
} finally {
    Remove-Item Env:ZITADEL_PREFLIGHT_PROVISION_TOKEN -ErrorAction SilentlyContinue
    Remove-Item Env:ZITADEL_PREFLIGHT_LOGIN_TOKEN -ErrorAction SilentlyContinue
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($provisionTokenPointer)
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($loginTokenPointer)
}
```

Before running, independently verify that `ZITADEL_ISSUER_URL` is the
non-production instance. The `--non-production` flag is an operator
confirmation, not a production-access control. Enter the phone and received
code at the line prompts. Expected output:

```text
status=challenge_sent attempt=<opaque> organization_id=<opaque> user_id=<opaque> session_id=<opaque>
status=otp_verified attempt=<same-opaque> user_factor=true otp_sms_factor=true
```

- [ ] **Step 4: Record the exact pass/fail evidence**

Use this complete schema with real redacted values:

```markdown
# ListingKit ZITADEL phone onboarding feasibility evidence

- ZITADEL Core: v4.17.1
- ZITADEL Login V2: v4.17.1
- SMS provider: active HTTP provider, credential values not inspected
- Provisioning role: IAM_ORG_MANAGER only
- Session role: IAM_LOGIN_CLIENT only
- Combined high-privilege token: not used
- User input: phone only; no email or password supplied
- Technical email: generated `@phone.invalid`, phone digits absent, mail not sent
- Password field: omitted
- Phone bootstrap: `isVerified=true`, provisional only
- OTP SMS factor enrollment: pass|fail
- One SMS challenge delivery: pass|fail
- ZITADEL OTP check: pass|fail
- Session user factor: pass|fail
- Session OTP SMS factor: pass|fail
- ListingKit project/user grant before verification: absent
- ListingKit subscription before verification: absent
- Sensitive-value scan: pass|fail
- Decision: pass|fail
- Blocking step: none|organization_create|user_create|otp_sms_add|challenge_create|code_verify|session_read|session_delete|organization_delete
```

Set `Decision: pass` only if every required item passes and both absence
assertions hold. Otherwise record `fail` and stop.

- [ ] **Step 5: Verify and commit the runbook/evidence**

```powershell
rg -n 'v4\.17\.1|phone only|Password field|One SMS challenge|Session OTP SMS factor|Decision:' docs/operations/listingkit-zitadel-phone-onboarding-feasibility.md docs/superpowers/verification/2026-08-25-listingkit-zitadel-phone-onboarding-feasibility.md
git diff --check
go -C hack/debug test ./listingkit-phone-onboarding-preflight -count=1
go test ./internal/listingkit/phoneonboardingpreflight ./internal/listingkit/zitadelsms -count=1
git add -- docs/operations/listingkit-zitadel-phone-onboarding-feasibility.md docs/superpowers/verification/2026-08-25-listingkit-zitadel-phone-onboarding-feasibility.md
git commit -m "docs: record ZITADEL phone onboarding feasibility"
```

The runner deletes the session and the exact temporary organization created by
the current attempt. Existing organizations from earlier runs are not searched
or bulk-deleted; clean those exact IDs only with separate approval.

---

### Task 5: Close the feasibility gate

**Files:**
- Modify: `docs/superpowers/specs/2026-08-25-listingkit-zitadel-native-phone-onboarding-design.md`
- Modify: `docs/superpowers/verification/2026-08-25-listingkit-zitadel-phone-onboarding-feasibility.md`

**Interfaces:**
- Consumes: Task 4 evidence with `Decision: pass` or `Decision: fail`.
- Produces: explicit permission to write downstream plans, or a blocker requiring design review.

- [ ] **Step 1: Add exactly one evidence-backed status line to the spec**

For pass:

```markdown
- Feasibility gate: Passed against ZITADEL v4.17.1; downstream implementation planning is authorized.
```

For failure:

```markdown
- Feasibility gate: Failed against ZITADEL v4.17.1 at the redacted step recorded in the evidence; downstream implementation is blocked pending design review.
```

- [ ] **Step 2: Verify evidence/spec consistency**

```powershell
$evidence = Get-Content -LiteralPath 'docs/superpowers/verification/2026-08-25-listingkit-zitadel-phone-onboarding-feasibility.md' -Raw
$spec = Get-Content -LiteralPath 'docs/superpowers/specs/2026-08-25-listingkit-zitadel-native-phone-onboarding-design.md' -Raw
if ($evidence -match 'Decision: pass' -and $spec -notmatch 'Feasibility gate: Passed') { throw 'passing evidence/spec mismatch' }
if ($evidence -match 'Decision: fail' -and $spec -notmatch 'Feasibility gate: Failed') { throw 'failed evidence/spec mismatch' }
git diff --check
```

Expected: exit 0.

- [ ] **Step 3: Commit and stop at the planning boundary**

```powershell
git add -- docs/superpowers/specs/2026-08-25-listingkit-zitadel-native-phone-onboarding-design.md docs/superpowers/verification/2026-08-25-listingkit-zitadel-phone-onboarding-feasibility.md
git commit -m "docs: close ZITADEL phone onboarding feasibility gate"
```

If the gate passed, write the three downstream plans listed under Program
decomposition. If it failed, return to brainstorming with the exact blocking
operation. Do not add a second OTP system or weaken the one-code requirement.
