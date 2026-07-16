# PAY-012 Tenant Store Ownership Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce authenticated-tenant ownership, platform compatibility, and enabled status for every ListingKit SHEIN or 1688 store before task persistence or remote action.

**Architecture:** Define a package-neutral `StoreAccessValidator` in `internal/listingkit`, implemented by the existing `listingadmin.StoreRepository` at the HTTP composition boundary. Inject it into task creation, SHEIN runtime/sync, and 1688 handoff. A task snapshot records selection only; remote actions revalidate current access before constructing a client or reading credentials.

**Tech Stack:** Go, Gin, `listingadmin.StoreRepository`, ListingKit service wiring, Testify.

## Global Constraints

- Reuse `listingadmin.StoreRepository`; add no store table, migration, token store, entitlement, billing, or object-storage behavior.
- The validator accepts only the PAY-011 trusted tenant context plus `storeID` and expected platform.
- Missing, foreign, and wrong-platform stores expose only `listingkit_store_unavailable`; disabled same-tenant stores expose `listingkit_store_disabled`.
- No rejected store may cause a task write, sync-job write, client construction, cookie read/refresh, cache access, or remote call.
- Platform-admin cross-tenant store operations and PAY-031 submission snapshot policy are out of scope.

---

### Task 1: Add the shared store-access contract and adapter

**Files:**
- Create: `internal/listingkit/store_access.go`
- Create: `internal/listingkit/store_access_test.go`
- Modify: `internal/listingkit/service_types.go`
- Modify: `internal/listingkit/httpapi/runtime_support_store_catalog.go`
- Modify: `internal/listingkit/httpapi/bootstrap_service_config.go`
- Modify: `internal/listingkit/httpapi/bootstrap_contracts.go`
- Modify: `internal/listingkit/httpapi/bootstrap_runtime.go`
- Modify: `internal/listingkit/httpapi/runtime_support_store_catalog_test.go`

**Interfaces:** Produces `StoreAccessValidator`, `StoreAccess`, `StoreAccessError`, and `StoreAccessErrorCode(error) string`; the production adapter is the only code that imports `listingadmin.StoreRepository`.

- [ ] **Step 1: Write failing contract and adapter tests**

```go
func TestStoreAccessErrorCodeHidesForeignStore(t *testing.T) {
	err := NewStoreAccessError(StoreAccessUnavailable, "store is unavailable")
	require.Equal(t, "listingkit_store_unavailable", StoreAccessErrorCode(err))
}

func TestListingAdminStoreAccessValidatorRejectsForeignDisabledAndWrongPlatform(t *testing.T) {
	validator := listingAdminStoreAccessValidator{repo: stores}
	_, err := validator.ValidateStoreAccess(ctx, 101, 202, "SHEIN")
	require.Equal(t, listingkit.StoreAccessUnavailable, listingkit.StoreAccessErrorCode(err))
}
```

- [ ] **Step 2: Verify the tests fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi -run 'Test(StoreAccess|ListingAdminStoreAccess)' -count=1`

Expected: compile failure because the contract does not exist.

- [ ] **Step 3: Implement the neutral contract and adapter**

```go
type StoreAccess struct { ID, TenantID int64; Platform string; Enabled bool }
type StoreAccessValidator interface {
	ValidateStoreAccess(context.Context, int64, int64, string) (StoreAccess, error)
}
const (
	StoreAccessUnavailable = "listingkit_store_unavailable"
	StoreAccessDisabled = "listingkit_store_disabled"
	StoreAccessStale = "listingkit_store_snapshot_stale"
)
```

The HTTP adapter calls `repo.GetStore(ctx, tenantID, storeID)`, rejects nil records, tenant mismatch, and case-insensitive platform mismatch as unavailable, and rejects `Status != 0` as disabled. It returns a sanitized copied record. Construct it once in the ListingKit HTTP module, wire it through `ListingKitServiceConfig.Shein`, and expose that same interface on the module result for source-handoff composition; do not import `listingadmin` from the ListingKit root.

- [ ] **Step 4: Verify the shared boundary**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi -run 'Test(StoreAccess|ListingAdminStoreAccess|BuildListingKitServiceConfig)' -count=1`

Expected: PASS; production wiring contains the validator and the three rejection modes are stable.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/store_access.go internal/listingkit/store_access_test.go internal/listingkit/service_types.go internal/listingkit/httpapi/runtime_support_store_catalog.go internal/listingkit/httpapi/bootstrap_service_config.go internal/listingkit/httpapi/bootstrap_contracts.go internal/listingkit/httpapi/bootstrap_runtime.go internal/listingkit/httpapi/runtime_support_store_catalog_test.go
git commit -m "security: add tenant store access validator"
```

### Task 2: Validate SHEIN stores before task persistence

**Files:**
- Modify: `internal/listingkit/task_lifecycle_service.go`
- Modify: `internal/listingkit/task_lifecycle_service_support.go`
- Modify: `internal/listingkit/service_task_wiring.go`
- Modify: `internal/listingkit/task_lifecycle_service_test.go`
- Modify: `internal/listingkit/api/handler_tasks.go`
- Modify: `internal/listingkit/api/handler_tasks_test.go`

**Interfaces:** `taskLifecycleServiceConfig` consumes `validateSheinStoreAccess func(context.Context, int64, int64) error`. `prepareGenerateTask` calls it before task construction; the API maps `StoreAccessError` to its stable JSON `error` code.

- [ ] **Step 1: Write failing lifecycle and handler tests**

Create a request for tenant `101`, platform `shein`, and store `202`; configure the injected validator to return `StoreAccessUnavailable`. Assert `CreateGenerateTask` returns that code and the repository did not receive `CreateTask`. Add a valid-store control case. In the authenticated handler test assert `403` and `{"error":"listingkit_store_unavailable"}` without raw repository detail.

- [ ] **Step 2: Verify the tests fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/api -run 'Test(CreateGenerateTask.*Store|GenerateListingKit.*Store)' -count=1`

Expected: a foreign store can create a task or yields generic `task_creation_failed`.

- [ ] **Step 3: Implement pre-persistence validation**

After defaults and `validateRequest`, but before constructing a task, validate only a SHEIN request with `SheinStoreID > 0`. Preserve non-SHEIN/no-store behavior. On failure do not snapshot, persist, or dispatch. Add one API error writer that recognizes `StoreAccessErrorCode` and returns a stable code plus a non-sensitive next-step message.

- [ ] **Step 4: Verify task creation boundaries**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/api -count=1`

Expected: PASS; valid stores retain behavior and invalid stores have zero task writes.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/task_lifecycle_service.go internal/listingkit/task_lifecycle_service_support.go internal/listingkit/service_task_wiring.go internal/listingkit/task_lifecycle_service_test.go internal/listingkit/api/handler_tasks.go internal/listingkit/api/handler_tasks_test.go
git commit -m "security: validate shein stores before task creation"
```

### Task 3: Use the shared validator in 1688 handoff

**Files:**
- Modify: `internal/product/sourcehandoff/a1688/command.go`
- Modify: `internal/product/sourcehandoff/a1688/command_test.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_builder_test.go`
- Modify: `internal/productenrich/httpapi/sourcea1688/handler.go`
- Modify: `internal/productenrich/httpapi/sourcea1688/handler_test.go`

**Interfaces:** `a1688.NewTaskCommandService` consumes `listingkit.StoreAccessValidator`, validates source `1688` then target `SHEIN`, and calls its task creator only after both pass.

- [ ] **Step 1: Write failing handoff tests**

Replace `testStoreLookup` with a validator fake. Cover foreign source, foreign target, wrong source platform, and disabled target; every case asserts no `CreateTask` command. Add an HTTP test with authenticated identity and a rejected source store expecting `listingkit_store_unavailable`.

- [ ] **Step 2: Verify the tests fail**

Run: `$env:GOWORK='off'; go test ./internal/product/sourcehandoff/a1688 ./internal/productenrich/httpapi/sourcea1688 -run 'Test.*(Store|ListingKitTask)' -count=1`

Expected: the private lookup is still used and the handler returns its old generic error.

- [ ] **Step 3: Replace the private lookup**

Remove `storeLookup` and the `listingadmin` import from the handoff package. Derive the numeric tenant from trusted context and call the validator for `SourceStoreID, "1688"` and `SheinStoreID, "SHEIN"`; a missing validator remains fail-closed. Pass the configured ListingKit validator from composition instead of constructing another repository path. Map only recognized store errors to stable client responses.

- [ ] **Step 4: Verify handoff and composition**

Run: `$env:GOWORK='off'; go test ./internal/product/sourcehandoff/a1688 ./internal/productenrich/httpapi/sourcea1688 ./internal/app/httpapi -count=1`

Expected: PASS; source and target ownership are checked before task creation.

- [ ] **Step 5: Commit**

```powershell
git add internal/product/sourcehandoff/a1688/command.go internal/product/sourcehandoff/a1688/command_test.go internal/app/httpapi/composition_builder.go internal/app/httpapi/composition_builder_test.go internal/productenrich/httpapi/sourcea1688/handler.go internal/productenrich/httpapi/sourcea1688/handler_test.go
git commit -m "security: validate source handoff store ownership"
```

### Task 4: Revalidate stores before SHEIN submission and sync

**Files:**
- Modify: `internal/listingkit/service_submit_runtime_context_resolver.go`
- Modify: `internal/listingkit/service_submit_wiring_resolution_support.go`
- Modify: `internal/listingkit/service_shein_store_client_test.go`
- Modify: `internal/listingkit/service_submit_store_context_test.go`
- Modify: `internal/listingkit/httpapi/shein_sync_runtime_bridge_helpers.go`
- Modify: `internal/listingkit/httpapi/shein_sync_runtime_bridge_helpers_test.go`
- Modify: `internal/listingkit/api/shein_sync_handler_products.go`
- Modify: `internal/listingkit/api/shein_sync_handler_test.go`

**Interfaces:** Runtime store resolution validates before returning `SheinStoreInfo` or constructing a client. A persisted snapshot that fails current validation returns `listingkit_store_snapshot_stale`. Sync validates before pending-job persistence or product API creation.

- [ ] **Step 1: Write failing runtime and sync tests**

Create a task snapshot naming store `901`; configure the validator to return unavailable or disabled. Assert `newSheinAPIClient` returns the stable code and the API-client factory was never called. Add a catalog result with `TenantID: 202` for requested tenant `101`, which must fail stale validation. For `TriggerSheinStoreSync` and `SyncSheinSourceSDSProduct`, reject the route store and assert no job is saved, no product API is built, and the HTTP error is stable.

- [ ] **Step 2: Verify the tests fail**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/listingkit/api -run 'Test.*(StoreSnapshot|StoreAccess|SheinStoreSync)' -count=1`

Expected: snapshot-backed submission and sync reach downstream factories without current store validation.

- [ ] **Step 3: Fail closed before remote dependencies**

Derive trusted numeric tenant and resolved store ID, validate access, then require the catalog result to match tenant, ID, platform `SHEIN`, and enabled status. If a snapshot existed, convert recognized validation failure to `StoreAccessStale`; never fall back to the request or profile. In the sync runtime bridge validate before saving a job or resolving product APIs. Map recognized errors in the two public sync handlers and preserve malformed-path and unrelated error behavior.

- [ ] **Step 4: Verify submission and sync boundaries**

Run: `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/listingkit/api -count=1`

Expected: PASS; rejected stores cannot reach cache, cookies, clients, jobs, or remote SHEIN calls.

- [ ] **Step 5: Commit**

```powershell
git add internal/listingkit/service_submit_runtime_context_resolver.go internal/listingkit/service_submit_wiring_resolution_support.go internal/listingkit/service_shein_store_client_test.go internal/listingkit/service_submit_store_context_test.go internal/listingkit/httpapi/shein_sync_runtime_bridge_helpers.go internal/listingkit/httpapi/shein_sync_runtime_bridge_helpers_test.go internal/listingkit/api/shein_sync_handler_products.go internal/listingkit/api/shein_sync_handler_test.go
git commit -m "security: revalidate stores before shein actions"
```

## Final verification

- [ ] Run `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/httpapi ./internal/product/sourcehandoff/a1688 ./internal/productenrich/httpapi/sourcea1688 ./internal/app/httpapi -count=1`.
- [ ] Run `go test ./tests/... -count=1`.
- [ ] Run `Set-Location web/listingkit-ui; npm run lint; npm run typecheck; npm test`.
- [ ] Run `git diff origin/master...HEAD --check`; confirm no migration, billing, entitlement, or object-storage changes.
- [ ] Update the PAY-012 checkbox and dated validation evidence in `docs/product/listingkit-paid-pilot-execution-plan.md` only after production-path tests pass.
