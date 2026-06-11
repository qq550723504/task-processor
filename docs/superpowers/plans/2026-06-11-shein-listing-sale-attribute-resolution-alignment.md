# SHEIN Listing Sale Attribute Resolution Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `shein-listing` heuristic primary/secondary sale attribute selection with a resolver-backed resolution phase while keeping the current SKC/SKU build and submit pipeline intact.

**Architecture:** Add a dedicated pipeline handler after `sale_attribute` that adapts `shein-listing` runtime data into `internal/publishing/shein` resolver inputs, stores a narrow `SaleAttributeSelectionState` on `TaskContext`, and teaches the SKC builder to prefer a strategy derived from that selection. Preserve the legacy strategy path as an explicit fallback for early rollout and comparison.

**Tech Stack:** Go, existing `internal/shein` pipeline handlers, `internal/publishing/shein` resolver stack, `go test`

---

## File Structure

**Create:**
- `internal/shein/product/attribute/sale_resolution_handler.go`
- `internal/shein/product/skc/resolution_strategy.go`
- `internal/shein/product/skc/resolution_strategy_test.go`
- `internal/shein/product/attribute/sale_resolution_handler_test.go`

**Modify:**
- `internal/shein/context/context.go`
- `internal/shein/pipeline/pipeline.go`
- `internal/shein/product/skc/builder.go`

**Read for implementation details:**
- `internal/publishing/shein/runtime_sale_attribute_resolver.go`
- `internal/publishing/shein/sale_attribute_resolver.go`
- `internal/publishing/shein/model.go`
- `internal/publishing/shein/source_dimensions.go`
- `internal/shein/product/attribute/sale/handler.go`
- `internal/shein/product/skc/attribute_strategy.go`
- `internal/shein/product/skc/skc_build_input.go`

**Test targets:**
- `go test ./internal/shein/product/attribute -run "TestSaleAttributeResolutionHandler|TestBuildCanonical"`
- `go test ./internal/shein/product/skc -run "TestBuildStrategyFromResolution|TestSKCBuilderUsesResolutionFallback"`
- `go test ./internal/publishing/shein -run "Test.*SaleAttribute.*|Test.*SourceDimension.*"`

---

### Task 1: Add Cycle-Free Sale Attribute Selection State To `TaskContext`

**Files:**
- Modify: `internal/shein/context/context.go`
- Test: `internal/shein/product/attribute/sale_resolution_handler_test.go`

- [ ] **Step 1: Write the failing context-storage test**

```go
func TestTaskContextStoresSaleAttributeSelectionState(t *testing.T) {
	t.Parallel()

	ctx := sheinctx.NewTaskContext(context.Background(), nil)
	selection := &sheinctx.SaleAttributeSelectionState{
		Source:                   "resolver",
		PrimaryAttributeID:       27,
		SecondaryAttributeID:     87,
		PrimarySourceDimension:   "Color",
		SecondarySourceDimension: "Size",
	}

	if ctx.SaleAttributeSelection != nil {
		t.Fatalf("SaleAttributeSelection = %+v, want nil before storage", ctx.SaleAttributeSelection)
	}

	ctx.SetSaleAttributeSelection(selection)

	if ctx.SaleAttributeSelection != selection {
		t.Fatalf("SaleAttributeSelection = %+v, want %+v", ctx.SaleAttributeSelection, selection)
	}
	if ctx.SaleAttributeSelection.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", ctx.SaleAttributeSelection.PrimaryAttributeID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shein/context ./internal/shein/product/attribute -run "TestTaskContextStoresSaleAttributeSelectionState"`

Expected: FAIL with missing `SaleAttributeSelection` field or missing setter/type errors.

- [ ] **Step 3: Add the minimal `TaskContext` storage fields**

```go
type ProductState struct {
	SupplierInfo            *other.SupplierOperateInfo
	SpuLimitCount           *other.SpuLimitCountInfo
	ShelfQuotaInfo          *other.ShelfQuotaInfo
	AmazonProduct           *model.Product
	Variants                *[]model.Product
	UnFilteredVariants      *[]model.Product
	VariantFilterMap        map[string]*VariantFilterInfo
	AsinSkuMap              map[string]string
	SupplierSkuMap          map[string]string
	ProductData             *product.Product
	FilterRule              *managementapi.FilterRuleRespDTO
	ProfitRule              *managementapi.ProfitRuleRespDTO
	Warehouses              *warehouse.WarehouseResponse
	SiteList                []product.SiteInfo
	CategoryTree            *sheincategory.CategoryTreeResponse
	AttributeTemplates      *sheinattribute.AttributeTemplateInfo
	BuildAttributeData      *BuildAttributeInfo
	GenerateAttribute       *AttributeData
	SaleSpecResult          *ResultSaleAttribute
	SaleAttributeSelection  *SaleAttributeSelectionState
	SaleAttributeStrategySource string
}

type SaleAttributeSelectionState struct {
	Source                   string
	PrimaryAttributeID       int
	SecondaryAttributeID     int
	PrimarySourceDimension   string
	SecondarySourceDimension string
}

func (ctx *TaskContext) SetSaleAttributeSelection(selection *SaleAttributeSelectionState) {
	ctx.SaleAttributeSelection = selection
}
```

- [ ] **Step 4: Run test to verify the storage scaffolding compiles**

Run: `go test ./internal/shein/context ./internal/shein/product/attribute -run "TestTaskContextStoresSaleAttributeSelectionState"`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/shein/context/context.go
git commit -m "refactor: add shein sale attribute selection state to task context"
```

### Task 2: Add The Resolver Adapter Pipeline Handler

**Files:**
- Create: `internal/shein/product/attribute/sale_resolution_handler.go`
- Test: `internal/shein/product/attribute/sale_resolution_handler_test.go`
- Read: `internal/publishing/shein/runtime_sale_attribute_resolver.go`
- Read: `internal/publishing/shein/model.go`
- Read: `internal/catalog/canonical/types.go`

- [ ] **Step 1: Write the failing adapter tests**

```go
func TestBuildCanonicalProductFromTaskContext(t *testing.T) {
	t.Parallel()

	ctx := makeSaleResolutionTaskContext()
	canonicalProduct, req, pkg, err := buildSaleAttributeResolutionInput(ctx)
	if err != nil {
		t.Fatalf("buildSaleAttributeResolutionInput() error = %v", err)
	}
	if canonicalProduct == nil || len(canonicalProduct.Variants) != 2 {
		t.Fatalf("canonical variants = %#v, want 2 variants", canonicalProduct)
	}
	if len(canonicalProduct.VariantDimensions) != 2 {
		t.Fatalf("variant dimensions = %#v, want Color and Size", canonicalProduct.VariantDimensions)
	}
	if req.SheinStoreID <= 0 {
		t.Fatalf("SheinStoreID = %d, want positive store id", req.SheinStoreID)
	}
	if pkg.CategoryID != 123 {
		t.Fatalf("package category id = %d, want 123", pkg.CategoryID)
	}
}

func TestSaleAttributeResolutionHandlerFallsBackWhenResolverInputIsIncomplete(t *testing.T) {
	t.Parallel()

	ctx := sheinctx.NewTaskContext(context.Background(), &model.Task{Region: "us"})
	handler := NewSaleAttributeResolutionHandler(newStubRuntimeResolver(nil))

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v, want nil fallback", err)
	}
	if ctx.SaleAttributeSelection != nil {
		t.Fatalf("selection = %+v, want nil fallback state", ctx.SaleAttributeSelection)
	}
	if ctx.SaleAttributeStrategySource != "legacy" {
		t.Fatalf("strategy source = %q, want legacy", ctx.SaleAttributeStrategySource)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shein/product/attribute -run "TestBuildCanonicalProductFromTaskContext|TestSaleAttributeResolutionHandlerFallsBackWhenResolverInputIsIncomplete"`

Expected: FAIL with undefined `buildSaleAttributeResolutionInput`, missing handler, or missing resolver stub types.

- [ ] **Step 3: Implement the minimal adapter and handler**

```go
type saleAttributeRuntimeResolver interface {
	Resolve(req *sheinpub.BuildRequest, canonical *canonical.Product, pkg *sheinpub.Package) *sheinpub.SaleAttributeResolution
}

type SaleAttributeResolutionHandler struct {
	resolver saleAttributeRuntimeResolver
}

func NewSaleAttributeResolutionHandler(resolver saleAttributeRuntimeResolver) *SaleAttributeResolutionHandler {
	return &SaleAttributeResolutionHandler{resolver: resolver}
}

func (h *SaleAttributeResolutionHandler) Name() string { return "sale_attribute_resolution" }

func (h *SaleAttributeResolutionHandler) Handle(ctx *sheinctx.TaskContext) error {
	if h == nil || h.resolver == nil || ctx == nil {
		if ctx != nil {
			ctx.SetSaleAttributeSelection(nil)
		}
		return nil
	}
	canonicalProduct, req, pkg, err := buildSaleAttributeResolutionInput(ctx)
	if err != nil {
		ctx.SetSaleAttributeSelection(nil)
		return nil
	}
	resolution := h.resolver.Resolve(req, canonicalProduct, pkg)
	if resolution == nil || resolution.PrimaryAttributeID <= 0 {
		ctx.SetSaleAttributeSelection(nil)
		return nil
	}
	ctx.SetSaleAttributeSelection(&sheinctx.SaleAttributeSelectionState{
		Source:                   "resolution",
		PrimaryAttributeID:       resolution.PrimaryAttributeID,
		SecondaryAttributeID:     resolution.SecondaryAttributeID,
		PrimarySourceDimension:   resolution.PrimarySourceDimension,
		SecondarySourceDimension: resolution.SecondarySourceDimension,
	})
	return nil
}
```

- [ ] **Step 4: Implement the minimal input builder**

```go
func buildSaleAttributeResolutionInput(ctx *sheinctx.TaskContext) (*canonical.Product, *sheinpub.BuildRequest, *sheinpub.Package, error) {
	if ctx == nil || ctx.AmazonProduct == nil || ctx.ProductData == nil || ctx.ProductData.CategoryID == 0 {
		return nil, nil, nil, fmt.Errorf("sale attribute resolution input is incomplete")
	}

	canonicalVariants := buildCanonicalVariants(ctx)
	canonicalProduct := &canonical.Product{
		Title:             strings.TrimSpace(ctx.AmazonProduct.Title),
		Brand:             strings.TrimSpace(ctx.AmazonProduct.Brand),
		Description:       strings.TrimSpace(ctx.AmazonProduct.Description),
		CategoryPath:      append([]string(nil), ctx.AmazonProduct.Categories...),
		Variants:          canonicalVariants,
		VariantDimensions: buildCanonicalVariantDimensions(ctx),
		Images:            buildCanonicalImages(ctx.AmazonProduct.Images),
	}

	storeID := int64(0)
	if ctx.StoreInfo != nil {
		storeID = ctx.StoreInfo.Id
	}
	req := &sheinpub.BuildRequest{
		Country:      strings.ToUpper(strings.TrimSpace(ctx.Task.Region)),
		Language:     "en",
		Text:         strings.TrimSpace(ctx.AmazonProduct.Title),
		SheinStoreID: storeID,
		Context:      ctx.Context,
	}
	pkg := &sheinpub.Package{
		CategoryID:    ctx.ProductData.CategoryID,
		SpuName:       strings.TrimSpace(ctx.ProductData.SPUName),
		ProductNameEn: strings.TrimSpace(ctx.AmazonProduct.Title),
	}
	return canonicalProduct, req, pkg, nil
}
```

- [ ] **Step 5: Run tests to verify the handler passes**

Run: `go test ./internal/shein/product/attribute -run "TestBuildCanonicalProductFromTaskContext|TestSaleAttributeResolutionHandlerFallsBackWhenResolverInputIsIncomplete|TestSaleAttributeResolutionHandlerStoresSelectionOnTaskContext"`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/shein/product/attribute/sale_resolution_handler.go internal/shein/product/attribute/sale_resolution_handler_test.go
git commit -m "feat: add shein sale attribute resolution adapter handler"
```

### Task 3: Insert The New Handler Into The SHEIN Pipeline

**Files:**
- Modify: `internal/shein/pipeline/pipeline.go`
- Test: `internal/shein/product/attribute/sale_resolution_handler_test.go`

- [ ] **Step 1: Write the failing pipeline-order test**

```go
func TestCreateTaskProcessingPipelineInsertsSaleAttributeResolutionBeforeBuildSkcList(t *testing.T) {
	t.Parallel()

	p := CreateTaskProcessingPipeline(newStubSheinProcessor(), newStubConfig())
	names := make([]string, 0, len(p.Handlers()))
	for _, handler := range p.Handlers() {
		names = append(names, handler.Name())
	}

	saleIndex := slices.Index(names, "sale_attribute")
	resolutionIndex := slices.Index(names, "sale_attribute_resolution")
	buildIndex := slices.Index(names, "build_skc_list")

	if saleIndex < 0 || resolutionIndex < 0 || buildIndex < 0 {
		t.Fatalf("handler order = %v, want sale_attribute, sale_attribute_resolution, build_skc_list present", names)
	}
	if !(saleIndex < resolutionIndex && resolutionIndex < buildIndex) {
		t.Fatalf("handler order = %v, want sale_attribute < sale_attribute_resolution < build_skc_list", names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shein/pipeline -run "TestCreateTaskProcessingPipelineInsertsSaleAttributeResolutionBeforeBuildSkcList"`

Expected: FAIL because `sale_attribute_resolution` is missing from the pipeline.

- [ ] **Step 3: Wire the handler into the pipeline**

```go
resolver := sheinpub.NewRuntimeSaleAttributeResolver(
	newSheinRuntimeAPIClientFactory(processor.GetManagementClient()),
	aiClient,
)

pipeline.AddHandler(sale.NewSaleAttributeHandler(aiClient))
pipeline.AddHandler(attribute.NewValidateRepairSaleAttributeHandler())
pipeline.AddHandler(attribute.NewSaleAttributeResolutionHandler(resolver))
pipeline.AddHandler(build.NewBuildSkcListHandler(imageDownloder, aiClient))
```

- [ ] **Step 4: Run the pipeline-order test**

Run: `go test ./internal/shein/pipeline -run "TestCreateTaskProcessingPipelineInsertsSaleAttributeResolutionBeforeBuildSkcList"`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/shein/pipeline/pipeline.go
git commit -m "feat: add shein sale attribute resolution phase to pipeline"
```

### Task 4: Convert Selection State Into SKC Strategy With Legacy Fallback

**Files:**
- Create: `internal/shein/product/skc/resolution_strategy.go`
- Create: `internal/shein/product/skc/resolution_strategy_test.go`
- Modify: `internal/shein/product/skc/builder.go`
- Read: `internal/shein/product/skc/attribute_strategy.go`
- Read: `internal/shein/product/skc/skc_build_input.go`

- [ ] **Step 1: Write the failing resolution-strategy tests**

```go
func TestBuildStrategyFromSelectionUsesResolvedPrimaryAndSecondaryDimensions(t *testing.T) {
	t.Parallel()

	ctx := makeResolutionBackedTaskContext()
	strategy, source, err := BuildStrategyFromSelection(ctx, ctx.SaleSpecResult)
	if err != nil {
		t.Fatalf("BuildStrategyFromSelection() error = %v", err)
	}
	if source != "resolution" {
		t.Fatalf("source = %q, want resolution", source)
	}
	if strategy.PrimaryAttribute.AttrID != 27 {
		t.Fatalf("primary attr id = %d, want 27", strategy.PrimaryAttribute.AttrID)
	}
	if strategy.SecondaryAttribute.AttrID != 87 {
		t.Fatalf("secondary attr id = %d, want 87", strategy.SecondaryAttribute.AttrID)
	}
}

func TestBuildSKCListWithSpecAdaptationFallsBackToLegacyWhenSelectionIsMissing(t *testing.T) {
	t.Parallel()

	builder := newTestSKCBuilder()
	input, ctx := makeLegacyFallbackSKCInput()
	out, err := builder.BuildSKCListWithSpecAdaptation(input, ctx, NewAttributeStrategyHandler())
	if err != nil {
		t.Fatalf("BuildSKCListWithSpecAdaptation() error = %v", err)
	}
	if out == nil || out.IsEmpty() {
		t.Fatal("expected fallback output")
	}
	if ctx.SaleAttributeStrategySource != "legacy" {
		t.Fatalf("strategy source = %q, want legacy", ctx.SaleAttributeStrategySource)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/shein/product/skc -run "TestBuildStrategyFromSelectionUsesResolvedPrimaryAndSecondaryDimensions|TestBuildSKCListWithSpecAdaptationFallsBackToLegacyWhenSelectionIsMissing"`

Expected: FAIL with undefined `BuildStrategyFromSelection` or unchanged legacy-only behavior.

- [ ] **Step 3: Implement the resolution-to-strategy conversion**

```go
func BuildStrategyFromSelection(ctx *shein.TaskContext, saleSpec *shein.ResultSaleAttribute) (sheinattr.AttributeStrategy, string, error) {
	if ctx == nil || ctx.SaleAttributeSelection == nil || saleSpec == nil {
		return sheinattr.AttributeStrategy{}, "legacy", fmt.Errorf("sale attribute selection is unavailable")
	}
	selection := ctx.SaleAttributeSelection
	primary := findResultAttributeByDimensionOrID(saleSpec, selection.PrimarySourceDimension, selection.PrimaryAttributeID)
	if primary.AttrID <= 0 || len(primary.AttrValue) == 0 {
		return sheinattr.AttributeStrategy{}, "legacy", fmt.Errorf("resolved primary attribute could not be materialized")
	}
	secondary := findResultAttributeByDimensionOrID(saleSpec, selection.SecondarySourceDimension, selection.SecondaryAttributeID)

	strategyType := "resolution_primary_only"
	if secondary.AttrID > 0 && len(secondary.AttrValue) > 0 {
		strategyType = "resolution_primary_secondary"
	}

	return sheinattr.AttributeStrategy{
		PrimaryAttribute:   primary,
		SecondaryAttribute: secondary,
		StrategyType:       strategyType,
	}, "resolution", nil
}
```

- [ ] **Step 4: Teach the SKC builder to prefer resolution and log fallback**

```go
config := strategyHandler.GetDynamicAttributePriorityConfig(input.AttributeTemplates)
strategy, strategySource, err := BuildStrategyFromSelection(ctx, &input.SaleAttributeOutput.Result)
if err != nil {
	strategy = strategyHandler.DetermineAttributeStrategy(input.SaleAttributeOutput.Result, config, input.AttributeTemplates)
	strategySource = "legacy"
}
ctx.SaleAttributeStrategySource = strategySource

logger.GetGlobalLogger("shein/product").Infof(
	"SKC strategy selected: source=%s primary=%d secondary=%d type=%s",
	strategySource,
	strategy.PrimaryAttribute.AttrID,
	strategy.SecondaryAttribute.AttrID,
	strategy.StrategyType,
)
```

- [ ] **Step 5: Run SKC tests to verify resolution and fallback behavior**

Run: `go test ./internal/shein/product/skc -run "TestBuildStrategyFromSelectionUsesResolvedPrimaryAndSecondaryDimensions|TestBuildSKCListWithSpecAdaptationFallsBackToLegacyWhenSelectionIsMissing|TestBuildSKUListForSingleVariantWithRuntime|TestValidateAttributeStrategy"`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/shein/product/skc/resolution_strategy.go internal/shein/product/skc/resolution_strategy_test.go internal/shein/product/skc/builder.go
git commit -m "feat: prefer resolved shein sale attributes for skc strategy"
```

### Task 5: Add Focused Regression Coverage And Verification Commands

**Files:**
- Modify: `internal/shein/product/attribute/sale_resolution_handler_test.go`
- Modify: `internal/shein/product/skc/resolution_strategy_test.go`
- Test: `internal/publishing/shein/sale_attribute_resolver_test.go`

- [ ] **Step 1: Add a regression test for a known wrong-primary legacy case**

```go
func TestBuildStrategyFromSelectionPrefersTemplateMatchedPrimaryOverLegacyColorFallback(t *testing.T) {
	t.Parallel()

	ctx := makeResolutionBackedTaskContext()
	ctx.SaleAttributeSelection.PrimaryAttributeID = 1001184
	ctx.SaleAttributeSelection.PrimarySourceDimension = "Style"
	ctx.SaleAttributeSelection.SecondaryAttributeID = 87
	ctx.SaleAttributeSelection.SecondarySourceDimension = "Size"
	ctx.SetSaleSpecResult(&shein.ResultSaleAttribute{
		SaleAttributes: []shein.ResultAttribute{
			{AttrID: 27, AttrValue: []shein.AttributeValue{{ID: 11, Value: "White"}}},
			{AttrID: 87, AttrValue: []shein.AttributeValue{{ID: 21, Value: "S"}, {ID: 22, Value: "M"}}},
			{AttrID: 1001184, AttrValue: []shein.AttributeValue{{ID: 31, Value: "Bandana"}, {ID: 32, Value: "Bow"}}},
		},
	})

	strategy, source, err := BuildStrategyFromSelection(ctx, ctx.SaleSpecResult)
	if err != nil {
		t.Fatalf("BuildStrategyFromSelection() error = %v", err)
	}
	if source != "resolution" {
		t.Fatalf("source = %q, want resolution", source)
	}
	if strategy.PrimaryAttribute.AttrID != 1001184 {
		t.Fatalf("primary attr id = %d, want 1001184", strategy.PrimaryAttribute.AttrID)
	}
}
```

- [ ] **Step 2: Run focused regression tests**

Run: `go test ./internal/shein/product/attribute ./internal/shein/product/skc -run "TestSaleAttributeResolutionHandler|TestBuildStrategyFromSelection"`

Expected: PASS

- [ ] **Step 3: Run broader resolver and SKC verification**

Run: `go test ./internal/publishing/shein -run "Test.*SaleAttribute.*|Test.*SourceDimension.*"`

Expected: PASS

Run: `go test ./internal/shein/product/skc ./internal/shein/product/attribute ./internal/shein/pipeline`

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/shein/product/attribute/sale_resolution_handler_test.go internal/shein/product/skc/resolution_strategy_test.go
git commit -m "test: cover shein sale attribute resolution alignment regressions"
```

## Self-Review

- **Spec coverage:** The plan covers the dedicated resolver phase, thin adapter, `TaskContext` storage, resolution-first SKC strategy construction, explicit legacy fallback, and targeted validation. The future single-SKC splitting feature is intentionally excluded.
- **Placeholder scan:** No `TODO`, `TBD`, or "handle appropriately" placeholders remain. Each task has explicit files, code, commands, and expected outcomes.
- **Type consistency:** The plan keeps `TaskContext` free of publishing-package imports by storing only a narrow `SaleAttributeSelectionState`, while the resolver handler remains the only place that translates from `sheinpub.SaleAttributeResolution` into downstream SKC selection inputs.

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-06-11-shein-listing-sale-attribute-resolution-alignment.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
