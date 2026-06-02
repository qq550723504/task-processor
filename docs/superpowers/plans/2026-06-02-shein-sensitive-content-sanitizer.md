# SHEIN Sensitive Content Sanitizer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a unified sensitive-content sanitizer for `listing kit pod -> shein` so preview/draft and submit flows both clean titles, descriptions, SKC names, multilingual copy, and free-text product attributes with the same rules.

**Architecture:** Reuse the existing `SensitiveWordService` text-cleaning behavior, but stop hardcoding cleanup only inside `ProcessProductData()`. Introduce a field-driven sanitizer that can clean `listingCopy`, `RequestDraft`, and `sheinproduct.Product`, then wire it into listing-copy generation, draft assembly, and submit preparation for an idempotent preview-first plus submit-final flow.

**Tech Stack:** Go, existing SHEIN publishing pipeline, existing `SensitiveWordService`, Go test.

---

### Task 1: Lock Listing Copy Sanitizer Behavior

**Files:**
- Modify: `internal/publishing/shein/listing_copy_test.go`
- Test: `internal/publishing/shein/listing_copy_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildSheinListingCopy_CleansSensitiveWords(t *testing.T) {
	restore := installSheinSensitiveWordsFixture(t, `{
  "static_words": {"en": ["bpa free", "amazon"]},
  "dynamic_words": {}
}`)
	defer restore()

	canonical := &canonical.Product{
		Title:       "Amazon BPA Free Vase",
		Description: "Amazon BPA Free vase for home decor.",
	}

	copy := buildSheinListingCopy(canonical, canonical.Title, nil)

	if strings.Contains(strings.ToLower(copy.Title), "amazon") || strings.Contains(strings.ToLower(copy.Title), "bpa free") {
		t.Fatalf("title = %q, want sensitive words removed", copy.Title)
	}
	if strings.Contains(strings.ToLower(copy.Description), "amazon") || strings.Contains(strings.ToLower(copy.Description), "bpa free") {
		t.Fatalf("description = %q, want sensitive words removed", copy.Description)
	}
	if strings.Contains(strings.ToLower(copy.SKCTitleBase), "amazon") || strings.Contains(strings.ToLower(copy.SKCTitleBase), "bpa free") {
		t.Fatalf("skc base = %q, want sensitive words removed", copy.SKCTitleBase)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/publishing/shein -run TestBuildSheinListingCopy_CleansSensitiveWords -count=1`
Expected: FAIL because `buildSheinListingCopy()` does not yet run the unified sensitive sanitizer.

- [ ] **Step 3: Write minimal implementation**

```go
func buildSheinListingCopy(canonical *canonical.Product, fallbackTitle string, aiClient openaiclient.ChatCompleter) listingCopy {
	// existing title/description generation...
	copy := listingCopy{...}
	sanitizeSheinListingCopy(&copy, sheinSensitiveContext(nil, canonical))
	return copy
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/publishing/shein -run TestBuildSheinListingCopy_CleansSensitiveWords -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/publishing/shein/listing_copy.go internal/publishing/shein/listing_copy_test.go
git commit -m "test: lock shein listing copy sensitive-word cleanup"
```

### Task 2: Lock Draft Payload and Free-Text Attribute Sanitizer Behavior

**Files:**
- Modify: `internal/publishing/shein/submit_prep_test.go`
- Test: `internal/publishing/shein/submit_prep_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestSanitizeSheinDraft_CleansDraftTextFields(t *testing.T) {
	restore := installSheinSensitiveWordsFixture(t, `{
  "static_words": {"en": ["bpa free", "amazon"]},
  "dynamic_words": {}
}`)
	defer restore()

	pkg := &Package{
		DraftPayload: &RequestDraft{
			MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Amazon BPA Free Vase"}},
			MultiLanguageDescList: []LocalizedText{{Language: "en", Name: "Amazon BPA Free vase for home decor."}},
			ProductAttributeList: []common.Attribute{
				{Name: "Material Detail", Value: "Amazon BPA Free acrylic"},
				{Name: "Length", Value: "12"},
			},
			SKCList: []SKCRequestDraft{{
				SkcName: "Amazon BPA Free Blue",
				MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Amazon BPA Free Blue"}},
			}},
		},
	}

	changed := SanitizeDraftPayloadSensitiveContent(pkg, nil)

	if !changed {
		t.Fatal("changed = false, want true")
	}
	if got := firstLocalizedText(pkg.DraftPayload.MultiLanguageNameList); strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("draft title = %q, want sensitive words removed", got)
	}
	if got := firstLocalizedText(pkg.DraftPayload.MultiLanguageDescList); strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("draft description = %q, want sensitive words removed", got)
	}
	if got := pkg.DraftPayload.SKCList[0].SkcName; strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("draft skc name = %q, want sensitive words removed", got)
	}
	if got := pkg.DraftPayload.ProductAttributeList[0].Value; strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("attribute value = %q, want sensitive words removed", got)
	}
	if got := pkg.DraftPayload.ProductAttributeList[1].Value; got != "12" {
		t.Fatalf("structured attribute = %q, want unchanged", got)
	}
}

func TestPrepareSubmitProductContent_CleansFreeTextAttributesAndSKCNames(t *testing.T) {
	restore := installSheinSensitiveWordsFixture(t, `{
  "static_words": {"en": ["bpa free", "amazon"]},
  "dynamic_words": {}
}`)
	defer restore()

	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Amazon BPA Free Vase"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Amazon BPA Free vase for home decor."}},
		ProductAttributeList: []common.Attribute{
			{Name: "Material Detail", Value: "Amazon BPA Free acrylic"},
			{Name: "Length", Value: "12"},
		},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "Amazon BPA Free Blue"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Amazon BPA Free Blue"}},
		}},
	}

	if err := PrepareSubmitProductContent(context.Background(), product, "US", nil, nil); err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	if got := submitprep.FirstLocalizedText(product.MultiLanguageNameList); strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("submit title = %q, want sensitive words removed", got)
	}
	if got := product.ProductAttributeList[0].Value; strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("submit attribute value = %q, want sensitive words removed", got)
	}
	if got := product.ProductAttributeList[1].Value; got != "12" {
		t.Fatalf("structured submit attribute = %q, want unchanged", got)
	}
	if got := product.SKCList[0].MultiLanguageName.Name; strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("submit skc name = %q, want sensitive words removed", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/publishing/shein -run "TestSanitizeSheinDraft_CleansDraftTextFields|TestPrepareSubmitProductContent_CleansFreeTextAttributesAndSKCNames" -count=1`
Expected: FAIL because draft sanitizing does not exist yet and submit cleanup does not cover `ProductAttributeList`.

- [ ] **Step 3: Write minimal implementation**

```go
func SanitizeDraftPayloadSensitiveContent(pkg *Package, ctx *sheinctx.TaskContext) bool {
	// iterate DraftPayload localized title/desc, SKC names, SKC localized names, free-text attributes
}

func sanitizeProductAttributeValues(attrs []common.Attribute, sanitize func(string) string) bool {
	// clean free-text values, skip obvious structured values
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/publishing/shein -run "TestSanitizeSheinDraft_CleansDraftTextFields|TestPrepareSubmitProductContent_CleansFreeTextAttributesAndSKCNames" -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/publishing/shein internal/shein/submitprep internal/publishing/shein/submit_prep_test.go
git commit -m "feat: sanitize shein draft and submit content"
```

### Task 3: Wire Unified Sanitizer into Assembly and Submit Flows

**Files:**
- Modify: `internal/publishing/shein/assembler.go`
- Modify: `internal/publishing/shein/listing_copy.go`
- Modify: `internal/publishing/shein/submit_prep.go`
- Modify: `internal/shein/content/word.go`
- Modify: `internal/shein/content/processor.go`
- Create: `internal/publishing/shein/sensitive_content_sanitizer.go`
- Test: `internal/publishing/shein/listing_copy_test.go`
- Test: `internal/publishing/shein/submit_prep_test.go`

- [ ] **Step 1: Write the failing integration check**

```go
func TestAssemblerBuild_SanitizesDraftPayloadSensitiveContent(t *testing.T) {
	restore := installSheinSensitiveWordsFixture(t, `{
  "static_words": {"en": ["bpa free", "amazon"]},
  "dynamic_words": {}
}`)
	defer restore()

	a := NewAssembler(AssemblerConfig{})
	req := &BuildRequest{Country: "US", Language: "en_US"}
	product := &canonical.Product{
		Title:       "Amazon BPA Free Vase",
		Description: "Amazon BPA Free vase for home decor.",
	}

	pkg := a.Build(req, product, nil)

	if pkg == nil || pkg.DraftPayload == nil {
		t.Fatalf("pkg = %+v, want draft payload", pkg)
	}
	if got := firstLocalizedText(pkg.DraftPayload.MultiLanguageNameList); strings.Contains(strings.ToLower(got), "amazon") || strings.Contains(strings.ToLower(got), "bpa free") {
		t.Fatalf("assembler draft title = %q, want sensitive words removed", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/publishing/shein -run TestAssemblerBuild_SanitizesDraftPayloadSensitiveContent -count=1`
Expected: FAIL until `assembler.Build()` calls the draft sanitizer.

- [ ] **Step 3: Write minimal implementation**

```go
func (a *assembler) Build(req *BuildRequest, product *canonical.Product, image *productimage.ImageProcessResult) *Package {
	// existing assembly
	SanitizeDraftPayloadSensitiveContent(pkg, sheinSensitiveContext(req, product))
	SetPreviewPayload(pkg, BuildPreviewProduct(pkg))
	return pkg
}
```

Also update submit cleanup internals so `ProcessProductData()` reuses the same field-driven product sanitizer instead of separate handwritten loops where practical.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/publishing/shein -run TestAssemblerBuild_SanitizesDraftPayloadSensitiveContent -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/publishing/shein/assembler.go internal/publishing/shein/sensitive_content_sanitizer.go internal/shein/content/word.go internal/shein/content/processor.go
git commit -m "refactor: unify shein sensitive content sanitizing"
```

### Task 4: Full Regression Verification

**Files:**
- Test: `internal/publishing/shein/listing_copy_test.go`
- Test: `internal/publishing/shein/submit_prep_test.go`
- Test: any newly added sanitizer tests

- [ ] **Step 1: Run focused package tests**

Run: `go test ./internal/publishing/shein -count=1`
Expected: PASS

- [ ] **Step 2: Run dependent submitprep/content coverage if needed**

Run: `go test ./internal/shein/... -run "Sensitive|Submit|Content" -count=1`
Expected: PASS or no matching tests with exit 0

- [ ] **Step 3: Review for spec coverage**

Check that the final diff covers:

```text
- listingCopy title/description/SKC base
- DraftPayload localized title/description/SKC names
- ProductAttributeList free-text values
- submit-time sheinproduct.Product fields
- retry cleanup path still intact
```

- [ ] **Step 4: Commit final polish if needed**

```bash
git add internal/publishing/shein internal/shein/content internal/shein/submitprep
git commit -m "test: verify shein sensitive sanitizer coverage"
```
