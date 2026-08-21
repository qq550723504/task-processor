# 1688 Local Agent POC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax.

**Goal:** Build a Windows CLI that claims one tenant-scoped public 1688 job, crawls it in local Chrome, and returns a server-reconstructed SourceEnvelope.

**Architecture:** internal/localagent owns the in-memory job state machine and source reconstruction. An authenticated HTTP adapter owns JSON routes. cmd/1688-local-agent owns device authorization, HTTP calls, and the existing 1688 integration adapter.

**Tech Stack:** Go 1.26, Gin, existing ZITADEL middleware, OAuth 2.0 Device Authorization, Playwright-Go.

**Spec:** docs/superpowers/specs/2026-08-21-1688-local-agent-poc-design.md

## Global Constraints

- Public detail.1688.com numeric offer URLs only; never pass cookies, source accounts, proxies, browser profiles, target stores, drafts, or publish data.
- Server derives SourceEnvelope and all lineage from the accepted snapshot.
- Routes use existing trusted ZITADEL identity; no request tenant ID.
- Job expiry is five minutes; claim expiry is three; snapshot max 1 MiB; diagnostic max 512 UTF-8 bytes.
- Tokens are process-memory only. Reject offline_access, refresh tokens, redirects, and cross-origin OAuth endpoints.
- Do not alter internal/listingsubscription/TestGormUsageLedgerStoragePayloadUsesCommitSnapshotTime.

---

### Task 1: Local-agent state machine

**Files:**
- Create: internal/localagent/types.go
- Create: internal/localagent/service.go
- Create: internal/localagent/service_test.go

**Interfaces:**
- Produces Actor, Job, Claim, Failure, JobState, and Service.Create, Claim, SubmitSuccess, SubmitFailure.
- Consumes sourcing.Alibaba1688ProductSnapshot and sourcing.Alibaba1688SourceEnvelope.

- [ ] **Step 1: Write failing ownership and reconstruction tests**

~~~go
func TestServiceClaimIsTenantScopedAndSingleUse(t *testing.T) {
    service := NewService(fixedClock(t))
    owner := Actor{TenantID: "tenant-a", UserID: "user-a"}
    _, err := service.Create(owner, offerURL)
    require.NoError(t, err)

    wrongTenant, err := service.Claim(Actor{TenantID: "tenant-b", UserID: "user-b"})
    require.NoError(t, err)
    require.Nil(t, wrongTenant)

    claim, err := service.Claim(owner)
    require.NoError(t, err)
    require.NotEmpty(t, claim.ExecutionToken)
    again, _ := service.Claim(owner)
    require.Nil(t, again)
}

func TestServiceBuildsEnvelopeFromAcceptedSnapshot(t *testing.T) {
    service := NewService(fixedClock(t))
    actor := Actor{TenantID: "tenant-a", UserID: "user-a"}
    job, _ := service.Create(actor, offerURL)
    claim, _ := service.Claim(actor)
    done, err := service.SubmitSuccess(actor, job.ID, claim.ExecutionToken,
        &sourcing.Alibaba1688ProductSnapshot{ID: "1052008074197", Title: "shirt", URL: offerURL})
    require.NoError(t, err)
    require.Equal(t, "crawler:1688:1052008074197", done.Envelope.Identity.SourceKey())
}
~~~

- [ ] **Step 2: Verify RED**

Run: go test ./internal/localagent -run 'TestService(ClaimIsTenantScopedAndSingleUse|BuildsEnvelopeFromAcceptedSnapshot)' -count=1

Expected: FAIL because NewService does not exist.

- [ ] **Step 3: Implement minimal service**

~~~go
type Actor struct{ TenantID, UserID string }
type JobState string
const (
    JobPending JobState = "pending"
    JobClaimed JobState = "claimed"
    JobSucceeded JobState = "succeeded"
    JobFailed JobState = "failed"
)
func (s *Service) SubmitSuccess(a Actor, id, token string, p *sourcing.Alibaba1688ProductSnapshot) (Job, error) {
    r, err := s.requireActiveClaim(a, id, token)
    if err != nil { return Job{}, err }
    e := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
        Request: sourcing.Alibaba1688CrawlRequestInput{URL: r.job.URL}, Product: p, SourceRunID: r.job.ID,
    })
    r.job.State, r.job.Envelope, r.executionToken = JobSucceeded, &e, ""
    return r.publicJob(), nil
}
~~~

Use crypto/rand IDs and tokens. Validate canonical offer URL on creation and submit. Keep tokens in internal records only. Reject nil or oversized snapshots, invalid failures, wrong tenant/token/state, expired lease, and duplicate terminal result.

- [ ] **Step 4: Add expiry and redaction tests; verify GREEN**

~~~go
func TestServiceRejectsExpiredClaim(t *testing.T) {
    clock := newMutableClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
    service := NewService(clock.Now)
    a := Actor{TenantID: "tenant-a", UserID: "user-a"}
    job, _ := service.Create(a, offerURL); claim, _ := service.Claim(a)
    clock.Advance(3*time.Minute + time.Nanosecond)
    _, err := service.SubmitFailure(a, job.ID, claim.ExecutionToken, Failure{Kind: FailureNavigation, Message: "timeout"})
    require.ErrorIs(t, err, ErrClaimExpired)
}
~~~

Run: go test ./internal/localagent -count=1

Expected: PASS.

- [ ] **Step 5: Commit**

~~~bash
git add internal/localagent/types.go internal/localagent/service.go internal/localagent/service_test.go
git commit -m "feat: add local agent job service"
~~~

### Task 2: Authenticated HTTP contract

**Files:**
- Create: internal/localagent/httpapi/handler.go
- Create: internal/localagent/httpapi/routes.go
- Create: internal/localagent/httpapi/http_module.go
- Create: internal/localagent/httpapi/handler_test.go
- Modify: internal/authz/listingkit.go
- Modify: internal/listingkit/httpapi/zitadel_auth_route_authorization.go
- Modify: internal/app/httpapi/composition_builder.go
- Modify: internal/app/httpapi/types.go
- Modify: internal/app/httpapi/composition_modules.go
- Test: internal/app/httpapi/server_test.go

**Interfaces:**
- Routes: POST /api/v1/local-agent/1688-jobs, POST /api/v1/local-agent/1688-jobs/claim, POST /api/v1/local-agent/1688-jobs/:job_id/result.
- All routes use ModuleName local-agent and authz.PermissionLocalAgentWrite.

- [ ] **Step 1: Write failing trusted-identity tests**

~~~go
func TestCreateJobUsesVerifiedTenant(t *testing.T) {
    h := NewHandler(localagent.NewService(fixedClock(t)))
    response := performVerifiedRequest(t, h.Create, http.MethodPost,
        "/api/v1/local-agent/1688-jobs",
        "{\"url\":\"https://detail.1688.com/offer/1052008074197.html\",\"tenant_id\":\"forged\"}",
        "tenant-real", "user-1")
    require.Equal(t, http.StatusCreated, response.Code)
    require.NotContains(t, response.Body.String(), "execution_token")
}
func TestLocalAgentRouteRequiresZitadelAndPermission(t *testing.T) {
    r := httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs/claim", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite}
    require.True(t, listingkithttpapi.RouteRequiresZitadelAuth(r))
    require.NotNil(t, listingkithttpapi.NewRouteRoleMiddleware(r))
}
~~~

- [ ] **Step 2: Verify RED**

Run: go test ./internal/localagent/httpapi ./internal/app/httpapi -run 'Test(CreateJobUsesVerifiedTenant|LocalAgentRouteRequiresZitadelAndPermission)' -count=1

Expected: FAIL because the module and permission are absent.

- [ ] **Step 3: Implement JSON adapter and composition**

~~~go
const ModuleName = "local-agent"
func AppendRouteDescriptors(rs []httproute.Descriptor, h *Handler) []httproute.Descriptor {
    return append(rs,
        httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: h.Create},
        httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs/claim", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: h.Claim},
        httproute.Descriptor{Method: http.MethodPost, Path: "/api/v1/local-agent/1688-jobs/:job_id/result", Module: ModuleName, Permission: authz.PermissionLocalAgentWrite, Handler: h.SubmitResult},
    )
}
~~~

Map lower-case DTOs to sourcing snapshots only in this package. Read tenant and user exclusively from listingkit.AuthenticatedIdentityFromContext. Return 400 malformed input, 401 no identity, 403 tenant/token mismatch, 409 expired/terminal state, 204 no job, 201 create, and 200 terminal result. Add local-agent to RouteRequiresZitadelAuth and PermissionLocalAgentWrite to operator/admin policies. Build the module only with ListingKit present and append it to runtimeModules.

- [ ] **Step 4: Verify mounted lifecycle**

Run: go test ./internal/localagent/httpapi ./internal/app/httpapi -count=1

Expected: PASS, including create, claim, submit, and source_platform=1688 response.

- [ ] **Step 5: Commit**

~~~bash
git add internal/localagent/httpapi internal/authz/listingkit.go internal/listingkit/httpapi/zitadel_auth_route_authorization.go internal/app/httpapi
git commit -m "feat: expose tenant-scoped local agent jobs"
~~~

### Task 3: Device authorization and local runner

**Files:**
- Create: internal/localagent/deviceauth/client.go
- Create: internal/localagent/deviceauth/client_test.go
- Create: internal/localagent/client/client.go
- Create: internal/localagent/client/client_test.go
- Create: internal/localagent/runner.go
- Create: internal/localagent/runner_test.go

**Interfaces:**
- Produces deviceauth.Authorize(context.Context, Config, Presenter) (string, error).
- Produces Runner.RunOnce(context.Context) (Outcome, error).
- Runner uses integration/crawler/a1688.NewLegacyProcessor and a1688.SnapshotFromLegacyProduct.

- [ ] **Step 1: Write failing device-flow and runner tests**

~~~go
func TestAuthorizeRejectsCrossOriginTokenEndpoint(t *testing.T) {
    issuer := newOIDCTestServer(t, discovery{DeviceEndpoint: "/device", TokenEndpoint: "https://elsewhere.example/token"})
    _, err := Authorize(context.Background(), Config{IssuerURL: issuer.URL, ClientID: "client", ProjectID: "project", HTTPClient: issuer.Client()}, recordingPresenter{})
    require.ErrorContains(t, err, "same origin")
}
func TestRunnerSubmitsSanitizedChallenge(t *testing.T) {
    api := &fakeJobsAPI{claim: &Claim{Job: Job{ID: "job-1", URL: offerURL}, ExecutionToken: "token"}}
    crawler := &fakeCrawler{err: alibaba1688.NewPublicAccessError(alibaba1688.PublicAccessFailureChallenge, errors.New("cookie=secret"))}
    _, err := (Runner{Jobs: api, Crawler: crawler}).RunOnce(context.Background())
    require.NoError(t, err)
    require.Equal(t, localagent.FailureChallenge, api.submittedFailure.Kind)
    require.NotContains(t, api.submittedFailure.Message, "secret")
}
~~~

- [ ] **Step 2: Verify RED**

Run: go test ./internal/localagent/deviceauth ./internal/localagent ./internal/localagent/client -run 'Test(AuthorizeRejectsCrossOriginTokenEndpoint|RunnerSubmitsSanitizedChallenge)' -count=1

Expected: FAIL because these packages and symbols are absent.

- [ ] **Step 3: Implement fail-closed OAuth and HTTP runner**

~~~go
type Config struct { IssuerURL, ClientID, ProjectID string; Timeout time.Duration; HTTPClient *http.Client }
type Presenter interface { Show(string, string) error }
func (r Runner) RunOnce(ctx context.Context) (Outcome, error) {
    claim, err := r.Jobs.Claim(ctx)
    if err != nil || claim == nil { return Outcome{State: OutcomeIdle}, err }
    product, err := r.Crawler.Process(ctx, claim.Job.URL)
    if err != nil {
        _, submitErr := r.Jobs.SubmitFailure(ctx, claim.Job.ID, claim.ExecutionToken, classifyFailure(err))
        if submitErr != nil { return Outcome{}, submitErr }
        return Outcome{State: OutcomeFailed, JobID: claim.Job.ID}, nil
    }
    _, err = r.Jobs.SubmitSuccess(ctx, claim.Job.ID, claim.ExecutionToken, a1688.SnapshotFromLegacyProduct(product))
    return Outcome{State: OutcomeSucceeded, JobID: claim.Job.ID}, err
}
~~~

Reuse the exact scope policy from scripts/lib/listingkit-device-auth.ps1. Reject offline_access, refresh tokens, redirects, foreign discovery/device/token/verification origins, and body-bearing errors. The API client accepts HTTPS or literal loopback only, sends bearer token only to its configured origin, and redacts errors. classifyFailure uses errors.As and sends a fixed diagnostic.

- [ ] **Step 4: Verify GREEN**

Run: go test ./internal/localagent/... -count=1

Expected: PASS for approved/pending/slow_down/denied/expired OAuth responses and browser/navigation/challenge/extraction runner outcomes.

- [ ] **Step 5: Commit**

~~~bash
git add internal/localagent/deviceauth internal/localagent/client internal/localagent/runner.go internal/localagent/runner_test.go
git commit -m "feat: add local agent runner and device auth"
~~~

### Task 4: Windows CLI and controlled local acceptance

**Files:**
- Create: cmd/1688-local-agent/main.go
- Create: cmd/1688-local-agent/main_test.go
- Create: internal/localagent/local_agent_e2e_test.go
- Create: docs/development/1688-local-agent-poc.md
- Create: scripts/1688-local-agent-acceptance.ps1
- Create: scripts/1688-local-agent-acceptance.Tests.ps1

**Interfaces:**
- CLI flags: -api-base-url, -issuer-url, -client-id, -project-id, -url, -open-browser.
- -url creates one job before RunOnce; omitting it only claims a pending job.

- [ ] **Step 1: Write failing CLI and protocol tests**

~~~go
func TestParseConfigAcceptsOneShotOfferURL(t *testing.T) {
    cfg, err := parseConfig([]string{"-api-base-url", "http://127.0.0.1:18086", "-issuer-url", "http://127.0.0.1:19000", "-client-id", "client", "-project-id", "project", "-url", offerURL})
    require.NoError(t, err)
    require.Equal(t, offerURL, cfg.CreateURL)
}
func TestLocalAgentProtocolCompletesWithoutListingKitTask(t *testing.T) {
    api := newAuthenticatedLocalAgentTestServer(t)
    createAgentJob(t, api.URL, offerURL)
    outcome, err := newRunnerForServer(t, api.URL, fakeProductCrawler()).RunOnce(context.Background())
    require.NoError(t, err)
    require.Equal(t, OutcomeSucceeded, outcome.State)
    require.NotEmpty(t, completedAgentJob(t, api).Envelope.Identity.SourceKey())
}
~~~

- [ ] **Step 2: Verify RED**

Run: go test ./cmd/1688-local-agent ./internal/localagent -run 'Test(ParseConfigAcceptsOneShotOfferURL|LocalAgentProtocolCompletesWithoutListingKitTask)' -count=1

Expected: FAIL because CLI parsing and protocol helpers are absent.

- [ ] **Step 3: Implement command, runbook, and confirmation-gated script**

~~~powershell
param([string]$ApiBaseUrl = "http://127.0.0.1:18086", [string]$Url, [string]$Confirm = "")
if ($Confirm -cne "CREATE-LOCAL-AGENT-JOB") { throw "Set -Confirm CREATE-LOCAL-AGENT-JOB to create a local-agent job." }
& go run ./cmd/1688-local-agent -api-base-url $ApiBaseUrl -url $Url -issuer-url $env:LISTINGKIT_ZITADEL_ISSUER_URL -client-id $env:LISTINGKIT_ZITADEL_CLIENT_ID -project-id $env:TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID
if ($LASTEXITCODE -ne 0) { throw "1688 local agent failed" }
~~~

The script validates API and offer URLs before device auth, prints no token or product payload, and has no listing_store or source_account_id argument. Document starting the local API, the exact confirmation, and the fact that no draft/publish endpoint is invoked.

- [ ] **Step 4: Verify focused behavior**

Run:

~~~bash
go test ./internal/localagent/... ./internal/app/httpapi ./internal/product/sourcing/... ./internal/integration/crawler/a1688/... -count=1
pwsh -NoProfile -Command "Invoke-Pester -Path scripts/1688-local-agent-acceptance.Tests.ps1 -PassThru"
go build ./cmd/1688-local-agent
git diff --check
~~~

Expected: all focused tests pass, the CLI builds, and diff has no whitespace errors. If full tests recur with the baseline timestamp-order failure, report it separately.

- [ ] **Step 5: Commit**

~~~bash
git add cmd/1688-local-agent internal/localagent/local_agent_e2e_test.go docs/development/1688-local-agent-poc.md scripts/1688-local-agent-acceptance.ps1 scripts/1688-local-agent-acceptance.Tests.ps1
git commit -m "test: document local agent acceptance"
~~~

## Plan Self-Review

- Tasks 1-4 cover every spec requirement: state, HTTP tenant isolation, device-token safety, local Chrome execution, and controlled acceptance.
- localagent.Job, localagent.Claim, localagent.Failure, and sourcing.Alibaba1688ProductSnapshot are the shared types; only the HTTP adapter owns JSON DTO conversion.
- Persistence, UI, target-store selection, source-account profiles, and ListingKit task creation are absent.
