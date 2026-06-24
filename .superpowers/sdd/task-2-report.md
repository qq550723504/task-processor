Status: completed

Files changed:
- `internal/listingkit/interfaces_services.go`
- `internal/listingkit/model_sds_retirement.go`
- `internal/listingkit/sds_retirement_matching.go`
- `internal/listingkit/sds_retirement_matching_test.go`
- `internal/listingkit/service_sds_retirement.go`
- `internal/listingkit/service_sds_retirement_test.go`

Commit hashes:
- `2c9b41f95a6483eaeac854bc1c8c3c183242f452` - `feat: discover sds retirement impact`

Exact tests/results:
1. `set GOWORK=off; go test ./internal/listingkit -run 'TestSDSRetirement' -count=1`
   - Initial red run failed with undefined `sdsRetirementTaskMatchesIdentity`, `sdsRetirementSourceSKUs`, `NewSDSRetirementService`, `CreateSDSRetirementRunRequest`, and `UpdateSDSRetirementSelectionRequest`.
2. `gofmt -w internal/listingkit/model_sds_retirement.go internal/listingkit/interfaces_services.go internal/listingkit/sds_retirement_matching.go internal/listingkit/service_sds_retirement.go internal/listingkit/sds_retirement_matching_test.go internal/listingkit/service_sds_retirement_test.go`
   - Succeeded.
3. `set GOWORK=off; go test ./internal/listingkit -run 'TestSDSRetirement' -count=1`
   - Passed: `ok  	task-processor/internal/listingkit	0.402s`
4. `set GOWORK=off; go test ./internal/listingkit -count=1`
   - Passed: `ok  	task-processor/internal/listingkit	2.461s`

Self-review:
- Implemented an independent `SDSRetirementService` instead of extending the main aggregate service.
- Kept Task 2 scoped to preview-run creation, retrieval, and selection updates only; no OffShelf execution path was added.
- Refreshes SHEIN synced products before preview matching and only matches refreshed `ON_SHELF` rows by extracted `source_sds_sku`.
- `buildSDSRetirementItems` now fails clearly when no source SDS SKUs are present, avoiding an unsafe fallback to all active products.
- Added tenant validation for SHEIN refresh so non-numeric tenant IDs fail loudly instead of silently becoming `0`.
- Left `.superpowers/` scratch artifacts and unrelated docs changes unstaged.

Concerns:
- `SourceTaskID` currently narrows the matched task set, but retirement items do not persist a source `TaskID`; if later execution/audit flows need per-item provenance, Task 3 may need to extend item attribution.
- `internal/listingkit/httpapi/bootstrap_module_service.go` was not changed because Task 2 does not expose HTTP routes or handler wiring yet; Task 3 should wire this service when an API consumer exists.

---

Review fix report (2026-06-24):

Fix summary:
- Rebound SDS retirement preview creation to the resolved tenant scope before task discovery, and required tenant-scoped read/update access for preview runs.
- Scoped GORM run fetch/update paths by tenant instead of raw `run_id`, so foreign-tenant run reads and selection updates now return `ErrTaskNotFound`.
- Paged through all matching task pages and synced-product pages with 100-row requests that match the repository caps, so matches beyond page 1 are included.
- Added an explicit immediate-refresh capability check for SHEIN sync services; async-only implementations now fail fast with `cannot guarantee refreshed SHEIN preview data with async-only sync service` instead of pretending the refresh is complete.

Files changed:
- `internal/listingkit/service_sds_retirement.go`
- `internal/listingkit/service_sds_retirement_test.go`
- `internal/listingkit/sheinsync/async_service.go`
- `internal/listingkit/sheinsync/service.go`
- `internal/listingkit/store/sds_retirement_repo.go`
- `internal/listingkit/store/sds_retirement_repo_test.go`

Commit hashes:
- `07a38ea9` - `fix: scope sds retirement preview discovery`

Covering tests run and exact output:
1. `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/store -run 'TestSDSRetirement' -count=1`
   - Initial red run:
     - `--- FAIL: TestSDSRetirementCreateRunScopesTaskDiscoveryToResolvedTenant`
     - `--- FAIL: TestSDSRetirementCreateRunPagesThroughAllTaskPages`
     - `--- FAIL: TestSDSRetirementCreateRunPagesThroughAllSyncedProducts`
     - `--- FAIL: TestSDSRetirementCreateRunRejectsAsyncOnlySheinRefresh`
     - `--- FAIL: TestSDSRetirementRepositoryGetRunHonorsTenantScope`
     - `--- FAIL: TestSDSRetirementRepositoryUpdateItemsHonorsTenantScope`
     - `FAIL    task-processor/internal/listingkit`
     - `FAIL    task-processor/internal/listingkit/store`
2. `gofmt -w internal/listingkit/service_sds_retirement.go internal/listingkit/service_sds_retirement_test.go internal/listingkit/store/sds_retirement_repo.go internal/listingkit/store/sds_retirement_repo_test.go internal/listingkit/sheinsync/service.go internal/listingkit/sheinsync/async_service.go`
   - Succeeded.
3. `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/store -run 'TestSDSRetirement' -count=1`
   - Passed:
     - `ok      task-processor/internal/listingkit        0.406s`
     - `ok      task-processor/internal/listingkit/store  0.392s`
4. `$env:GOWORK='off'; go test ./internal/listingkit ./internal/listingkit/store ./internal/listingkit/sheinsync -count=1`
   - Passed:
     - `ok      task-processor/internal/listingkit        2.062s`
     - `ok      task-processor/internal/listingkit/store  0.469s`
     - `ok      task-processor/internal/listingkit/sheinsync      4.323s`

---

Re-review fix report (2026-06-24):

Fix summary:
- Tightened `requireSDSRetirementTenantScope` to use `TenantScopeFromContext` instead of `TenantIDFromContext`, so unscoped `context.Background()` reads/updates no longer inherit the `"default"` fallback.
- Changed SDS retirement repository access scope to fail closed when tenant scope is absent, preventing unscoped `GetSDSRetirementRun` and `UpdateSDSRetirementItems` from reading or mutating preview runs by raw `run_id`.
- Added service and repository regression tests covering unscoped get/update calls.

Files changed:
- `internal/listingkit/service_sds_retirement.go`
- `internal/listingkit/service_sds_retirement_test.go`
- `internal/listingkit/store/sds_retirement_repo.go`
- `internal/listingkit/store/sds_retirement_repo_test.go`

Covering tests run and exact output:
1. `$env:GOWORK='off'; go test ./internal/listingkit -run 'TestSDSRetirement(GetRunRequiresExplicitTenantScope|UpdateSelectionRequiresExplicitTenantScope)' -count=1`
   - Initial red run:
     - `--- FAIL: TestSDSRetirementGetRunRequiresExplicitTenantScope`
     - `--- FAIL: TestSDSRetirementUpdateSelectionRequiresExplicitTenantScope`
     - `FAIL    task-processor/internal/listingkit`
2. `$env:GOWORK='off'; go test ./internal/listingkit/store -run 'TestSDSRetirementRepository(GetRunRequiresExplicitTenantScope|UpdateItemsRequiresExplicitTenantScope)' -count=1`
   - Initial red run:
     - `--- FAIL: TestSDSRetirementRepositoryGetRunRequiresExplicitTenantScope`
     - `--- FAIL: TestSDSRetirementRepositoryUpdateItemsRequiresExplicitTenantScope`
     - `FAIL    task-processor/internal/listingkit/store`
3. `gofmt -w internal/listingkit/service_sds_retirement.go internal/listingkit/service_sds_retirement_test.go internal/listingkit/store/sds_retirement_repo.go internal/listingkit/store/sds_retirement_repo_test.go`
   - Succeeded.
4. `$env:GOWORK='off'; go test ./internal/listingkit -run 'TestSDSRetirement(GetRunRequiresExplicitTenantScope|UpdateSelectionRequiresExplicitTenantScope)' -count=1`
   - Passed: `ok      task-processor/internal/listingkit        0.287s`
5. `$env:GOWORK='off'; go test ./internal/listingkit/store -run 'TestSDSRetirementRepository(GetRunRequiresExplicitTenantScope|UpdateItemsRequiresExplicitTenantScope)' -count=1`
   - Passed: `ok      task-processor/internal/listingkit/store  0.430s`
6. `$env:GOWORK='off'; go test ./internal/listingkit -run 'TestSDSRetirement' -count=1`
   - Passed: `ok      task-processor/internal/listingkit        0.253s`

Concerns:
- The repository now fails closed for missing tenant scope on SDS retirement preview reads and item selection updates only; other repository surfaces that still intentionally allow default/unscoped access were left alone for this task.
