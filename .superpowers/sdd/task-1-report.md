Status: DONE

Files changed:
- `internal/listingkit/model_sds_retirement.go`
- `internal/listingkit/store/sds_retirement_repo.go`
- `internal/listingkit/store/sds_retirement_repo_test.go`
- `internal/listingkit/interfaces_dependencies.go`
- `internal/listingkit/httpapi/builders_repository_schema.go`
- `internal/listingkit/sheinsync/model_sync_products.go`
- `internal/listingkit/sheinsync/service_records.go`
- `internal/listingkit/sheinsync/model_test.go`
- `internal/listingkit/sheinsync/service_test.go`
- `internal/listingkit/store/shein_sync_repo_products.go`
- `internal/listingkit/store/shein_sync_repo_test.go`

Commit hash(es):
- `51d3cc20`

Tests run and exact results:
- `$env:GOWORK='off'; go test ./internal/listingkit/store -run 'TestSDSRetirementRepository' -count=1` -> `ok  	task-processor/internal/listingkit/store	0.326s`
- `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -run 'TestSheinSyncedProductRecordCarriesBusinessModel|TestSyncSheinOnShelfProductsUsesOnShelfRequestAndPersistsRows' -count=1` -> `ok  	task-processor/internal/listingkit/sheinsync	4.682s`
- `$env:GOWORK='off'; go test ./internal/listingkit/store -run 'TestSDSRetirementRepository|TestSheinSyncRepositoryUpsertSyncedProductsByStoreAndSKC' -count=1` -> `ok  	task-processor/internal/listingkit/store	0.325s`
- `$env:GOWORK='off'; go test ./internal/listingkit/store -count=1` -> `ok  	task-processor/internal/listingkit/store	0.549s`
- `$env:GOWORK='off'; go test ./internal/listingkit/sheinsync -count=1` -> `ok  	task-processor/internal/listingkit/sheinsync	5.434s`
- `$env:GOWORK='off'; go test ./internal/listingkit/httpapi -run 'TestAutoMigrateListingKitRuntimeSchemaRejectsNilDB|TestRepositorySchemaBootstrapperRunsMigrationOncePerDatabase|TestRepositorySchemaBootstrapper_Injectable' -count=1` -> `ok  	task-processor/internal/listingkit/httpapi	0.445s`

Self-review notes:
- Kept scope inside Task 1 ownership: persistence models/repository, schema registration, SHEIN synced-product field retention, and tests only.
- Added `platform` on SDS retirement run/item records per feature-wide constraint.
- Persisted SHEIN `business_model` from sync fetch through synced-product storage so later OffShelf work can use the recorded value without re-deriving it.
- Kept `MarkSyncedProductOffShelf` as persistence-only state update after a successful external execution; it does not replace actual SHEIN OffShelf execution.
- Added repository coverage for create/load, selection updates, execution save, synced-product off-shelf marking, and synced-product `business_model` persistence.

Concerns:
- None.
