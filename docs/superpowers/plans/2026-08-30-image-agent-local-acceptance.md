# Image Agent Local Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-contained local ZITADEL identity path and guarded, token-owned ListingKit seed so a real user can create and dispatch a manual Image Agent run without typing tenant or user IDs.

**Architecture:** A reusable ZITADEL verifier derives the same canonical identity for the API middleware and seed command. The existing ZITADEL provisioner owns local project/application/role resources; a focused acceptance package owns local runtime validation, database marker checks and repository-backed task seeding. PowerShell composes only local Docker services and these CLI adapters.

**Tech Stack:** Go, Gin, GORM/PostgreSQL, ZITADEL Management and OIDC APIs, Docker Compose, Temporal, PowerShell 7.

**Spec:** docs/superpowers/specs/2026-08-30-image-agent-local-acceptance-design.md

## Global Constraints

- Reuse the existing Temporal Image Agent runtime, ListingKit workspace routes, task repository, ZITADEL runtime and internal/zitadelprovision package. Do not build a second workflow engine or direct task SQL path.
- The base ZITADEL Compose file cannot require docker_yudao-network; only an explicit overlay may use it.
- Tokens, management tokens, client secrets, DSNs, environment markers and source URLs remain in ignored .local/image-agent-acceptance/ and never appear in full in output.
- The seed command accepts no tenant/user ID, arbitrary DSN, run ID, plan, budget, provider credential or generated result.
- image_agent_acceptance is the only writable database name. Database name, environment marker and Compose-project identity must all pass before mutation.
- Preserve fail-closed token verification and safe public-HTTPS image validation. Do not introduce forged identity headers, fake provider responses, MinIO-as-COS evidence, or fallbacks.
- Creation and Temporal receipt are the scope of this local acceptance. Remote generation and COS publication require separately authorized governed credentials and storage.

---

## File Structure

| Path | Responsibility |
| --- | --- |
| deployments/docker/zitadel/docker-compose.yml | Local ZITADEL default with local PostgreSQL and no external network. |
| deployments/docker/zitadel/docker-compose.yudao-db.yml | Explicit external Yudao network/DSN mode. |
| deployments/docker/zitadel/README.md | Correct local and overlay command sequences. |
| scripts/tests/listingkit-local-zitadel-compose-test.ps1 | Rendered Compose contract test. |
| internal/authruntime/zitadel/verifier.go | Shared bearer-token-to-verified-identity port. |
| internal/authruntime/zitadel/verifier_test.go | Discovery/introspection and identity failure tests. |
| internal/zitadelprovision/provisioner.go | Project, role, API app, OIDC app and local role-grant lifecycle. |
| cmd/listingkit-zitadel-provision/main.go | provision and authorize CLI adapter. |
| internal/listingkit/imageagentacceptance/config.go | Acceptance runtime-file model. |
| internal/listingkit/imageagentacceptance/environment.go | Database marker and Compose identity guard. |
| internal/listingkit/imageagentacceptance/seed.go | Repository-backed, idempotent SHEIN task seed. |
| cmd/listingkit-image-agent-acceptance-seed/main.go | Thin seed executable. |
| scripts/image-agent-local-acceptance.ps1 | start, provision, authorize, seed, status and stop orchestration. |
| docs/development/image-agent-local-acceptance.md | Manual acceptance runbook and boundaries. |

## Task 1: Make local ZITADEL Compose self-contained

**Files:**
- Modify: deployments/docker/zitadel/docker-compose.yml
- Create: deployments/docker/zitadel/docker-compose.yudao-db.yml
- Modify: deployments/docker/zitadel/README.md
- Create: scripts/tests/listingkit-local-zitadel-compose-test.ps1

**Interfaces:**
- Consumes: deployments/docker/zitadel/.env.example.
- Produces: base Compose renders without external network; overlay adds docker_yudao-network intentionally.

- [ ] **Step 1: Write the failing Compose contract test**

~~~powershell
$base = docker compose --env-file deployments/docker/zitadel/.env.example -f deployments/docker/zitadel/docker-compose.yml config | Out-String
if ($base -match 'docker_yudao-network') { throw 'base Compose must not require docker_yudao-network' }
if ($base -notmatch 'postgres:') { throw 'base Compose must include local postgres' }
$overlay = docker compose --env-file deployments/docker/zitadel/.env.example -f deployments/docker/zitadel/docker-compose.yml -f deployments/docker/zitadel/docker-compose.yudao-db.yml config | Out-String
if ($overlay -notmatch 'docker_yudao-network') { throw 'Yudao overlay must opt in to docker_yudao-network' }
~~~

- [ ] **Step 2: Run it to verify failure**

Run: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/tests/listingkit-local-zitadel-compose-test.ps1

Expected: FAIL because the base file currently connects to docker_yudao-network and profiles out postgres.

- [ ] **Step 3: Implement the minimal Compose split**

Remove the postgres profile and docker_yudao-network declaration/attachment from the base. Add the overlay:

~~~yaml
services:
  zitadel-api:
    networks:
      - docker_yudao-network

networks:
  docker_yudao-network:
    external: true
~~~

Move the existing external DSN setting into the overlay. Document base-only start, explicit two-file Yudao start, and each matching down/reset command.

- [ ] **Step 4: Run Compose checks**

Run: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/tests/listingkit-local-zitadel-compose-test.ps1

Expected: PASS; the test only renders config and starts no containers.

- [ ] **Step 5: Commit**

~~~powershell
git add deployments/docker/zitadel/docker-compose.yml deployments/docker/zitadel/docker-compose.yudao-db.yml deployments/docker/zitadel/README.md scripts/tests/listingkit-local-zitadel-compose-test.ps1
git commit -m "fix: make local zitadel compose self contained"
~~~

## Task 2: Extract a shared ZITADEL verifier

**Files:**
- Create: internal/authruntime/zitadel/verifier.go
- Create: internal/authruntime/zitadel/verifier_test.go
- Modify: internal/authruntime/zitadel/config.go
- Modify: internal/authruntime/zitadel/middleware.go
- Modify: internal/authruntime/zitadel/middleware_test.go

**Interfaces:**
- Consumes: current Config, IntrospectionResponse, ParseRoles and authidentity.AuthenticatedIdentity.
- Produces:

~~~go
type Verifier interface {
    Verify(context.Context, string) (authidentity.AuthenticatedIdentity, error)
}

func NewVerifier(Config) Verifier
~~~

The verifier applies provider protocol and canonical tenant/subject validation. ListingKit allowlist policy remains at the existing middleware boundary.

- [ ] **Step 1: Write failing verifier tests**

~~~go
func TestVerifierReturnsCanonicalIdentity(t *testing.T) {
    verifier := NewVerifier(Config{
        IssuerURL: server.URL, ClientID: "api", ClientSecret: "secret",
        HTTPClient: server.Client(),
    })
    got, err := verifier.Verify(context.Background(), "user-token")
    require.NoError(t, err)
    require.Equal(t, authidentity.AuthenticatedIdentity{
        TenantID: "org-1", UserID: "user-1", Roles: []string{"listingkit_operator"},
    }, got)
}

func TestVerifierRejectsInactiveAndIncompleteIdentity(t *testing.T) {
    // Table entries: inactive token, missing resource owner, missing subject.
}
~~~

Use httptest discovery and introspection handlers; assert the API client Basic authorization header and one discovery request for repeated verification.

- [ ] **Step 2: Run the failing tests**

Run: go test ./internal/authruntime/zitadel -run TestVerifier -count=1

Expected: FAIL because Verifier and NewVerifier do not exist.

- [ ] **Step 3: Implement the verifier and delegate middleware**

Move the discovery cache and form-encoded introspection call out of middleware into a private verifier implementation. Verify rejects blank token, unavailable/malformed discovery, non-2xx/inactive responses, missing resource owner and missing subject.

~~~go
return authidentity.AuthenticatedIdentity{
    TenantID: strings.TrimSpace(payload.ResourceID),
    UserID:   strings.TrimSpace(payload.Subject),
    Roles:    append([]string(nil), payload.Roles...),
}, nil
~~~

Middleware calls Verify, preserves its existing response keys/statuses, clears forged headers, projects the returned identity to context, then applies its unchanged route authorization rule.

- [ ] **Step 4: Run focused regression tests**

Run: go test ./internal/authruntime/zitadel ./internal/listingkit/httpapi -run 'TestVerifier|Test.*Zitadel' -count=1

Expected: PASS; missing token, invalid token, missing owner/subject, forged-header and allowlist tests remain green.

- [ ] **Step 5: Commit**

~~~powershell
git add internal/authruntime/zitadel internal/listingkit/httpapi
git commit -m "refactor: share zitadel identity verification"
~~~

## Task 3: Provision local applications and grant the derived user

**Files:**
- Modify: internal/zitadelprovision/provisioner.go
- Modify: internal/zitadelprovision/provisioner_test.go
- Create: cmd/listingkit-zitadel-provision/main.go
- Create: cmd/listingkit-zitadel-provision/main_test.go

**Interfaces:**
- Consumes: existing Config, a local management-token file, and Task 2 verified browser identity.
- Produces:

~~~go
type LocalApplicationConfig struct {
    APIName, OIDCName string
    RedirectURIs, PostLogoutRedirectURIs []string
}
type LocalApplicationResult struct {
    ProjectID, APIAppID, APIClientID, APIClientSecret string
    OIDCAppID, OIDCClientID, OIDCClientSecret string
    RecommendedScopes []string
}
func ProvisionLocalApplications(context.Context, Config, LocalApplicationConfig) (LocalApplicationResult, error)
func GrantLocalOperator(context.Context, Config, string, authidentity.AuthenticatedIdentity) error
~~~

- [ ] **Step 1: Write failing provisioner tests**

~~~go
func TestProvisionLocalApplicationsCreatesAPIAndOIDCApps(t *testing.T) {
    // Assert POST /management/v1/projects/project-1/apps/api sends
    // name ListingKit Local API and API_AUTH_METHOD_TYPE_BASIC.
    // Assert POST /management/v1/projects/project-1/apps/oidc sends only local
    // redirects, code flow, bearer token and role assertion.
}
func TestGrantLocalOperatorUsesVerifiedIdentity(t *testing.T) {
    // Assert CreateAuthorization includes userId=user-1, organizationId=org-1,
    // projectId=project-1 and roleKeys=[listingkit_operator].
}
~~~

The HTTP server must also assert errors and result formatting never echo management tokens or returned client secrets.

- [ ] **Step 2: Run provisioner tests**

Run: go test ./internal/zitadelprovision ./cmd/listingkit-zitadel-provision -count=1

Expected: FAIL because local application and grant APIs are absent.

- [ ] **Step 3: Implement idempotent management operations**

Extend internal/zitadelprovision rather than adding another management client. Search applications by stable local names before posting. Create the API app using API_AUTH_METHOD_TYPE_BASIC. Create the OIDC app with exact localhost redirect/post-logout URLs, OIDC_APP_TYPE_USER_AGENT, authorization-code grant, bearer access token and accessTokenRoleAssertion=true.

The provisioner returns generated secrets only to the CLI runtime-file writer. It never includes them in Error or normal console Result values. GrantLocalOperator rejects blank verified identity, searches for an equivalent user grant, then uses the existing CreateAuthorization Connect protocol only if missing. Grant admin only when the CLI receives -grant-admin.

- [ ] **Step 4: Implement two explicit CLI phases**

provision reads -management-token-file and writes generated values to -runtime-file. authorize reads the browser token file and runtime file, uses Task 2 Verifier to derive its identity, and grants listingkit_operator. Neither command has a tenant or user flag.

~~~powershell
go run ./cmd/listingkit-zitadel-provision provision -management-token-file .local/image-agent-acceptance/management-token.txt -runtime-file .local/image-agent-acceptance/runtime.env
go run ./cmd/listingkit-zitadel-provision authorize -token-file .local/image-agent-acceptance/user-token.txt -runtime-file .local/image-agent-acceptance/runtime.env
~~~

- [ ] **Step 5: Run focused checks and commit**

Run: go test ./internal/zitadelprovision ./cmd/listingkit-zitadel-provision ./internal/authruntime/zitadel -count=1

Expected: PASS; resource re-runs are idempotent and role assignment cannot precede a verified browser token.

~~~powershell
git add internal/zitadelprovision cmd/listingkit-zitadel-provision
git commit -m "feat: provision local listingkit zitadel applications"
~~~

## Task 4: Implement guarded runtime and repository-backed seed

**Files:**
- Create: internal/listingkit/imageagentacceptance/config.go
- Create: internal/listingkit/imageagentacceptance/environment.go
- Create: internal/listingkit/imageagentacceptance/seed.go
- Create: internal/listingkit/imageagentacceptance/config_test.go
- Create: internal/listingkit/imageagentacceptance/environment_test.go
- Create: internal/listingkit/imageagentacceptance/seed_test.go

**Interfaces:**
- Consumes: Task 2 Verifier, listingkit.Repository, store.NewTaskRepository, imageagent.ValidateSafeImageURL and generated runtime config.
- Produces:

~~~go
const DatabaseName = "image_agent_acceptance"

type RuntimeConfig struct {
    DatabaseDSN, EnvironmentMarker, ComposeProject string
    IssuerURL, APIClientID, APIClientSecret string
}
type EnvironmentGuard interface {
    Verify(context.Context, RuntimeConfig) (*gorm.DB, error)
}
type SeedRequest struct { Token, SourceURL, StyleURL string }
type SeedResult struct { TaskID, TenantID, UserID, WorkspaceURL string }

func Seed(context.Context, EnvironmentGuard, zitadel.Verifier, listingkit.Repository, SeedRequest) (SeedResult, error)
~~~

- [ ] **Step 1: Write failing environment guard tests**

~~~go
func TestEnvironmentGuardRejectsNonAcceptanceDatabase(t *testing.T) { /* current_database is postgres */ }
func TestEnvironmentGuardRejectsMissingOrMismatchedMarker(t *testing.T) { /* absent and mismatched marker */ }
func TestEnvironmentGuardRejectsOtherComposeProject(t *testing.T) { /* injected Docker label probe */ }
~~~

Inject database-name, marker-lookup and Docker-project probe functions so the test suite requires neither Docker nor a real DSN.

- [ ] **Step 2: Run guard tests**

Run: go test ./internal/listingkit/imageagentacceptance -run TestEnvironmentGuard -count=1

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement config parsing and guard**

Read only the generated acceptance runtime file. Reject missing fields and database names other than image_agent_acceptance. The guard checks known Compose project identity before opening the connection, then checks current_database() and the environment marker table before returning GORM. It closes the database on every command exit.

- [ ] **Step 4: Write failing seed tests**

~~~go
func TestSeedDerivesOwnerAndCreatesMinimalSheinTask(t *testing.T) {
    // verifier returns org-1/user-1/listingkit_operator; repository receives
    // owned task with StandardProductSnapshot and a shein source asset.
}
func TestSeedRerunIsIdempotentButDifferentSourceOrOwnerFails(t *testing.T) {}
func TestSeedRejectsMissingRoleAndUnsafeURLs(t *testing.T) {}
~~~

- [ ] **Step 5: Run seed tests**

Run: go test ./internal/listingkit/imageagentacceptance -run TestSeed -count=1

Expected: FAIL because Seed does not exist.

- [ ] **Step 6: Implement minimal, idempotent fixture construction**

Require listingkit_operator, listingkit_admin or platform_admin. Validate source and optional style URLs with imageagent.ValidateSafeImageURL. Derive a stable 36-character task ID from SHA-256 over image-agent-acceptance:v1 plus verified tenant and user. Read it through repo.GetTask using authidentity.WithAuthenticatedIdentity; create only when absent.

~~~go
task := &listingkit.Task{
    ID: taskID, TenantID: identity.TenantID, UserID: identity.UserID,
    Result: &listingkit.ListingKitResult{
        StandardProductSnapshot: &listingkit.StandardProductSnapshot{},
        AssetBundlesByTarget: map[string]*asset.Bundle{"shein": {Assets: assets}},
    },
}
~~~

The source asset ID is image-agent-acceptance-source. The optional style is non-source. Equivalent reruns return the existing result; changed owner, target or URL returns an error and never overwrites. Create uses repo.CreateTask only.

- [ ] **Step 7: Run focused regression and commit**

Run: go test ./internal/listingkit/imageagentacceptance ./internal/listingkit/store ./internal/listingkit/httpapi -run 'TestEnvironmentGuard|TestSeed|TestImageAgentWorkspace' -count=1

Expected: PASS; the real workspace catalog accepts the seeded source and preserves ownership.

~~~powershell
git add internal/listingkit/imageagentacceptance
git commit -m "feat: add guarded image agent acceptance seed"
~~~

## Task 5: Add seed executable and prove blank-database migration

**Files:**
- Create: cmd/listingkit-image-agent-acceptance-seed/main.go
- Create: cmd/listingkit-image-agent-acceptance-seed/main_test.go
- Modify: internal/listingkit/schema/runtime_test.go
- Modify: internal/app/runtime/listingkitschemamigrate/runtime_test.go
- Modify: internal/listingkit/schema/runtime.go only if the canonical migration test proves a prerequisite missing.

**Interfaces:**
- Consumes: Task 4 runtime/guard/seed interfaces and the canonical listingkitschema.AutoMigrateRuntime entry point.
- Produces: go run ./cmd/listingkit-image-agent-acceptance-seed -runtime-file ... -token-file ... -source-url ....

- [ ] **Step 1: Write failing CLI and blank-schema tests**

~~~go
func TestAutoMigrateRuntimeCreatesTaskRepositoryPrerequisites(t *testing.T) {
    db := openFreshDatabase(t)
    require.NoError(t, listingkitschema.AutoMigrateRuntime(db))
    require.True(t, db.Migrator().HasTable("listing_kit_tasks"))
    require.True(t, db.Migrator().HasTable("listing_store"))
}
func TestSeedCommandRequiresLocalFilesAndPublicSourceURL(t *testing.T) {
    require.Error(t, run([]string{
        "-runtime-file", "", "-token-file", "", "-source-url", "http://localhost/a.png",
    }))
}
~~~

- [ ] **Step 2: Run to identify the actual migration gap**

Run: go test ./internal/listingkit/schema ./internal/app/runtime/listingkitschemamigrate ./cmd/listingkit-image-agent-acceptance-seed -count=1

Expected: the new command test fails. If a canonical prerequisite table is absent, report its exact owner and fix it only in internal/listingkit/schema or its current authoritative migration owner.

- [ ] **Step 3: Implement the thin executable**

The command loads runtime/token files, builds zitadel.NewVerifier, invokes Task 4 guard and Seed, then emits JSON with task_id, tenant_id, user_id and workspace_url only. It requires -source-url and accepts optional -style-url. It performs no raw SQL and has no DSN flag.

- [ ] **Step 4: Run tests and commit**

Run: go test ./internal/listingkit/schema ./internal/app/runtime/listingkitschemamigrate ./cmd/listingkit-image-agent-acceptance-seed -count=1

Expected: PASS; blank acceptance schema originates in canonical migration and the CLI cannot target another database.

~~~powershell
git add cmd/listingkit-image-agent-acceptance-seed internal/listingkit/schema internal/app/runtime/listingkitschemamigrate
git commit -m "feat: add image agent local seed command"
~~~

## Task 6: Add local orchestration and operator runbook

**Files:**
- Create: scripts/image-agent-local-acceptance.ps1
- Create: scripts/tests/image-agent-local-acceptance-script-test.ps1
- Create: docs/development/image-agent-local-acceptance.md
- Modify: .gitignore only if .local/ is not already ignored.

**Interfaces:**
- Consumes: Tasks 1, 3 and 5.
- Produces:

~~~powershell
./scripts/image-agent-local-acceptance.ps1 start
./scripts/image-agent-local-acceptance.ps1 provision -ManagementTokenFile .local/image-agent-acceptance/management-token.txt
./scripts/image-agent-local-acceptance.ps1 authorize -TokenFile .local/image-agent-acceptance/user-token.txt
./scripts/image-agent-local-acceptance.ps1 seed -SourceUrl https://public.example/image.png
./scripts/image-agent-local-acceptance.ps1 status
./scripts/image-agent-local-acceptance.ps1 stop
./scripts/image-agent-local-acceptance.ps1 stop -Reset
~~~

- [ ] **Step 1: Write failing script contract tests**

~~~powershell
& $scriptPath seed -SourceUrl 'http://localhost/nope.png'
if ($LASTEXITCODE -eq 0) { throw 'seed accepted an unsafe URL' }
& $scriptPath stop
if ($LASTEXITCODE -ne 0) { throw 'non-destructive stop must not require Reset' }
& $scriptPath stop -Reset -WhatIf
if ($LASTEXITCODE -ne 0) { throw 'reset preview must identify only the acceptance project' }
~~~

Inject command paths for docker compose and go run so unit tests do not start containers or read local secrets.

- [ ] **Step 2: Run the failing script test**

Run: powershell -NoProfile -ExecutionPolicy Bypass -File scripts/tests/image-agent-local-acceptance-script-test.ps1

Expected: FAIL because the orchestrator is absent.

- [ ] **Step 3: Implement explicit lifecycle controls**

Use fixed Compose project task-processor-image-agent-acceptance and fixed database name. start creates random local marker/secrets, renders Compose before up --wait, runs only the canonical schema migration and persists the marker. status checks known local service names, API health and Temporal reachability. stop only stops known project; stop -Reset prints exact project/volumes and refuses deletion without Reset.

provision, authorize and seed only delegate to fixed Task 3/5 runtime paths. The script never parses or prints bearer tokens.

- [ ] **Step 4: Write runbook**

Document the order: start, provision, local UI login, token save, authorize, seed, workspace preflight/run. Include public HTTPS source requirement, ownership negative tests, provider/COS boundary and reset semantics.

- [ ] **Step 5: Run static checks and commit**

Run:

~~~powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/tests/listingkit-local-zitadel-compose-test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/tests/image-agent-local-acceptance-script-test.ps1
git diff --check
~~~

Expected: PASS without Docker startup and without tracked local files.

~~~powershell
git add scripts/image-agent-local-acceptance.ps1 scripts/tests/image-agent-local-acceptance-script-test.ps1 docs/development/image-agent-local-acceptance.md deployments/docker/zitadel .gitignore
git commit -m "feat: add local image agent acceptance runtime"
~~~

## Task 7: Verify the real local manual chain

**Files:**
- Modify: docs/development/image-agent-local-acceptance.md only when runtime validation proves a command differs from the runbook.

**Interfaces:**
- Consumes: a local user-entered management token, real browser bearer token and public HTTPS source URL.
- Produces: local evidence for introspection, owned preflight, run creation and Temporal receipt; no remote generation success claim.

- [ ] **Step 1: Re-run focused automated suites before runtime work**

Run:

~~~powershell
go test ./internal/authruntime/zitadel ./internal/zitadelprovision ./internal/listingkit/imageagentacceptance ./internal/listingkit/schema ./internal/app/runtime/listingkitschemamigrate ./cmd/listingkit-zitadel-provision ./cmd/listingkit-image-agent-acceptance-seed -count=1
go test ./internal/listingkit/httpapi ./internal/imageagent/... ./internal/app/runtime ./internal/app/worker/imageagent -count=1
~~~

Expected: PASS. Stop and diagnose failures before containers start.

- [ ] **Step 2: Start and provision**

Run documented start, provision, local browser-login/token-save and authorize sequence. Verify local ZITADEL discovery, API health and token introspection without printing credentials.

- [ ] **Step 3: Seed and prove ownership**

Run seed with public HTTPS source. Use the saved token to fetch task Image Agent preflight. Repeat missing-token and different-user requests; expect denial/no task disclosure.

- [ ] **Step 4: Create and observe manual run**

Use the workspace create action or task-scoped POST with target shein and source image-agent-acceptance-source. Record task/run IDs, HTTP status and Temporal task-queue receipt only. Confirm missing/unsupported governed provider credential produces its real blocked/error state, never synthetic image output.

- [ ] **Step 5: Final verification**

Run:

~~~powershell
git diff --check
git status --short
git log --oneline -6
~~~

Expected: no secret/generated runtime files staged. If validation corrects a documented command, stage only the runbook correction and commit with docs: correct local image agent acceptance runbook.

## Plan Self-Review

- Spec coverage: Task 1 fixes Compose isolation; Tasks 2-3 establish real identity and local role grant; Tasks 4-5 enforce guarded repository seeding and canonical schema proof; Task 6 provides safe orchestration; Task 7 verifies the user-facing API and Temporal chain while preserving real provider/COS failure behavior.
- Placeholder scan: each task names files, explicit behavior, commands and expected test outcomes.
- Type consistency: zitadel.Verifier yields authidentity.AuthenticatedIdentity to both middleware and seed. RuntimeConfig is the only runtime configuration source. SeedResult exposes only derived identifiers and workspace URL.
