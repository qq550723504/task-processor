# ListingKit canonical ZITADEL subject implementation plan

> **For Codex:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Before editing production code, also use `superpowers:using-git-worktrees` and `superpowers:test-driven-development`; before reporting completion, use `superpowers:verification-before-completion`.

**Goal:** Make the verified ZITADEL `sub` the sole ListingKit user-ownership identifier, make owner scoping non-configurable, move privileged actor checks to authenticated context, and block releases when persisted owner IDs do not match ZITADEL subjects in the same tenant.

**Architecture:** Keep the Go API introspection middleware as the authoritative trust boundary. Normalize `sub` once into `listingkit.AuthenticatedIdentity`, share one canonical-claim extractor in the Next.js server layer, preserve compatibility headers only as downstream transport hints, and introduce a read-only preflight binary that compares an explicit owner-table inventory with ZITADEL's official Users API. Run that binary as a one-shot Kubernetes Job before changing the API deployment image.

**Tech stack:** Go 1.26, Gin, GORM/PostgreSQL, Next.js/Auth.js, TypeScript/Vitest, Kubernetes Jobs, GitHub Actions.

**Approved design:** `docs/superpowers/specs/2026-08-09-listingkit-canonical-zitadel-subject-design.md`

---

## Implementation constraints

- Work in an isolated `codex/` worktree created from commit `49ebeea45` or its descendant. Do not mix unrelated checkout changes into these commits.
- Do not add a legacy ID mapping table, `user_id` fallback, username fallback, data rewrite, or runtime compatibility flag.
- Do not add a new ZITADEL SDK just for one read-only endpoint. Reuse the repository's existing small HTTP-client pattern and the official `POST /v2/users` contract.
- Never log a complete user ID or tenant ID from the preflight. Report deterministic SHA-256 fingerprints only.
- Keep each commit below independently reviewable. Run the focused red test before implementation and the focused green test after it.
- The API and UI changes form one coordinated release. Do not deploy either from this plan until the preflight Job succeeds against the target environment.

## Task 1: Make the Go authentication boundary canonical and fail closed

**Files:**

- Modify: `internal/listingkit/httpapi/zitadel_auth_middleware.go`
- Modify: `internal/listingkit/httpapi/zitadel_auth_test.go`
- Modify: `internal/listingkit/authenticated_identity.go`
- Modify: `internal/listingkit/authenticated_identity_test.go`

### Step 1: Replace the fallback-preference test with the canonical-subject contract

In `internal/listingkit/httpapi/zitadel_auth_test.go`, replace
`TestListingKitZitadelAuthPrefersBusinessUserIDOverSubject` with a test named
`TestListingKitZitadelAuthUsesSubjectWhenClaimsDiffer`.

Build an active introspection response containing deliberately different values:

```go
{
    "active": true,
    "sub": "zitadel-subject-123",
    "user_id": "legacy-business-user-456",
    "username": "display-name",
    "urn:zitadel:iam:user:resourceowner:id": "tenant-789",
}
```

The downstream handler must assert all of the following:

```go
identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
require.True(t, ok)
assert.Equal(t, "tenant-789", identity.TenantID)
assert.Equal(t, "zitadel-subject-123", identity.UserID)
assert.Equal(t, "zitadel-subject-123", c.GetHeader("X-User-ID"))
assert.Equal(t, "tenant-789", c.GetHeader("X-Tenant-ID"))
```

Keep the existing spoofed-header assertions and make the request send forged
`X-User-ID`, `X-Tenant-ID`, and role headers so this test proves they cannot
win over introspection.

### Step 2: Add the missing-sub rejection test

Add `TestListingKitZitadelAuthRejectsMissingSubject`. Return an otherwise valid
active token with tenant, `user_id`, username, and roles but no `sub`. Assert:

```go
assert.Equal(t, http.StatusForbidden, recorder.Code)
assert.JSONEq(t, `{
  "error":"zitadel_user_missing",
  "message":"ZITADEL subject is required"
}`, recorder.Body.String())
assert.False(t, handlerCalled)
```

Run the two focused tests and confirm the first still sees the legacy ID and the
second currently reaches the handler:

```powershell
go test ./internal/listingkit/httpapi -run 'TestListingKitZitadelAuth(UsesSubjectWhenClaimsDiffer|RejectsMissingSubject)$' -count=1
```

Expected: FAIL for the intended reasons, not a compile or fixture error.

### Step 3: Require both tenant and subject in the middleware

In `internal/listingkit/httpapi/zitadel_auth_middleware.go`, normalize the
introspected subject once and use it for both context and compatibility header:

```go
tenantID := strings.TrimSpace(identity.ResourceID)
userID := strings.TrimSpace(identity.Subject)
if tenantID == "" {
    c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
        "error": "zitadel_tenant_missing",
        "message": "ZITADEL resource owner is required",
    })
    return
}
if userID == "" {
    c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
        "error": "zitadel_user_missing",
        "message": "ZITADEL subject is required",
    })
    return
}

authenticated := listingkit.AuthenticatedIdentity{
    TenantID: tenantID,
    UserID: userID,
    Roles: append([]string{}, identity.Roles...),
}
```

Delete `firstNonEmptyZitadelValue` if this was its last use. Keep `user_id` and
username fields in the introspection response struct only for diagnostics or
future display; do not pass either into ownership state.

### Step 4: Strengthen the context invariant

Change `AuthenticatedIdentityFromContext` in
`internal/listingkit/authenticated_identity.go` so it returns `false` when
either normalized `TenantID` or normalized `UserID` is empty:

```go
if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
    return AuthenticatedIdentity{}, false
}
```

Expand the table test in `internal/listingkit/authenticated_identity_test.go`
with missing-user and whitespace-user cases. Preserve successful normalization
and defensive role-copy assertions.

### Step 5: Run focused and package tests

```powershell
go test ./internal/listingkit/httpapi ./internal/listingkit -run 'Test(ListingKitZitadelAuth|AuthenticatedIdentity)' -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit -count=1
```

Expected: PASS.

### Step 6: Commit the backend boundary

```powershell
git add internal/listingkit/httpapi/zitadel_auth_middleware.go internal/listingkit/httpapi/zitadel_auth_test.go internal/listingkit/authenticated_identity.go internal/listingkit/authenticated_identity_test.go
git commit -m "fix: canonicalize ListingKit user identity to ZITADEL subject"
```

## Task 2: Give Auth.js and token verification one subject-only extractor

**Files:**

- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.ts`
- Modify: `web/listingkit-ui/src/lib/server/zitadel-auth.test.ts`
- Modify: `web/listingkit-ui/src/auth.config.ts`
- Modify: `web/listingkit-ui/src/app/api/zitadel-auth/session/route.test.ts`
- Modify if an existing proxy assertion needs the stronger contract: `web/listingkit-ui/src/proxy.test.ts`

### Step 1: Write failing extractor tests

In `zitadel-auth.test.ts`, replace the business-ID preference test with:

```ts
it("uses sub when user_id and username differ", async () => {
  // verified JWT payload contains sub, user_id, username, resource owner, roles
  expect(result?.identity.userId).toBe("zitadel-subject-123");
});

it("rejects an otherwise valid token without sub", async () => {
  // payload still contains user_id and username
  await expect(verifyZitadelAccessToken(token, config)).resolves.toBeNull();
});
```

Add a pure helper test if the file's crypto fixture makes claim extraction hard
to see directly:

```ts
expect(extractZitadelIdentityFromClaims({
  sub: "subject",
  user_id: "legacy",
  username: "name",
  [RESOURCE_OWNER_CLAIM]: "tenant",
})).toMatchObject({ tenantId: "tenant", userId: "subject" });
```

Run:

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- --run src/lib/server/zitadel-auth.test.ts
```

Expected: FAIL because `user_id` currently wins and missing `sub` still builds an identity.

### Step 2: Implement one shared canonical extractor

Export a server-only helper from `src/lib/server/zitadel-auth.ts`:

```ts
export function extractZitadelIdentityFromClaims(
  payload: ZitadelTokenPayload,
): ListingKitSessionIdentity | null {
  const tenantId = normalizeClaim(payload[RESOURCE_OWNER_CLAIM]);
  const userId = normalizeClaim(payload.sub);
  if (!tenantId || !userId) return null;

  return {
    tenantId,
    userId,
    username: normalizeClaim(payload.preferred_username ?? payload.username),
    roles: extractProjectRoles(payload),
  };
}
```

Use this helper inside `verifyZitadelAccessToken`; remove the
`payload.user_id ?? payload.sub ?? payload.username` expression. Keep the
optional `user_id` field in the payload type only because tokens may still
carry it; it must have no effect on identity.

In `src/auth.config.ts`, route both initial OIDC profile extraction and refresh
token extraction through this helper. Delete the duplicate
`businessUserId/subject/preferredUsername` preference function instead of
maintaining two canonicalization rules.

If importing the server helper into Auth.js creates a module-cycle, move the
claim types and pure extractor to a new
`web/listingkit-ui/src/lib/server/zitadel-identity.ts`, import it from both
callers, and test that file directly. Do not duplicate logic to avoid the cycle.

### Step 3: Prove missing subject cannot produce a valid session

In `src/app/api/zitadel-auth/session/route.test.ts`, add a request whose mocked
session/token contains tenant, `user_id`, and roles but no subject-derived
`identity.userId`. Assert `401` and that no `X-User-ID` value is returned.

If `src/proxy.test.ts` currently permits a session with an empty `userId`, add a
test proving it redirects to login and does not forward identity headers.

### Step 4: Run frontend tests and type checks

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- --run src/lib/server/zitadel-auth.test.ts src/app/api/zitadel-auth/session/route.test.ts src/proxy.test.ts
npm.cmd run typecheck
```

Expected: PASS.

### Step 5: Commit the frontend boundary

```powershell
git add web/listingkit-ui/src/lib/server/zitadel-auth.ts web/listingkit-ui/src/lib/server/zitadel-auth.test.ts web/listingkit-ui/src/auth.config.ts web/listingkit-ui/src/app/api/zitadel-auth/session/route.test.ts web/listingkit-ui/src/proxy.test.ts
git commit -m "fix: use ZITADEL subject in ListingKit sessions"
```

If the cycle-safe helper file was created, add that exact file as well.

## Task 3: Remove the owner-scope configuration fiction

**Files:**

- Modify: `internal/core/config/type_listingkit.go`
- Modify: `internal/core/config/loader_builder.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/config_test.go`
- Modify: `internal/core/config/config_env_test.go`
- Delete: `internal/core/config/listingkit_owner_scope_test.go`
- Modify: `internal/listingadmin/access_scope.go`
- Modify: `config/config-dev.yaml`
- Modify: `config/config-prod.yaml`
- Modify: `config/config-test.yaml`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Modify: `docs/development/listingkit-local-debug.md`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`

### Step 1: Convert configuration tests to an absence contract

Delete assertions that load or toggle `ListingKit.OwnerScopeRequired`. Add a
test next to the existing environment-binding boundary tests:

```go
func TestKnownEnvBindingsDoNotExposeListingKitOwnerScopeToggle(t *testing.T) {
    bindings := knownEnvBindings()
    _, ok := bindings["listingkit.ownerScopeRequired"]
    assert.False(t, ok)

    for _, binding := range bindings {
        assert.NotEqual(t, "TASK_PROCESSOR_LISTINGKIT_OWNER_SCOPE_REQUIRED", binding.Primary)
        assert.NotContains(t, binding.Deprecated, "TASK_PROCESSOR_LISTINGKIT_ZITADEL_OWNER_SCOPE_REQUIRED")
    }
}
```

Add a YAML repository-boundary assertion that searches the three committed
config files and deployment examples for both `ownerScopeRequired` and
`OWNER_SCOPE_REQUIRED` and expects neither. Keep it in
`internal/core/config/config_env_test.go` so a future reintroduction fails CI.

Run:

```powershell
go test ./internal/core/config -run 'Test.*OwnerScope' -count=1
```

Expected: FAIL while the field, bindings, and YAML keys still exist.

### Step 2: Remove the switch everywhere operators can see it

Remove:

- `OwnerScopeRequired` from `ListingKitConfig`.
- `v.GetBool("listingkit.ownerScopeRequired")` from `loader_builder.go`.
- primary and deprecated owner-scope env bindings from `config.go`.
- `ownerScopeRequired: false` from all three YAML files.
- the dedicated configuration behavior test file.
- documentation claiming the value can be enabled or disabled.

Update local and Kubernetes documentation to state: owner filtering is a fixed
startup invariant; tests may use package-local helpers, but deployment config
cannot disable it.

### Step 3: Prove bootstrap always enables both owner scopes

In `internal/listingkit/httpapi/bootstrap_test.go`, add a non-parallel test that
temporarily sets both package states false through their test helpers, invokes
the existing bootstrap preparation path, and asserts:

```go
assert.True(t, listingkit.OwnerScopeEnabled())
assert.True(t, listingadmin.OwnerScopeEnabled())
```

Add the read-only exported `OwnerScopeEnabled()` getter to
`internal/listingadmin/access_scope.go`, matching ListingKit's existing getter;
do not export or add any production setter beyond the existing bootstrap hook.
Use `t.Cleanup` to restore true. Do not introduce a production parameter for
this assertion.

### Step 4: Run config/bootstrap tests

```powershell
go test ./internal/core/config ./internal/listingkit/httpapi -count=1
rg -n 'ownerScopeRequired|OWNER_SCOPE_REQUIRED' config deployments docs/development internal/core/config
```

Expected: tests PASS and `rg` returns no production/config/documentation match.
Test-only owner-scope helper names under ListingKit packages are allowed; narrow
the final search if necessary and inspect every remaining result.

### Step 5: Commit the invariant

```powershell
git add internal/core/config internal/listingkit/httpapi/bootstrap_test.go config docs/development/listingkit-local-debug.md deployments/kubernetes/listingkit-workbench/README.md
git commit -m "refactor: make ListingKit owner scope non-configurable"
```

## Task 4: Stop privileged authorization and audit from trusting headers

**Files:**

- Create: `internal/listingkit/api/authenticated_actor.go`
- Create: `internal/listingkit/api/authenticated_actor_test.go`
- Modify: `internal/listingkit/api/subscription_guard.go`
- Modify: `internal/listingkit/api/subscription_handler_platform.go`
- Modify: `internal/listingkit/api/member_invitation_handler.go`
- Modify: `internal/listingkit/api/subscription_handler_test.go`

### Step 1: Write failing forged-header and missing-context tests

Add focused handler tests covering:

1. A request with forged `X-User-ID: configured-platform-admin` and forged
   admin role headers but no `AuthenticatedIdentity` receives `403`.
2. A request with a non-admin forged header and an authenticated context whose
   subject is a configured platform admin succeeds.
3. A platform mutation writes the context subject to its audit actor even when
   `X-User-ID` differs.
4. A member invitation with no authenticated context fails before the provider
   is called and before an audit row is recorded.
5. A member invitation with context plus a forged user header records the
   context subject.

Attach identity in tests with:

```go
req = req.WithContext(listingkit.WithAuthenticatedIdentity(req.Context(), listingkit.AuthenticatedIdentity{
    TenantID: "tenant-101",
    UserID: "zitadel-subject-101",
    Roles: []string{"platform_admin"},
}))
```

Run:

```powershell
go test ./internal/listingkit/api -run 'Test.*(PlatformSubscriptionAccess|AuditActor|MemberInvitation).*' -count=1
```

Expected: FAIL because the handlers currently read caller-controlled headers.

### Step 2: Add one fail-closed authenticated actor helper

Implement in `authenticated_actor.go`:

```go
func authenticatedActor(c *gin.Context) (listingkit.AuthenticatedIdentity, bool) {
    if c == nil || c.Request == nil {
        return listingkit.AuthenticatedIdentity{}, false
    }
    identity, ok := listingkit.AuthenticatedIdentityFromContext(c.Request.Context())
    if !ok {
        c.JSON(http.StatusForbidden, gin.H{
            "error": "zitadel_user_missing",
            "message": "authenticated ZITADEL subject is required",
        })
        return listingkit.AuthenticatedIdentity{}, false
    }
    return identity, true
}
```

Keep response writing in this helper so direct handler invocation also fails
closed. The middleware remains responsible for normal protected-route failure.

### Step 3: Convert platform authorization to context

In `requirePlatformSubscriptionAccess`:

- Call `authenticatedActor` first.
- Compare `identity.UserID` to `platformAdminUsers`.
- Compare `identity.Roles` to configured/default platform admin roles.
- Delete the reads of `X-User-ID`, `X-User-Roles`, and `X-Zitadel-Roles`.
- Delete `splitCSVHeaders` if no trusted use remains.

### Step 4: Convert every platform mutation audit actor

In `subscription_handler_platform.go`, obtain the actor once after access is
authorized, or add a private helper returning the already-verified subject.
Replace every `c.GetHeader("X-User-ID")` passed into these service methods:

- `UpsertEntitlementWithAudit`
- `UpsertPlan`
- `UpsertPlanModule`
- `DeletePlanModule`
- `SetPlanActive`
- `ApplyPlan`
- `SetUsage`

Use `identity.UserID`. Do not silently substitute an empty actor.

### Step 5: Convert member-invitation audit actor

At the start of the mutation path in `member_invitation_handler.go`, call
`authenticatedActor`; if it fails, return before request/provider/audit
mutation. Pass `identity.UserID` as `ActorUserID` to the invitation service.

Update existing direct-handler fixtures in `subscription_handler_test.go` to
install `AuthenticatedIdentity` rather than relying on `X-User-ID`.

### Step 6: Run focused and package tests

```powershell
go test ./internal/listingkit/api -run 'Test.*(Platform|Subscription|Invitation|Actor).*' -count=1
go test ./internal/listingkit/api -count=1
```

Expected: PASS.

### Step 7: Commit trusted actor changes

```powershell
git add internal/listingkit/api/authenticated_actor.go internal/listingkit/api/authenticated_actor_test.go internal/listingkit/api/subscription_guard.go internal/listingkit/api/subscription_handler_platform.go internal/listingkit/api/member_invitation_handler.go internal/listingkit/api/subscription_handler_test.go
git commit -m "fix: trust authenticated context for ListingKit admin actors"
```

## Task 5: Build the read-only identity preflight core

**Files:**

- Create: `internal/listingkit/userdirectory/directory.go`
- Create: `internal/listingkit/userdirectory/directory_test.go`
- Create: `internal/listingkit/identitypreflight/inventory.go`
- Create: `internal/listingkit/identitypreflight/inventory_test.go`
- Create: `internal/listingkit/identitypreflight/repository.go`
- Create: `internal/listingkit/identitypreflight/repository_test.go`
- Create: `internal/listingkit/identitypreflight/preflight.go`
- Create: `internal/listingkit/identitypreflight/preflight_test.go`

### Step 1: Specify the user-directory boundary with HTTP tests

Define only the data needed by the preflight:

```go
type User struct {
    Subject  string
    TenantID string
}

type Directory interface {
    ListByTenant(ctx context.Context, tenantID string) ([]User, error)
}
```

Write `httptest.Server` tests proving the client:

- sends `POST /v2/users`;
- sends the bearer directory token without logging it;
- includes a list query with `limit`, string `offset`, ascending order, stable
  `USER_FIELD_NAME_CREATION_DATE` sorting, and an `organizationIdQuery`;
- decodes `result[].userId` and `result[].details.resourceOwner`;
- accepts `details.totalResult` in either JSON string or number form, as emitted
  by different protobuf-JSON gateway versions;
- continues pages until `details.totalResult` is exhausted;
- rejects a returned row whose `resourceOwner` differs from the requested
  tenant;
- returns a sanitized error for non-2xx responses and transport failures.

Use this request shape in the test contract:

```json
{
  "query": {"offset":"0","limit":100,"asc":true},
  "sortingColumn":"USER_FIELD_NAME_CREATION_DATE",
  "queries":[{"organizationIdQuery":{"organizationId":"tenant-101"}}]
}
```

The official ZITADEL response fields used are `userId` and
`details.resourceOwner`; keep the client aligned with the
[official Search Users contract](https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.ListUsers).
Do not decode email, profile, phone, or login names.

Run:

```powershell
go test ./internal/listingkit/userdirectory -count=1
```

Expected: FAIL because the package does not exist yet.

### Step 2: Implement the minimal paginated HTTP client

Mirror the repository's `tenantdirectory` construction rules:

```go
type ClientConfig struct {
    IssuerURL string
    Token string
    HTTPClient *http.Client
}
```

Normalize the issuer, require the read-only token, use `http.DefaultClient`
when absent, and make all error messages operation-oriented. For a non-2xx
response, report status and at most a bounded sanitized body; redact the token
and any exact requested identifiers before returning the error. Prefer omitting
the body entirely if safe redaction cannot be guaranteed.

### Step 3: Define the explicit owner-table inventory

Use a closed struct with hard-coded identifiers so no SQL identifier comes from
flags, config, or network input:

```go
type OwnerTable struct {
    Table string
    TenantColumn string
    UserColumn string
}
```

The initial inventory is:

```text
listing_kit_tasks                              tenant_id  user_id
listingkit_shein_pod_image_indexes             tenant_id  user_id
listingkit_studio_async_jobs                   tenant_id  user_id
listingkit_studio_batches                      tenant_id  user_id
listingkit_studio_batch_items                  tenant_id  user_id
listingkit_studio_generation_attempts          tenant_id  user_id
listingkit_studio_materialized_designs         tenant_id  user_id
listingkit_studio_batch_task_links              tenant_id  user_id
listingkit_studio_batch_runs                   tenant_id  user_id
listingkit_studio_batch_run_items              tenant_id  user_id
shein_studio_sessions                          tenant_id  user_id
listing_store                                  tenant_id  owner_user_id
listing_category                               tenant_id  owner_user_id
listing_filter_rule                            tenant_id  owner_user_id
listing_generation_topic_override              tenant_id  owner_user_id
listing_generation_topic_policy                tenant_id  owner_user_id
listing_product_import_task                     tenant_id  owner_user_id
listing_operation_strategy                     tenant_id  owner_user_id
listing_pricing_rule                            tenant_id  owner_user_id
listing_product_import_mapping                  tenant_id  owner_user_id
listing_profit_rule                             tenant_id  owner_user_id
listing_product_data                            tenant_id  owner_user_id
listing_scheduled_task_config                   tenant_id  owner_user_id
listing_sensitive_word                          tenant_id  owner_user_id
```

Do not include `shein_studio_designs`: its ownership is inherited from
`shein_studio_sessions` and it has no independent user column. Do not include
member-invitation audit target IDs or generic AI telemetry because neither is
used as an owner-scope filter.

### Step 4: Add a repository-structure guard for the inventory

In `inventory_test.go`, use Go AST parsing over the production model files, not
a brittle plain-text-only assertion. Collect structs that contain both:

- a GORM `tenant_id` column; and
- a GORM `user_id` or `owner_user_id` column.

Resolve their `TableName` string literals and compare the resulting set to the
inventory. Scan `internal/listingkit` and `internal/listingadmin`, excluding
`*_test.go`. Fail with explicit `missing from inventory` and `stale inventory`
sets. The test should deliberately document the exclusions above.

If a model relies on GORM's default table naming and has no `TableName` method,
add an explicit production `TableName` method rather than reimplementing GORM's
pluralization in the test.

### Step 5: Implement the read-only repository

Define:

```go
type PersistedOwner struct {
    Table string
    TenantID string
    UserID string
    RowCount int64
}

type OwnerRepository interface {
    List(ctx context.Context) ([]PersistedOwner, error)
}
```

For each hard-coded inventory entry, run a read-only aggregate:

```sql
SELECT CAST(tenant_id AS text) AS tenant_id,
       CAST(user_id AS text) AS user_id,
       COUNT(*) AS row_count
FROM listing_kit_tasks
WHERE NULLIF(BTRIM(CAST(tenant_id AS text)), '') IS NOT NULL
  AND NULLIF(BTRIM(CAST(user_id AS text)), '') IS NOT NULL
GROUP BY tenant_id, user_id
```

Generate the repeated statement only from inventory constants after validating
each identifier against `^[a-z][a-z0-9_]*$`. Skip a table only when PostgreSQL
reports that it does not exist; this permits environments that have not enabled
every optional module. Any other database error fails the preflight.

Use SQL-mock or focused GORM callback tests to prove aggregation, absent-table
handling, query cancellation, and no `INSERT`, `UPDATE`, `DELETE`, migration,
or transaction write is executed.

### Step 6: Implement comparison and redacted reporting

Define a preflight service taking `OwnerRepository`, `userdirectory.Directory`,
and `io.Writer`. Its algorithm must:

1. Group persisted owners by tenant.
2. Fetch each tenant directory exactly once.
3. Build a set keyed by `(tenantID, subject)`.
4. Emit one finding for every persisted owner absent from that tenant's set.
5. Return a typed `ErrUnknownOwners` when findings exist.

Fingerprint identifiers as:

```go
func fingerprint(value string) string {
    sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
    return "sha256:" + hex.EncodeToString(sum[:6])
}
```

Example safe output:

```text
status=blocked table=listing_store tenant=sha256:17d8b1c2e3f4 owner=sha256:49af778899aa rows=3 reason=unknown_subject
```

Tests must cover success, unknown owner, same subject in the wrong tenant,
directory failure, deterministic ordering, and redaction. The redaction test
must assert the output does not contain raw tenant, user ID, token, email, name,
or mocked response body.

### Step 7: Run core package tests

```powershell
go test ./internal/listingkit/userdirectory ./internal/listingkit/identitypreflight -count=1
```

Expected: PASS.

### Step 8: Commit the preflight core

```powershell
git add internal/listingkit/userdirectory internal/listingkit/identitypreflight
git commit -m "feat: add ListingKit identity ownership preflight"
```

## Task 6: Add a testable preflight CLI and package it in the API image

**Files:**

- Create: `internal/app/runtime/listingkitidentitypreflight/options.go`
- Create: `internal/app/runtime/listingkitidentitypreflight/options_test.go`
- Create: `internal/app/runtime/listingkitidentitypreflight/runtime.go`
- Create: `internal/app/runtime/listingkitidentitypreflight/runtime_test.go`
- Create: `cmd/listingkit-identity-preflight/main.go`
- Modify: `deployments/docker/Dockerfile.product-listing-api`

### Step 1: Write runtime dependency-injection tests

Follow the existing `listingkitschemamigrate` runtime pattern. Test:

- default config path and `-config` parsing;
- config load failure does not open the database;
- missing database fails before directory calls;
- missing issuer or directory token fails closed;
- the database is always closed;
- a core preflight failure is returned unchanged enough for the process to exit
  non-zero, without leaking credentials;
- success writes only the safe summary.

Run:

```powershell
go test ./internal/app/runtime/listingkitidentitypreflight -count=1
```

Expected: FAIL because the runtime does not exist.

### Step 2: Implement options and runtime wiring

Use `config.LoadConfigFromFile`, `database.NewDatabaseFromConfig`,
`userdirectory.NewClient`, and `identitypreflight.New`. Accept only operational
flags:

```text
-config     default config/config-prod.yaml
-log-level  default info
```

Read issuer and `TenantDirectoryToken` from the existing
`ListingKit.Zitadel` config. Do not add a new secret or accept a token on the
command line.

The `main` package should only parse options, attach build metadata, call
`Run(context.Background(), opts)`, and `log.Fatal` on error, matching existing
one-shot commands.

### Step 3: Add the binary to the existing image

In `Dockerfile.product-listing-api`:

- copy `cmd/listingkit-identity-preflight/` into the builder;
- build `/out/listingkit-identity-preflight` with the same CGO/GOOS/ldflags;
- copy it to `/app/listingkit-identity-preflight`;
- include it in the `chmod +x` list.

Do not create a second image or base image.

### Step 4: Verify runtime and image build

```powershell
go test ./internal/app/runtime/listingkitidentitypreflight -count=1
go build ./cmd/listingkit-identity-preflight
docker build -f deployments/docker/Dockerfile.product-listing-api -t task-processor/listingkit-api:identity-preflight-test .
docker run --rm --entrypoint /app/listingkit-identity-preflight task-processor/listingkit-api:identity-preflight-test -h
```

Expected: tests/build PASS; help exits without starting the API. Remove the
host `listingkit-identity-preflight.exe` artifact if `go build` created it in
the repository root; do not stage it.

### Step 5: Commit CLI packaging

```powershell
git add internal/app/runtime/listingkitidentitypreflight cmd/listingkit-identity-preflight deployments/docker/Dockerfile.product-listing-api
git commit -m "build: package ListingKit identity preflight"
```

## Task 7: Make the preflight a release gate

**Files:**

- Create: `deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml`
- Modify: `.github/workflows/listingkit-deploy.yml`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`
- Modify: `deployments/kubernetes/listingkit-workbench/base/secret.example.yaml`
- Modify: `docs/development/listingkit-local-debug.md`

### Step 1: Add a manifest/config boundary test before the manifest

Add the deployment assertions to the most relevant existing Go config test or
a new focused test under `internal/app/runtime/listingkitidentitypreflight`:

- Job command is `/app/listingkit-identity-preflight`.
- Job image contains `REPLACE_WITH_DEPLOYED_TAG`.
- Job uses only `listingkit-workbench-config` and the existing
  `listingkit-workbench-secret`.
- Job has `restartPolicy: Never`, `backoffLimit: 0`, and no service account with
  write privileges.
- workflow runs the Job after the image build and before `set image`.
- workflow waits for completion and prints logs on success or failure.

Run the test and confirm it fails because the gate is absent.

### Step 2: Create the one-shot Job

Model it after `listingkit-schema-migrate-job.yaml`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  generateName: listingkit-identity-preflight-
  labels:
    app: listingkit-identity-preflight
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 86400
  template:
    metadata:
      labels:
        app: listingkit-identity-preflight
    spec:
      restartPolicy: Never
      containers:
        - name: listingkit-identity-preflight
          image: docker.io/xuwei190/task-processor-product-listing-api:REPLACE_WITH_DEPLOYED_TAG
          imagePullPolicy: IfNotPresent
          command: ["/app/listingkit-identity-preflight"]
          args: ["-config", "/app/config/config-prod.yaml", "-log-level", "info"]
          envFrom:
            - configMapRef:
                name: listingkit-workbench-config
            - secretRef:
                name: listingkit-workbench-secret
                optional: false
```

The shared secret is required here because it carries both DB credentials and
the existing read-only directory token. Do not mount the dedicated member
invitation write token.

### Step 3: Insert the GitHub Actions release gate

In `deploy-api`, after applying runtime configuration and checking legacy
invitation secrets, but before updating the API deployment image:

1. Copy the Job manifest to `$RUNNER_TEMP`.
2. Replace `REPLACE_WITH_DEPLOYED_TAG` with `${{ needs.prepare.outputs.tag }}`.
3. Create it and capture the generated name.
4. Wait up to 15 minutes for completion.
5. Always print the Job logs when the wait succeeds or fails.
6. Exit non-zero on a failed/timeout Job so `kubectl set image` never runs.

Use a shell `trap` or explicit failure branch so logs are not lost:

```bash
if ! kubectl -n "$K8S_NAMESPACE" wait --for=condition=complete "job/$job_name" --timeout=15m; then
  kubectl -n "$K8S_NAMESPACE" logs "job/$job_name" || true
  kubectl -n "$K8S_NAMESPACE" describe "job/$job_name" || true
  exit 1
fi
kubectl -n "$K8S_NAMESPACE" logs "job/$job_name"
```

Do not apply the Job through base Kustomization; it is release-scoped and uses
`generateName`.

### Step 4: Document credentials, manual use, and rollback prerequisite

Update the Kubernetes README and local debug guide with:

- required ZITADEL permission: read access sufficient for `POST /v2/users` for
  every organization represented in the database;
- manual immutable-tag Job command;
- safe output format and the meaning of an unknown-subject blocker;
- explicit statement that the preflight never mutates data;
- API/UI coordinated release order;
- rollback allowed only after confirming `user_id` is absent or equals `sub`;
- no complete identifier or secret may be pasted into an issue or release log.

Update `secret.example.yaml` comments so the tenant directory token is clearly
shared with API and this preflight Job, but not UI, browser workers, imgproxy,
or schema migration Jobs.

### Step 5: Verify rendered release assets

```powershell
go test ./internal/app/runtime/listingkitidentitypreflight -count=1
kubectl create --dry-run=client -f deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml -o yaml | Out-Null
rg -n 'listingkit-identity-preflight|REPLACE_WITH_DEPLOYED_TAG' .github/workflows/listingkit-deploy.yml deployments/kubernetes/listingkit-workbench
```

Expected: PASS, and manual inspection confirms the preflight step precedes
`Update API deployment image`.

### Step 6: Commit the release gate

```powershell
git add deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml .github/workflows/listingkit-deploy.yml deployments/kubernetes/listingkit-workbench/README.md deployments/kubernetes/listingkit-workbench/base/secret.example.yaml docs/development/listingkit-local-debug.md internal/app/runtime/listingkitidentitypreflight
git commit -m "ci: gate ListingKit deploys on identity preflight"
```

## Task 8: Run cross-boundary verification and prepare review

**Files:**

- Modify only if verification exposes a real issue: files already listed above
- Create: `docs/superpowers/verification/2026-08-09-listingkit-canonical-zitadel-subject.md`

### Step 1: Run the complete focused Go suite

```powershell
go test ./internal/core/config ./internal/listingkit/... ./internal/listingadmin/... ./internal/app/runtime/listingkitidentitypreflight -count=1
```

Expected: PASS. If a package times out, rerun it alone with an adequate timeout;
do not report a timeout as success.

### Step 2: Run repository-wide Go checks

```powershell
go test ./... -count=1 -timeout=30m
go vet ./...
```

Expected: PASS. Record exact command, duration, and result.

### Step 3: Run the complete frontend suite

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- --run
npm.cmd run typecheck
npm.cmd run lint
npm.cmd run build
```

Expected: PASS. Return to repository root afterward.

### Step 4: Run static identity-fallback searches

From the repository root:

```powershell
rg -n 'user_id.*\?\?.*sub|UserID.*firstNonEmpty|businessUserId.*subject|GetHeader\("X-User-ID"\)' internal/listingkit web/listingkit-ui/src
rg -n 'ownerScopeRequired|OWNER_SCOPE_REQUIRED' config deployments docs/development internal/core/config
```

Expected:

- no ownership fallback from `user_id` or username to `sub`;
- no security-sensitive actor/authorization read of `X-User-ID`;
- no owner-scope runtime configuration;
- any remaining compatibility header read is individually inspected and
  documented as non-authoritative.

### Step 5: Rebuild and inspect deployment artifacts

```powershell
docker build -f deployments/docker/Dockerfile.product-listing-api -t task-processor/listingkit-api:canonical-subject-verify .
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod | Out-File -Encoding utf8 $env:TEMP\listingkit-canonical-subject-rendered.yaml
kubectl create --dry-run=client -f deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml -o yaml | Out-Null
```

Inspect the rendered manifest for accidental owner-scope configuration and
credential spread. Delete only the temporary rendered file after resolving its
absolute path under `$env:TEMP`; do not delete repository files.

### Step 6: Record evidence without claiming production acceptance

Create the verification note with:

- commits under review;
- exact local test/build results;
- static-search results;
- image build result;
- manifest validation result;
- remaining external gates: target-environment preflight, coordinated API/UI
  rollout, real-token role matrix, and audited platform mutation.

Do not include raw tenant IDs, subjects, tokens, or PII. State clearly that local
verification is not deployment/runtime/business acceptance.

### Step 7: Request code review

Use `superpowers:requesting-code-review` and address only evidence-backed,
actionable findings. Re-run the relevant test after every correction.

### Step 8: Commit verification evidence

```powershell
git add docs/superpowers/verification/2026-08-09-listingkit-canonical-zitadel-subject.md
git commit -m "docs: record canonical identity verification"
git status --short
```

Expected: clean worktree. Stop before push, PR creation, deployment, or merge
unless the user explicitly authorizes those actions.

---

## Definition of done

- Every protected Go request uses introspected `sub` as `AuthenticatedIdentity.UserID` and rejects missing `sub`.
- Auth.js and server token verification use one subject-only extractor.
- `ownerScopeRequired` and both environment variable names are absent from production configuration and docs.
- Platform-admin authorization and security-sensitive audit actors read authenticated context, never request headers.
- The preflight inventories every direct ListingKit/ListingAdmin owner-scoped table, is read-only, paginates ZITADEL users per tenant, redacts identifiers, and fails on unknown or cross-tenant subjects.
- The API image contains the preflight binary and the deployment workflow runs its one-shot Job before changing the API image.
- Focused tests, full Go tests, frontend tests/typecheck/lint/build, Docker build, and manifest validation pass.
- No push, PR, deployment, or merge occurs without separate authorization.
