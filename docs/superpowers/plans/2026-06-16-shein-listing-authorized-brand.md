# SHEIN Listing Authorized Brand Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add store-scoped authorized-brand behavior to `shein-listing` so authorized stores publish with a configured SHEIN `brand_code` and preserve only that approved brand in title, description, and SKC title cleanup.

**Architecture:** Extend the management store DTO with authorized-brand fields, resolve the configured brand through the new SHEIN `QueryBrandList()` API once product APIs are available, and carry the resolved brand through both `TaskContext` and `context.Context`. Apply the resolved `brand_code` at final SHEIN payload mutation points and teach sensitive-word cleanup to preserve only the authorized brand while continuing to strip other brands.

**Tech Stack:** Go, existing SHEIN product API client, management DTO/local provider, context helpers, `go test`

---

## File Structure

### Existing files to modify

- `internal/infra/clients/management/api/store.go`
  Purpose: add store-facing authorized-brand fields to `StoreRespDTO`.
- `internal/infra/clients/management/local_data_provider.go`
  Purpose: keep local debug store loading aligned with the remote management DTO.
- `internal/shein/context/context.go`
  Purpose: store resolved authorized-brand state on `TaskContext`.
- `internal/shein/pipeline/task.go`
  Purpose: resolve authorized brand after `ProductAPI` is initialized and attach it to runtime context and task context.
- `internal/publishing/shein/assembler.go`
  Purpose: prefer resolved authorized-brand display name and metadata when building the package.
- `internal/publishing/shein/preview_adapter.go`
  Purpose: write resolved `brand_code` into final `sheinproduct.Product`.
- `internal/publishing/shein/sensitive_content_sanitizer.go`
  Purpose: ensure preview/draft sanitization reads authorized-brand preservation rules.
- `internal/shein/content/processor.go`
  Purpose: preserve only the resolved authorized brand during text cleanup.
- `internal/shein/submitprep/sensitive_words.go`
  Purpose: pass resolved authorized-brand context into submit-time cleanup calls.

### New files to create

- `internal/shein/authorizedbrand/types.go`
  Purpose: define config and resolved-brand structs plus task/context helpers.
- `internal/shein/authorizedbrand/context.go`
  Purpose: carry resolved authorized-brand state through `context.Context`.
- `internal/shein/authorizedbrand/resolver.go`
  Purpose: resolve configured authorized brand through `ProductAPI.QueryBrandList()`.
- `internal/shein/authorizedbrand/resolver_test.go`
  Purpose: cover code match, exact name match, missing config, and not-found behavior.
- `internal/publishing/shein/authorized_brand_test.go`
  Purpose: cover payload brand-code injection and preview sanitization preservation.
- `internal/shein/content/processor_authorized_brand_test.go`
  Purpose: cover preservation of the allowed brand while still removing other brands.

## Task 1: Add Store And Runtime Authorized-Brand State

**Files:**
- Create: `internal/shein/authorizedbrand/types.go`
- Create: `internal/shein/authorizedbrand/context.go`
- Modify: `internal/infra/clients/management/api/store.go`
- Modify: `internal/infra/clients/management/local_data_provider.go`
- Modify: `internal/shein/context/context.go`

- [ ] **Step 1: Write the failing test for local store DTO mapping**

```go
func TestLocalListingStoreToDTO_IncludesAuthorizedBrandFields(t *testing.T) {
    enabled := true
    row := localListingStore{
        ID:                       968,
        EnableBrandAuthorization: &enabled,
        AuthorizedBrandCode:      "2fd1n",
        AuthorizedBrandName:      "Logitech",
    }

    dto := row.toDTO()

    if dto.EnableBrandAuthorization == nil || !*dto.EnableBrandAuthorization {
        t.Fatalf("EnableBrandAuthorization = %#v, want true", dto.EnableBrandAuthorization)
    }
    if dto.AuthorizedBrandCode != "2fd1n" {
        t.Fatalf("AuthorizedBrandCode = %q, want 2fd1n", dto.AuthorizedBrandCode)
    }
    if dto.AuthorizedBrandName != "Logitech" {
        t.Fatalf("AuthorizedBrandName = %q, want Logitech", dto.AuthorizedBrandName)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/clients/management -run TestLocalListingStoreToDTO_IncludesAuthorizedBrandFields -v`  
Expected: FAIL with unknown struct fields or zero-value DTO fields.

- [ ] **Step 3: Add store DTO fields, local provider fields, and task runtime types**

```go
// internal/infra/clients/management/api/store.go
type StoreRespDTO struct {
    // ...
    EnableBrandAuthorization *bool  `json:"enableBrandAuthorization,omitempty"`
    AuthorizedBrandCode      string `json:"authorizedBrandCode,omitempty"`
    AuthorizedBrandName      string `json:"authorizedBrandName,omitempty"`
}

// internal/infra/clients/management/local_data_provider.go
type localListingStore struct {
    // ...
    EnableBrandAuthorization *bool   `gorm:"column:enable_brand_authorization"`
    AuthorizedBrandCode      string  `gorm:"column:authorized_brand_code"`
    AuthorizedBrandName      string  `gorm:"column:authorized_brand_name"`
}

func (s localListingStore) toDTO() *api.StoreRespDTO {
    return &api.StoreRespDTO{
        // ...
        EnableBrandAuthorization: s.EnableBrandAuthorization,
        AuthorizedBrandCode:      s.AuthorizedBrandCode,
        AuthorizedBrandName:      s.AuthorizedBrandName,
    }
}

// internal/shein/authorizedbrand/types.go
package authorizedbrand

type Config struct {
    Enabled   bool
    Code      string
    Name      string
}

type Resolved struct {
    Enabled   bool
    Code      string
    Name      string
    NameEn    string
}

func ConfigFromStore(store *managementapi.StoreRespDTO) Config {
    if store == nil || store.EnableBrandAuthorization == nil || !*store.EnableBrandAuthorization {
        return Config{}
    }
    return Config{
        Enabled: true,
        Code:    strings.TrimSpace(store.AuthorizedBrandCode),
        Name:    strings.TrimSpace(store.AuthorizedBrandName),
    }
}

// internal/shein/context/context.go
type RuntimeState struct {
    // ...
    AuthorizedBrand *authorizedbrand.Resolved
}

func (ctx *TaskContext) SetAuthorizedBrand(value *authorizedbrand.Resolved) {
    ctx.AuthorizedBrand = value
}
```

- [ ] **Step 4: Add context carrier helpers**

```go
// internal/shein/authorizedbrand/context.go
package authorizedbrand

type contextKey struct{}

func WithResolved(ctx context.Context, value *Resolved) context.Context {
    if ctx == nil || value == nil || !value.Enabled {
        return ctx
    }
    return context.WithValue(ctx, contextKey{}, *value)
}

func FromContext(ctx context.Context) (*Resolved, bool) {
    if ctx == nil {
        return nil, false
    }
    value, ok := ctx.Value(contextKey{}).(Resolved)
    if !ok || !value.Enabled {
        return nil, false
    }
    return &value, true
}
```

- [ ] **Step 5: Run tests to verify the plumbing passes**

Run: `go test ./internal/infra/clients/management ./internal/shein/context -v`  
Expected: PASS for the new DTO-mapping test and existing package tests.

- [ ] **Step 6: Commit**

```bash
git add internal/infra/clients/management/api/store.go internal/infra/clients/management/local_data_provider.go internal/shein/context/context.go internal/shein/authorizedbrand/types.go internal/shein/authorizedbrand/context.go
git commit -m "feat: add shein authorized brand store config plumbing"
```

## Task 2: Resolve Authorized Brand Through SHEIN Brand List

**Files:**
- Create: `internal/shein/authorizedbrand/resolver.go`
- Create: `internal/shein/authorizedbrand/resolver_test.go`
- Modify: `internal/shein/api/product/interface.go`
- Modify: `internal/shein/pipeline/task.go`

- [ ] **Step 1: Write the failing resolver tests**

```go
type stubProductAPI struct {
    brandResp *sheinproduct.BrandListResponse
    brandErr  error
}

func (s *stubProductAPI) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
    return s.brandResp, s.brandErr
}

func TestResolveAuthorizedBrand_PrefersCodeMatch(t *testing.T) {
    resolver := NewResolver(&stubProductAPI{
        brandResp: &sheinproduct.BrandListResponse{
            Info: struct {
                Data []sheinproduct.BrandItem `json:"data"`
                Meta struct {
                    Count     int `json:"count"`
                    CustomObj any `json:"customObj"`
                } `json:"meta"`
            }{
                Data: []sheinproduct.BrandItem{{BrandCode: "2fd1n", BrandName: "Logitech罗技", BrandNameEn: "Logitech"}},
            },
        },
    })

    got, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: "2fd1n", Name: "Logitech"})
    if err != nil {
        t.Fatalf("Resolve() error = %v", err)
    }
    if got.Code != "2fd1n" || got.NameEn != "Logitech" {
        t.Fatalf("resolved brand = %+v", got)
    }
}

func TestResolveAuthorizedBrand_FailsWhenConfiguredBrandMissing(t *testing.T) {
    resolver := NewResolver(&stubProductAPI{brandResp: &sheinproduct.BrandListResponse{}})
    _, err := resolver.Resolve(context.Background(), Config{Enabled: true, Code: "missing", Name: "Missing"})
    if err == nil {
        t.Fatal("Resolve() error = nil, want failure")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shein/authorizedbrand -run ResolveAuthorizedBrand -v`  
Expected: FAIL with missing resolver types and methods.

- [ ] **Step 3: Implement the resolver and integrate it into task initialization**

```go
// internal/shein/authorizedbrand/resolver.go
type ProductAPI interface {
    QueryBrandList() (*sheinproduct.BrandListResponse, error)
}

type Resolver struct {
    productAPI ProductAPI
}

func NewResolver(productAPI ProductAPI) *Resolver {
    return &Resolver{productAPI: productAPI}
}

func (r *Resolver) Resolve(ctx context.Context, cfg Config) (*Resolved, error) {
    if !cfg.Enabled {
        return nil, nil
    }
    if strings.TrimSpace(cfg.Code) == "" && strings.TrimSpace(cfg.Name) == "" {
        return nil, shein.NewNonRetryableError("authorized brand config is empty", nil)
    }
    resp, err := r.productAPI.QueryBrandList()
    if err != nil {
        return nil, shein.NewRetryableError("query authorized brand list failed", err)
    }
    for _, item := range resp.Info.Data {
        if strings.TrimSpace(item.BrandCode) == strings.TrimSpace(cfg.Code) {
            return &Resolved{Enabled: true, Code: item.BrandCode, Name: item.BrandName, NameEn: item.BrandNameEn}, nil
        }
    }
    if name := strings.TrimSpace(cfg.Name); name != "" {
        for _, item := range resp.Info.Data {
            if strings.TrimSpace(item.BrandName) == name || strings.TrimSpace(item.BrandNameEn) == name {
                return &Resolved{Enabled: true, Code: item.BrandCode, Name: item.BrandName, NameEn: item.BrandNameEn}, nil
            }
        }
    }
    return nil, shein.NewNonRetryableError("configured authorized brand was not found in SHEIN brand list", nil)
}

// internal/shein/pipeline/task.go
func (h *TaskHandler) initShopClient(taskCtx *sheincontext.TaskContext) error {
    // existing ProductAPI init...
    if storeCfg := authorizedbrand.ConfigFromStore(taskCtx.StoreInfo); storeCfg.Enabled {
        resolved, err := authorizedbrand.NewResolver(taskCtx.ProductAPI).Resolve(taskCtx.Context, storeCfg)
        if err != nil {
            return err
        }
        taskCtx.SetAuthorizedBrand(resolved)
        taskCtx.Context = authorizedbrand.WithResolved(taskCtx.Context, resolved)
    }
    return nil
}
```

- [ ] **Step 4: Run tests to verify the resolver passes**

Run: `go test ./internal/shein/authorizedbrand ./internal/shein/pipeline -v`  
Expected: PASS, including the code-match and missing-brand tests.

- [ ] **Step 5: Commit**

```bash
git add internal/shein/authorizedbrand/resolver.go internal/shein/authorizedbrand/resolver_test.go internal/shein/pipeline/task.go
git commit -m "feat: resolve shein authorized brand from brand list"
```

## Task 3: Apply Resolved Brand To Final SHEIN Payload

**Files:**
- Modify: `internal/publishing/shein/assembler.go`
- Modify: `internal/publishing/shein/preview_adapter.go`
- Create: `internal/publishing/shein/authorized_brand_test.go`

- [ ] **Step 1: Write the failing payload test**

```go
func TestBuildPreviewProduct_UsesAuthorizedBrandCode(t *testing.T) {
    pkg := &Package{
        BrandName: "Generic Brand",
        DraftPayload: &RequestDraft{
            SpuName: "Demo",
            MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Logitech Mouse"}},
            MultiLanguageDescList: []LocalizedText{{Language: "en", Name: "Logitech wireless mouse"}},
        },
        Metadata: map[string]string{},
    }
    resolved := &authorizedbrand.Resolved{
        Enabled: true,
        Code:    "2fd1n",
        Name:    "Logitech罗技",
        NameEn:  "Logitech",
    }
    ctx := authorizedbrand.WithResolved(context.Background(), resolved)
    pkg.Metadata["authorized_brand_code"] = resolved.Code
    pkg.Metadata["authorized_brand_name"] = resolved.NameEn

    preview := BuildPreviewProduct(pkg)
    if preview.BrandCode == nil || *preview.BrandCode != "2fd1n" {
        t.Fatalf("preview brand_code = %#v, want 2fd1n", preview.BrandCode)
    }
    _ = ctx
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/publishing/shein -run TestBuildPreviewProduct_UsesAuthorizedBrandCode -v`  
Expected: FAIL because `BrandCode` is still empty.

- [ ] **Step 3: Write the minimal implementation for package metadata and preview payload**

```go
// internal/publishing/shein/assembler.go
func (a *assembler) Build(req *BuildRequest, product *canonical.Product, image *productimage.ImageProcessResult) *Package {
    // ...
    brand := common.ResolveBrand(req.BrandHint, product)
    if resolved, ok := authorizedbrand.FromContext(req.Context); ok {
        brand = firstNonEmpty(resolved.NameEn, resolved.Name, brand)
    }
    pkg := &Package{
        BrandName: brand,
        Metadata: map[string]string{
            // ...
        },
    }
    if resolved, ok := authorizedbrand.FromContext(req.Context); ok {
        pkg.Metadata["authorized_brand_code"] = resolved.Code
        pkg.Metadata["authorized_brand_name"] = firstNonEmpty(resolved.NameEn, resolved.Name)
    }
}

// internal/publishing/shein/preview_adapter.go
func BuildPreviewProduct(pkg *Package) *sheinproduct.Product {
    // ...
    var brandCode *string
    if pkg != nil {
        if value := strings.TrimSpace(pkg.Metadata["authorized_brand_code"]); value != "" {
            brandCode = &value
        }
    }
    return &sheinproduct.Product{
        BrandCode: brandCode,
        // ...
    }
}
```

- [ ] **Step 4: Extend the payload test to verify metadata and preview brand-code wiring**

```go
func TestAssemblerBuild_WritesAuthorizedBrandMetadataAndPreviewBrandCode(t *testing.T) {
    resolved := &authorizedbrand.Resolved{Enabled: true, Code: "2fd1n", NameEn: "Logitech"}
    req := &BuildRequest{
        Country:   "US",
        Language:  "en",
        Context:   authorizedbrand.WithResolved(context.Background(), resolved),
        BrandHint: "",
    }
    product := &canonical.Product{Title: "Wireless Mouse", Brand: "UnapprovedBrand"}

    pkg := NewAssembler(AssemblerConfig{}).Build(req, product, nil)

    if got := pkg.Metadata["authorized_brand_code"]; got != "2fd1n" {
        t.Fatalf("authorized_brand_code = %q, want 2fd1n", got)
    }
    if got := pkg.Metadata["authorized_brand_name"]; got != "Logitech" {
        t.Fatalf("authorized_brand_name = %q, want Logitech", got)
    }
    if pkg.PreviewPayload == nil || pkg.PreviewPayload.BrandCode == nil || *pkg.PreviewPayload.BrandCode != "2fd1n" {
        t.Fatalf("preview brand code = %v, want 2fd1n", pkg.PreviewPayload.BrandCode)
    }
}
```

- [ ] **Step 5: Run tests to verify the payload path passes**

Run: `go test ./internal/publishing/shein -run AuthorizedBrand -v`  
Expected: PASS for preview payload and assembler brand metadata tests.

- [ ] **Step 6: Commit**

```bash
git add internal/publishing/shein/assembler.go internal/publishing/shein/preview_adapter.go internal/publishing/shein/authorized_brand_test.go
git commit -m "feat: apply authorized brand to shein payload"
```

## Task 4: Preserve Only The Authorized Brand During Cleanup

**Files:**
- Modify: `internal/shein/content/processor.go`
- Modify: `internal/publishing/shein/sensitive_content_sanitizer.go`
- Modify: `internal/shein/submitprep/sensitive_words.go`
- Create: `internal/shein/content/processor_authorized_brand_test.go`
- Modify: `internal/publishing/shein/authorized_brand_test.go`

- [ ] **Step 1: Write the failing cleanup tests**

```go
func TestSanitizeDisplayTextWithContext_PreservesAuthorizedBrand(t *testing.T) {
    service := NewSensitiveWordServiceInMemory()
    taskCtx := &sheinctx.TaskContext{
        RuntimeState: sheinctx.RuntimeState{
            AuthorizedBrand: &authorizedbrand.Resolved{
                Enabled: true,
                Code:    "2fd1n",
                Name:    "Logitech罗技",
                NameEn:  "Logitech",
            },
        },
    }

    got := service.SanitizeDisplayTextWithContext(taskCtx, "Logitech mouse by Sony for office")

    if !strings.Contains(got, "Logitech") {
        t.Fatalf("sanitized text = %q, want Logitech preserved", got)
    }
    if strings.Contains(strings.ToLower(got), "sony") {
        t.Fatalf("sanitized text = %q, want Sony removed", got)
    }
}

func TestSanitizeSheinListingCopy_PreservesAuthorizedBrandFromRuntimeContext(t *testing.T) {
    copy := &listingCopy{
        Title:        "Logitech wireless mouse",
        Description:  "Logitech office mouse by Sony",
        SKCTitleBase: "Logitech mouse",
    }
    runtimeCtx := authorizedbrand.WithResolved(context.Background(), &authorizedbrand.Resolved{
        Enabled: true,
        Code:    "2fd1n",
        NameEn:  "Logitech",
    })

    sanitizeSheinListingCopy(copy, runtimeCtx, nil)

    if !strings.Contains(copy.Title, "Logitech") || !strings.Contains(copy.SKCTitleBase, "Logitech") {
        t.Fatalf("copy after sanitize = %+v", copy)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/shein/content ./internal/publishing/shein -run AuthorizedBrand -v`  
Expected: FAIL because brand cleanup still strips all brands.

- [ ] **Step 3: Add allowlist-aware brand cleanup**

```go
// internal/shein/content/processor.go
func (s *SensitiveWordService) authorizedBrandAllowlist(ctx *sheinctx.TaskContext) []string {
    if ctx == nil || ctx.AuthorizedBrand == nil || !ctx.AuthorizedBrand.Enabled {
        return nil
    }
    return uniqueNonEmpty(
        strings.TrimSpace(ctx.AuthorizedBrand.Name),
        strings.TrimSpace(ctx.AuthorizedBrand.NameEn),
    )
}

func (s *SensitiveWordService) removeAmazonBrandWordsExcept(text string, allowed []string) string {
    for _, brandWord := range s.getAmazonBrandWords() {
        if containsFoldedText(allowed, brandWord) {
            continue
        }
        text = s.removeWordFromText(text, brandWord)
    }
    return text
}

func (s *SensitiveWordService) removeContextBrandWordsExcept(ctx *sheinctx.TaskContext, text string, allowed []string) string {
    if ctx == nil || ctx.AmazonProduct == nil {
        return text
    }
    brandWord := strings.TrimSpace(ctx.AmazonProduct.Brand)
    if brandWord == "" || containsFoldedText(allowed, brandWord) {
        return text
    }
    return s.removeWordFromText(text, brandWord)
}

func (s *SensitiveWordService) removeSensitiveWordsAndBrandsWithContext(ctx *sheinctx.TaskContext, text string) string {
    text = s.removeSensitiveWords(text)
    allowed := s.authorizedBrandAllowlist(ctx)
    text = s.removeAmazonBrandWordsExcept(text, allowed)
    text = s.removeContextBrandWordsExcept(ctx, text, allowed)
    return s.cleanTextForSheinPlatform(text)
}
```

- [ ] **Step 4: Bridge runtime context into preview and submit sanitizers**

```go
// internal/publishing/shein/sensitive_content_sanitizer.go
func sanitizeStringField(service sheinSensitiveWordSanitizer, ctx *sheinctx.TaskContext, value *string) bool {
    // unchanged except callers pass authorized-brand-aware task context
}

func sanitizeSheinListingCopy(copy *listingCopy, runtimeCtx context.Context, ctx *sheinctx.TaskContext) bool {
    if ctx == nil {
        if resolved, ok := authorizedbrand.FromContext(runtimeCtx); ok {
            ctx = &sheinctx.TaskContext{}
            ctx.SetAuthorizedBrand(resolved)
        }
    }
    // existing sanitize calls...
}

// internal/shein/submitprep/sensitive_words.go
func CleanSensitiveWordsWithContext(ctx context.Context, product *sheinproduct.Product) error {
    service := NewSensitiveWordServiceForContext(ctx)
    taskCtx := &sheinctx.TaskContext{}
    taskCtx.ProductData = product
    if resolved, ok := authorizedbrand.FromContext(ctx); ok {
        taskCtx.SetAuthorizedBrand(resolved)
    }
    return service.ProcessProductData(taskCtx)
}
```

- [ ] **Step 5: Run tests to verify cleanup behavior passes**

Run: `go test ./internal/shein/content ./internal/publishing/shein ./internal/shein/submitprep -v`  
Expected: PASS, with authorized brand preserved and other brands still removed.

- [ ] **Step 6: Commit**

```bash
git add internal/shein/content/processor.go internal/publishing/shein/sensitive_content_sanitizer.go internal/shein/submitprep/sensitive_words.go internal/shein/content/processor_authorized_brand_test.go internal/publishing/shein/authorized_brand_test.go
git commit -m "feat: preserve authorized brand during shein cleanup"
```

## Task 5: Full Regression Pass And Documentation Sync

**Files:**
- Modify: `docs/superpowers/specs/2026-06-16-shein-listing-authorized-brand-design.md` only if implementation wording changed

- [ ] **Step 1: Run the focused regression suite**

Run:

```bash
go test ./internal/infra/clients/management ./internal/shein/authorizedbrand ./internal/shein/context ./internal/shein/pipeline ./internal/shein/content ./internal/shein/submitprep ./internal/publishing/shein ./internal/shein/api/product
```

Expected: PASS across all modified packages.

- [ ] **Step 2: Manually verify the end-to-end invariants in code**

Check these exact conditions in the final code:

```text
1. ConfigFromStore() returns Enabled=false for stores without the switch.
2. TaskHandler.initShopClient() resolves authorized brand only after ProductAPI exists.
3. BuildPreviewProduct() writes Product.BrandCode from package metadata.
4. sanitizeSheinListingCopy() preserves the authorized brand when only runtime context is available.
5. CleanSensitiveWordsWithContext() preserves the authorized brand at submit time.
```

- [ ] **Step 3: Sync the design doc only if implementation names changed**

```markdown
- If field names remain `EnableBrandAuthorization`, `AuthorizedBrandCode`, and `AuthorizedBrandName`, leave the spec unchanged.
- If implementation names change, update the spec to match exact names.
```

- [ ] **Step 4: Create the final commit**

```bash
git add internal/infra/clients/management/api/store.go internal/infra/clients/management/local_data_provider.go internal/shein/context/context.go internal/shein/authorizedbrand internal/shein/pipeline/task.go internal/publishing/shein/assembler.go internal/publishing/shein/preview_adapter.go internal/publishing/shein/sensitive_content_sanitizer.go internal/shein/content/processor.go internal/shein/submitprep/sensitive_words.go docs/superpowers/specs/2026-06-16-shein-listing-authorized-brand-design.md
git commit -m "feat: support authorized shein brands in listing flow"
```

## Self-Review

- Spec coverage: store config, SHEIN brand query, payload `brand_code`, cleanup preservation, and error handling are each covered by Tasks 1-4.
- Placeholder scan: no `TODO`, `TBD`, or implicit “handle later” steps remain.
- Type consistency: the plan uses one set of names across tasks: `EnableBrandAuthorization`, `AuthorizedBrandCode`, `AuthorizedBrandName`, `authorizedbrand.Config`, and `authorizedbrand.Resolved`.
