# 1688 Public Source Account Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or **superpowers:executing-plans** to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 1688 公开页面抓取不再依赖登录账号，并把账号辅助抓取与 SHEIN 目标店铺彻底拆开；`listing_store` 只保留目标上架店铺职责，1688 账号只从独立的 source-account 边界读取。

**Architecture:** 新增 `internal/sourceaccount` 领域边界和 `source_account` 表。HTTP 请求中的 `source_account_id` 保持向后兼容：缺省或 0 表示 public，正数表示 account-assisted。worker 先执行现有公开浏览器流程，只有明确的公开访问失败（登录/挑战/缺失必要字段）且请求带正数账号 ID 时，才按租户解析独立 SourceAccount 并重试。SHEIN `shein_store_id` 继续由 `listing_store` + `StoreAccessValidator` 校验；source account 由独立 `SourceAccountAccessValidator` 校验，二者不共享查询或错误域。

**Tech Stack:** Go, GORM/PostgreSQL, existing Playwright 1688 crawler, `net/http`, PowerShell/Pester, existing crawler stats/Prometheus text endpoint, existing ListingKit schema-migration runner.

**Spec:** [docs/superpowers/specs/2026-08-20-1688-public-source-account-boundary-design.md](../specs/2026-08-20-1688-public-source-account-boundary-design.md)

## Global Constraints

- `listing_store` is never queried with platform `1688` after this change. Any source-account lookup must go through `internal/sourceaccount` and `source_account`.
- Do not backfill or copy rows from `listing_store`; the current production data has no valid 1688 source-account rows.
- Public crawl must work when `source_account_id` is omitted or `0`. Negative IDs are invalid requests.
- Account records persist only opaque references (`profile_ref`, `proxy_ref`) and non-secret metadata. Do not persist passwords, cookies, browser profile paths, proxy credentials, or raw provider tokens.
- A SHEIN target remains required for ListingKit task creation and is still validated with expected platform `SHEIN`; public source access does not make the target store optional.
- `source_store_id` remains rejected at the HTTP boundary.
- Source identity and neutral product facts must not contain the login account ID.
- Public fallback is narrow: invalid URL, parser errors, non-retryable transport/browser startup errors, sensitive-content rejection, and other deterministic failures must not trigger account lookup.
- Preserve tenant scoping and sanitized customer-visible errors. Never log database URLs, secret references, profile paths, cookies, or proxy credentials.
- Work test-first in small commits. Each task below requires a failing test before implementation, a focused passing test after implementation, and a commit before moving to the next task.

---

## Task 1: Define the source-account domain and explicit access metadata

**Files:**

- Create `internal/sourceaccount/account.go`.
- Create `internal/sourceaccount/errors.go`.
- Create `internal/sourceaccount/repository.go`.
- Create `internal/sourceaccount/account_test.go`.
- Create `internal/sourceaccount/errors_test.go`.
- Modify `internal/crawler/shared/task.go`.
- Modify `internal/crawler/shared/result.go`.
- Modify `internal/crawler/shared/base_service.go` only if the new stats map needs a shared helper.

**Interfaces and types to add:**

```go
package sourceaccount

type AccessMode string

const (
	AccessModePublic          AccessMode = "public"
	AccessModeAccountAssisted AccessMode = "account_assisted"
)

const (
	PlatformAlibaba1688 = "1688"
	StatusEnabled int16 = 0
	StatusDisabled int16 = 1
)

type SourceAccount struct {
	ID             int64
	TenantID       int64
	Platform       string
	Label          string
	ProfileRef     string
	ProxyRef       string
	LoginURL       string
	Status         int16
	Deleted        int16
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Repository interface {
	Get(context.Context, int64, int64) (*SourceAccount, error)
}

type Access struct {
	ID       int64
	TenantID int64
	Platform string
	Enabled  bool
}

type AccessValidator interface {
	ValidateSourceAccountAccess(context.Context, int64, int64) (Access, error)
}

func SelectAccessMode(int64) (AccessMode, error)
func ErrorCode(error) string
```

Use stable error codes `source_account_unavailable` and `source_account_disabled`. `SelectAccessMode(0)` returns public, a positive ID returns account-assisted, and a negative ID returns a sanitized unavailable/invalid error.

**Steps (TDD):**

- [ ] Add table-driven tests for zero/positive/negative ID mode selection, disabled/unavailable error codes, and rejection of a non-1688 account in a fake repository validator. Run `go test ./internal/sourceaccount ./internal/crawler/shared`; the new tests must fail because the package and metadata fields do not exist.
- [ ] Implement the domain types, errors, repository/validator interfaces, and `CrawlerTask.SourceAccessMode` plus `CrawlerResult.SourceAccessMode` and `CrawlerResult.SourceFallbackReason` JSON fields. Keep `SourceAccountID int64` for compatibility and make mode inference deterministic (`0 => public`, `>0 => account_assisted`) when old serialized tasks have no mode.
- [ ] Add unit tests proving result JSON round-trips the two redacted metadata fields and that no account ID is projected into a source identity helper. Run `go test ./internal/sourceaccount ./internal/crawler/shared` and require passing output.
- [ ] Commit with `git add internal/sourceaccount internal/crawler/shared && git commit -m "feat: define 1688 source account boundary"`.

## Task 2: Add the dedicated `source_account` schema, repository, and runtime builder

**Files:**

- Create `internal/sourceaccount/gorm_repository.go`.
- Create `internal/sourceaccount/gorm_repository_test.go`.
- Create `internal/listingkit/httpapi/builders_source_account.go`.
- Create `internal/listingkit/httpapi/builders_source_account_test.go`.
- Modify `internal/listingkit/schema/runtime.go`.
- Modify `internal/listingkit/httpapi/bootstrap_repositories_contracts.go` only if the repository builder needs to be exposed through existing build input.

**Concrete schema:**

Use GORM row `sourceAccountRow` with `TableName() string { return "source_account" }` and columns `id BIGINT`, `tenant_id BIGINT`, `platform VARCHAR(32)`, `label VARCHAR(128)`, `profile_ref VARCHAR(256)`, `proxy_ref VARCHAR(256)`, `login_url TEXT`, `status SMALLINT`, `deleted SMALLINT`, `last_verified_at TIMESTAMP NULL`, `created_at TIMESTAMP`, and `updated_at TIMESTAMP`. Add indexes for `(tenant_id, platform, status, deleted)` and `(tenant_id, id, deleted)`. The repository must require `tenant_id > 0`, `id > 0`, `platform = 1688`, `deleted = 0`, and must map nonzero status to the disabled error. It must never read `listing_store`.

**Interfaces and functions to add:**

```go
func NewGormRepository(*gorm.DB) *GormRepository
func AutoMigrateRepository(*gorm.DB) error
func (r *GormRepository) Get(context.Context, int64, int64) (*SourceAccount, error)
func (r *GormRepository) ValidateSourceAccountAccess(context.Context, int64, int64) (Access, error)
```

Add `BuildSourceAccountRepository(*config.Config, *logrus.Logger) (sourceaccount.Repository, []func() error, error)` in `internal/listingkit/httpapi`. Reuse the existing database opener and fallback behavior: no configured database returns `(nil, nil, nil)` so public crawl remains available; configured database errors are sanitized in logs. Add `sourceaccount.AutoMigrateRepository(db)` to `internal/listingkit/schema.AutoMigrateRuntime` with an explicit error prefix. No migration may inspect or mutate `listing_store`.

**Steps (TDD):**

- [ ] Write GORM tests that migrate only `source_account`, insert two tenants plus a SHEIN-like row in `listing_store`, and prove: tenant isolation, disabled/deleted filtering, platform mismatch rejection, and no query against the target-store row can satisfy a source-account lookup. Run `go test ./internal/sourceaccount -run 'Test(Gorm|AutoMigrate)'`; tests must fail before the repository exists.
- [ ] Implement the row mapping, migration, tenant-scoped `Get`, access validator, and builder. Keep `profile_ref`/`proxy_ref` opaque and never include their values in returned errors.
- [ ] Add schema runtime tests asserting `AutoMigrateRuntime` calls the source-account migration and preserves existing migration error prefixes. Add builder tests for no-DB fallback and sanitized DB-builder failure. Run `go test ./internal/sourceaccount ./internal/listingkit/schema ./internal/listingkit/httpapi`.
- [ ] Commit with `git add internal/sourceaccount internal/listingkit/schema internal/listingkit/httpapi && git commit -m "feat: persist independent 1688 source accounts"`.

## Task 3: Replace listing-store account resolution and implement public-first crawler fallback

**Files:**

- Modify `internal/crawler/alibaba1688/account_profile.go`.
- Modify `internal/crawler/alibaba1688/account_profile_test.go`.
- Create `internal/crawler/alibaba1688/public_access_failure.go`.
- Create `internal/crawler/alibaba1688/public_access_failure_test.go`.
- Modify `internal/crawler/alibaba1688/single_processor.go`.
- Modify `internal/crawler/alibaba1688/page_operator.go` and its tests where challenge errors are wrapped.
- Modify `internal/crawler/alibaba1688/worker_processor.go`.
- Modify `internal/crawler/alibaba1688/worker_processor_test.go`.
- Modify `internal/crawler/alibaba1688/crawler_service.go`.

**Resolver contract:**

Change `NewAccountProfileResolver` to accept `sourceaccount.Repository`, not `listingadmin.StoreRepository`. A valid record must match tenant and ID, be platform `1688`, enabled, non-deleted, and have a nonblank `profile_ref`; derive the runtime profile directory only from the configured root plus tenant/account IDs. Keep `AccountProfileErrorCode` compatibility aliases mapped to `source_account_unavailable` and `source_account_disabled` so existing task status consumers do not change.

**Failure classification contract:**

Add:

```go
type PublicAccessFailureKind string

const (
	PublicAccessFailureChallenge     PublicAccessFailureKind = "challenge"
	PublicAccessFailureMissingFields PublicAccessFailureKind = "missing_fields"
	PublicAccessFailureInvalidURL    PublicAccessFailureKind = "invalid_url"
	PublicAccessFailureTransport     PublicAccessFailureKind = "transport"
)

type PublicAccessError struct { Kind PublicAccessFailureKind; Err error }
func IsAccountFallbackEligible(error) bool
```

Wrap only known challenge/captcha/login and incomplete extraction failures from the public browser path. URL validation, browser startup, non-retryable navigation/transport, sensitive-word, pricing, and image validation failures remain ineligible. Use `errors.Is`/typed errors, not a broad “any error means retry with account” rule.

**Worker algorithm:**

Add a small `fetchProduct` method on `Crawler1688Processor` returning `(*model.Product1688, sourceaccount.AccessMode, string, error)`. It must:

1. Reject negative IDs.
2. Call `processor1688.Process(task.URL)` first for every task.
3. Return public success with mode `public` when it succeeds.
4. If the public error is not eligible or the ID is zero, return sanitized code `source_public_unavailable` without touching the source-account repository.
5. If eligible and ID is positive, resolve the independent account, lock it, and call `ProcessWithAccountProfile`; return mode `account_assisted` and fallback reason `public_<kind>` on success.
6. Return `source_account_unavailable`/`source_account_disabled` unchanged when resolution fails; never call `listingadmin.StoreRepository`.

Record the selected mode and redacted fallback reason on `CrawlerResult`. Add a `map[string]int64` source counter in `Service`, protected by the existing service mutex or atomics, and expose it through `GetStats()` as `source_access_total` with keys `public`, `account_assisted`, `source_public_unavailable`, `source_account_unavailable`, and `source_account_disabled`; the existing `/metrics` renderer will emit labeled metrics.

**Steps (TDD):**

- [ ] Replace account-profile tests with a fake `sourceaccount.Repository`; add a regression test that a `listingadmin.StoreRepository` containing a `platform=1688` row is not accepted or referenced. Add classifier tests for each eligible/ineligible failure kind and worker tests for public success, public failure with no account (no repository call), fallback with account, disabled account, and tenant mismatch. Run `go test ./internal/crawler/alibaba1688`; the new tests must fail against the current listing-store resolver and account-first worker behavior.
- [ ] Implement the independent resolver, typed public-access errors, narrow fallback algorithm, result metadata, and source counters. Preserve profile locking and existing `Process`/`ProcessWithAccountProfile` processor interfaces.
- [ ] Run `go test ./internal/crawler/alibaba1688 ./internal/crawler/shared` and assert `/metrics` test output contains `crawler_source_access_total{key="public"}` and `crawler_source_access_total{key="account_assisted"}` for exercised paths.
- [ ] Commit with `git add internal/crawler/alibaba1688 internal/crawler/shared && git commit -m "feat: make 1688 crawling public first"`.

## Task 4: Wire the optional source account through HTTP and application composition

**Files:**

- Modify `internal/infra/httpx/crawler_1688_handler.go`.
- Modify `internal/infra/httpx/crawler_1688_handler_test.go`.
- Modify `internal/crawler/alibaba1688/api_service.go` and `api_service_test.go`.
- Modify `internal/app/httpapi/crawler_1688_module.go`.
- Modify `internal/app/httpapi/composition_builder.go` and `composition_builder_test.go`.
- Modify any constructor call sites found by `rg -n "NewAPIServiceWithStoreRepository|NewAccountProfileResolver|newCrawler1688HTTPModule"`.

**Contract changes:**

- Keep request JSON `source_account_id` optional and reject only negative values. Set `CrawlerTask.SourceAccessMode` from the field while retaining the int64 field for old Redis/task payloads.
- Public requests still receive a trusted tenant when the authenticated tenant resolver is installed so result reads remain tenant-scoped. Change the error text from “tenant context is required for source_account_id” to “trusted tenant context is required for crawler task scope”.
- Replace `NewAPIServiceWithStoreRepository(... listingadmin.StoreRepository)` with `NewAPIServiceWithSourceAccountRepository(... sourceaccount.Repository)`. `NewAPIService` must call `BuildSourceAccountRepository`, never `BuildListingAdminStoreRepository`.
- In the app composition, build the source-account repository once, pass it to both the crawler module and handoff module, append its closer, and keep the existing listing-admin repository only for SHEIN/target features. If the source builder is unavailable, public crawling still starts.

**Steps (TDD):**

- [ ] Add HTTP tests for omitted/zero ID public submission, positive ID account-assisted submission, negative ID 400, trusted-tenant requirement, and JSON/task mode inference. Add API wiring tests that inject a source-account fake and assert it is the resolver dependency while a listing-store fake is never passed. Run `go test ./internal/infra/httpx ./internal/crawler/alibaba1688 ./internal/app/httpapi`; tests must fail before wiring changes.
- [ ] Implement handler mode assignment, constructor renaming, source builder wiring, closer handling, and composition updates. Update all affected fakes and call sites in the same commit.
- [ ] Run the focused test command again and inspect `git diff --check`; require no `listingadmin` import in `internal/crawler/alibaba1688/api_service.go` or `account_profile.go`.
- [ ] Commit with `git add internal/infra/httpx internal/crawler/alibaba1688 internal/app/httpapi && git commit -m "refactor: wire 1688 source accounts independently"`.

## Task 5: Split ListingKit handoff validation into source-account and SHEIN-target domains

**Files:**

- Modify `internal/compatibility/listingkit/sourcehandoff/a1688/command.go`.
- Modify `internal/compatibility/listingkit/sourcehandoff/a1688/command_test.go`.
- Modify `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/handler.go`.
- Modify `internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/handler_test.go`.
- Modify `internal/app/httpapi/composition_builder.go` for the second validator dependency.
- Modify `internal/product/sourcing/a1688_source_result.go` and tests only if the account field needs an `omitempty`/pointer boundary representation.
- Modify `tests/a1688_source_to_task_flow_test.go`.

**Constructor and validation contract:**

Use an explicit constructor:

```go
func NewTaskCommandService(
	creator sourcehandoff.GenerateTaskCreator,
	storeAccessValidator listingkit.StoreAccessValidator,
	sourceAccountAccessValidator sourceaccount.AccessValidator,
) *TaskCommandService
```

`validateStores` always validates `SheinStoreID` with expected platform `SHEIN`. It validates `SourceAccountID` only when positive, through `ValidateSourceAccountAccess(ctx, legacyTenantID, id)`. A zero source ID skips source validation; a negative source ID is a bad request. Remove the loop that sends `{id, "1688"}` through `StoreAccessValidator`. Map source-account errors to their own stable codes and user messages; keep `source_store_id` rejection and SHEIN error behavior unchanged.

When building `Alibaba1688CrawlRequestInput`, omit the account field for public handoff at the external JSON boundary and include it only for account-assisted requests. Preserve the existing neutral envelope rule: `SourceIdentity.StoreID` remains zero and no account ID is written into product facts or source identity.

**Steps (TDD):**

- [ ] Rewrite command tests around two fakes: a SHEIN `StoreAccessValidator` that records only `SHEIN` calls, and a source-account validator that records only positive account IDs. Add tests for public handoff with zero ID (no source validator call), account-assisted handoff, disabled/unavailable source account, missing SHEIN target, and rejection of a target-store row masquerading as an account. Run `go test ./internal/compatibility/listingkit/sourcehandoff/a1688 ./tests`; the old tests must fail because they expect a generic platform `1688` store call.
- [ ] Implement the split constructor/dependencies, validation order, sanitized source-account errors, and conditional account field serialization. Update the composition builder to pass both validators.
- [ ] Run `go test ./internal/compatibility/listingkit/sourcehandoff/a1688 ./internal/compatibility/listingkit/sourcehandoff/a1688/httpapi ./tests` and assert all generated neutral identities have `StoreID == 0`.
- [ ] Commit with `git add internal/compatibility/listingkit/sourcehandoff internal/product/sourcing internal/app/httpapi tests && git commit -m "refactor: separate 1688 account and SHEIN store validation"`.

## Task 6: Make the runtime acceptance script public-first and update operator evidence

**Files:**

- Modify `scripts/1688-runtime-acceptance.ps1`.
- Modify `scripts/1688-runtime-acceptance.Tests.ps1`.
- Modify `docs/product/validation/2026-08-08-1688-controlled-replay.md`.
- Modify `docs/superpowers/specs/2026-08-20-1688-public-source-account-boundary-design.md` only to add implementation status links after code is verified.

**Script behavior:**

- `Invoke-Crawl` accepts `-SourceAccountID 0`; its request body contains `source_account_id` only when the value is positive.
- `New-ListingKitHandoffPayload` omits `source_account_id` for public mode and includes it for positive account-assisted mode; it never emits `source_store_id`.
- `Invoke-EndToEnd` continues requiring `-SheinStoreID` and can use public source mode.
- Keep exact `CREATE-1688-TASK` confirmation and source-only preflight before Crawl. Do not add any automatic POST or provisioning behavior.

**Steps (TDD):**

- [ ] Add Pester tests for omitted field at zero, included field at positive ID, public `Invoke-Crawl` request, public EndToEnd handoff with SHEIN store, and unchanged exact-confirmation/source-preflight gates. Run `Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -EnableExit`; new tests must fail because current `Invoke-Crawl` rejects zero.
- [ ] Implement conditional hashtable construction and remove the positive-ID guard while preserving URL/confirmation/deadline checks and redacted errors.
- [ ] Run the full Pester file and `git diff --check`. Update the validation doc with the exact safe sequence: Preflight -> public Crawl using the approved confirmation -> inspect result -> account-assisted test only after independent account provisioning has been verified.
- [ ] Commit with `git add scripts/1688-runtime-acceptance.ps1 scripts/1688-runtime-acceptance.Tests.ps1 docs/product/validation/2026-08-08-1688-controlled-replay.md docs/superpowers/specs/2026-08-20-1688-public-source-account-boundary-design.md && git commit -m "test: allow public 1688 runtime acceptance"`.

## Task 7: Run repository-level verification and prepare the guarded release handoff

**Files:**

- No new production files unless a failing verification identifies a concrete issue.
- Review all changed files from Tasks 1–6 and the final plan/spec links.

**Verification sequence:**

- [ ] Run focused Go tests for every changed package:

  ```powershell
  go test ./internal/sourceaccount ./internal/crawler/shared ./internal/crawler/alibaba1688 ./internal/infra/httpx ./internal/listingkit/schema ./internal/listingkit/httpapi ./internal/compatibility/listingkit/sourcehandoff/... ./internal/product/sourcing ./internal/app/httpapi ./tests
  ```

- [ ] Run the complete Pester acceptance suite:

  ```powershell
  Invoke-Pester -Path scripts/1688-runtime-acceptance.Tests.ps1 -EnableExit
  ```

- [ ] Run repository Go verification from the root module and each nested Go module listed by `rg --files -g 'go.mod'`, without changing unrelated worktrees. Record failures separately as code, environment, CI, or deployment evidence.
- [ ] Run `git diff --check`, `go vet` for the changed Go packages, and a repository search proving the source-account path contains no `listingadmin.StoreRepository`, `listing_store`, or platform `1688` call. The only allowed `listing_store` references are SHEIN/target validation paths and unrelated listing-admin code.
- [ ] Validate schema generation with `go run ./cmd/listingkit-schema-migrate --scope all` only against an explicitly authorized non-production database. For the production cluster, perform read-only migration/preflight inspection first; do not mutate production or deploy without a separate user authorization.
- [ ] Before claiming completion, run the verification-before-completion checklist and capture: focused tests, Pester output, diff cleanliness, schema migration result, and the exact public/assisted metric keys. Then commit any verification-only fix with a focused message and leave deployment/merge as a separate authorized action.

**Release acceptance criteria:** public Crawl succeeds without `source_account_id`; account-assisted fallback is attempted only after a classified public access failure and an explicitly supplied positive ID; a SHEIN row can never satisfy source-account lookup; source identity contains no account ID; and metrics distinguish `public`, `account_assisted`, `source_public_unavailable`, `source_account_unavailable`, and `source_account_disabled`.
