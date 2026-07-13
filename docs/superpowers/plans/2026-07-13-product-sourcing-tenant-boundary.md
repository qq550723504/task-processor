# Product Sourcing Tenant Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox ( - [ ] ) syntax for tracking.

**Goal:** Make the 1688-to-ListingKit task endpoint authorize verified identities and reject tenant, identity, or store-boundary violations before task creation.

**Architecture:** The shared ZITADEL route policy recognizes product-sourcing and a dedicated product_sourcing.write permission. The HTTP handler builds a ListingKit tenant/request identity context only from middleware identity. The 1688 command service verifies that identity, resolves its legacy tenant ID, and uses the existing tenant-scoped listingadmin.StoreRepository to require an enabled 1688 source store and SHEIN target store before invoking task creation.

**Tech Stack:** Go, Gin, Casbin, ZITADEL token introspection middleware, GORM-backed listingadmin.StoreRepository, Testify.

## Global Constraints

- Do not create a crawler credential table or a duplicate store ownership model.
- source_store_id and shein_store_id are listing_store.id values; each must be owned by the verified tenant, have Status == 0, and use platforms 1688 and SHEIN respectively, case-insensitively.
- Do not accept tenant_id or user_id in the public JSON contract, and do not trust client-provided identity headers.
- Keep store authorization in internal/product/sourcehandoff/a1688 so future callers receive the same guard.
- A cross-tenant store is reported as unavailable and its existence is never disclosed.
- Preserve valid 1688 envelope-to-ListingKit task behavior.

---

## File structure

| File | Responsibility |
| --- | --- |
| internal/authz/listingkit.go | Declare and grant the Product Sourcing write permission. |
| internal/listingkit/httpapi/zitadel_auth_route_authorization.go | Mark Product Sourcing as authenticated and resolve descriptor permissions. |
| internal/productenrich/httpapi/sourcea1688/routes.go | Declare the explicit write permission. |
| internal/productenrich/httpapi/sourcea1688/handler.go | Create commands and context from verified identity only. |
| internal/product/sourcehandoff/a1688/command.go | Validate identity consistency and tenant-scoped stores. |
| internal/listingkit/httpapi/bootstrap_contracts.go | Expose the already-built store repository. |
| internal/listingkit/httpapi/bootstrap_runtime.go | Populate the module store repository. |
| internal/app/httpapi/composition_builder.go | Inject the shared store repository into the command service. |

### Task 1: Declare and enforce Product Sourcing write permission

**Files:**

- Modify: internal/authz/listingkit.go:11-79
- Modify: internal/listingkit/httpapi/zitadel_auth_route_authorization.go:13-37
- Modify: internal/productenrich/httpapi/sourcea1688/routes.go:1-21
- Modify: internal/authz/listingkit_test.go
- Modify: internal/listingkit/httpapi/zitadel_auth_test.go
- Modify: internal/productenrich/httpapi/sourcea1688/handler_test.go:97-111

**Interfaces:**

- Produces: authz.PermissionProductSourcingWrite == "product_sourcing.write".
- Produces: the create descriptor has Module "product-sourcing" and Permission authz.PermissionProductSourcingWrite.
- Produces: RouteRequiresZitadelAuth and NewRouteRoleMiddleware apply token and permission checks to this route.

- [ ] **Step 1: Write failing authorization tests**

Add this policy test to internal/authz/listingkit_test.go:

~~~
func TestListingKitAuthorizerAllowsOperationalRolesToWriteProductSourcing(t *testing.T) {
    authorizer, err := NewListingKitAuthorizer(nil, nil)
    require.NoError(t, err)
    require.True(t, authorizer.Authorize("", []string{"listingkit_operator"}, PermissionProductSourcingWrite))
    require.True(t, authorizer.Authorize("", []string{"listingkit_admin"}, PermissionProductSourcingWrite))
    require.True(t, authorizer.Authorize("", []string{"platform_admin"}, PermissionProductSourcingWrite))
    require.False(t, authorizer.Authorize("", []string{"viewer"}, PermissionProductSourcingWrite))
}
~~~

In internal/listingkit/httpapi/zitadel_auth_test.go, use existing newZitadelRoleServer and mountRoutes to mount a POST descriptor for product-sourcing with the new permission. Assert missing bearer token and an empty-role token both fail (401 and 403); a listingkit_operator token succeeds. In internal/productenrich/httpapi/sourcea1688/handler_test.go extend TestAppendRouteDescriptorsIncludesCreateRoute with:

~~~
require.Equal(t, authz.PermissionProductSourcingWrite, routes[0].Permission)
~~~

- [ ] **Step 2: Run test to verify it fails**

Run:

~~~
go test ./internal/authz ./internal/listingkit/httpapi ./internal/productenrich/httpapi/sourcea1688
~~~

Expected: FAIL because PermissionProductSourcingWrite and the descriptor permission do not exist, and product-sourcing is not protected.

- [ ] **Step 3: Implement the smallest route policy**

In internal/authz/listingkit.go add the constant and the following policy rows while retaining existing policies:

~~~
PermissionProductSourcingWrite = "product_sourcing.write"

{"listingkit_operator", PermissionProductSourcingWrite},
{"listingkit_admin", PermissionProductSourcingWrite},
{"platform_admin", PermissionProductSourcingWrite},
~~~

In internal/listingkit/httpapi/zitadel_auth_route_authorization.go add product-sourcing to RouteRequiresZitadelAuth. Keep the existing non-empty route.Permission branch as the first permission lookup. In internal/productenrich/httpapi/sourcea1688/routes.go import internal/authz and set:

~~~
Permission: authz.PermissionProductSourcingWrite,
~~~

- [ ] **Step 4: Run test to verify it passes**

Run:

~~~
go test ./internal/authz ./internal/listingkit/httpapi ./internal/productenrich/httpapi/sourcea1688
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~
git add internal/authz/listingkit.go internal/authz/listingkit_test.go internal/listingkit/httpapi/zitadel_auth_route_authorization.go internal/listingkit/httpapi/zitadel_auth_test.go internal/productenrich/httpapi/sourcea1688/routes.go internal/productenrich/httpapi/sourcea1688/handler_test.go
git commit -m "fix: protect product sourcing task route"
~~~

### Task 2: Bind handler commands to verified identity

**Files:**

- Modify: internal/productenrich/httpapi/sourcea1688/handler.go:25-114
- Modify: internal/productenrich/httpapi/sourcea1688/handler_test.go:19-95

**Interfaces:**

- Consumes: middleware-overwritten X-Tenant-ID and X-User-ID values.
- Produces: no TenantID or UserID JSON fields in CreateListingKitTaskRequest.
- Produces: the command and its context have equal listingkit.RequestIdentity and tenant scope.

- [ ] **Step 1: Write failing handler tests**

Post JSON containing tenant_id: "attacker-tenant" and user_id: "attacker-user", while the request headers contain X-Tenant-ID: "verified-tenant" and X-User-ID: "verified-user". Assert the fake service captures verified values and that its context reports both:

~~~
listingkit.TenantIDFromContext(ctx) == "verified-tenant"
listingkit.RequestIdentityFromContext(ctx) == listingkit.RequestIdentity{
    TenantID: "verified-tenant", UserID: "verified-user",
}
~~~

Add a no-identity-header test expecting HTTP 400 and asserting the fake service is not called.

- [ ] **Step 2: Run test to verify it fails**

Run:

~~~
go test ./internal/productenrich/httpapi/sourcea1688 -run TestCreateListingKitTask
~~~

Expected: FAIL because request JSON currently controls tenant and user, no identity context is attached, and a missing identity reaches the service.

- [ ] **Step 3: Implement verified identity binding**

Remove TenantID and UserID from CreateListingKitTaskRequest. Add this helper to handler.go:

~~~
func verifiedRequestContext(c *gin.Context) (context.Context, listingkit.RequestIdentity, error) {
    if c == nil || c.Request == nil {
        return nil, listingkit.RequestIdentity{}, errors.New("verified request identity is required")
    }
    identity := listingkit.RequestIdentity{
        TenantID: strings.TrimSpace(c.GetHeader("X-Tenant-ID")),
        UserID:   strings.TrimSpace(c.GetHeader("X-User-ID")),
    }
    if identity.TenantID == "" || identity.UserID == "" {
        return nil, listingkit.RequestIdentity{}, errors.New("verified request identity is required")
    }
    ctx := listingkit.WithTenantID(c.Request.Context(), identity.TenantID)
    return listingkit.WithRequestIdentity(ctx, identity), identity, nil
}
~~~

Call it before CreateTask. On error return HTTP 400. Change toCommand to accept listingkit.RequestIdentity and always assign command.TenantID and command.UserID from it; leave only source and task business fields in JSON.

- [ ] **Step 4: Run test to verify it passes**

Run:

~~~
go test ./internal/productenrich/httpapi/sourcea1688
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~
git add internal/productenrich/httpapi/sourcea1688/handler.go internal/productenrich/httpapi/sourcea1688/handler_test.go
git commit -m "fix: derive product sourcing identity from auth"
~~~

### Task 3: Enforce context consistency and tenant-scoped store ownership

**Files:**

- Modify: internal/product/sourcehandoff/a1688/command.go:11-92
- Modify: internal/product/sourcehandoff/a1688/command_test.go:1-106

**Interfaces:**

- Consumes: listingkit.TenantIDFromContext, listingkit.RequestIdentityFromContext, tenantbridge.ResolveLegacyTenantID, and listingadmin.StoreRepository.GetStore.
- Produces: NewTaskCommandService(creator sourcehandoff.GenerateTaskCreator, stores storeLookup) *TaskCommandService.
- Produces: ErrUnauthenticatedIdentity and ErrStoreUnavailable safe sentinel errors.

- [ ] **Step 1: Write failing service tests**

Create a fake store repository keyed by "tenantID:storeID"; only GetStore needs a real implementation. Add:

~~~
func authenticatedCommandContext(tenantID, userID string) context.Context {
    ctx := listingkit.WithTenantID(context.Background(), tenantID)
    return listingkit.WithRequestIdentity(ctx, listingkit.RequestIdentity{
        TenantID: tenantID, UserID: userID,
    })
}

func TestTaskCommandServiceRejectsMismatchedContextIdentity(t *testing.T) {
    service := NewTaskCommandService(&fakeGenerateTaskCreator{}, fakeStoreRepository{})
    _, err := service.CreateTask(authenticatedCommandContext("101", "verified-user"), CreateTaskCommand{
        TenantID: "202", UserID: "verified-user", URL: "https://detail.1688.com/offer/1.html",
    })
    require.ErrorIs(t, err, ErrUnauthenticatedIdentity)
}
~~~

Add table rows for missing context identity, mismatched user, missing source store, an item only available to tenant 202, source platform other than 1688, disabled source store, missing target store, target platform other than SHEIN, disabled target store, and a valid 1688/SHEIN pair. Every failure must assert fakeGenerateTaskCreator.request is nil.

- [ ] **Step 2: Run test to verify it fails**

Run:

~~~
go test ./internal/product/sourcehandoff/a1688 -run TestTaskCommandService
~~~

Expected: FAIL because the command service has no store dependency and does not compare identity context to command identity.

- [ ] **Step 3: Implement the application guard**

In command.go add imports for errors, internal/listingadmin, internal/tenantbridge, and retain strings. Add:

~~~
var (
    ErrUnauthenticatedIdentity = errors.New("verified request identity is required")
    ErrStoreUnavailable        = errors.New("requested store is unavailable")
)

type storeLookup interface {
    GetStore(context.Context, int64, int64) (*listingadmin.Store, error)
}

type TaskCommandService struct {
    creator sourcehandoff.GenerateTaskCreator
    stores  storeLookup
}

func NewTaskCommandService(creator sourcehandoff.GenerateTaskCreator, stores storeLookup) *TaskCommandService {
    return &TaskCommandService{creator: creator, stores: stores}
}
~~~

Before URL or envelope construction, call validateRequestScope:

~~~
func (s *TaskCommandService) validateRequestScope(ctx context.Context, command CreateTaskCommand) error {
    tenantID := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
    identity := listingkit.RequestIdentityFromContext(ctx)
    if tenantID == "" || identity.TenantID != tenantID || identity.UserID == "" ||
        strings.TrimSpace(command.TenantID) != tenantID || strings.TrimSpace(command.UserID) != identity.UserID {
        return ErrUnauthenticatedIdentity
    }
    legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, tenantID)
    if err != nil || legacyTenantID <= 0 || s.stores == nil {
        return ErrStoreUnavailable
    }
    if err := validateStore(ctx, s.stores, legacyTenantID, command.SourceStoreID, "1688"); err != nil {
        return err
    }
    return validateStore(ctx, s.stores, legacyTenantID, command.SheinStoreID, "SHEIN")
}

func validateStore(ctx context.Context, stores storeLookup, tenantID, storeID int64, platform string) error {
    if storeID <= 0 {
        return ErrStoreUnavailable
    }
    store, err := stores.GetStore(ctx, tenantID, storeID)
    if err != nil || store == nil || store.TenantID != tenantID || store.Status != 0 ||
        !strings.EqualFold(strings.TrimSpace(store.Platform), platform) {
        return ErrStoreUnavailable
    }
    return nil
}
~~~

Keep URL fallback, source envelope conversion, and creator delegation unchanged. Update current success tests to use authenticatedCommandContext and a valid fake store pair.

- [ ] **Step 4: Run test to verify it passes**

Run:

~~~
go test ./internal/product/sourcehandoff/a1688
~~~

Expected: PASS.

- [ ] **Step 5: Commit**

~~~
git add internal/product/sourcehandoff/a1688/command.go internal/product/sourcehandoff/a1688/command_test.go
git commit -m "fix: validate product sourcing store ownership"
~~~

### Task 4: Wire the existing store repository and verify composed errors

**Files:**

- Modify: internal/listingkit/httpapi/bootstrap_contracts.go:23-28
- Modify: internal/listingkit/httpapi/bootstrap_runtime.go:76-82
- Modify: internal/app/httpapi/composition_builder.go:89-93
- Modify: internal/app/httpapi/composition_builder_test.go:104-171
- Modify: internal/productenrich/httpapi/sourcea1688/handler.go:63-114
- Modify: internal/productenrich/httpapi/sourcea1688/handler_test.go:44-95

**Interfaces:**

- Consumes: listingkithttpapi.Module.StoreRepository built once by ListingKit.
- Produces: Product Sourcing only exists if task lifecycle and store repository are both non-nil.
- Produces: safe HTTP 400 for both guard errors, without handoff details or task creation.

- [ ] **Step 1: Write failing composition and error tests**

In the composition test, return a non-nil fake listingadmin.StoreRepository in listingkithttpapi.Module and assert Product Sourcing is built. Add a test where TaskLifecycleService is set but StoreRepository is nil and assert productSourcingModule is nil.

In handler tests return a1688.ErrStoreUnavailable and a1688.ErrUnauthenticatedIdentity from fakeTaskCommandService. Each response must be HTTP 400, contain the safe error, omit source_identity/source_warnings, and receive no task result.

- [ ] **Step 2: Run test to verify it fails**

Run:

~~~
go test ./internal/app/httpapi ./internal/productenrich/httpapi/sourcea1688
~~~

Expected: FAIL because Module does not expose the repository, composition uses a one-argument constructor, and the handler does not classify guard errors.

- [ ] **Step 3: Implement one-time wiring and error classification**

Add the repository to the ListingKit module and runtime return:

~~~
type Module struct {
    Handler              RouteHandler
    StudioSessionHandler listingkit.StudioSessionHandler
    TaskLifecycleService listingkit.TaskLifecycleService
    StoreRepository      listingadmin.StoreRepository
    Pool                 worker.WorkerPool
    Closers              []func() error
}

// in build runtime return:
StoreRepository: repositories.storeRepository,
~~~

Require and pass it in composition:

~~~
if composition.listingKitModule != nil &&
    composition.listingKitModule.TaskLifecycleService != nil &&
    composition.listingKitModule.StoreRepository != nil {
    composition.productSourcingModule = sourcea1688httpapi.BuildModule(
        a1688handoff.NewTaskCommandService(
            composition.listingKitModule.TaskLifecycleService,
            composition.listingKitModule.StoreRepository,
        ),
    )
}
~~~

At the start of isBadRequestError add:

~~~
if errors.Is(err, a1688.ErrUnauthenticatedIdentity) || errors.Is(err, a1688.ErrStoreUnavailable) {
    return true
}
~~~

- [ ] **Step 4: Run focused tests to verify they pass**

Run:

~~~
go test ./internal/app/httpapi ./internal/productenrich/httpapi/sourcea1688 ./internal/product/sourcehandoff/a1688
~~~

Expected: PASS.

- [ ] **Step 5: Run PR 0 verification**

Run:

~~~
gofmt -w internal/authz/listingkit.go internal/authz/listingkit_test.go internal/listingkit/httpapi/zitadel_auth_route_authorization.go internal/listingkit/httpapi/zitadel_auth_test.go internal/listingkit/httpapi/bootstrap_contracts.go internal/listingkit/httpapi/bootstrap_runtime.go internal/app/httpapi/composition_builder.go internal/app/httpapi/composition_builder_test.go internal/productenrich/httpapi/sourcea1688/routes.go internal/productenrich/httpapi/sourcea1688/handler.go internal/productenrich/httpapi/sourcea1688/handler_test.go internal/product/sourcehandoff/a1688/command.go internal/product/sourcehandoff/a1688/command_test.go
go test ./internal/authz ./internal/listingkit/httpapi ./internal/productenrich/httpapi/sourcea1688 ./internal/product/sourcehandoff/a1688 ./internal/app/httpapi
git diff --check
~~~

Expected: all listed tests pass and git diff --check produces no output.

- [ ] **Step 6: Commit**

~~~
git add internal/listingkit/httpapi/bootstrap_contracts.go internal/listingkit/httpapi/bootstrap_runtime.go internal/app/httpapi/composition_builder.go internal/app/httpapi/composition_builder_test.go internal/productenrich/httpapi/sourcea1688/handler.go internal/productenrich/httpapi/sourcea1688/handler_test.go
git commit -m "fix: wire product sourcing store guard"
~~~

## Final review checklist

- [ ] Run go test ./... from the final candidate SHA; preserve its raw result as closure evidence without mixing unrelated fixes into this PR.
- [ ] In a configured ZITADEL test or staging environment, test no token, non-writer token, forged JSON identity, forged pre-middleware identity headers, a cross-tenant store ID, and a valid owned 1688/SHEIN pair.
- [ ] Confirm the valid smoke creates a ListingKit task but does not submit or publish a marketplace product.

