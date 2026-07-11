# SDS POD Canonical Metadata Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Move deterministic SDS POD canonical title, identity, style, and rendered-image mapping from root ListingKit into internal/product/sourcing/sdspod while preserving production behavior except for the approved anonymous successful variant product-image union refinement.

**Architecture:** ListingKit remains the anti-corruption shell for legacy SDSSyncSummary, SDSSyncOptions, and decorated SHEIN supplier SKU compatibility. A platform-neutral functional core receives CanonicalMetadata and mutates canonical.Product deterministically with the existing traces, precedence, fallback, ordering, and idempotency, subject only to the approved anonymous successful variant product-image union refinement.

**Tech Stack:** Go 1.26+, standard library, internal/catalog/canonical, existing ListingKit characterization tests and AST import guards.

## Global Constraints

- Preserve SDSSyncSummary, SDSSyncOptions, GenerateRequest, canonical.Product, and all public JSON contracts.
- Preserve title, attribute, style, image, trace, precedence, fallback,
  ordering, de-duplication, and bool changed behavior exactly except for the
  approved anonymous successful variant product-image union refinement below.
- Approved exception to exact image preservation: `product.Images` unions all
  successful variant image groups even when every variant has an empty
  normalized SKU and Color. Legacy behavior used top-level default mockups or
  the first successful variant group in that case. Per-variant lookup and
  default fallback behavior remain unchanged.
- The ai_style canonical variant attribute key remains exactly ai_style.
- Variant lookup normalization remains strings.ToLower(strings.TrimSpace(value)); empty normalized keys are ignored.
- internal/product/sourcing/sdspod may import only the Go standard library and task-processor/internal/catalog/canonical.
- Do not modify SDS baseline validation, remote sync, browser/login, rendered-image polling, SHEIN payload images, Temporal, persistence, or Studio batch orchestration.
- Keep decorated SHEIN supplier SKU parsing in ListingKit compatibility code.
- Do not stage or change the existing go.work.sum working-tree modification.
- Use TDD, mechanical movement, and independently passing commits.

---

## File Map

Create:

- internal/product/sourcing/sdspod/model.go — CanonicalMetadata, VariantMetadata, and VariantLookup.
- internal/product/sourcing/sdspod/apply.go — ApplyCanonical orchestration, title, identity, and style rules.
- internal/product/sourcing/sdspod/images.go — mockup conversion, image indexing, assignment, fallback, equality, and copies.
- internal/product/sourcing/sdspod/apply_test.go — title, attributes, style, traces, nil safety, and idempotency.
- internal/product/sourcing/sdspod/images_test.go — default and multi-variant image behavior.
- internal/product/sourcing/sdspod/boundary_guard_test.go — production import guard.

Modify:

- internal/listingkit/sds_canonical_metadata.go — retain only legacy DTO adaptation, compatibility lookup construction, and delegation.
- internal/listingkit/workflow_studio_sds_metadata_support.go — remove applyStudioStyleDimension after delegation; retain unrelated Studio SKU and metadata helpers.
- internal/listingkit/workflow_studio_sds_metadata_test.go — retain facade/workflow compatibility tests and add decorated supplier SKU regression coverage if absent.
- internal/listingkit/workflow_studio_fallback_test.go — retain title compatibility coverage.
- internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go — replace the old ownership expectation with a retirement/delegation expectation.
- docs/refactoring/listingkit-boundary-checkpoint.md — record the new source-normalization owner.

---

### Task 1: Establish the SDS POD Canonical Contract and Boundary

**Files:**
- Create: internal/product/sourcing/sdspod/model.go
- Create: internal/product/sourcing/sdspod/apply.go
- Create: internal/product/sourcing/sdspod/apply_test.go
- Create: internal/product/sourcing/sdspod/boundary_guard_test.go

**Interfaces:**
- Consumes: canonical.Product plus platform-neutral CanonicalMetadata.
- Produces: ApplyCanonical(product *canonical.Product, metadata CanonicalMetadata) bool.

- [ ] **Step 1: Write failing contract tests**

Create apply_test.go:

~~~go
package sdspod

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestApplyCanonicalNilProductIsNoOp(t *testing.T) {
	if ApplyCanonical(nil, CanonicalMetadata{ProductName: "Clock"}) {
		t.Fatal("ApplyCanonical(nil) = true, want false")
	}
}

func TestApplyCanonicalEmptyMetadataIsNoOp(t *testing.T) {
	product := &canonical.Product{Title: "Existing"}
	if ApplyCanonical(product, CanonicalMetadata{}) {
		t.Fatal("ApplyCanonical(empty) = true, want false")
	}
	if product.Title != "Existing" {
		t.Fatalf("Title = %q, want Existing", product.Title)
	}
}
~~~

- [ ] **Step 2: Verify RED**

~~~powershell
go test ./internal/product/sourcing/sdspod -run TestApplyCanonical -count=1
~~~

Expected: compile failure because CanonicalMetadata and ApplyCanonical do not exist.

- [ ] **Step 3: Add exact public models**

Create model.go:

~~~go
package sdspod

type CanonicalMetadata struct {
	ProductName   string
	ProductSKU    string
	VariantSKU    string
	VariantSize   string
	VariantColor  string
	StyleName     string
	Attributes    map[string]string
	MockupURLs    []string
	Variants      []VariantMetadata
	VariantLookup []VariantLookup
}

type VariantMetadata struct {
	SKU        string
	Color      string
	Status     string
	MockupURLs []string
}

type VariantLookup struct {
	CanonicalVariantIndex int
	Keys                  []string
}
~~~

- [ ] **Step 4: Add the minimal contract**

Create apply.go:

~~~go
package sdspod

import "task-processor/internal/catalog/canonical"

func ApplyCanonical(product *canonical.Product, metadata CanonicalMetadata) bool {
	if product == nil {
		return false
	}
	return false
}
~~~

- [ ] **Step 5: Add the package import guard**

Create boundary_guard_test.go:

~~~go
package sdspod

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPackageImportsStayPlatformNeutral(t *testing.T) {
	allowed := map[string]struct{}{
		"task-processor/internal/catalog/canonical": {},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(".", entry.Name())
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := allowed[importPath]; ok {
				continue
			}
			if strings.Contains(importPath, ".") ||
				strings.HasPrefix(importPath, "task-processor/") {
				t.Errorf("%s imports forbidden package %s", path, importPath)
			}
		}
	}
}
~~~

- [ ] **Step 6: Format, test, and commit**

~~~powershell
$files = Get-ChildItem internal/product/sourcing/sdspod -Filter *.go | ForEach-Object FullName
gofmt -w $files
go test ./internal/product/sourcing/sdspod -count=1
git add internal/product/sourcing/sdspod
git commit -m "refactor: define sds pod canonical contract"
~~~

Expected: PASS and a commit containing only the new contract package.

---

### Task 2: Implement Title, Identity, and Studio Style Mapping

**Files:**
- Modify: internal/product/sourcing/sdspod/apply.go
- Modify: internal/product/sourcing/sdspod/apply_test.go

**Interfaces:**
- Consumes: CanonicalMetadata from Task 1.
- Produces: deterministic title, product attributes, variant ai_style, and exact field traces.

- [ ] **Step 1: Add reflect and exact trace helpers to the tests**

Add `"reflect"` to the existing apply_test.go import block, then append:

Append:

~~~go
func wantTrace(detail string, confidence float64) canonical.FieldTrace {
	return canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: detail,
		}},
		Confidence:  confidence,
		IsInferred:  false,
		NeedsReview: false,
	}
}
~~~

- [ ] **Step 2: Add failing title, identity, and style tests**

Append:

~~~go
func TestApplyCanonicalMapsTitleIdentityAndStyle(t *testing.T) {
	product := &canonical.Product{
		Title: "Old title",
		Attributes: map[string]canonical.Attribute{
			"sku": {Value: "old"},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-1"},
			{SKU: "SKU-2", Attributes: map[string]canonical.Attribute{}},
		},
	}
	metadata := CanonicalMetadata{
		ProductName:  "  Rendered Clock  ",
		ProductSKU:   " PARENT-1 ",
		VariantSKU:   " CHILD-1 ",
		VariantSize:  " 40x60cm ",
		VariantColor: " White ",
		StyleName:    " Style B6C753EB ",
		Attributes: map[string]string{
			"material": " Cotton ",
			"sku":      "fallback-sku",
		},
	}

	if !ApplyCanonical(product, metadata) {
		t.Fatal("ApplyCanonical() = false, want true")
	}
	if product.Title != "Rendered Clock" {
		t.Fatalf("Title = %q", product.Title)
	}
	wantValues := map[string]string{
		"material":      "Cotton",
		"sku":           "PARENT-1",
		"product_sku":   "PARENT-1",
		"variant_sku":   "CHILD-1",
		"variant_size":  "40x60cm",
		"variant_color": "White",
	}
	for key, want := range wantValues {
		if got := product.Attributes[key].Value; got != want {
			t.Fatalf("attribute %s = %q, want %q", key, got, want)
		}
		if !reflect.DeepEqual(product.Attributes[key].Trace, wantTrace(
			"SDS design product identity", 0.96)) {
			t.Fatalf("attribute %s trace = %+v", key, product.Attributes[key].Trace)
		}
	}
	for i := range product.Variants {
		attr := product.Variants[i].Attributes["ai_style"]
		if attr.Value != "Style B6C753EB" {
			t.Fatalf("variant[%d] ai_style = %q", i, attr.Value)
		}
		if !reflect.DeepEqual(attr.Trace,
			wantTrace("SDS studio AI style dimension", 0.94)) {
			t.Fatalf("variant[%d] style trace = %+v", i, attr.Trace)
		}
	}
	if !reflect.DeepEqual(product.FieldTraces["title"],
		wantTrace("SDS design product detail", 0.96)) {
		t.Fatalf("title trace = %+v", product.FieldTraces["title"])
	}
	if !reflect.DeepEqual(product.FieldTraces["attributes"],
		wantTrace("SDS design product identity", 0.96)) {
		t.Fatalf("attributes trace = %+v", product.FieldTraces["attributes"])
	}
}

func TestApplyCanonicalTitleIdentityAndStyleAreIdempotent(t *testing.T) {
	product := &canonical.Product{Variants: []canonical.Variant{{SKU: "SKU"}}}
	metadata := CanonicalMetadata{
		ProductName: "Clock",
		ProductSKU:  "PARENT",
		StyleName:   "Style A1",
	}
	if !ApplyCanonical(product, metadata) {
		t.Fatal("first ApplyCanonical() = false")
	}
	if ApplyCanonical(product, metadata) {
		t.Fatal("second ApplyCanonical() = true, want false")
	}
}
~~~

FieldTrace contains a slice, so every trace comparison above intentionally uses reflect.DeepEqual.

- [ ] **Step 3: Verify RED**

~~~powershell
go test ./internal/product/sourcing/sdspod -run "TestApplyCanonical(Maps|Title)" -count=1
~~~

Expected: tests fail because ApplyCanonical still returns false.

- [ ] **Step 4: Implement exact deterministic mapping**

Replace apply.go with:

~~~go
package sdspod

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

const studioStyleAttributeKey = "ai_style"

func ApplyCanonical(product *canonical.Product, metadata CanonicalMetadata) bool {
	if product == nil {
		return false
	}
	changed := applyIdentity(product, metadata)
	if applyStyle(product, metadata.StyleName) {
		changed = true
	}
	if applyImages(product, metadata) {
		changed = true
	}
	if applyTitle(product, metadata.ProductName) {
		changed = true
	}
	return changed
}

func applyTitle(product *canonical.Product, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.TrimSpace(product.Title) == value {
		return false
	}
	product.Title = value
	if product.FieldTraces == nil {
		product.FieldTraces = map[string]canonical.FieldTrace{}
	}
	product.FieldTraces["title"] = canonicalTrace(
		"SDS design product detail", 0.96)
	return true
}

func applyIdentity(product *canonical.Product, metadata CanonicalMetadata) bool {
	values := copyStringMap(metadata.Attributes)
	setPreferred(values, "sku", metadata.ProductSKU)
	setPreferred(values, "product_sku", metadata.ProductSKU)
	setPreferred(values, "variant_sku", metadata.VariantSKU)
	setPreferred(values, "variant_size", metadata.VariantSize)
	setPreferred(values, "variant_color", metadata.VariantColor)

	trace := canonicalTrace("SDS design product identity", 0.96)
	changed := false
	for key, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if product.Attributes == nil {
			product.Attributes = map[string]canonical.Attribute{}
		}
		if strings.TrimSpace(product.Attributes[key].Value) == value {
			continue
		}
		product.Attributes[key] = canonical.Attribute{Value: value, Trace: trace}
		changed = true
	}
	if changed {
		if product.FieldTraces == nil {
			product.FieldTraces = map[string]canonical.FieldTrace{}
		}
		product.FieldTraces["attributes"] = trace
	}
	return changed
}

func applyStyle(product *canonical.Product, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(product.Variants) == 0 {
		return false
	}
	trace := canonicalTrace("SDS studio AI style dimension", 0.94)
	changed := false
	for i := range product.Variants {
		if product.Variants[i].Attributes == nil {
			product.Variants[i].Attributes = map[string]canonical.Attribute{}
		}
		if strings.TrimSpace(
			product.Variants[i].Attributes[studioStyleAttributeKey].Value) == value {
			continue
		}
		product.Variants[i].Attributes[studioStyleAttributeKey] =
			canonical.Attribute{Value: value, Trace: trace}
		changed = true
	}
	return changed
}

func canonicalTrace(detail string, confidence float64) canonical.FieldTrace {
	return canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: detail,
		}},
		Confidence:  confidence,
		IsInferred:  false,
		NeedsReview: false,
	}
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input)+5)
	for key, value := range input {
		out[key] = value
	}
	return out
}

func setPreferred(values map[string]string, key, preferred string) {
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		values[key] = preferred
	}
}
~~~

Until Task 3 creates images.go, add this temporary private stub at the end:

~~~go
func applyImages(*canonical.Product, CanonicalMetadata) bool {
	return false
}
~~~

Task 3 removes the stub when it adds the real image implementation.

- [ ] **Step 5: Format, test, and commit**

~~~powershell
$files = Get-ChildItem internal/product/sourcing/sdspod -Filter *.go | ForEach-Object FullName
gofmt -w $files
go test ./internal/product/sourcing/sdspod -count=1
git add internal/product/sourcing/sdspod
git commit -m "refactor: move sds pod canonical identity policy"
~~~

Expected: PASS.

---

### Task 3: Implement Rendered Mockup and Variant Image Mapping

**Files:**
- Create: internal/product/sourcing/sdspod/images.go
- Create: internal/product/sourcing/sdspod/images_test.go
- Modify: internal/product/sourcing/sdspod/apply.go

**Interfaces:**
- Consumes: CanonicalMetadata.MockupURLs, Variants, and VariantLookup.
- Produces: canonical product and variant images with exact current trace,
  ordering, fallback, and idempotency, plus the approved anonymous successful
  variant product-image union refinement.

- [ ] **Step 1: Write failing multi-variant image test**

Create images_test.go:

~~~go
package sdspod

import (
	"reflect"
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestApplyCanonicalAssignsRenderedVariantImages(t *testing.T) {
	product := &canonical.Product{
		Images: []canonical.Image{{URL: "old", Role: "primary"}},
		Variants: []canonical.Variant{
			{SKU: "BLACK"},
			{SKU: "WHITE"},
		},
	}
	metadata := CanonicalMetadata{
		Variants: []VariantMetadata{
			{
				SKU: "BLACK",
				Color: "Black",
				Status: "completed",
				MockupURLs: []string{
					" https://cdn/black-main.jpg ",
					"https://cdn/black-side.jpg",
					"https://cdn/black-main.jpg",
				},
			},
			{
				SKU: "FAILED",
				Color: "Red",
				Status: "failed",
				MockupURLs: []string{"https://cdn/failed.jpg"},
			},
			{
				SKU: "WHITE",
				Color: "White",
				Status: "completed",
				MockupURLs: []string{"https://cdn/white-main.jpg"},
			},
		},
		VariantLookup: []VariantLookup{
			{CanonicalVariantIndex: 0, Keys: []string{"BLACK", "Black"}},
			{CanonicalVariantIndex: 1, Keys: []string{"WHITE", "White"}},
		},
	}

	if !ApplyCanonical(product, metadata) {
		t.Fatal("ApplyCanonical() = false")
	}
	wantProductURLs := []string{
		"https://cdn/black-main.jpg",
		"https://cdn/black-side.jpg",
		"https://cdn/white-main.jpg",
	}
	if got := imageURLs(product.Images); !reflect.DeepEqual(got, wantProductURLs) {
		t.Fatalf("product image URLs = %#v", got)
	}
	if got := imageURLs(product.Variants[0].Images); !reflect.DeepEqual(
		got, wantProductURLs[:2]) {
		t.Fatalf("black image URLs = %#v", got)
	}
	if got := imageURLs(product.Variants[1].Images); !reflect.DeepEqual(
		got, wantProductURLs[2:]) {
		t.Fatalf("white image URLs = %#v", got)
	}
	if product.Images[0].Role != "primary" ||
		product.Images[1].Role != "gallery" {
		t.Fatalf("roles = %+v", product.Images)
	}
	wantTrace := canonicalTrace("SDS rendered mockup images", 0.98)
	if !reflect.DeepEqual(product.FieldTraces["images"], wantTrace) {
		t.Fatalf("images trace = %+v", product.FieldTraces["images"])
	}
	if ApplyCanonical(product, metadata) {
		t.Fatal("second ApplyCanonical() = true, want false")
	}
}

func imageURLs(images []canonical.Image) []string {
	out := make([]string, 0, len(images))
	for _, image := range images {
		out = append(out, image.URL)
	}
	return out
}
~~~

- [ ] **Step 2: Add default fallback and invalid lookup tests**

Append:

~~~go
func TestApplyCanonicalUsesDefaultImagesAndIgnoresInvalidLookups(t *testing.T) {
	product := &canonical.Product{
		Variants: []canonical.Variant{{SKU: "A"}, {SKU: "B"}},
	}
	metadata := CanonicalMetadata{
		MockupURLs: []string{" main.jpg ", "side.jpg", "main.jpg"},
		VariantLookup: []VariantLookup{
			{CanonicalVariantIndex: -1, Keys: []string{"A"}},
			{CanonicalVariantIndex: 9, Keys: []string{"B"}},
			{CanonicalVariantIndex: 0, Keys: []string{"", "missing"}},
		},
	}
	if !ApplyCanonical(product, metadata) {
		t.Fatal("ApplyCanonical() = false")
	}
	want := []string{"main.jpg", "side.jpg"}
	if !reflect.DeepEqual(imageURLs(product.Images), want) {
		t.Fatalf("product images = %+v", product.Images)
	}
	for i := range product.Variants {
		if !reflect.DeepEqual(imageURLs(product.Variants[i].Images), want) {
			t.Fatalf("variant[%d] images = %+v", i, product.Variants[i].Images)
		}
	}
}
~~~

- [ ] **Step 3: Verify RED**

~~~powershell
go test ./internal/product/sourcing/sdspod -run "TestApplyCanonical(AssignsRendered|UsesDefault)" -count=1
~~~

Expected: FAIL because the Task 2 stub never changes images.

- [ ] **Step 4: Implement the image policy mechanically**

Create images.go with these private functions, preserving the algorithms from internal/listingkit/sds_canonical_metadata.go except for the approved anonymous successful variant product-image union refinement:

~~~text
applyImages
renderedImagesByKey
imagesFromVariants
firstVariantImages
imagesFromMockups
resolveVariantImages
imagesEqual
normalizeKey
uniqueNonEmpty
copyImages
~~~

Use these exact rules, subject to the approved anonymous successful variant
product-image union refinement stated below:

~~~go
func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func imagesFromMockups(urls []string, trace canonical.FieldTrace) []canonical.Image {
	urls = uniqueNonEmpty(urls)
	images := make([]canonical.Image, 0, len(urls))
	for i, url := range urls {
		role := "gallery"
		if i == 0 {
			role = "primary"
		}
		images = append(images, canonical.Image{
			URL: url, Role: role, Trace: trace,
		})
	}
	return images
}

func imagesEqual(left, right []canonical.Image) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(left[i].URL) != strings.TrimSpace(right[i].URL) ||
			strings.TrimSpace(left[i].Role) != strings.TrimSpace(right[i].Role) {
			return false
		}
	}
	return true
}

func copyImages(input []canonical.Image) []canonical.Image {
	return append([]canonical.Image(nil), input...)
}
~~~

Implementation requirements for applyImages:

1. Build the image trace with detail SDS rendered mockup images and confidence 0.98.
2. Build by-key images from successful VariantMetadata values, indexing both SKU and Color after normalizeKey.
3. Build default images from CanonicalMetadata.MockupURLs; if empty, use the first successful variant image set.
4. If variant images exist, product.Images becomes the ordered de-duplicated union of successful variant images.
5. Otherwise product.Images uses default images.
6. For each valid VariantLookup index, test Keys in order and take the first matching image set; otherwise use default images.
7. Copy image slices when assigning them.
8. Write product.FieldTraces["images"] only when an image assignment changed.
9. Return false when no usable images exist or every URL/role sequence is already equal.

**Human-approved Task 3 refinement:** Treat successful variant images as
available for the product union independently of whether SKU or Color produced
a lookup key. Anonymous successful variants therefore participate in
`product.Images`; `byKey` remains only the per-variant lookup index.

Remove the temporary applyImages stub from apply.go.

- [ ] **Step 5: Format, test, and commit**

~~~powershell
$files = Get-ChildItem internal/product/sourcing/sdspod -Filter *.go | ForEach-Object FullName
gofmt -w $files
go test ./internal/product/sourcing/sdspod -count=1
git add internal/product/sourcing/sdspod
git commit -m "refactor: move sds pod canonical image policy"
~~~

Expected: PASS.

---

### Task 4: Add the ListingKit Compatibility Adapter and Delegate

**Files:**
- Modify: internal/listingkit/sds_canonical_metadata.go
- Modify: internal/listingkit/workflow_studio_sds_metadata_support.go
- Modify: internal/listingkit/workflow_studio_sds_metadata_test.go
- Modify: internal/listingkit/workflow_studio_fallback_test.go
- Modify: internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go

**Interfaces:**
- Consumes: sdspod.CanonicalMetadata and ApplyCanonical from Tasks 1-3.
- Produces: unchanged applySDSSyncMetadataToCanonical facade behavior except
  for the approved anonymous successful variant product-image union refinement.

- [ ] **Step 1: Preserve the existing facade characterization tests**

Run before production edits:

~~~powershell
go test ./internal/listingkit -run "TestApplySDSSyncMetadataToCanonical|TestStudioAttributesAndSpecificationsIncludeRichSDSFields" -count=1
~~~

Expected: PASS.

- [ ] **Step 2: Add a decorated supplier SKU compatibility test**

Add a facade test that creates:

~~~go
product := &canonical.Product{
	Variants: []canonical.Variant{{
		SKU: "MG17701061001-B6C753EB",
	}},
}
summary := &SDSSyncSummary{
	VariantResults: []SDSSyncSummary{{
		VariantSKU:      "MG17701061001",
		Status:          "completed",
		MockupImageURLs: []string{"https://cdn.sdspod.com/out/main.jpg"},
	}},
}
~~~

Call applySDSSyncMetadataToCanonical and assert the only canonical variant receives main.jpg. Run the test before the adapter change; it must PASS against the legacy implementation and becomes the compatibility oracle.

- [ ] **Step 3: Build the narrow adapter**

Retain applySDSSyncMetadataToCanonical in sds_canonical_metadata.go but replace its policy body with:

~~~go
func applySDSSyncMetadataToCanonical(
	product *canonical.Product,
	summary *SDSSyncSummary,
	options *SDSSyncOptions,
) bool {
	if product == nil {
		return false
	}
	return sdspod.ApplyCanonical(
		product,
		buildSDSPODCanonicalMetadata(product, summary, options),
	)
}
~~~

Add buildSDSPODCanonicalMetadata with these exact conversions:

~~~go
func buildSDSPODCanonicalMetadata(
	product *canonical.Product,
	summary *SDSSyncSummary,
	options *SDSSyncOptions,
) sdspod.CanonicalMetadata {
	metadata := sdspod.CanonicalMetadata{}
	if summary != nil {
		metadata.ProductName = summary.ProductName
		metadata.ProductSKU = summary.ProductSKU
		metadata.VariantSKU = firstSDSVariantValue(
			summary.VariantSKU, summary.VariantResults,
			func(item SDSSyncSummary) string { return item.VariantSKU })
		metadata.VariantSize = firstSDSVariantValue(
			summary.VariantSize, summary.VariantResults,
			func(item SDSSyncSummary) string { return item.VariantSize })
		metadata.VariantColor = firstSDSVariantValue(
			summary.VariantColor, summary.VariantResults,
			func(item SDSSyncSummary) string { return item.VariantColor })
		metadata.MockupURLs = append([]string(nil), summary.MockupImageURLs...)
		metadata.Variants = make([]sdspod.VariantMetadata, 0,
			len(summary.VariantResults))
		for _, item := range summary.VariantResults {
			metadata.Variants = append(metadata.Variants,
				sdspod.VariantMetadata{
					SKU:        item.VariantSKU,
					Color:      item.VariantColor,
					Status:     item.Status,
					MockupURLs: append([]string(nil), item.MockupImageURLs...),
				})
		}
	}
	if options != nil {
		if strings.TrimSpace(metadata.ProductName) == "" {
			metadata.ProductName = options.ProductName
		}
		metadata.StyleName = studioStyleName(options)
		metadata.Attributes = map[string]string{}
		for key, attr := range studioAttributes(
			options, canonical.FieldTrace{}) {
			metadata.Attributes[key] = attr.Value
		}
	}
	metadata.VariantLookup = buildSDSPODVariantLookups(product)
	return metadata
}
~~~

Add helpers:

~~~go
func firstSDSVariantValue(
	direct string,
	items []SDSSyncSummary,
	pick func(SDSSyncSummary) string,
) string {
	if value := strings.TrimSpace(direct); value != "" {
		return value
	}
	for _, item := range items {
		if value := strings.TrimSpace(pick(item)); value != "" {
			return value
		}
	}
	return ""
}

func buildSDSPODVariantLookups(
	product *canonical.Product,
) []sdspod.VariantLookup {
	if product == nil || len(product.Variants) == 0 {
		return nil
	}
	result := make([]sdspod.VariantLookup, 0, len(product.Variants))
	for i := range product.Variants {
		variant := &product.Variants[i]
		result = append(result, sdspod.VariantLookup{
			CanonicalVariantIndex: i,
			Keys: []string{
				variant.Attributes["source_sds_sku"].Value,
				sheinpub.SourceSDSSKUFromSupplierSKU(variant.SKU),
				variant.SKU,
				variant.Attributes["Color"].Value,
				variant.Attributes["color"].Value,
			},
		})
	}
	return result
}
~~~

Imports must include:

~~~go
sdspod "task-processor/internal/product/sourcing/sdspod"
sheinpub "task-processor/internal/publishing/shein"
~~~

- [ ] **Step 4: Remove duplicated ListingKit policy**

Delete from sds_canonical_metadata.go:

~~~text
applySDSIdentityAttributesToCanonical
summaryProductSKU
summaryVariantSKU
summaryVariantSize
summaryVariantColor
applySDSRenderedImagesToCanonical
sdsRenderedImagesByVariant
canonicalImagesFromSDSVariantResults
firstSDSVariantResultImages
canonicalImagesFromSDSMockups
resolveSDSCanonicalImagesForVariant
canonicalImagesEqual
trustedSDSProductName
~~~

Delete applyStudioStyleDimension from workflow_studio_sds_metadata_support.go. Keep studioStyleName, normalizeStyleIDSuffix, SKU helpers, and other unrelated functions.

Update phase6_workflow_studio_sds_metadata_support_boundary_test.go so it rejects a revived applyStudioStyleDimension and requires the sdspod delegate import/call in sds_canonical_metadata.go.

- [ ] **Step 5: Format and run compatibility suites**

~~~powershell
$files = @(
	'internal/listingkit/sds_canonical_metadata.go',
	'internal/listingkit/workflow_studio_sds_metadata_support.go',
	'internal/listingkit/workflow_studio_sds_metadata_test.go',
	'internal/listingkit/workflow_studio_fallback_test.go',
	'internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go'
)
gofmt -w $files
go test ./internal/product/sourcing/sdspod -count=1
go test ./internal/listingkit -run "TestApplySDSSyncMetadataToCanonical|TestStudioAttributesAndSpecificationsIncludeRichSDSFields|TestRunStandardProductWorkflow" -count=1
go test ./internal/listingkit/... -count=1
~~~

Expected: PASS with unchanged facade and workflow behavior except for the
approved anonymous successful variant product-image union refinement.

- [ ] **Step 6: Commit**

~~~powershell
git add internal/listingkit/sds_canonical_metadata.go internal/listingkit/workflow_studio_sds_metadata_support.go internal/listingkit/workflow_studio_sds_metadata_test.go internal/listingkit/workflow_studio_fallback_test.go internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go
git commit -m "refactor: delegate sds pod canonical metadata"
~~~

Expected: a commit containing only the ListingKit compatibility switch and tests.

---

### Task 5: Record Ownership and Verify the Repository

**Files:**
- Modify: docs/refactoring/listingkit-boundary-checkpoint.md

**Interfaces:**
- Consumes: completed sdspod package and ListingKit delegate.
- Produces: current ownership documentation and final validation evidence.

- [ ] **Step 1: Update the active checkpoint**

Add:

~~~markdown
### internal/product/sourcing/sdspod

Owns deterministic, platform-neutral SDS POD normalization into canonical
product facts, including trusted title, SDS identity attributes, Studio style
metadata, rendered mockup normalization, variant image assignment, and
canonical field traces.

Guardrail:

- internal/product/sourcing/sdspod may import only the Go standard library and
  internal/catalog/canonical; it must not import ListingKit, marketplace or
  publishing packages, SDS runtime/client packages, app/runtime, infra, HTTP,
  Temporal, or external SDKs.

Root internal/listingkit retains legacy SDS DTO adaptation, historical
decorated supplier-SKU lookup compatibility, task/workflow orchestration, and
changed-result propagation.
~~~

Set Last reviewed to 2026-07-11.

- [ ] **Step 2: Run hygiene and focused verification**

~~~powershell
$files = Get-ChildItem internal/product/sourcing/sdspod -Filter *.go | ForEach-Object FullName
$files += @(
	'internal/listingkit/sds_canonical_metadata.go',
	'internal/listingkit/workflow_studio_sds_metadata_support.go',
	'internal/listingkit/workflow_studio_sds_metadata_test.go',
	'internal/listingkit/workflow_studio_fallback_test.go',
	'internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go'
)
gofmt -w $files
git diff --check
go test ./internal/product/sourcing/sdspod -count=1
go test ./internal/product/sourcing/... ./internal/catalog/... -count=1
go test ./internal/listingkit/... -count=1
go test ./tests/... -count=1
~~~

Expected: no diff errors and all focused tests PASS.

- [ ] **Step 3: Run repository-wide verification**

~~~powershell
go test ./... -count=1
~~~

Expected: PASS. If the Windows worktree reproduces the known CRLF-sensitive internal/app/bootstrap/TestLifecycleRegistrationDoesNotDependOnAppServices failure, report the exact result without claiming repository-wide PASS and confirm no other package failed.

- [ ] **Step 4: Verify scope**

~~~powershell
git status --short
git diff --name-only HEAD~4
git diff -- go.work.sum
~~~

Allowed project scope:

~~~text
internal/product/sourcing/sdspod/*
internal/listingkit/sds_canonical_metadata.go
internal/listingkit/workflow_studio_sds_metadata_support.go
internal/listingkit/workflow_studio_sds_metadata_test.go
internal/listingkit/workflow_studio_fallback_test.go
internal/listingkit/phase6_workflow_studio_sds_metadata_support_boundary_test.go
docs/refactoring/listingkit-boundary-checkpoint.md
~~~

go.work.sum must remain an unstaged pre-existing modification with no task-owned diff.

- [ ] **Step 5: Commit the checkpoint**

~~~powershell
git add docs/refactoring/listingkit-boundary-checkpoint.md
git commit -m "docs: record sds pod canonical ownership"
~~~

---

## Final Acceptance Checklist

- [ ] sdspod owns deterministic title, identity, style, image, trace, precedence, fallback, and idempotency rules.
- [ ] `product.Images` includes the approved anonymous successful variant union;
      all other image precedence and per-variant lookup/fallback behavior remain
      unchanged.
- [ ] sdspod imports only standard library and internal/catalog/canonical.
- [ ] ListingKit retains DTO conversion, decorated SHEIN supplier SKU compatibility, orchestration, and changed-result propagation.
- [ ] SDSSyncSummary, SDSSyncOptions, public JSON, remote sync, baseline, Temporal, persistence, and SHEIN payload behavior are unchanged.
- [ ] New package, product sourcing, catalog, ListingKit, architecture, and repository tests pass or only the documented CRLF baseline remains.
- [ ] go.work.sum remains outside every task commit.
- [ ] The current ListingKit checkpoint records the new owner.
