# Store Center Vertical Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a real, Organization-isolated `店铺中心 / 我的店铺` slice with list, filters, pagination, create, edit, enable, disable, controlled delete, optimistic concurrency, quota enforcement, audit, and honest platform-connection state.

**Architecture:** Create a new `storecenter` bounded context keyed by opaque ZITADEL Organization IDs and UUID store IDs. Do not reuse the legacy numeric-tenant `listing_store` aggregate. Store quota remains owned by `listingsubscription` through a dedicated idempotent allocation ledger; the Store Center application service coordinates the cross-module reserve/create/commit state machine. Every route runs after the workbench effective-organization resolver from the prerequisite plan.

**Tech Stack:** Go, Gin, GORM, PostgreSQL/SQLite tests, UUID, existing ListingKit subscription domain, Next.js 16 App Router, React 19, TanStack Query 5, React Hook Form, Zod, TypeScript, Vitest, Testing Library.

**Spec:** `docs/superpowers/specs/2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md`

**Prerequisite:** `docs/superpowers/plans/2026-08-30-zitadel-multi-organization-workbench-context.md` is complete, including real ZITADEL acceptance.

## Global Constraints

- Do not read from, write to, migrate, dual-write, or backfill `listing_store` for the new workbench.
- Do not call `tenantbridge`; `organization_id` is always the exact effective ZITADEL Organization ID.
- Do not persist a store username, password, bearer token, cookie, refresh token, or other provider secret in `workbench_stores`, API responses, audit payloads, or logs.
- A `connection_ref` is an opaque reference only. When it is empty, `disconnected` is the real status; do not fabricate `connected` demo data.
- The current SHEIN login implementation is keyed by numeric tenant/store IDs and cannot safely serve the new model. This slice may expose honest `disconnected` state and a disabled “connect” action; adapting provider authorization to opaque IDs requires a separately approved slice because third-party login-flow rewriting is explicitly out of scope.
- Every repository operation includes `organization_id`. Cross-Organization lookup returns the same not-found result as an unknown UUID.
- Every write uses live ZITADEL grant verification through the prerequisite resolver. Reads use its bounded cache policy.
- Store limit is `store_count` under `store_management`. All non-deleted stores, including disabled and transitional stores, consume one allocation.
- Never enforce quota with an unlocked `COUNT(*)` followed by insert. The subscription-owned allocation ledger is authoritative for admission.
- Do not delete legacy data in this plan. Creating new empty tables is the logical reset; destructive cleanup must be a separate, explicitly authorized operation with an exact dependency audit.
- Use `apply_patch`, preserve unrelated changes, and stage only the files named by the active task.

---

## File Map

### Store Center domain and persistence

- Create: `internal/storecenter/store.go`
- Create: `internal/storecenter/store_test.go`
- Create: `internal/storecenter/repository.go`
- Create: `internal/storecenter/gorm_repository.go`
- Create: `internal/storecenter/gorm_repository_test.go`
- Create: `internal/storecenter/service.go`
- Create: `internal/storecenter/service_test.go`
- Create: `internal/storecenter/connection_status.go`
- Create: `internal/storecenter/connection_status_test.go`
- Create: `internal/storecenter/audit.go`
- Create: `internal/storecenter/audit_test.go`

### Subscription-owned quota allocation

- Modify: `internal/listingsubscription/types.go`
- Modify: `internal/listingsubscription/gorm_repository.go`
- Modify: `internal/listingsubscription/service.go`
- Modify: `internal/listingsubscription/service_test.go`
- Create: `internal/listingsubscription/store_quota.go`
- Create: `internal/listingsubscription/store_quota_test.go`
- Create: `internal/listingsubscription/store_quota_gorm.go`
- Create: `internal/listingsubscription/store_quota_gorm_test.go`

### HTTP, permissions, composition, and schema

- Create: `internal/storecenter/httpapi/module.go`
- Create: `internal/storecenter/httpapi/handler.go`
- Create: `internal/storecenter/httpapi/handler_test.go`
- Create: `internal/storecenter/httpapi/error_response.go`
- Create: `internal/storecenter/httpapi/error_response_test.go`
- Modify: `internal/authz/listingkit.go`
- Modify: `internal/authz/listingkit_test.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Create: `internal/app/httpapi/storecenter_module.go`
- Create: `internal/app/httpapi/storecenter_module_test.go`
- Create: `internal/workbench/schema/runtime.go`
- Create: `internal/workbench/schema/runtime_test.go`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime.go`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime_test.go`

### Frontend API and pages

- Create: `web/listingkit-ui/src/lib/api/workbench-stores.ts`
- Create: `web/listingkit-ui/src/lib/api/workbench-stores.test.ts`
- Create: `web/listingkit-ui/src/lib/query/use-workbench-stores.ts`
- Create: `web/listingkit-ui/src/lib/query/use-workbench-stores.test.tsx`
- Create: `web/listingkit-ui/src/lib/validation/workbench-store.ts`
- Create: `web/listingkit-ui/src/lib/validation/workbench-store.test.ts`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-list-page.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-list-page.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-table.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-table.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-form.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-form.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-detail-page.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-detail-page.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-lifecycle-actions.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-lifecycle-actions.test.tsx`
- Create: `web/listingkit-ui/src/app/workbench/stores/page.tsx`
- Create: `web/listingkit-ui/src/app/workbench/stores/new/page.tsx`
- Create: `web/listingkit-ui/src/app/workbench/stores/[storeId]/page.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/workspace-app-shell.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/workspace-app-shell.test.tsx`

---

## Domain and API Contract

The persisted aggregate is:

```go
type Store struct {
    ID                string
    OrganizationID    string
    Name              string
    Platform          string
    Region            string
    ExternalStoreID   string
    LifecycleStatus   LifecycleStatus // provisioning, active, disabled, deleting
    ConnectionRef     string
    QuotaAllocationID string
    Version           int64
    CreatedBy         string
    UpdatedBy         string
    CreatedAt         time.Time
    UpdatedAt         time.Time
    DeletedAt         *time.Time
}
```

Connection status is a response projection, not a caller-controlled persisted field:

```go
type ConnectionStatus string // disconnected, connected, expired, unavailable
```

List responses include the authoritative quota projection:

```go
type StoreQuotaSummary struct {
    Used     int64
    Reserved int64
    Limit    *int64
    Allowed  bool
    Reason   string
}
```

Tables:

- `workbench_stores`
- `workbench_store_audit_logs`
- `saas_store_quota_allocations`
- `saas_store_quota_buckets`

Routes:

```text
GET    /api/v1/workbench/stores
POST   /api/v1/workbench/stores
GET    /api/v1/workbench/stores/:store_id
PUT    /api/v1/workbench/stores/:store_id
POST   /api/v1/workbench/stores/:store_id/disable
POST   /api/v1/workbench/stores/:store_id/enable
DELETE /api/v1/workbench/stores/:store_id
```

Create requires `Idempotency-Key`. Update, enable, disable, and delete require `If-Match: "<version>"`. Every successful mutation returns the new version.

Role mapping:

| Scoped role | Permissions |
|---|---|
| `listingkit_viewer` | `workbench.store.read` |
| `listingkit_operator` | read, create, update, lifecycle |
| `listingkit_admin` | read, create, update, lifecycle, delete |
| `platform_admin` | read, create, update, lifecycle, delete |

---

## Task 1: Define the New Store Aggregate and State Machine

**Files:**

- Create: `internal/storecenter/store.go`
- Create: `internal/storecenter/store_test.go`
- Create: `internal/storecenter/repository.go`

- [ ] Add failing table tests for create normalization, required fields, supported initial platform values, state transitions, UUID validation, immutable Organization/ID/platform identity, and version increments.
- [ ] Define supported first-slice platforms from real backend capability, starting with `shein`; reject arbitrary strings instead of storing an unvalidated marketplace name.
- [ ] Define lifecycle edges: `provisioning → active`, `active → disabled`, `disabled → active`, and `active|disabled → deleting`. No other edge is valid.
- [ ] Normalize name/region/external store ID without altering opaque Organization IDs. Reject blank Organization ID, subject, name, platform, and idempotency key.
- [ ] Define typed errors: `ErrNotFound`, `ErrAlreadyExists`, `ErrVersionConflict`, `ErrInvalidTransition`, `ErrLimitReached`, and `ErrDependencyUnavailable`.
- [ ] Define a repository interface whose every read/write method receives Organization ID explicitly. Do not expose a method that loads by store UUID alone.
- [ ] Keep secrets out of all types. Add a reflection-based regression test failing if JSON-visible fields named password, token, cookie, secret, credential, or username appear on `Store` or API DTOs.

**Verification:**

```powershell
go test ./internal/storecenter -run "TestStore|TestLifecycle|TestStoreContract" -count=1
```

Expected: pass.

**Commit:**

```powershell
git add internal/storecenter/store.go internal/storecenter/store_test.go internal/storecenter/repository.go
git commit -m "feat: define organization-scoped store aggregate"
```

## Task 2: Persist Stores With Organization and Version Guards

**Files:**

- Create: `internal/storecenter/gorm_repository.go`
- Create: `internal/storecenter/gorm_repository_test.go`

- [ ] Add SQLite integration tests for schema creation, create replay, Organization-scoped pagination/filtering, cross-Organization not-found behavior, optimistic updates, soft delete, and concurrent version conflicts.
- [ ] Map `Store` to `workbench_stores` with UUID string primary key, non-null `organization_id`, `version`, lifecycle/quota state, audit actor fields, timestamps, and soft-delete timestamp.
- [ ] Add indexes for `(organization_id, lifecycle_status, updated_at)`, `(organization_id, platform, region)`, and unique `(organization_id, create_idempotency_key)`.
- [ ] Store a normalized `identity_key` and enforce unique `(organization_id, identity_key)` to prevent duplicate platform/region/external-store identity. When external store ID is absent, derive the key from the generated store UUID so two unnamed external identities are not falsely merged.
- [ ] Implement list with page size 20 by default, maximum 100, deterministic `updated_at DESC, id ASC`, platform/status filters, and total count in the same Organization scope.
- [ ] Implement update with `WHERE organization_id=? AND id=? AND version=? AND deleted_at IS NULL`; distinguish same-Organization version conflict from not-found without probing another Organization.
- [ ] Implement soft delete only after lifecycle is `deleting`. Never hard delete from a request handler.

**Verification:**

```powershell
go test ./internal/storecenter -run "TestGormStoreRepository" -count=1 -race
```

Expected: pass.

**Commit:**

```powershell
git add internal/storecenter/gorm_repository.go internal/storecenter/gorm_repository_test.go
git commit -m "feat: persist workbench stores by organization"
```

## Task 3: Add a Subscription-Owned Store Quota Allocation Ledger

**Files:**

- Modify: `internal/listingsubscription/types.go`
- Modify: `internal/listingsubscription/gorm_repository.go`
- Create: `internal/listingsubscription/store_quota.go`
- Create: `internal/listingsubscription/store_quota_test.go`
- Create: `internal/listingsubscription/store_quota_gorm.go`
- Create: `internal/listingsubscription/store_quota_gorm_test.go`

- [ ] Add interface tests for `Reserve`, `Commit`, `ReleaseReservation`, `Deallocate`, idempotent `GetByRequestKey`, and `Summary`.
- [ ] Model an allocation with Organization ID, allocation ID, store ID, request key, status (`reserved`, `allocated`, `released`), actor, and timestamps.
- [ ] Model a bucket with Organization ID primary key, committed count, reserved count, and version. Store it in `saas_store_quota_buckets`.
- [ ] Resolve the effective `store_management.store_count` limit inside the subscription package from tenant entitlement first and current plan second. Status/expiry evaluation must reuse existing subscription functions.
- [ ] Add explicit first-slice plan defaults in `DefaultPlans`: Basic `store_count=1`, Professional `store_count=5`, Enterprise `store_count=20`. Preserve tenant entitlement override precedence and add tests for all three defaults plus an override.
- [ ] In one GORM transaction, lock or create the Organization bucket, replay an existing request key, verify `committed + reserved + 1 <= limit`, create the allocation, and increment reserved.
- [ ] On PostgreSQL use row locking; preserve the repository's existing bounded retry behavior for SQLite busy/locked tests. A concurrent test with limit 1 must produce exactly one reservation and one `ErrStoreQuotaExceeded`.
- [ ] Commit atomically moves one from reserved to committed. Releasing an uncommitted reservation decrements reserved. Deallocate transitions an allocated row to released and decrements committed once.
- [ ] `Summary` returns the same effective `store_count` limit plus the bucket's committed/reserved values so Store Center never derives quota by counting its own table.
- [ ] Reject underflow, invalid transitions, mismatched store/request identity, and a non-positive/missing limit according to product policy. Missing entitlement returns subscription-required, not unlimited; `Summary` exposes `Allowed=false` and a stable reason without inventing a limit.
- [ ] Keep this ledger separate from the generic `UsageLedger`; do not weaken the latter's job/storage metric validation.
- [ ] Include both new tables in `AutoMigrateRepository` and assert idempotent migration.

**Verification:**

```powershell
go test ./internal/listingsubscription -run "TestStoreQuota" -count=1 -race
```

Expected: pass, including the concurrent limit-1 case.

**Commit:**

```powershell
git add internal/listingsubscription/types.go internal/listingsubscription/gorm_repository.go internal/listingsubscription/service.go internal/listingsubscription/service_test.go internal/listingsubscription/store_quota*
git commit -m "feat: allocate store quota atomically"
```

## Task 4: Implement Idempotent Store Creation and Compensation

**Files:**

- Create: `internal/storecenter/service.go`
- Create: `internal/storecenter/service_test.go`
- Create: `internal/storecenter/audit.go`
- Create: `internal/storecenter/audit_test.go`

- [ ] Add failing service tests for happy path, request replay, duplicate identity, limit reached, repository failure before create, unknown create result, quota commit failure, retry after commit failure, and audit failure.
- [ ] Reserve quota first using Organization ID and request key. Let the quota allocation return the canonical generated store UUID so concurrent retries converge on one store.
- [ ] Create the store as `provisioning` with the returned allocation ID. If create definitively fails and a same-Organization read confirms no row, release the reservation.
- [ ] If create returns an ambiguous error, read by Organization/store ID before compensation. Never release quota when a store may exist.
- [ ] Commit the allocation, then transition the store to `active`. If commit fails, leave the store in `provisioning`; replay with the same request key resumes commit instead of creating another store.
- [ ] Write a redacted audit entry for reserve, create, commit, and failure outcomes. Audit payload contains field names and safe old/new state only; never provider credentials.
- [ ] Treat audit persistence as part of business acceptance for mutations. If it fails after the store is durable, return dependency unavailable and make replay finish the missing audit idempotently.
- [ ] Return `ErrLimitReached` with safe limit/used values, mapped later to `STORE_LIMIT_REACHED`.

**Verification:**

```powershell
go test ./internal/storecenter -run "TestServiceCreate|TestAudit" -count=1
```

Expected: pass.

**Commit:**

```powershell
git add internal/storecenter/service.go internal/storecenter/service_test.go internal/storecenter/audit.go internal/storecenter/audit_test.go
git commit -m "feat: create stores with idempotent quota compensation"
```

## Task 5: Implement Read, Edit, Lifecycle, and Controlled Delete

**Files:**

- Modify: `internal/storecenter/service.go`
- Modify: `internal/storecenter/service_test.go`
- Create: `internal/storecenter/connection_status.go`
- Create: `internal/storecenter/connection_status_test.go`

- [ ] Add failing tests for list/get isolation, filter/page validation, version conflict, immutable field edits, disable/enable transitions, delete from invalid state, deallocation failure, retry after deallocation, and cross-Organization UUID probing.
- [ ] Add a `ConnectionStatusProvider` interface accepting Organization ID, store UUID, platform, and opaque connection ref. For blank refs, return `disconnected` without calling a provider.
- [ ] For nonblank refs, return provider-derived `connected`, `expired`, or `unavailable`; never accept status from request JSON. Do not implement a numeric-ID SHEIN adapter in this slice.
- [ ] Project connection status on list/get with bounded per-request concurrency and a short timeout. A status-provider failure returns the store with `unavailable`; it does not expose internal errors or block basic store administration.
- [ ] Include the quota ledger's Organization-scoped summary in list responses as `used`, `reserved`, and `limit`; do not calculate it with `COUNT(workbench_stores)`.
- [ ] Implement basic edit with `If-Match` version and immutable Organization/platform/external identity fields.
- [ ] Disable and enable through explicit state transitions. Disabled stores continue consuming quota.
- [ ] Controlled delete first transitions to `deleting`, then deallocates quota, then soft-deletes. If deallocation fails, keep `deleting`; a retry with the returned/current version resumes safely.
- [ ] After successful soft delete, list/get return not found and quota committed count is lower by one. Repeating delete is idempotent only when the same delete operation key is supplied; otherwise return not found.
- [ ] Write idempotent audit events for edit, disable, enable, delete-start, quota-deallocate, and delete-complete.

**Verification:**

```powershell
go test ./internal/storecenter -run "TestService(List|Get|Update|Disable|Enable|Delete)|TestConnectionStatus" -count=1 -race
```

Expected: pass.

**Commit:**

```powershell
git add internal/storecenter
git commit -m "feat: manage store lifecycle safely"
```

## Task 6: Add Scoped Permissions and HTTP Routes

**Files:**

- Create: `internal/storecenter/httpapi/module.go`
- Create: `internal/storecenter/httpapi/handler.go`
- Create: `internal/storecenter/httpapi/handler_test.go`
- Create: `internal/storecenter/httpapi/error_response.go`
- Create: `internal/storecenter/httpapi/error_response_test.go`
- Modify: `internal/authz/listingkit.go`
- Modify: `internal/authz/listingkit_test.go`

- [ ] Add permission constants and role policies from the contract table. Add tests proving viewer cannot mutate, operator cannot delete, and A-admin roles do not apply in B-viewer context.
- [ ] Register GET routes with `OrganizationAccessCachedRead` and mutation routes with `OrganizationAccessLiveWrite`.
- [ ] Parse page/platform/status inputs with explicit bounds. Parse JSON with unknown-field rejection and body-size limits.
- [ ] Require `Idempotency-Key` for create and delete operation identity. Require a quoted integer `If-Match` for all versioned mutations; missing/malformed values return a field error.
- [ ] Never accept Organization ID in request bodies or query parameters. Read effective Organization and subject only from the resolved authenticated identity.
- [ ] Return `404` for same-Organization unknown IDs and cross-Organization IDs alike. Return `409` for version conflict/duplicate, `422` for invalid state/input, `403` for permission, `429` or `409` consistently for limit reached, and `503` for dependencies.
- [ ] Emit the stable error envelope `{code,message,requestId,fieldErrors}` and exact design error codes. Tests assert codes, not Chinese/English message text.
- [ ] Add request tests for every route, spoofed Organization input, scoped roles, optimistic conflicts, lifecycle actions, and redacted responses.

**Verification:**

```powershell
go test ./internal/authz ./internal/storecenter/httpapi -count=1
```

Expected: pass.

**Commit:**

```powershell
git add internal/authz/listingkit.go internal/authz/listingkit_test.go internal/storecenter/httpapi
git commit -m "feat: expose scoped store center api"
```

## Task 7: Wire Store Center and Its Schema Into the Modular Monolith

**Files:**

- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/composition_modules.go`
- Create: `internal/app/httpapi/storecenter_module.go`
- Create: `internal/app/httpapi/storecenter_module_test.go`
- Create: `internal/workbench/schema/runtime.go`
- Create: `internal/workbench/schema/runtime_test.go`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime.go`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime_test.go`

- [ ] Add failing composition tests proving one shared GORM connection creates the store repository, subscription repository/quota ledger, audit repository, service, and route module.
- [ ] Build Store Center only when workbench is enabled and database configuration is present. Fail startup rather than serving an in-memory store in production.
- [ ] Add the module to `runtimeModules()` after workbench context is constructed. Route middleware still owns request order; module order must not become an authorization assumption.
- [ ] Implement `workbench/schema.AutoMigrateRuntime` for Store Center and subscription quota tables. It must not migrate or read `listing_store`.
- [ ] Extend the existing schema migration runtime with scope `workbench`; scope `all` includes it. Keep this as a thin deployment entry point, while schema ownership remains in `internal/workbench/schema`.
- [ ] Add migration tests asserting all four new tables exist, repeated migration is idempotent, and representative `organization_id` columns are strings/non-null.
- [ ] Run migration against an empty disposable PostgreSQL database and record table existence/count-zero evidence. Do not truncate legacy tables.

**Verification:**

```powershell
go test ./internal/app/httpapi ./internal/workbench/schema ./internal/app/runtime/listingkitschemamigrate -run "Test.*StoreCenter|Test.*Workbench.*Migrate|Test.*WorkBench" -count=1
```

Expected: pass.

**Commit:**

```powershell
git add internal/app/httpapi/types.go internal/app/httpapi/composition_builder.go internal/app/httpapi/composition_modules.go internal/app/httpapi/storecenter_module* internal/workbench/schema internal/app/runtime/listingkitschemamigrate
git commit -m "feat: wire store center runtime and schema"
```

## Task 8: Build the Typed Frontend Store API and Query Layer

**Files:**

- Create: `web/listingkit-ui/src/lib/api/workbench-stores.ts`
- Create: `web/listingkit-ui/src/lib/api/workbench-stores.test.ts`
- Create: `web/listingkit-ui/src/lib/query/use-workbench-stores.ts`
- Create: `web/listingkit-ui/src/lib/query/use-workbench-stores.test.tsx`
- Create: `web/listingkit-ui/src/lib/validation/workbench-store.ts`
- Create: `web/listingkit-ui/src/lib/validation/workbench-store.test.ts`

- [ ] Define exact TypeScript response/request types matching Go JSON fields and stable error codes. Do not copy legacy `tenant-stores.ts` numeric IDs or credential fields.
- [ ] Add Zod validation for name, supported platform, region, and optional external store ID. Organization and role fields are never part of form input.
- [ ] Implement list/get/create/update/enable/disable/delete calls under `/api/workbench/stores`.
- [ ] Generate a UUID idempotency key once per create form submission attempt and reuse it across retries. Generate a separate stable operation key per delete confirmation.
- [ ] Send `If-Match` from the last server version for every versioned mutation. Parse stable error JSON into a typed `WorkbenchAPIError`.
- [ ] Root every query key at `['workbench', effectiveOrganizationId, 'stores']`; require the Organization ID from `WorkbenchContextProvider` and disable queries until it exists.
- [ ] Invalidate only the current Organization's store keys after mutations. The context provider's switch behavior remains responsible for clearing all Organization-scoped state.
- [ ] Add tests for key isolation, request headers, retry key reuse, filter encoding, error-code parsing, and no credential-shaped fields.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/lib/api/workbench-stores.test.ts src/lib/query/use-workbench-stores.test.tsx src/lib/validation/workbench-store.test.ts
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/lib/api/workbench-stores* web/listingkit-ui/src/lib/query/use-workbench-stores* web/listingkit-ui/src/lib/validation/workbench-store*
git commit -m "feat: add organization-scoped store client"
```

## Task 9: Build Store List, Filters, Pagination, and Empty States

**Files:**

- Create: `web/listingkit-ui/src/components/workbench/stores/store-list-page.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-list-page.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-table.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-table.test.tsx`
- Create: `web/listingkit-ui/src/app/workbench/stores/page.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/workspace-app-shell.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/workspace-app-shell.test.tsx`

- [ ] Add component tests for loading, error, no stores, no filter results, populated results, expired/unavailable connections, quota summary, permission-hidden create action, and pagination.
- [ ] Implement the Figma-aligned `店铺中心 / 我的店铺` navigation item and active state. Do not add hidden/archived or unimplemented menu items.
- [ ] Render columns: store name, platform, region, external store ID, lifecycle status, connection status, updated time, and actions.
- [ ] Keep filters in URL search parameters so refresh/back behavior is stable. Reset page to 1 when platform/status changes.
- [ ] Display `disconnected` honestly for blank refs and `unavailable` when provider status cannot be checked. Do not label either as connected.
- [ ] Show quota `used / limit` from the list response and an upgrade/administrator-contact hint on `STORE_LIMIT_REACHED`; do not route to a nonexistent payment page.
- [ ] Ensure a context switch removes the old table immediately before the new Organization query starts.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/components/workbench/stores/store-list-page.test.tsx src/components/workbench/stores/store-table.test.tsx src/components/workbench/workspace-app-shell.test.tsx
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/components/workbench/stores/store-list-page* web/listingkit-ui/src/components/workbench/stores/store-table* web/listingkit-ui/src/app/workbench/stores/page.tsx web/listingkit-ui/src/components/workbench/workspace-app-shell*
git commit -m "feat: render workbench store directory"
```

## Task 10: Build Create and Edit Forms With Conflict Recovery

**Files:**

- Create: `web/listingkit-ui/src/components/workbench/stores/store-form.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-form.test.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-detail-page.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-detail-page.test.tsx`
- Create: `web/listingkit-ui/src/app/workbench/stores/new/page.tsx`
- Create: `web/listingkit-ui/src/app/workbench/stores/[storeId]/page.tsx`

- [ ] Add tests for client validation, server field errors, create idempotency retry, successful redirect, quota rejection, read-only platform identity, save conflict, and Organization switch during an open form.
- [ ] Build accessible React Hook Form + Zod create/edit flows. Never render username/password/token fields.
- [ ] Disable submit while a mutation is active, but retain the generated idempotency key when a network/503 retry is offered.
- [ ] On `STORE_VERSION_CONFLICT`, keep the user's draft, fetch the latest server record, show changed fields, and require an explicit retry against the new version. Do not silently overwrite.
- [ ] On Organization switch, discard the draft and navigate to the new Organization's store list after a confirmation if the form is dirty.
- [ ] Show connection state as read-only. If no connection flow exists, label the action `连接能力尚未开放` and keep it disabled; do not link to the numeric legacy SHEIN login route.
- [ ] Include the current Organization name in create confirmation and page header.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/components/workbench/stores/store-form.test.tsx src/components/workbench/stores/store-detail-page.test.tsx
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/components/workbench/stores/store-form* web/listingkit-ui/src/components/workbench/stores/store-detail-page* "web/listingkit-ui/src/app/workbench/stores/[storeId]" web/listingkit-ui/src/app/workbench/stores/new
git commit -m "feat: create and edit workbench stores"
```

## Task 11: Build Lifecycle Actions and High-Risk Confirmation

**Files:**

- Create: `web/listingkit-ui/src/components/workbench/stores/store-lifecycle-actions.tsx`
- Create: `web/listingkit-ui/src/components/workbench/stores/store-lifecycle-actions.test.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/stores/store-detail-page.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/stores/store-detail-page.test.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/stores/store-table.tsx`
- Modify: `web/listingkit-ui/src/components/workbench/stores/store-table.test.tsx`

- [ ] Add tests for role-based visibility, enable/disable labels, disabled stores still consuming quota, delete confirmation, version conflict, deleting/retry state, and revoked grant during mutation.
- [ ] Require delete confirmation text containing both the exact store name and current Organization name. Do not use a generic browser confirm dialog.
- [ ] Disable lifecycle actions while requests run and prevent double submission. Reuse the same delete operation key for a retry of an interrupted deletion.
- [ ] On `ORGANIZATION_ACCESS_REVOKED`, clear workbench queries, clear the selected Organization cookie through the context endpoint behavior, and show the blocking access state.
- [ ] On a `deleting` response after dependency failure, present a retry action and prevent edit/enable/disable.
- [ ] After successful delete, invalidate current Organization stores, navigate to list, and show a recoverability statement: the UI does not offer restore; database soft-delete remains an operational recovery mechanism.

**Verification:**

```powershell
Set-Location web/listingkit-ui
npm.cmd test -- src/components/workbench/stores/store-lifecycle-actions.test.tsx src/components/workbench/stores/store-detail-page.test.tsx src/components/workbench/stores/store-table.test.tsx
```

Expected: pass.

**Commit:**

```powershell
git add web/listingkit-ui/src/components/workbench/stores
git commit -m "feat: control workbench store lifecycle"
```

## Task 12: Verify the Full Vertical Slice

**Files:**

- Modify only files already named above when a failed verification demonstrates a defect.

- [ ] Run all Store Center, subscription quota, authorization, workbench context, schema, and composition Go tests with the race detector.
- [ ] Run all new frontend tests, then full typecheck and lint. Confirm named Vitest patterns execute tests rather than reporting no tests.
- [ ] Apply `workbench` schema to an empty disposable PostgreSQL database; verify all four tables exist and contain zero rows before acceptance data is created.
- [ ] With Organization A limit 1 and Organization B limit 2, create stores concurrently and prove each Organization enforces its own limit.
- [ ] Refresh the page and prove persisted records remain. Switch Organizations and prove old rows disappear before the new query resolves.
- [ ] Forge store UUID, Organization cookie, request header, and request body values; prove cross-Organization access remains indistinguishable from not found.
- [ ] Revoke the active Organization during a mutation and prove the write fails immediately through live ZITADEL validation.
- [ ] Verify edit conflicts, disable/enable, controlled delete, quota release only after delete, and request replay after simulated quota commit failure.
- [ ] Inspect API responses, logs, new tables, and audits for passwords, tokens, cookies, credentials, and legacy numeric tenant IDs.
- [ ] Report unit/integration results, local runtime smoke, real ZITADEL acceptance, deployment, and business acceptance separately.

**Verification:**

```powershell
go test ./internal/storecenter/... ./internal/listingsubscription ./internal/authz ./internal/workbenchcontext/... ./internal/workbench/schema ./internal/app/httpapi ./internal/app/runtime/listingkitschemamigrate -count=1 -race
Set-Location web/listingkit-ui
npm.cmd test -- src/lib/api/workbench-stores.test.ts src/lib/query/use-workbench-stores.test.tsx src/lib/validation/workbench-store.test.ts src/components/workbench/stores src/components/workbench/workspace-app-shell.test.tsx
npm.cmd run typecheck
npm.cmd run lint
```

Expected: every command exits 0; race detector is clean; the test runner reports executed tests.

**Security scan:**

```powershell
rg -n "password|refresh_token|access_token|cookie|tenantbridge|listing_store" internal/storecenter internal/workbench web/listingkit-ui/src/components/workbench/stores web/listingkit-ui/src/lib/api/workbench-stores.ts
```

Expected: only deliberate negative-test text or explanatory comments; no persisted/API credential fields, tenant bridge calls, or legacy table access.

**Commit:**

```powershell
git status --short
git diff --cached --check
```

Create a final verification commit only if verification required source/test fixes; otherwise keep the task commits as the complete history.

---

## Completion Gate

This plan is complete only when:

1. Store Center uses only new Organization-string/UUID tables and never reaches `listing_store` or `tenantbridge`.
2. Same-subject Organization switching changes rows, roles, quota, and actions without leaking prior client state.
3. Concurrent creates cannot exceed `store_count`; request replay cannot create duplicates.
4. Updates use optimistic concurrency and surface conflicts without silent overwrite.
5. Disabled stores consume quota; only completed controlled deletion releases it.
6. Connection state is provider-derived or honestly `disconnected`/`unavailable`; no fake connected state exists.
7. Cross-Organization store IDs are indistinguishable from unknown IDs.
8. Real ZITADEL revocation blocks writes immediately and cached reads within 60 seconds.
