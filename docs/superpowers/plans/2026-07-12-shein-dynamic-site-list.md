# SHEIN Dynamic Site List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Query the current store's enabled SHEIN sites and populate the existing task/product site list with a region-based fallback.

**Architecture:** Extend the existing SHEIN product API client with the supplier site-list endpoint, normalize the response in a focused `siteconfig` package, and update `SiteInfoHandler` to query and fall back before calling the existing `TaskContext.SetSiteList`. Downstream size, SKU, and SKC code remains unchanged.

**Tech Stack:** Go, `net/http/httptest`, existing SHEIN `BaseAPIClient`, standard `testing` package.

## Global Constraints

- Preserve existing `product.SiteInfo` and downstream site consumers.
- Keep `GetSiteListByRegion` as fallback.
- Do not add cross-task persistence or change inventory shelf-site behavior.
- Preserve API order after filtering and deduplication.
- Preserve unrelated `go.work.sum` changes.

---

### Task 1: Site-list API contract

**Files:**
- Modify: `internal/shein/client/endpoint.go`
- Modify: `internal/shein/client/endpoints.go`
- Create: `internal/shein/api/product/site_list.go`
- Create: `internal/shein/api/product/site_list_test.go`
- Modify: `internal/shein/api/product/interface.go`
- Modify: `internal/shein/api/product/client.go`

**Interfaces:**
- Produces: `SiteListGroup` with `MainSite`, `MainSiteName`, and `SubSiteList []SiteListSubSite`
- Produces: `SiteListSubSite` with `SiteName`, `SiteAbbr`, `SiteStatus`, `StoreType`, and `Currency`
- Produces: `func (*Client) QuerySiteList() ([]SiteListGroup, error)`

- [ ] Add an `httptest.Server` contract test asserting `POST`, exact path `/spmp-api-prefix/spmp/supplier/query_site_list`, and `{}` request body; return the supplied SHEIN US response and assert every decoded field.
- [ ] Run `go test ./internal/shein/api/product -run TestClientQuerySiteList -count=1` with `GOWORK=off`; expect compile failure for missing method/types.
- [ ] Add endpoint, response models, manager method, public method, and interface method using existing `APIRequest` and `ProcessAPIResponse` conventions.
- [ ] Run `go test ./internal/shein/api/product -count=1`; expect PASS.
- [ ] Commit with `git commit -m "feat: query SHEIN supplier sites"`.

### Task 2: Ordered site normalization

**Files:**
- Create: `internal/shein/siteconfig/sites.go`
- Create: `internal/shein/siteconfig/sites_test.go`

**Interfaces:**
- Consumes: `[]product.SiteListGroup`
- Produces: `func Normalize([]product.SiteListGroup) []product.SiteInfo`

- [ ] Write table tests proving empty main sites are ignored; only `SiteStatus == 1` and non-empty site abbreviations survive; duplicate main sites merge; duplicate sub-sites are removed; and first-seen main/sub-site order remains stable.
- [ ] Run `go test ./internal/shein/siteconfig -count=1`; expect compile/package failure.
- [ ] Implement normalization with an ordered result slice plus index/membership maps. Trim identifiers, omit groups with no valid sub-sites, and allocate independent `SubSiteList` slices.
- [ ] Run `go test ./internal/shein/siteconfig -count=1`; expect PASS.
- [ ] Commit with `git commit -m "feat: normalize SHEIN supplier sites"`.

### Task 3: Dynamic SiteInfoHandler with fallback

**Files:**
- Modify: `internal/shein/store/site.go`
- Create: `internal/shein/store/site_test.go`
- Preserve: `internal/shein/store/region.go`

**Interfaces:**
- Consumes: `ctx.ProductAPI.QuerySiteList()` and `siteconfig.Normalize`
- Produces: populated `ctx.SiteList` and `ctx.ProductData.SiteList` through `ctx.SetSiteList`

- [ ] Write a success test using the real product client with `httptest.Server`; assert one request, disabled sites excluded, and both context/product lists equal `[{MainSite:"shein", SubSiteList:["shein-us"]}]`.
- [ ] Run `go test ./internal/shein/store -run TestSiteInfoHandlerUsesDynamicSites -count=1`; expect failure because the handler still uses the region mapping and never requests the endpoint.
- [ ] Update `SiteInfoHandler.Handle`: query once when `ProductAPI` exists, normalize, use dynamic results when non-empty, otherwise log a warning and call `GetSiteListByRegion`.
- [ ] Add API-error and empty-valid-response tests for region fallback. Add a test with missing task/unknown region only if the existing mapping can return empty; assert a clear error when no site can be resolved.
- [ ] Run `go test ./internal/shein/store -count=1`; expect PASS.
- [ ] Run `go test ./internal/shein/product/size ./internal/shein/product/sku ./internal/shein/product/skc -count=1`; expect PASS without downstream changes.
- [ ] Commit with `git commit -m "feat: use dynamic SHEIN supplier sites"`.

### Task 4: Final verification

**Files:**
- Verify all files from Tasks 1–3; no production files added in this task.

- [ ] Run `gofmt` on all modified Go files.
- [ ] Run `go test ./internal/shein/api/product ./internal/shein/siteconfig ./internal/shein/store ./internal/shein/product/size ./internal/shein/product/sku ./internal/shein/product/skc -count=1`; expect PASS.
- [ ] Run `go test ./internal/shein/... -count=1`; expect PASS.
- [ ] Run `go vet ./internal/shein/api/product ./internal/shein/siteconfig ./internal/shein/store`; expect PASS.
- [ ] Run `git diff --check` and `git status --short`; expect no whitespace errors and no uncommitted files.
