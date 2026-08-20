# Decouple 1688 Source Handoff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove production dependencies from `internal/product/**` to ListingKit, crawler, integration, and compatibility packages while preserving the existing 1688-to-ListingKit HTTP contract and source-lineage behavior.

**Architecture:** Add a product-owned immutable-by-convention 1688 snapshot at the sourcing boundary, convert the legacy crawler DTO into that snapshot inside the 1688 integration adapter, and move ListingKit handoff orchestration from the product domain into `internal/compatibility/listingkit`. Enforce the direction with Go AST import-boundary tests that already run in the repository CI.

**Tech Stack:** Go 1.24, standard-library `go/parser` architecture tests, existing `testing` and `testify` test conventions, PowerShell, Git.

**Spec:** `docs/superpowers/specs/2026-08-20-decouple-1688-source-handoff-design.md`

## Global Constraints

- Work only in the isolated worktree `C:\Users\Henry\code\task-processor\.worktrees\decouple-1688-source-handoff` on branch `codex/decouple-1688-source-handoff`.
- Keep `internal/product/sourcing.SourceEnvelope` neutral. Production files under `internal/product/**` must not import `internal/listingkit`, `internal/compatibility`, `internal/crawler`, or `internal/integration`.
- Preserve the current HTTP route, request and response JSON, verified tenant/user identity checks, source and SHEIN store-access checks, source identity, raw reference, trace fields, warnings, and ListingKit task request behavior.
- Keep `CreateTaskCommand.Product` as `*alibaba1688model.Product1688` at the compatibility HTTP/application boundary so the wire contract does not change. Convert it before calling product sourcing.
- Do not add a new service, perform URL crawling inside the create-task command, redesign `listingkit.GenerateOptions`, change deployment/configuration/data, or add aliases at the old `internal/product/sourcehandoff` path.
- Use the existing Go AST boundary-test infrastructure. A repository-wide `golangci-lint`/`depguard` migration is a separate follow-up because lint is not currently pinned as a CI authority.
- Follow test-driven development: add or strengthen one focused test, observe the intended failure, implement the smallest production change, then rerun the focused and neighboring suites.
- Stage only paths named by the active task. Do not use `git add -A` or include unrelated worktree changes.

---

### Task 1: Add the product-owned 1688 snapshot and legacy adapter

**Files:**

- Create: `internal/product/sourcing/a1688_snapshot.go`
- Create: `internal/integration/crawler/a1688/legacy_product_snapshot.go`
- Test: `internal/integration/crawler/a1688/legacy_product_snapshot_test.go`

- [ ] **Step 1: Write failing adapter tests**

Create table-driven tests for nil input, full field conversion, and deep-copy isolation. The full fixture must populate every field consumed by the product domain: identity and URLs, images/videos, price-range count and prices, ordering, supplier facts, specifications, details, pack facts, variations, variants, sales/review/rating, shipping, category/brand/keywords, and customization.

The isolation test must mutate the legacy source after conversion and prove the snapshot's `Images`, `Keywords`, detail images, package images, variation values, variant slice, and variant attribute map did not change:

```go
func TestSnapshotFromLegacyProductDeepCopiesMutableFields(t *testing.T) {
	legacy := populatedLegacyProduct()
	snapshot := SnapshotFromLegacyProduct(legacy)

	legacy.Images[0] = "mutated-image"
	legacy.Keywords[0] = "mutated-keyword"
	legacy.ProductDetails[0].Images[0] = "mutated-detail"
	legacy.PackInfo.PackageImages[0] = "mutated-package"
	legacy.VariationsValues[0].Values[0] = "mutated-value"
	legacy.Variants[0].Attributes["color"] = "mutated-color"

	require.Equal(t, "image-1", snapshot.Images[0])
	require.Equal(t, "keyword-1", snapshot.Keywords[0])
	require.Equal(t, "detail-image-1", snapshot.ProductDetails[0].Images[0])
	require.Equal(t, "package-image-1", snapshot.PackInfo.PackageImages[0])
	require.Equal(t, "red", snapshot.VariationValues[0].Values[0])
	require.Equal(t, "red", snapshot.Variants[0].Attributes["color"])
}
```

- [ ] **Step 2: Run the new test and confirm the intended compile failure**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/integration/crawler/a1688 -run TestSnapshotFromLegacyProduct -count=1
```

Expected: non-zero exit because `SnapshotFromLegacyProduct` and `sourcing.Alibaba1688ProductSnapshot` do not exist yet.

- [ ] **Step 3: Define the exact product-owned snapshot contract**

Add these types to `internal/product/sourcing/a1688_snapshot.go`. They intentionally omit crawler timestamps, video IDs/state, individual price-range entries, shipping methods/free-shipping, package dimensions/contents, and detail section labels because product sourcing does not consume them.

```go
package sourcing

type Alibaba1688ProductSnapshot struct {
	ID                 string
	Title              string
	URL                string
	Images             []string
	MainImage          string
	Videos             []Alibaba1688VideoSnapshot
	PriceRangeCount    int
	MinPrice           float64
	MaxPrice           float64
	Currency           string
	MinOrderQuantity   int
	Unit               string
	Supplier           Alibaba1688SupplierSnapshot
	Specifications     []Alibaba1688SpecificationSnapshot
	ProductDetails     []Alibaba1688ProductDetailSnapshot
	PackInfo           *Alibaba1688PackInfoSnapshot
	VariationValues    []Alibaba1688VariationValueSnapshot
	Variants           []Alibaba1688VariantSnapshot
	SalesVolume        int
	ReviewCount        int
	Rating             float64
	Shipping           Alibaba1688ShippingSnapshot
	Category           string
	Brand              string
	Keywords           []string
	IsCustomized       bool
}

type Alibaba1688VideoSnapshot struct {
	VideoURL string
	CoverURL string
}

type Alibaba1688SupplierSnapshot struct {
	ID              string
	Name            string
	CompanyName     string
	Location        string
	ShopURL         string
	CardType        string
	YearsInBusiness int
	Rating          float64
	ResponseRate    float64
	IsGoldSupplier  bool
	IsVerified      bool
}

type Alibaba1688SpecificationSnapshot struct {
	Name  string
	Value string
}

type Alibaba1688ProductDetailSnapshot struct {
	Content string
	Images  []string
}

type Alibaba1688PackInfoSnapshot struct {
	PackageType   string
	Weight        float64
	PackageImages []string
	Instructions  string
}

type Alibaba1688VariationValueSnapshot struct {
	Name   string
	Values []string
}

type Alibaba1688VariantSnapshot struct {
	Attributes map[string]any
	Name       string
	Image      string
	Stock      int
	Price      float64
}

type Alibaba1688ShippingSnapshot struct {
	ShippingFrom   string
	ProcessingTime string
}
```

- [ ] **Step 4: Implement the legacy-to-snapshot adapter with explicit mapping**

Add this public entry point to `internal/integration/crawler/a1688/legacy_product_snapshot.go`:

```go
func SnapshotFromLegacyProduct(product *model.Product1688) *sourcing.Alibaba1688ProductSnapshot
```

Map fields explicitly as follows:

```go
snapshot := &sourcing.Alibaba1688ProductSnapshot{
	ID:               product.ID,
	Title:            product.Title,
	URL:              product.URL,
	Images:           slices.Clone(product.Images),
	MainImage:        product.MainImage,
	PriceRangeCount:  len(product.PriceRanges),
	MinPrice:         product.MinPrice,
	MaxPrice:         product.MaxPrice,
	Currency:         product.Currency,
	MinOrderQuantity: product.MinOrderQuantity,
	Unit:             product.Unit,
	Supplier: sourcing.Alibaba1688SupplierSnapshot{
		ID: product.Supplier.ID, Name: product.Supplier.Name,
		CompanyName: product.Supplier.CompanyName, Location: product.Supplier.Location,
		ShopURL: product.Supplier.ShopURL, CardType: product.Supplier.CardType,
		YearsInBusiness: product.Supplier.YearsInBusiness, Rating: product.Supplier.Rating,
		ResponseRate: product.Supplier.ResponseRate, IsGoldSupplier: product.Supplier.IsGoldSupplier,
		IsVerified: product.Supplier.IsVerified,
	},
	SalesVolume: product.SalesVolume,
	ReviewCount: product.ReviewCount,
	Rating: product.Rating,
	Shipping: sourcing.Alibaba1688ShippingSnapshot{
		ShippingFrom: product.ShippingInfo.ShippingFrom,
		ProcessingTime: product.ShippingInfo.ProcessingTime,
	},
	Category: product.Category, Brand: product.Brand,
	Keywords: slices.Clone(product.Keywords), IsCustomized: product.IsCustomized,
}
```

Allocate and fill `Videos`, `Specifications`, `ProductDetails`, `VariationValues`, and `Variants` with indexed loops. Deep-copy each nested string slice with `slices.Clone`; deep-copy each variant `Attributes` map by allocating `make(map[string]any, len(source.Attributes))` and assigning every key/value. If `product.PackInfo != nil`, allocate `Alibaba1688PackInfoSnapshot` and copy exactly `PackageType`, `Weight`, `PackageImages`, and `Instructions`.

- [ ] **Step 5: Format and run focused tests**

Run:

```powershell
gofmt -w internal/product/sourcing/a1688_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot_test.go
$env:GOWORK='off'
go test ./internal/integration/crawler/a1688 -run TestSnapshotFromLegacyProduct -count=1
go test ./internal/product/sourcing/... ./internal/integration/crawler/a1688/... -count=1
```

Expected: both commands exit 0.

- [ ] **Step 6: Self-review and commit Task 1**

Inspect the diff for accidental crawler-field leakage and confirm only the three Task 1 files are staged:

```powershell
git diff --check
git diff -- internal/product/sourcing/a1688_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot_test.go
git add internal/product/sourcing/a1688_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot_test.go
git diff --cached --name-only
git commit -m "refactor: add 1688 source snapshot adapter"
```

### Task 2: Migrate product sourcing and enrichment off the crawler DTO

**Files:**

- Modify: `internal/product/sourcing/a1688_source_result.go`
- Modify: `internal/product/sourcing/a1688_source_result_test.go`
- Modify: `internal/product/sourcing/a1688_source_envelope.go`
- Modify: `internal/product/sourcing/a1688_source_envelope_test.go`
- Modify: `internal/product/sourcing/a1688_scraped_data.go`
- Modify: `internal/product/sourcing/a1688_scraped_data_test.go`
- Modify: `internal/product/sourcing/boundary_guard_test.go`
- Modify: `internal/productenrich/enrich/scraper_adapter.go`
- Modify: `tests/a1688_source_facts_flow_test.go`

- [ ] **Step 1: Make the product-sourcing boundary test fail on every legacy crawler import**

Rename the guard test to `TestProductSourcingDoesNotDependOnCrawlerOrIntegrationAdapters`, set forbidden prefixes to the three outer packages, and remove the model exception:

```go
forbiddenPrefixes := []string{
	"task-processor/internal/crawler",
	"task-processor/internal/integration",
	"task-processor/internal/listingkit",
}
assertProductSourcingDoesNotImportPrefixes(t, productSourcingPackageDir(t), forbiddenPrefixes)
```

Run:

```powershell
$env:GOWORK='off'
go test ./internal/product/sourcing -run TestProductSourcingDoesNotDependOnCrawlerOrIntegrationAdapters -count=1
```

Expected: non-zero exit naming the current imports in `a1688_source_result.go`, `a1688_source_envelope.go`, and `a1688_scraped_data.go`.

- [ ] **Step 2: Change product sourcing APIs to accept only the snapshot**

Use these exact signatures and types:

```go
type Alibaba1688SourceEnvelopeInput struct {
	Request     Alibaba1688CrawlRequestInput
	Product     *Alibaba1688ProductSnapshot
	RawSnapshot string
	SourceRunID string
	RequestID   string
	Error       error
}

type Alibaba1688CrawlResultInput struct {
	Product *Alibaba1688ProductSnapshot
	Error   error
}

type Alibaba1688SourceProductResult struct {
	Identity SourceIdentity
	Product  *Alibaba1688ProductSnapshot
	Error    error
}

func NormalizeAlibaba1688SourceResult(input Alibaba1688CrawlRequestInput, product *Alibaba1688ProductSnapshot, err error) Alibaba1688SourceProductResult
func NormalizeAlibaba1688BatchResults(requests []Alibaba1688CrawlRequestInput, results []Alibaba1688CrawlResultInput) []Alibaba1688SourceProductResult
func Convert1688ProductToScrapedData(product *Alibaba1688ProductSnapshot) *productenrich.ScrapedData
```

Update all private helpers in `a1688_source_envelope.go` and `a1688_scraped_data.go` to snapshot types. Replace `len(product.PriceRanges)` with `product.PriceRangeCount`, `product.ShippingInfo` with `product.Shipping`, and `VariationValue.VariantName` with `VariationValue.Name`. Preserve all normalization, warning, asset-role, supplier-fact, variant, price, SKU, and description logic unchanged.

- [ ] **Step 3: Update sourcing tests to construct snapshots**

Replace crawler-model fixtures with `Alibaba1688ProductSnapshot` fixtures in the three sourcing test files and `tests/a1688_source_facts_flow_test.go`. Remove the legacy crawler-model import from that flow test while preserving its ListingKit request assertions. For batch normalization, build `[]Alibaba1688CrawlRequestInput` and `[]Alibaba1688CrawlResultInput` and retain assertions for URL identity, request/result ordering, missing trailing results, products, and errors.

- [ ] **Step 4: Convert at the enrichment adapter boundary**

Change the last line of `scraper1688.Scrape` to:

```go
return sourcing.Convert1688ProductToScrapedData(crawler1688.SnapshotFromLegacyProduct(product)), nil
```

There is no injectable processor seam in `scraper1688`, so do not add an integration-runtime fake for this one-line composition change. Task 1 covers the legacy conversion and the existing sourcing tests cover scraped-data conversion.

- [ ] **Step 5: Format and verify the dependency direction**

Run:

```powershell
gofmt -w internal/product/sourcing/a1688_source_result.go internal/product/sourcing/a1688_source_result_test.go internal/product/sourcing/a1688_source_envelope.go internal/product/sourcing/a1688_source_envelope_test.go internal/product/sourcing/a1688_scraped_data.go internal/product/sourcing/a1688_scraped_data_test.go internal/product/sourcing/boundary_guard_test.go internal/productenrich/enrich/scraper_adapter.go
$env:GOWORK='off'
go test ./internal/product/sourcing/... ./internal/productenrich/enrich/... ./internal/integration/crawler/a1688/... -count=1
rg -n '"task-processor/internal/(crawler|integration|listingkit)' internal/product/sourcing -g '*.go' -g '!*_test.go'
```

Expected: tests exit 0 and `rg` returns no matches. A no-match `rg` exit code of 1 is success for this check.

- [ ] **Step 6: Self-review and commit Task 2**

Run:

```powershell
git diff --check
git add internal/product/sourcing/a1688_source_result.go internal/product/sourcing/a1688_source_result_test.go internal/product/sourcing/a1688_source_envelope.go internal/product/sourcing/a1688_source_envelope_test.go internal/product/sourcing/a1688_scraped_data.go internal/product/sourcing/a1688_scraped_data_test.go internal/product/sourcing/boundary_guard_test.go internal/productenrich/enrich/scraper_adapter.go
git diff --cached --name-only
git commit -m "refactor: decouple product sourcing from 1688 crawler DTOs"
```

### Task 3: Move ListingKit handoff ownership to the compatibility layer

**Files:**

- Move: `internal/product/sourcehandoff/listingkit_request.go` -> `internal/compatibility/listingkit/sourcehandoff/listingkit_request.go`
- Move: `internal/product/sourcehandoff/listingkit_request_test.go` -> `internal/compatibility/listingkit/sourcehandoff/listingkit_request_test.go`
- Move: `internal/product/sourcehandoff/a1688/command.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/command.go`
- Move: `internal/product/sourcehandoff/a1688/command_test.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/command_test.go`
- Move: `internal/product/sourcehandoff/a1688/listingkit_task.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/listingkit_task.go`
- Move: `internal/product/sourcehandoff/a1688/listingkit_task_test.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/listingkit_task_test.go`
- Move: `internal/product/sourcehandoff/a1688/httpapi/handler.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/handler.go`
- Move: `internal/product/sourcehandoff/a1688/httpapi/handler_test.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/handler_test.go`
- Move: `internal/product/sourcehandoff/a1688/httpapi/routes.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/routes.go`
- Move: `internal/product/sourcehandoff/a1688/httpapi/http_module.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/http_module.go`
- Move: `internal/product/sourcehandoff/a1688/httpapi/http_module_test.go` -> `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/http_module_test.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `internal/app/httpapi/http_module_test.go`
- Modify: `tests/a1688_source_to_task_flow_test.go`
- Modify: `tests/import_boundaries_test.go`

- [ ] **Step 1: Add a failing whole-product import boundary test**

Add this test beside the existing domain-boundary tests in `tests/import_boundaries_test.go`:

```go
func TestProductDomainDoesNotDependOnOuterAdapters(t *testing.T) {
	t.Parallel()

	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "product"), []string{
		"task-processor/internal/listingkit",
		"task-processor/internal/compatibility",
		"task-processor/internal/crawler",
		"task-processor/internal/integration",
	}, nil)
}
```

Run:

```powershell
$env:GOWORK='off'
go test ./tests -run TestProductDomainDoesNotDependOnOuterAdapters -count=1
```

Expected: non-zero exit naming the current `internal/product/sourcehandoff` production files.

- [ ] **Step 2: Move the source handoff tree without compatibility aliases**

Create `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi`, move each file listed above, and keep the current package names `sourcehandoff`, `a1688`, and `httpapi`. Update internal imports to:

```go
"task-processor/internal/compatibility/listingkit/sourcehandoff"
a1688 "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688"
a1688httpapi "task-processor/internal/compatibility/listingkit/sourcehandoff/a1688/httpapi"
```

Do not leave Go files, aliases, forwarding functions, or forwarding packages under `internal/product/sourcehandoff`.

- [ ] **Step 3: Convert the legacy product before entering product sourcing**

Keep this field in the moved command so JSON behavior remains stable:

```go
Product *alibaba1688model.Product1688
```

Add the adapter import:

```go
crawler1688 "task-processor/internal/integration/crawler/a1688"
```

Change the source-envelope input to:

```go
Source: sourcing.Alibaba1688SourceEnvelopeInput{
	Request: sourcing.Alibaba1688CrawlRequestInput{
		URL:       url,
		AccountID: command.SourceAccountID,
	},
	Product:     crawler1688.SnapshotFromLegacyProduct(command.Product),
	RawSnapshot: command.RawSnapshot,
	SourceRunID: command.SourceRunID,
	RequestID:   command.RequestID,
	Error:       command.Error,
},
```

Retain the existing identity and two-store validation order before conversion/task creation.

- [ ] **Step 4: Update every production and test import to the new owner**

Update `internal/app/httpapi/composition_builder.go`, `internal/app/httpapi/types.go`, `internal/app/httpapi/http_module_test.go`, `tests/a1688_source_to_task_flow_test.go`, and all moved files. Discover any additional live imports with:

```powershell
rg -n 'task-processor/internal/product/sourcehandoff' . -g '*.go'
```

Every match in current Go code must be changed to the compatibility path; dated plans/specs are handled separately in Task 4 and are not executable imports.

Update all moved handoff test fixtures that populate `sourcing.Alibaba1688SourceEnvelopeInput.Product` to use `sourcing.Alibaba1688ProductSnapshot` values. The moved tests must retain their existing identity, warning, task request, HTTP status, response-shape, and store-access assertions; they must not reintroduce the legacy crawler model merely to construct a test input.

- [ ] **Step 5: Format and run focused handoff, composition, and boundary tests**

Run:

```powershell
gofmt -w (Get-ChildItem internal/compatibility/listingkit/sourcehandoff -Recurse -Filter *.go).FullName internal/app/httpapi/composition_builder.go internal/app/httpapi/types.go internal/app/httpapi/http_module_test.go tests/a1688_source_to_task_flow_test.go tests/import_boundaries_test.go
$env:GOWORK='off'
go test ./internal/compatibility/listingkit/sourcehandoff/... ./internal/app/httpapi ./tests -run 'Test(ProductDomainDoesNotDependOnOuterAdapters|Alibaba1688|TaskCommandService|CreateListingKitTask|HTTPModule)' -count=1
go test ./internal/product/sourcing/... ./internal/integration/crawler/a1688/... ./internal/compatibility/listingkit/sourcehandoff/... ./internal/app/httpapi ./tests -count=1
if (Test-Path internal/product/sourcehandoff) { throw 'old product sourcehandoff path still exists' }
if (rg -n '"task-processor/internal/(listingkit|compatibility|crawler|integration)' internal/product -g '*.go' -g '!*_test.go') { throw 'product domain still imports an outer adapter' }
```

Expected: all tests exit 0, the old path is absent, and the product-domain import scan produces no matches.

- [ ] **Step 6: Self-review and commit Task 3**

Use scoped staging for the moved source and its known consumers:

```powershell
git diff --check
git add -- internal/compatibility/listingkit/sourcehandoff internal/app/httpapi/composition_builder.go internal/app/httpapi/types.go internal/app/httpapi/http_module_test.go tests/a1688_source_to_task_flow_test.go tests/import_boundaries_test.go
git add -u -- internal/product/sourcehandoff
git diff --cached --name-status
git commit -m "refactor: move 1688 handoff to listingkit compatibility"
```

### Task 4: Align authoritative architecture and feature documentation

**Files:**

- Modify: `docs/architecture/project-target-architecture.md`
- Modify: `docs/architecture/project-boundaries.md`
- Modify: `internal/integration/crawler/a1688/README.md`
- Modify: `internal/product/sourcing/README.md`
- Modify: `internal/compatibility/listingkit/README.md`
- Modify: `docs/refactoring/current-refactoring-status.md`
- Modify: `docs/architecture/pay-041-usage-ledger.md`
- Modify: `docs/product/product-sourcing-mvp-plan.md`
- Modify: `tests/architecture_docs_test.go`

- [ ] **Step 1: Strengthen documentation assertions before changing documentation**

Add `TestProjectBoundaryDocumentDefines1688SourceHandoffOwnership` to `tests/architecture_docs_test.go`. It must read `docs/architecture/project-boundaries.md` and require these exact text spans:

```text
internal/product must not import internal/listingkit, internal/compatibility, internal/crawler, or internal/integration
internal/integration/crawler/a1688 converts legacy crawler DTOs into internal/product/sourcing snapshots
internal/compatibility/listingkit/sourcehandoff owns the 1688 to ListingKit application handoff
```

Use this test body:

```go
func TestProjectBoundaryDocumentDefines1688SourceHandoffOwnership(t *testing.T) {
	path := filepath.Join("..", "docs", "architecture", "project-boundaries.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	for _, phrase := range []string{
		"internal/product must not import internal/listingkit, internal/compatibility, internal/crawler, or internal/integration",
		"internal/integration/crawler/a1688 converts legacy crawler DTOs into internal/product/sourcing snapshots",
		"internal/compatibility/listingkit/sourcehandoff owns the 1688 to ListingKit application handoff",
	} {
		if !strings.Contains(string(content), phrase) {
			t.Errorf("%s must mention %q", path, phrase)
		}
	}
}
```

Run:

```powershell
$env:GOWORK='off'
go test ./tests -run 'Test(.*Architecture.*Doc|ProjectBoundaryDocumentDefines1688SourceHandoffOwnership)' -count=1
```

Expected: non-zero exit until the authoritative docs contain the new dependency rule and ownership paths.

- [ ] **Step 2: Update current architecture and component documentation**

Update the eight listed documents consistently:

- Product owns `SourceEnvelope`, the 1688 snapshot, normalization, and enrichment conversion.
- `internal/integration/crawler/a1688` may depend inward on the narrow `internal/product/sourcing` snapshot contract and owns legacy DTO conversion.
- `internal/compatibility/listingkit/sourcehandoff` owns cross-boundary orchestration, HTTP/application command compatibility, store validation, and ListingKit request creation.
- Production `internal/product/**` cannot import ListingKit, compatibility, crawler, or integration packages.
- The HTTP route and external JSON contract remain unchanged.

Add `TestProductDomainDoesNotDependOnOuterAdapters` to the `Current Enforcement` list in `project-boundaries.md`, because `TestArchitectureReviewChecklistTracksEveryImportBoundaryGuard` requires every active import guard to be documented.

Correct references to the former `internal/product/sourcehandoff` path in current/authoritative docs. Do not rewrite dated records under `docs/superpowers/plans`, old specs, or validation evidence; those documents describe historical repository states.

- [ ] **Step 3: Verify documentation consistency**

Run:

```powershell
$env:GOWORK='off'
go test ./tests -run 'Test(.*Architecture.*Doc|ProjectBoundaryDocumentDefines1688SourceHandoffOwnership)' -count=1
rg -n 'internal/product/sourcehandoff' docs/architecture docs/product docs/refactoring internal/product/sourcing/README.md internal/integration/crawler/a1688/README.md internal/compatibility/listingkit/README.md
```

Expected: the documentation test exits 0. Inspect any `rg` matches and remove stale current-state ownership references; historical migration wording may mention the old path only when explicitly labeled as former.

- [ ] **Step 4: Self-review and commit Task 4**

Run:

```powershell
git diff --check
git add docs/architecture/project-target-architecture.md docs/architecture/project-boundaries.md internal/integration/crawler/a1688/README.md internal/product/sourcing/README.md internal/compatibility/listingkit/README.md docs/refactoring/current-refactoring-status.md docs/architecture/pay-041-usage-ledger.md docs/product/product-sourcing-mvp-plan.md tests/architecture_docs_test.go
git diff --cached --name-only
git commit -m "docs: align 1688 source handoff ownership"
```

### Task 5: Verify the complete refactor and branch readiness

**Files:**

- No planned production changes; fix only evidence-backed regressions found by this task, with a focused failing test before each fix.

- [ ] **Step 1: Run focused package suites**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/product/sourcing/... ./internal/integration/crawler/a1688/... -count=1
go test ./internal/compatibility/listingkit/sourcehandoff/... ./internal/app/httpapi/... -count=1
go test ./tests/... -count=1
```

Expected: every command exits 0.

- [ ] **Step 2: Run the root module suite and build affected binaries**

Run:

```powershell
$env:GOWORK='off'
go test ./... -count=1
go build ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing
```

Expected: tests and builds exit 0.

- [ ] **Step 3: Prove the dependency boundary and removal of the old owner**

Run:

```powershell
if (rg -n '"task-processor/internal/(listingkit|compatibility|crawler|integration)' internal/product -g '*.go' -g '!*_test.go') { throw 'product domain still imports an outer adapter' }
if (Test-Path internal/product/sourcehandoff) { throw 'old product sourcehandoff path still exists' }
rg -n 'task-processor/internal/product/sourcehandoff' . -g '*.go'
```

Expected: all three checks produce no matches and no exception.

- [ ] **Step 4: Review branch scope and cleanliness**

Run:

```powershell
git diff --check origin/master...HEAD
git log --oneline origin/master..HEAD
git status --short --branch
```

Expected: `git diff --check` exits 0, the log contains the design/plan and four scoped implementation commits, and status shows a clean `codex/decouple-1688-source-handoff` branch.

- [ ] **Step 5: Handle verification findings without broadening scope**

If a verification command fails, identify the exact regression, add a focused failing test when coverage is missing, fix only that regression, rerun its package test and then all Task 5 commands, and create a scoped commit whose message names the regression. Do not deploy, push, open a PR, or modify GitHub state without separate user authorization.
