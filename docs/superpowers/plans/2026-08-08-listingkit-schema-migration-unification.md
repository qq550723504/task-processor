# ListingKit Schema Migration Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Replace the duplicated HTTP and CLI ListingKit runtime migration lists with one authoritative schema package and close the missing SDS child retry table regression.

**Architecture:** internal/listingkit/schema owns the ordered GORM migration sequence. The HTTP bootstrap and standalone CLI retain compatibility wrappers but delegate directly to the shared package, while the narrower shein-sync CLI scope remains unchanged.

**Tech Stack:** Go 1.24, GORM, modernc SQLite test driver, standard testing package.

## Global Constraints

- Preserve httpapi.AutoMigrateListingKitRuntimeSchema(*gorm.DB) error.
- Preserve CLI flags, scopes, logging, configuration loading, and database lifecycle.
- Preserve the current migration order and component-specific error messages.
- Include listingkit.SDSChildRetryJob in the standalone all path.
- Do not add destructive schema operations, a generic registry, or production database execution.
- Observe the regression test fail before changing production code.

---

### Task 1: Unify the Migration Sequence with TDD

**Files:**
- Modify: internal/app/runtime/listingkitschemamigrate/runtime_test.go
- Modify: internal/app/runtime/listingkitschemamigrate/runtime.go
- Create: internal/listingkit/schema/runtime_test.go
- Create: internal/listingkit/schema/runtime.go
- Modify: internal/listingkit/httpapi/builders_repository_schema.go
- Modify: internal/listingkit/httpapi/builders_test.go

**Interfaces:**
- Consumes: autoMigrateListingKitRuntimeSchema(*gorm.DB) error and openRuntimeSchemaTestDB(*testing.T) *gorm.DB.
- Produces: a failing regression test followed by one authoritative schema.AutoMigrateRuntime(*gorm.DB) error implementation, delegating HTTP/CLI entry points, and a green targeted suite. Do not commit or request review while the test is red or while duplicate concrete migration lists remain.

- [ ] **Step 1: Add the failing regression test**

Append this test after TestAutoMigrateListingKitRuntimeSchemaCreatesAIInvocationsTable:

~~~go
func TestAutoMigrateListingKitRuntimeSchemaCreatesSDSChildRetryTable(t *testing.T) {
	db := openRuntimeSchemaTestDB(t)

	if err := autoMigrateListingKitRuntimeSchema(db); err != nil {
		t.Fatalf("autoMigrateListingKitRuntimeSchema() error = %v", err)
	}
	if !db.Migrator().HasTable(&listingkit.SDSChildRetryJob{}) {
		t.Fatal("expected SDS child retry table to be created")
	}
}
~~~

- [ ] **Step 2: Run the test and verify RED**

Run:

~~~powershell
go test ./internal/app/runtime/listingkitschemamigrate -run TestAutoMigrateListingKitRuntimeSchemaCreatesSDSChildRetryTable -count=1
~~~

Expected: FAIL with expected SDS child retry table to be created. A compilation error or unrelated migration error is not the expected RED state.

---

#### Phase 2: Create the Authoritative Schema Package

**Files:**
- Create: internal/listingkit/schema/runtime_test.go
- Create: internal/listingkit/schema/runtime.go

**Interfaces:**
- Consumes: the existing migration functions and GORM models already used by the HTTP migration sequence.
- Produces: schema.AutoMigrateRuntime(db *gorm.DB) error, the only concrete ordered ListingKit runtime migration sequence.

- [ ] **Step 1: Add shared-package tests before the implementation**

Create internal/listingkit/schema/runtime_test.go:

~~~go
package schema

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
)

func TestAutoMigrateRuntimeRejectsNilDB(t *testing.T) {
	t.Parallel()

	err := AutoMigrateRuntime(nil)
	if err == nil || !strings.Contains(err.Error(), "database is nil") {
		t.Fatalf("AutoMigrateRuntime(nil) error = %v, want database is nil", err)
	}
}

func TestAutoMigrateRuntimeCreatesRepresentativeTables(t *testing.T) {
	db := openSchemaTestDB(t)

	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime() error = %v", err)
	}
	for _, table := range []any{
		"ai_invocations",
		"ai_async_jobs",
		"listing_kit_sds_baseline_cache",
		&listingkit.SDSChildRetryJob{},
		&listingkit.SheinPODImageLookupIndex{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table for %T(%v) to be created", table, table)
		}
	}
	if !db.Migrator().HasColumn(&listingkit.SheinPODImageLookupIndex{}, "sds_gallery_image_urls") {
		t.Fatal("expected POD image lookup index table to store SDS gallery image URLs")
	}
}

func openSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	for _, table := range []string{
		"listing_store",
		"listing_product_import_task",
		"listing_filter_rule",
		"listing_profit_rule",
		"listing_pricing_rule",
		"listing_operation_strategy",
		"listing_sensitive_word",
		"listing_product_import_mapping",
		"listing_category",
		"listing_product_data",
	} {
		if err := db.Exec("CREATE TABLE " + table + " (id integer)").Error; err != nil {
			t.Fatalf("create legacy %s table: %v", table, err)
		}
	}
	return db
}
~~~

- [ ] **Step 2: Run the new package test and verify RED**

Run:

~~~powershell
go test ./internal/listingkit/schema -count=1
~~~

Expected: FAIL to compile because AutoMigrateRuntime is undefined.

- [ ] **Step 3: Implement the shared migration sequence**

Create internal/listingkit/schema/runtime.go with the following imports, public contract, exact ordered calls, and private task helper:

~~~go
package schema

import (
	"fmt"

	"gorm.io/gorm"

	aicapabilitystore "task-processor/internal/aicapability/store"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

// AutoMigrateRuntime creates and updates every table required by ListingKit.
func AutoMigrateRuntime(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := autoMigrateTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit task repository: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateInvocationLedger(db); err != nil {
		return fmt.Errorf("ai invocation ledger auto-migrate failed: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateAsyncJobBindings(db); err != nil {
		return fmt.Errorf("ai async job binding auto-migrate failed: %w", err)
	}
	if err := listingkit.AutoMigrateStudioAsyncJobRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio async job repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchRunRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch run repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchTaskLinkRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch task link repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		return fmt.Errorf("migrate listingkit sds child retry repository: %w", err)
	}
	if err := listingkitstore.AutoMigrateSheinSyncRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit shein sync repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSRetirementRunRecord{}, &listingkit.SDSRetirementItemRecord{}); err != nil {
		return fmt.Errorf("migrate listingkit sds retirement repository: %w", err)
	}
	if err := listingkit.AutoMigrateUploadedImageRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit uploaded image repository: %w", err)
	}
	if err := listingkit.AutoMigrateStoreProfileRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit store profile repository: %w", err)
	}
	if err := listingadmin.AutoMigrateStoreRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin store repository: %w", err)
	}
	if err := listingadmin.AutoMigrateStoreStatisticsRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin store statistics repository: %w", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin import task repository: %w", err)
	}
	if err := listingadmin.AutoMigrateFilterRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin filter rule repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProfitRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin profit rule repository: %w", err)
	}
	if err := listingadmin.AutoMigratePricingRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin pricing rule repository: %w", err)
	}
	if err := listingadmin.AutoMigrateOperationStrategyRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin operation strategy repository: %w", err)
	}
	if err := listingadmin.AutoMigrateScheduledTaskConfigRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin scheduled task config repository: %w", err)
	}
	if err := listingadmin.AutoMigrateSensitiveWordRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin sensitive word repository: %w", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin generation topic policy repository: %w", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicOverrideRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin generation topic override repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProductImportMappingRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin product import mapping repository: %w", err)
	}
	if err := listingadmin.AutoMigrateCategoryRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin category repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin product data repository: %w", err)
	}
	if err := db.AutoMigrate(&sheinpub.SheinResolutionCacheEntry{}); err != nil {
		return fmt.Errorf("migrate shein resolution cache store: %w", err)
	}
	if err := db.AutoMigrate(&assetrepo.InventorySnapshot{}, &assetrepo.GenerationTaskSnapshot{}); err != nil {
		return fmt.Errorf("migrate asset repository: %w", err)
	}
	if err := db.AutoMigrate(&reviewstore.ReviewRecord{}); err != nil {
		return fmt.Errorf("migrate listingkit review repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		return fmt.Errorf("migrate listingkit studio session repository: %w", err)
	}
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit subscription repository: %w", err)
	}
	return nil
}

func autoMigrateTaskRepository(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&listingkit.Task{},
		&listingkit.CanonicalProductCacheEntry{},
		&listingkit.SDSBaselineCacheEntry{},
	); err != nil {
		return err
	}
	return listingkitstore.AutoMigrateSheinPODImageLookupIndex(db)
}
~~~

- [ ] **Step 4: Run shared-package tests and verify GREEN**

Run:

~~~powershell
go test ./internal/listingkit/schema -count=1
~~~

Expected: PASS.

---

#### Phase 3: Delegate CLI and HTTP Entry Points

**Files:**
- Modify: internal/app/runtime/listingkitschemamigrate/runtime.go
- Modify: internal/listingkit/httpapi/builders_repository_schema.go
- Modify: internal/listingkit/httpapi/builders_test.go
- Test: internal/app/runtime/listingkitschemamigrate/runtime_test.go
- Test: internal/listingkit/schema/runtime_test.go

**Interfaces:**
- Consumes: schema.AutoMigrateRuntime(*gorm.DB) error from Task 2.
- Produces: unchanged HTTP and CLI compatibility entry points with no local migration list.

- [ ] **Step 1: Delegate the CLI wrapper**

Replace the import block with:

~~~go
import (
	"context"
	"flag"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	listingkitschema "task-processor/internal/listingkit/schema"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/pkg/appenv"

	"gorm.io/gorm"
)
~~~

Replace the concrete list with:

~~~go
func autoMigrateListingKitRuntimeSchema(db *gorm.DB) error {
	return listingkitschema.AutoMigrateRuntime(db)
}
~~~

Delete autoMigrateListingKitTaskRepository and remove imports used only by the old list. Keep listingkitstore because the shein-sync scope uses it.

- [ ] **Step 2: Run the CLI regression and package tests**

Run:

~~~powershell
go test ./internal/app/runtime/listingkitschemamigrate -count=1
~~~

Expected: PASS, including TestAutoMigrateListingKitRuntimeSchemaCreatesSDSChildRetryTable.

- [ ] **Step 3: Delegate the HTTP wrapper**

Replace the import block with:

~~~go
import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	listingkitschema "task-processor/internal/listingkit/schema"
)
~~~

Replace the concrete function body with:

~~~go
func runListingKitRepositoryAutoMigrations(db *gorm.DB) error {
	return listingkitschema.AutoMigrateRuntime(db)
}
~~~

Delete imports used only by the old list. Preserve repositorySchemaBootstrapper, environment parsing, AutoMigrateListingKitRuntimeSchema, and the runListingKitRepositoryAutoMigrations function name.

- [ ] **Step 4: Move task-repository assertions to the owner package**

Delete these HTTP-private-helper tests because the helper no longer belongs to HTTP:

~~~text
TestAutoMigrateListingKitTaskRepositoryCreatesSDSBaselineCacheTable
TestAutoMigrateListingKitTaskRepositoryCreatesSheinPODImageLookupIndexTable
~~~

The shared representative-table test covers the baseline cache migration path and POD index table/column. Remove the listingkit import from builders_test.go if unused; retain listingkitstore for the shein-sync test.

- [ ] **Step 5: Format and run targeted tests**

Run:

~~~powershell
gofmt -w internal/listingkit/schema/runtime.go internal/listingkit/schema/runtime_test.go internal/listingkit/httpapi/builders_repository_schema.go internal/listingkit/httpapi/builders_test.go internal/app/runtime/listingkitschemamigrate/runtime.go internal/app/runtime/listingkitschemamigrate/runtime_test.go
go test ./internal/listingkit/schema ./internal/listingkit/httpapi ./internal/app/runtime/listingkitschemamigrate -count=1
~~~

Expected: all three packages PASS.

---

### Task 2: Prove Single Ownership and Complete Verification

**Files:**
- Verify: all files changed in Task 1.
- Commit: the implementation and tests scoped to this worktree; the design and plan remain in their existing documentation commits.

**Interfaces:**
- Consumes: the finished shared migration package and delegating entry points.
- Produces: evidence that the duplicate migration authority is gone and the regression is fixed.

- [ ] **Step 1: Verify concrete migration calls live only in the schema package**

Run:

~~~powershell
rg -n "AutoMigrateInvocationLedger|AutoMigrateStudioAsyncJobRepository|SDSChildRetryJob" internal/listingkit/schema internal/listingkit/httpapi/builders_repository_schema.go internal/app/runtime/listingkitschemamigrate/runtime.go
~~~

Expected: concrete migration calls appear in internal/listingkit/schema/runtime.go; HTTP and CLI files only reference listingkitschema.AutoMigrateRuntime.

- [ ] **Step 2: Run static checks**

Run:

~~~powershell
go vet ./internal/listingkit/schema ./internal/listingkit/httpapi ./internal/app/runtime/listingkitschemamigrate
git diff --check
~~~

Expected: both commands exit 0 with no diagnostics.

- [ ] **Step 3: Run the full backend suite with an extended timeout**

Run from a command invocation that permits at least ten minutes:

~~~powershell
go test ./... -count=1
~~~

Expected: PASS. If it fails or times out, capture the exact package and output and do not report the suite as passing.

- [ ] **Step 4: Review the final diff and worktree scope**

Run:

~~~powershell
git status --short
git diff --stat HEAD
git diff HEAD -- internal/listingkit/schema internal/listingkit/httpapi/builders_repository_schema.go internal/listingkit/httpapi/builders_test.go internal/app/runtime/listingkitschemamigrate/runtime.go internal/app/runtime/listingkitschemamigrate/runtime_test.go
~~~

Expected: only schema-unification implementation and tests are uncommitted; the design and implementation plan are already committed separately.

- [ ] **Step 5: Commit the implementation intentionally**

Run:

~~~powershell
git add -- internal/listingkit/schema/runtime.go internal/listingkit/schema/runtime_test.go internal/listingkit/httpapi/builders_repository_schema.go internal/listingkit/httpapi/builders_test.go internal/app/runtime/listingkitschemamigrate/runtime.go internal/app/runtime/listingkitschemamigrate/runtime_test.go
git diff --cached --check
git diff --cached --stat
git commit -m "refactor: unify ListingKit schema migrations"
~~~

Expected: commit succeeds and git status --short is empty.
