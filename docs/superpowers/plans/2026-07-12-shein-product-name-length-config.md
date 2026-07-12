# SHEIN Product Name Length Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fetch category-specific, per-language SHEIN name limits once per task and apply them to both product names and SKC names.

**Architecture:** Extend the existing SHEIN product client with the remote config endpoint, normalize the response into a focused `NameLengthLimits` value, and load it into `TaskContext` after category selection. Product and SKC translation paths consume that shared value and use a rune-safe truncation helper; remote failures produce a warning and the existing English limit of 150 remains the fallback.

**Tech Stack:** Go, standard `net/http` test server, existing SHEIN `BaseAPIClient`, Go `testing` package.

## Global Constraints

- Product descriptions are unchanged.
- Do not add a cross-task persistent cache.
- Do not change minimum-name rules, target languages, or category selection.
- English defaults to 150 when configuration is unavailable; other missing languages retain current no-truncation behavior.
- Count and truncate Unicode code points, not UTF-8 bytes.
- Preserve unrelated uncommitted workspace changes.

---

### Task 1: Product name-length configuration API

**Files:**
- Modify: `internal/shein/client/endpoint.go`
- Modify: `internal/shein/client/endpoints.go`
- Create: `internal/shein/api/product/name_length_config.go`
- Modify: `internal/shein/api/product/interface.go`
- Modify: `internal/shein/api/product/client.go`
- Test: `internal/shein/api/product/name_length_config_test.go`

**Interfaces:**
- Produces: `type NameLengthConfigItem struct { Language string; MaxLength int }`
- Produces: `func (*Client) QueryProductNameLengthConfig(categoryID int) ([]NameLengthConfigItem, error)`

- [ ] **Step 1: Write a failing HTTP contract test**

Create a test server that asserts `POST`, the exact path `/spmp-api-prefix/spmp/product/publish/config/query_product_name_length_config`, and JSON body `{"category_id":1772}`; respond with code `0` and the three supplied language limits, then assert the returned slice.

```go
func TestClient_QueryProductNameLengthConfig(t *testing.T) {
	// Build the client with the package's existing test BaseAPIClient helper.
	// Assert request method/path/body in httptest.HandlerFunc.
	got, err := client.QueryProductNameLengthConfig(1772)
	if err != nil { t.Fatal(err) }
	if diff := cmp.Diff([]NameLengthConfigItem{{"en", 150}, {"zh-cn", 100}, {"zh-tw", 105}}, got); diff != "" {
		t.Fatalf("config mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./internal/shein/api/product -run TestClient_QueryProductNameLengthConfig -count=1`

Expected: compile failure because `QueryProductNameLengthConfig` and `NameLengthConfigItem` do not exist.

- [ ] **Step 3: Implement the minimal endpoint and client method**

Add the endpoint constant/getter, response types, and method. Use the existing `APIRequest` and `ProcessAPIResponse` conventions:

```go
func (m *productManager) queryProductNameLengthConfig(categoryID int) ([]NameLengthConfigItem, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetProductNameLengthConfigEndpoint())
	var result struct {
		api.APIResponse
		Info []NameLengthConfigItem `json:"info"`
	}
	if err := m.baseClient.APIRequest(http.MethodPost, url, struct {
		CategoryID int `json:"category_id"`
	}{CategoryID: categoryID}, &result); err != nil { return nil, err }
	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "获取产品名称长度配置失败"); err != nil { return nil, err }
	return result.Info, nil
}
```

Expose the method through `Client` and `ProductAPI`.

- [ ] **Step 4: Run the focused test and package tests**

Run: `go test ./internal/shein/api/product -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the API slice**

```powershell
git add internal/shein/client/endpoint.go internal/shein/client/endpoints.go internal/shein/api/product/name_length_config.go internal/shein/api/product/name_length_config_test.go internal/shein/api/product/interface.go internal/shein/api/product/client.go
git commit -m "feat: query SHEIN product name limits"
```

### Task 2: Shared normalized limits and rune-safe truncation

**Files:**
- Create: `internal/shein/namelimit/limits.go`
- Test: `internal/shein/namelimit/limits_test.go`
- Modify: `internal/shein/context/context.go`

**Interfaces:**
- Consumes: `[]product.NameLengthConfigItem`
- Produces: `type Limits map[string]int`
- Produces: `func Normalize([]product.NameLengthConfigItem) Limits`
- Produces: `func (Limits) Max(language string) (int, bool)`
- Produces: `func Truncate(text string, maxLength int) string`
- Produces: `TaskContext.ProductNameLengthLimits namelimit.Limits`

- [ ] **Step 1: Write failing table tests**

Cover lowercase/trim normalization, ignored empty language and non-positive limit, case-insensitive lookup, Chinese rune truncation, and English word-boundary truncation.

```go
func TestTruncateCountsRunes(t *testing.T) {
	if got := Truncate("一二三四五", 3); got != "一二三" { t.Fatalf("got %q", got) }
}

func TestNormalize(t *testing.T) {
	got := Normalize([]product.NameLengthConfigItem{{Language: " ZH-CN ", MaxLength: 100}, {Language: "", MaxLength: 9}, {Language: "en", MaxLength: 0}})
	if max, ok := got.Max("zh-cn"); !ok || max != 100 { t.Fatalf("got %d, %v", max, ok) }
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run: `go test ./internal/shein/namelimit -count=1`

Expected: compile/package failure because `namelimit` does not exist.

- [ ] **Step 3: Implement the focused value object**

Use `[]rune(strings.TrimSpace(text))` for length and slicing. Preserve the existing SKC behavior of preferring a nearby space only when it occurs within the final 50 runes.

```go
func (l Limits) Max(language string) (int, bool) {
	max, ok := l[strings.ToLower(strings.TrimSpace(language))]
	return max, ok && max > 0
}
```

Add the limits field to `ProductState` or the nearest existing task product state struct rather than adding global state.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/shein/namelimit ./internal/shein/context -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the shared model**

```powershell
git add internal/shein/namelimit internal/shein/context/context.go
git commit -m "feat: add shared SHEIN name limits"
```

### Task 3: Load configuration once per task and apply product-name limits

**Files:**
- Modify: `internal/shein/translate/translate.go`
- Modify: `internal/shein/submitprep/localized_content.go`
- Test: `internal/shein/translate/translate_test.go`
- Test: `internal/shein/submitprep/localized_content_test.go`

**Interfaces:**
- Consumes: `ProductAPI.QueryProductNameLengthConfig(int)` and `namelimit.Limits`
- Changes: `BuildLocalizedTitleAndDescription(..., limits namelimit.Limits)`
- Produces: one request per `TranslateHandler.Handle`, shared on `TaskContext.ProductNameLengthLimits`

- [ ] **Step 1: Write failing product-name tests**

Add a localized-content test using long English and Chinese translations and limits `{en: 12, zh-cn: 4}`. Assert rune counts do not exceed the configured values. Add a handler test backed by `httptest.Server` and the real `product.Client`; count requests in the handler, return config, and assert one call with `ProductData.CategoryID` plus the limits stored on context.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/shein/submitprep ./internal/shein/translate -run 'NameLimit|LengthConfig' -count=1`

Expected: compile/assertion failure because localized content does not accept or apply limits and the handler does not query config.

- [ ] **Step 3: Implement minimal loading and product truncation**

In `TranslateHandler.Handle`, query only when the context has no previously initialized limits and category ID is positive. Normalize successful data; on failure log a warning and set an empty non-nil map to mark the attempted load. Pass the map to localized content and apply:

```go
if max, ok := limits.Max(targetLang); ok {
	text = namelimit.Truncate(text, max)
}
```

Do not truncate descriptions.

- [ ] **Step 4: Add and verify fallback test**

Make the test server return a SHEIN error response. Assert `Handle` still succeeds, the API is called once, and repeated consumption does not retry. Run:

`go test ./internal/shein/submitprep ./internal/shein/translate -count=1`

Expected: PASS.

- [ ] **Step 5: Commit product-name integration**

```powershell
git add internal/shein/translate/translate.go internal/shein/translate/translate_test.go internal/shein/submitprep/localized_content.go internal/shein/submitprep/localized_content_test.go
git commit -m "feat: apply dynamic SHEIN product name limits"
```

### Task 4: Apply shared limits to SKC names and prompts

**Files:**
- Modify: `internal/shein/product/skc/skc_build_input.go`
- Modify: `internal/shein/product/skc/translation.go`
- Modify: `internal/shein/product/skc/translation_test.go`

**Interfaces:**
- Consumes: `TaskContext.ProductNameLengthLimits`
- Changes: `SKCRuntimeInput` gains `NameLengthLimits namelimit.Limits`
- Replaces: hardcoded byte-based `sheinSKCTitleMaxLength` checks with `maxNameLength(language)` and `namelimit.Truncate`

- [ ] **Step 1: Write failing SKC tests**

Replace the existing constant-bound test with a configured English limit of 25. Add a Chinese `LanguageContent` of five Han characters with limit 3 and assert the result is exactly three runes. Add a prompt test using a fake chat completer and assert the system prompt says `10-25 characters`.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/shein/product/skc -run 'NameLimit|Truncates|Prompt' -count=1`

Expected: FAIL because SKC translation still uses the hardcoded 150-byte limit and only processes English truncation.

- [ ] **Step 3: Implement dynamic SKC handling**

Copy the task-level limits into `SKCRuntimeInput`. Use 150 only for a missing English entry:

```go
func (h *SKCTranslationHandler) maxNameLength(language string) (int, bool) {
	if max, ok := h.runtime.NameLengthLimits.Max(language); ok { return max, true }
	if strings.EqualFold(language, "en") { return 150, true }
	return 0, false
}
```

Apply configured limits to every language after translation/optimization and build the English optimization prompt with `fmt.Sprintf` using the resolved English maximum. Remove the old byte-slicing helper.

- [ ] **Step 4: Run focused and regression tests**

Run: `go test ./internal/shein/product/skc ./internal/shein/submitprep ./internal/shein/translate ./internal/shein/api/product -count=1`

Expected: PASS with no warnings or panics.

- [ ] **Step 5: Run formatting and broader SHEIN verification**

Run: `gofmt -w internal/shein/client/endpoint.go internal/shein/client/endpoints.go internal/shein/api/product internal/shein/namelimit internal/shein/context/context.go internal/shein/translate internal/shein/submitprep/localized_content.go internal/shein/product/skc`

Run: `go test ./internal/shein/... -count=1`

Expected: PASS. If unrelated pre-existing failures occur, record the exact package and output without changing unrelated code.

- [ ] **Step 6: Commit SKC integration**

```powershell
git add internal/shein/product/skc/skc_build_input.go internal/shein/product/skc/translation.go internal/shein/product/skc/translation_test.go
git commit -m "feat: apply dynamic SHEIN SKC name limits"
```
