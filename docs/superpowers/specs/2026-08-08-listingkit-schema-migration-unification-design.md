# ListingKit Schema Migration Unification Design

## Background

ListingKit runtime schema migration is currently implemented twice:

- `internal/listingkit/httpapi/builders_repository_schema.go` runs during API repository bootstrap.
- `internal/app/runtime/listingkitschemamigrate/runtime.go` runs from the standalone `listingkit-schema-migrate` command.

The two functions contain the same ordered migration sequence, but they have already drifted. The HTTP bootstrap creates `listingkit.SDSChildRetryJob`, while the standalone `all` migration does not. A database prepared only by the standalone command can therefore be missing `listingkit_sds_child_retry_jobs` even though the runtime repository supports durable SDS child retries.

## Goal

Establish one authoritative ListingKit runtime schema migration function and make both HTTP bootstrap and the standalone migration command delegate to it, while preserving existing public entry points, migration order, error semantics, and CLI scopes.

## Chosen Approach

Create `internal/listingkit/schema` as a focused schema-composition package. It will own the full runtime migration sequence and the task-repository migration helper. The HTTP and CLI packages will retain thin wrappers where their existing tests or callers rely on those function names, but neither wrapper will contain a migration list.

This is preferred over importing the HTTP package from the CLI because schema ownership must not depend on a transport layer. It is preferred over a generic migration registry because the repository currently needs one explicit ordered ListingKit migration sequence, not a plugin framework.

## Architecture

### Authoritative schema package

`internal/listingkit/schema/runtime.go` will provide:

```go
func AutoMigrateRuntime(db *gorm.DB) error
```

The function will:

1. reject a nil database with `database is nil`;
2. run the existing task, AI capability, Studio, SDS, store, administration, asset, review, and subscription migrations in their current order;
3. include `listingkit.SDSChildRetryJob` in that ordered sequence;
4. preserve the existing contextual error messages so operational diagnostics do not regress.

The task repository migration and SHEIN POD image lookup index migration will be private helpers in the same package because they are part of the same schema composition boundary.

### HTTP bootstrap

`internal/listingkit/httpapi.AutoMigrateListingKitRuntimeSchema` and the repository bootstrapper will continue to exist. `runListingKitRepositoryAutoMigrations` will become a thin call to `schema.AutoMigrateRuntime` so existing API construction and tests retain their entry point without retaining a second migration authority.

Environment-controlled bootstrap behavior, per-database `sync.Once` handling, and the default enabled state remain unchanged.

### Standalone migration command

`internal/app/runtime/listingkitschemamigrate` will retain its dependency injection structure and the `all` and `shein-sync` scopes. The default `MigrateAll` dependency will call `schema.AutoMigrateRuntime`. The private `autoMigrateListingKitRuntimeSchema` wrapper may remain for package compatibility, but it will only delegate to the shared function.

The `shein-sync` scope remains a deliberately narrower migration and will continue to call `listingkitstore.AutoMigrateSheinSyncRepository` directly.

## Error Handling

- Nil database behavior remains an error rather than a no-op.
- Each failed migration remains wrapped with its existing component-specific message.
- No migration is silently skipped after a previous step fails.
- The change adds no destructive schema operation and does not drop or rename tables.

## Testing

The implementation will use test-driven development.

1. Add a failing CLI-package regression test proving that the default `all` migration creates the `listingkit_sds_child_retry_jobs` table. It fails against the current duplicated CLI migration list.
2. Add shared-package tests for nil database rejection and representative tables from the ordered sequence, including the SDS child retry table, AI invocation ledger, and SHEIN POD image lookup index.
3. Keep the existing HTTP migration tests green to prove that its compatibility entry point still creates the expected schema.
4. Run the HTTP, CLI, and shared schema package tests together, followed by `go test ./...` with a timeout long enough for the repository's cold-start baseline.
5. Run `git diff --check` and search both entry-point packages to verify that the concrete migration list exists only in `internal/listingkit/schema`.

## Compatibility

- `httpapi.AutoMigrateListingKitRuntimeSchema(*gorm.DB) error` remains available.
- CLI flags, scopes, logging, configuration loading, and database lifecycle remain unchanged.
- Existing databases receive only the previously missing idempotent `AutoMigrate` call for the SDS child retry table when using the standalone `all` scope.
- No application service or repository interface changes.

## Non-Goals

- Replacing GORM `AutoMigrate` with versioned SQL migrations.
- Introducing a generic migration plugin registry.
- Changing the product-listing API schema migration owned by `internal/app/httpapi/adapters_schema_migration.go`.
- Running a migration against a production database as part of this code change.
- Refactoring unrelated ListingKit repository construction.

## Acceptance Criteria

1. There is one concrete ListingKit runtime migration sequence under `internal/listingkit/schema`.
2. HTTP bootstrap and standalone `all` migration both delegate to that sequence.
3. The standalone `all` path creates `listingkit_sds_child_retry_jobs`.
4. Existing migration order and component-specific error messages are preserved.
5. Targeted HTTP, CLI, and shared schema tests pass.
6. Full Go tests either pass within the extended verification timeout or any pre-existing failure is reported with exact evidence; a timeout is not reported as success.
7. The worktree contains only the design, implementation, and tests for schema migration unification.
