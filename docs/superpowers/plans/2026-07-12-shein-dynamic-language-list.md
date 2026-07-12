# SHEIN Dynamic Language List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fetch the current store's supported product languages once per task and use the same ordered language list for product names and SKC names.

**Architecture:** Extend the existing SHEIN product client with the language-list endpoint, normalize API rows in a small `languageconfig` package, and load the result in the translation handler alongside name-length configuration. Store the resolved list on `TaskContext`, pass it explicitly to product localization, and copy it into `SKCRuntimeInput`; retain the existing region mapping only as a non-failing fallback.

**Tech Stack:** Go, `net/http/httptest`, existing SHEIN `BaseAPIClient`, standard `testing` package.

## Global Constraints

- Do not change language detection, translation APIs, AI optimization, or product-description behavior.
- Do not add cross-task persistent caching.
- Preserve `GetTargetLanguagesByRegion` as fallback.
- Keep API order after filtering and normalized-code deduplication.
- Preserve unrelated `go.work.sum` changes.

---

### Task 1: Language-list API contract

**Files:**
- Modify: `internal/shein/client/endpoint.go`
- Modify: `internal/shein/client/endpoints.go`
- Create: `internal/shein/api/product/language_list.go`
- Create: `internal/shein/api/product/language_list_test.go`
- Modify: `internal/shein/api/product/interface.go`
- Modify: `internal/shein/api/product/client.go`

**Interfaces:**
- Produces: `type LanguageListItem struct { LanguageAbbr string; LanguageName string; InputMode int }`
- Produces: `func (*Client) QueryLanguageList() ([]LanguageListItem, error)`

- [ ] Write `TestClientQueryLanguageList`, backed by `httptest.Server`, asserting `POST`, exact path `/spmp-api-prefix/spmp/basic/get_language_list`, and an empty JSON object request. Respond with the supplied `en` and `fr` rows and assert both decoded items.
- [ ] Run `go test ./internal/shein/api/product -run TestClientQueryLanguageList -count=1` with `GOWORK=off`; expect compile failure because the method and type do not exist.
- [ ] Add the endpoint, response models, product-manager method, public client method, and interface method using existing `APIRequest` and `ProcessAPIResponse` patterns.
- [ ] Run `go test ./internal/shein/api/product -count=1`; expect PASS.
- [ ] Commit with `git commit -m "feat: query SHEIN product languages"`.

### Task 2: Ordered language normalization

**Files:**
- Create: `internal/shein/languageconfig/languages.go`
- Create: `internal/shein/languageconfig/languages_test.go`
- Modify: `internal/shein/context/context.go`

**Interfaces:**
- Consumes: `[]product.LanguageListItem`
- Produces: `func Normalize([]product.LanguageListItem) []string`
- Produces: `func Resolve(items []product.LanguageListItem, region string) []string`
- Produces: `TaskContext.TargetLanguages []string`

- [ ] Write table tests proving that `Normalize` filters `InputMode <= 0`, ignores empty codes, lowercases and trims codes, removes duplicates, and preserves first-seen order. Add a `Resolve` test showing empty valid results fall back to `submitprep.GetTargetLanguagesByRegion`, with a final `en` fallback.
- [ ] Run `go test ./internal/shein/languageconfig -count=1`; expect compile/package failure.
- [ ] Implement normalization with a `map[string]struct{}` only for membership and a slice for order. Return fresh slices so callers cannot mutate shared storage.
- [ ] Add `TargetLanguages []string` to the product/task state in `internal/shein/context/context.go`.
- [ ] Run `go test ./internal/shein/languageconfig ./internal/shein/context -count=1`; expect PASS.
- [ ] Commit with `git commit -m "feat: normalize SHEIN product languages"`.

### Task 3: Load once and apply to product names

**Files:**
- Modify: `internal/shein/translate/translate.go`
- Modify: `internal/shein/translate/translate_test.go`
- Modify: `internal/shein/submitprep/localized_content.go`
- Modify: `internal/shein/submitprep/localized_content_test.go`

**Interfaces:**
- Consumes: `ProductAPI.QueryLanguageList()` and `languageconfig.Resolve`
- Changes: `BuildLocalizedTitleAndDescription` accepts `targetLanguages []string` rather than deriving languages from region internally.
- Produces: a non-empty `TaskContext.TargetLanguages`, initialized once per task.

- [ ] Extend the real-client handler test server to serve both configuration endpoints, count language-list requests, and return `en`, disabled `es`, and `fr`. Assert two `Handle` calls make one language request and product names contain exactly `en`, `fr` in that order.
- [ ] Run `go test ./internal/shein/translate -run 'LanguageList|TargetLanguages' -count=1`; expect assertion/compile failure because the handler does not query or store languages.
- [ ] Add `loadTargetLanguages`: if context languages are non-empty, return; otherwise query once, normalize, and fall back on error or empty result with a warning. Pass a copied list to localized content.
- [ ] Change localized content to consume the explicit list and remove its normal-path call to `GetTargetLanguagesByRegion`.
- [ ] Add failure and empty-response tests asserting the existing region mapping is used and no retry occurs.
- [ ] Run `go test ./internal/shein/translate ./internal/shein/submitprep -count=1`; expect PASS.
- [ ] Commit with `git commit -m "feat: use dynamic SHEIN product languages"`.

### Task 4: Share target languages with SKC generation

**Files:**
- Modify: `internal/shein/product/skc/skc_build_input.go`
- Modify: `internal/shein/product/skc/translation.go`
- Modify: `internal/shein/product/skc/translation_test.go`

**Interfaces:**
- Changes: `SKCRuntimeInput` gains `TargetLanguages []string` copied from `TaskContext.TargetLanguages`.
- Changes: `CreateSKC` consumes runtime target languages; it uses the region mapping only if the runtime slice is empty.

- [ ] Write a failing test with runtime languages `[]string{"en", "fr"}` and assert `CreateSKC` creates exactly those `MultiLanguageNameList` entries in order, independently of region.
- [ ] Run `go test ./internal/shein/product/skc -run TestCreateSKCUsesTaskTargetLanguages -count=1`; expect failure because `SKCRuntimeInput` lacks the field and `CreateSKC` still derives languages from region.
- [ ] Copy the context slice in `newSKCRuntimeInput`; update `CreateSKC` to copy runtime languages and use region fallback only for an empty slice.
- [ ] Confirm configured name limits still apply to every dynamic language with an `fr` limit regression assertion.
- [ ] Run `gofmt` on modified files, then `go test ./internal/shein/api/product ./internal/shein/languageconfig ./internal/shein/translate ./internal/shein/submitprep ./internal/shein/product/skc -count=1`; expect PASS.
- [ ] Run `go test ./internal/shein/... -count=1` and related-package `go vet`; expect PASS.
- [ ] Commit with `git commit -m "feat: share SHEIN languages with SKC names"`.
