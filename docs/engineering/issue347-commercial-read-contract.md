# #347 Commercial read contract

Status: implemented against the frozen IMPLEMENTATION_READY baseline accepted by two narrow independent reviews. The PR records the current implementation HEAD, validation results and final independent review. #350 consumes the single exported contract below after the implemented handoff.

## Authority and scope

- Issue #347 and its coordination comments; #350 is the UI consumer. Closed #42 is superseded, not permission to rebuild billing.
- `docs/product/listingkit-paid-pilot-product-catalog.md`: invitation-only `paid_pilot`, no customer contract prices/internal AI cost.
- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md` §§4.5,9–10: `base_payg` product description, actual grants separate from subscriptions; cash/payment/withdrawal deferred.
- Current multi-organization design defines business tenant ID as the opaque ZITADEL Organization ID. `store_quota_gorm.go:storeQuotaLimit` already reads subscription/entitlement using that exact ID. No integer conversion, metadata bridge, legacy fallback or migration.
- `internal/listingsubscription` remains the subscription/entitlement/metering owner. `internal/ledger/orgresource` and its integration adapter own resource balances. They are distinct from cash.

Must: verified effective organization and its roles; bounded side-effect-free reads; actual subscriptions and granted rows; explicit unknown; exact integers and windows; no cross-organization late response; no prices inferred from costs/designs.

Out of scope: purchase, grant, repair, backfill, onboarding, billing, renew/refund/withdrawal, IAM expansion, legacy migration, detailed audit/bills, pages. Resource/cash read permission has no approved customer reader in this slice, so those fields are explicitly unsupported. No new accepted risk is introduced. Existing ordinary-read grant caching is accepted up to 60 seconds by the multi-org design; this endpoint instead reuses the existing live resolver policy to check current grants on each request.

## Field/source matrix

| Field | Authoritative source/read | Organization owner | Permission | Unit/precision/window | Missing/unknown and errors | Actual existing call path |
| --- | --- | --- | --- | --- | --- | --- |
| `organization_id` | verified identity EffectiveOrganizationID | live selected grant; home is separate | existing `listingkit.admin.read` | opaque string ≤128 bytes | absent/denied/revoked fails closed | app HTTP auth → workbenchcontext Resolver → existing Casbin authorizer |
| `plans` | approved `base_payg` description; `paid_pilot` description only when this org has that actual subscription | product description scoped to authorized caller | same | no amount/currency; non-purchasable | no Basic/Professional/Enterprise sellable catalog | Console §9 / paid-pilot catalog; never DefaultPlans |
| `subscription` | `saas_tenant_subscriptions`, optional name from exact `saas_plans` row | `tenant_id = verified organization_id` | same; existing admin subscription GET already exposes these facts | raw status + evaluated status; UTC times | confirmed missing = null; missing plan name = null; DB failure =503 | platform ApplyPlan → existing repository; StoreQuota reads same key |
| `entitlements` | actual `saas_tenant_entitlements` rows for current six module codes | same exact key | same | no inherited/fallback grants; UTC validity | [] = no granted current module rows; missing OSS not synthesized | existing UpsertEntitlement/ApplyPlan → repository |
| `entitlements[].limits` | actual entitlement limits for known metric keys only | same exact key | same | signed decimal raw configured value; semantic finite/unlimited; store_count unit store, generation/publish/product-image jobs unit operation, storage unit byte | absent key = omitted, never default 0; unknown keys counted in `uninterpreted_limit_count` | existing StoreQuota and UsageLedger quota policies; no plan-name inference |
| `usage` | exact `saas_usage_buckets` rows; committed/reserved retained separately | exact tenant/module/metric/period key | same | int64 decimal strings; current UTC calendar month for operation counts, `__current__` for storage bytes | each missing bucket has state unknown + null quantities/time; DB failure=503; zero only from existing row | existing GormUsageLedger Reserve/Commit/Release/Reverse; no counters/mirrors added |
| `resource_balance` | `ledger/orgresource` / `saas_organization_resource_buckets` is authority, but no approved customer read permission/port in scope | organization_id/resource_type | unresolved for customer read | store_renewal_period/ai_point/data_row; not money/model tokens | `{state:"unsupported",value:null}`; not queried | trusted grant/reservation/settlement domain and persistence adapter |
| `cash_balance` | no implemented approved money/withdrawal owner in Phase1 | unavailable | unavailable | no currency or amount | `{state:"unsupported",value:null}` | deferred product contract |
| `observed_at` | server clock at read transaction | same request | same | UTC RFC3339Nano | not a billing timestamp | one database repeatable-read read-only transaction |

The six module codes are `store_management`, `task_import`, `rules`, `operation_strategy`, `listingkit`, `oss_storage`. This reports persisted granted facts only; it does not reinstate retired task/workflow entrypoints. Known limits: `store_count`, `listingkit_generations_succeeded`, `product_image_jobs`, `shein_drafts_succeeded`, `shein_publishes_succeeded`, `storage_bytes`. Stored explicit 0 for usage limits means unlimited under existing UsageLedger policy; store_count 0 is finite zero. Negative configured values are invalid. Canonical keys product_image_jobs_succeeded/storage_bytes_current take priority over product_image_jobs/storage_bytes via the existing usageMetricLimitKeys; source_key preserves the selected key. limits_scope=explicit_grant_only: no plan fallback or complete effective quota calculation; UI must not calculate it. Missing keys do not mean unlimited. `raw_value` preserves the configured decimal, while `value` is null for unlimited. The five usage metrics are `listingkit_generations_succeeded`, `product_image_jobs_succeeded`, `shein_drafts_succeeded`, `shein_publishes_succeeded`, `storage_bytes_current`; storage is a retained-byte gauge. These are recorded ledger observations, not a bill, aggregate counter mirror, resource wallet or complete new Product/Agent usage coverage.

## Implemented wire contract

Only `GET /api/v1/workbench/commercial/overview` (Go) and `GET /api/workbench/commercial/overview` (BFF). No query parameters, request body, arbitrary organization/user selector or write method. Browser supplies `X-Expected-Organization-ID` as an assertion against the existing HttpOnly organization cookie. BFF forwards only server token, selected organization, Accept and generated request ID to a fixed `COMMERCIAL_API_ORIGIN` plus the exact Go path. Explicit origin must be scheme+host+port only; no credentials/path/query/fragment. No default origin. Redirect mode manual, request and response no-store.

Go DTOs in `internal/listingsubscription/commercial_read.go`; identical snake_case JSON and TypeScript types:

```ts
type CommercialPlanOption = {
  code: "base_payg" | "paid_pilot"; name: string;
  source: "approved_product_description";
  availability: "not_for_sale" | "invitation_only";
  price: null; currency: null;
};
type CommercialSubscription = {
  plan_code: string; plan_name: string | null;
  status: "active" | "trialing" | "expired" | "disabled";
  effective_status: "active" | "trialing" | "expired" | "disabled" | "not_started";
  starts_at: string | null; expires_at: string | null; updated_at: string;
};
type CommercialLimit = {
  metric: string; source_key: string; unit: "store" | "operation" | "byte";
  kind: "finite" | "unlimited"; raw_value: string; value: string | null;
};
type CommercialEntitlement = {
  module_code: string;
  status: CommercialSubscription["status"];
  effective_status: CommercialSubscription["effective_status"];
  starts_at: string | null; expires_at: string | null; updated_at: string;
  limits_scope: "explicit_grant_only"; limits: CommercialLimit[]; uninterpreted_limit_count: number;
};
type CommercialUsage = {
  module_code: "listingkit" | "oss_storage"; metric: string;
  source: "subscription_usage_ledger"; unit: "operation" | "byte";
  period_key: string; window_start: string | null; window_end: string | null;
  state: "known" | "unknown";
  committed: string | null; reserved: string | null; updated_at: string | null;
};
type UnsupportedCommercialValue = {state: "unsupported"; value: null};
type CommercialOverview = {
  organization_id: string; observed_at: string;
  plans: CommercialPlanOption[]; subscription: CommercialSubscription | null;
  entitlements: CommercialEntitlement[]; usage: CommercialUsage[];
  resource_balance: UnsupportedCommercialValue; cash_balance: UnsupportedCommercialValue;
};
```

TypeScript single owner: `web/listingkit-ui/src/lib/api/commercial.ts`, exports all types above, `parseCommercialOverview(payload: unknown): CommercialOverview | null`, `getCommercialOverview(expectedOrganizationId: string, signal?: AbortSignal): Promise<CommercialOverview>`, and `CommercialReadError` with `status/code/requestId`. #350 imports this module, adds no duplicate schema/client/proxy, and cancels/drops pending state on organization/user/role changes and logout. A completed response is not authority for subsequent access. Consumers must include subject/org/role state in their query lifetime; no previous-organization placeholder or persisted sensitive cache.

No subscription is a successful response with `subscription:null`, not an entitlement or free-plan grant. Expired/disabled retain actual rows and validity; no reactivation. Missing timestamps are open validity bounds in the existing owner, not guessed dates. Unsupported resource/cash remain distinct from unknown ledger buckets. All quantities stay strings and never pass through JS Number. `window_end` is exclusive; observations are inside the current UTC month, not a purchased billing cycle. Retained bytes have null window boundaries. storage reserved is a signed net pending byte delta (may be negative); it is not wallet reservation. product_image_jobs_succeeded counts recorded job metering units, not generated image count.

## Errors and bounds

Existing Workbench envelope `{code,message,requestId,fieldErrors:[]}`. 400 `INVALID_REQUEST`; 401 `AUTHENTICATION_REQUIRED`; 403 `PERMISSION_DENIED`, `ORGANIZATION_ACCESS_DENIED`, `ORGANIZATION_ACCESS_REVOKED`, `ORGANIZATION_SUSPENDED`; 409 `ORGANIZATION_SELECTION_REQUIRED`/`ORGANIZATION_CONTEXT_CHANGED`; 503 `DEPENDENCY_UNAVAILABLE`; 504 `DEADLINE_EXCEEDED`; BFF/client malformed or mismatched upstream response=502 `INVALID_UPSTREAM_RESPONSE`; unsupported BFF methods=405 (the Go router only registers GET). Safe fixed messages; no upstream bodies/tokens/SQL. Revocation handling follows the existing organization-cookie clearing behavior; no success fallback on errors.

Limits: 0 body/query bytes; organization 128 ASCII identifier bytes; ≤2 plan descriptions, ≤6 entitlements, ≤6 limits per entitlement, exactly5 usage observations; plan/module strings ≤128/256 bytes, limits JSON ≤4096 bytes per row and ≤64 keys; successful response ≤64KiB actual UTF-8 bytes, errors ≤8KiB. BFF/client one 15s deadline includes auth/network/body parsing; Go read handler/query deadline10s and PostgreSQL statement timeout≤10s. No pagination or arbitrary time range. Read-only repeatable-read transaction covers database observations; product descriptions and authorization retain their own sources, not a global distributed snapshot. Cancellation ends work; no recovery/repair writes or retry owners introduced.

## Independent admission and validation

Independent reviewer `commercial_admission` accepted the narrow scope, existing AdminRead, direct verified Org key and unsupported resource/cash. Findings classified IMPLEMENTATION_TEST: constructor writes; historical aggregation; synthetic OSS fallback; counter/gauge/wallet confusion. Tests must exercise each through the new call path. No BLOCKER or new accepted risk.

TDD must first fail on the new reader/HTTP/BFF/client. Task-isolated PostgreSQL fixtures are built through existing ApplyPlan/UpsertEntitlement and UsageLedger operations. Read with a database read-only transaction and compare subscription/entitlement/usage/ledger table row values plus xmin before/after. Test org A home/B effective, denied role, live revocation/cache drift, malformed data, no subscription, expiration/disabled/not-started, zero/unlimited/unknown, >JS-safe quantities, current-window boundaries, cancellation/errors and actual API→BFF→client. Identity verification/grant source is an explicit test substitute; OpenMeter is not queried and no external metering fake supplies commercial facts. Production/payment/customer data NOT_RUN.

Legacy decision: EXTRACT. Reusable behavior: current subscription row models, validity policy, UsageLedger facts, existing auth and bounded strict-JSON transport. Current owner: listingsubscription and its dedicated commercial HTTP projection. Cutover/deletion condition: this consumer never calls old Summary/RequestTenantID/default constructor or tenantbridge; old callers remain drain debt outside this slice.

The existing HTTP-package guard explicitly registers `internal/listingsubscription/httpapi` alongside other current domain HTTP adapters. Subscription business implementation remains prohibited from importing Gin except its already registered historical handler. No legacy consumer baseline, depguard exclusion, CI check or legacy fallback is expanded.

## Reproduce the isolated HTTP chain

Prerequisites: repository Go version and dependencies, Node/pnpm from the UI package, and a running Docker daemon with `postgres:17.2-alpine` available. Install UI dependencies with `pnpm install --frozen-lockfile` from `web/listingkit-ui` (`pnpm.cmd` on Windows), then run from the repository root:

```sh
go test -race -tags integration ./internal/app/httpapi -run '^TestCommercialHTTPPostgresBFFClientZeroWrites$' -count=1 -v
```

The test refuses caller-supplied database configuration: it creates its own PostgreSQL container with a unique `issue347-commercial-<UUID>` name, synthetic credentials and a random loopback-only port. It seeds data through current domain operations, creates a SELECT-only reader with `default_transaction_read_only=on`, mounts the actual Go route and invokes the actual Next route over HTTP from the exported TypeScript client. The standalone Vitest config is `web/listingkit-ui/e2e/issue347-commercial.config.ts`; ordinary UI unit tests do not silently substitute or skip this integration requirement. The Go harness supplies the private loopback origin and launches that suite itself.

Both the PostgreSQL fixture tables and their `xmin` versions are hashed before/after the authorized, denied, cancelled and dependency-error requests. The test also revokes SELECT inside its own fixture to exercise a real database read failure. All `saas_*` tables, including subscription/entitlement/usage and organization resource ledger tables, must be identical; success emits `ZERO_WRITE` and the digest. Deferred cleanup closes both HTTP servers and database connections and stops only the uniquely named test container (`--rm` removes it). If the test process is forcibly killed, locate its exact name with `docker ps --filter name=issue347-commercial-` and stop only that test container; it owns no persistent volume.

Explicit substitutes: external token/session retrieval and the ZITADEL grant provider; real grant resolution/cache policy, Casbin checks, Go assembly/reader, BFF transport, shared schema/client, current domain seeding and PostgreSQL execute. OpenMeter is not queried. Actual ZITADEL accounts, payment providers, customer data and production wiring are NOT_RUN and not authorized by this test. Runtime BFF configuration requires a fixed `COMMERCIAL_API_ORIGIN`; this PR does not deploy or configure a live environment. #350 must still verify its page lifecycle with this client and ignore cancelled responses after subject, organization or role changes.

## Browser serve/stop handoff for #350

The same Go integration fixture can stay alive while an actual Next development server runs against it. Following the existing SHEIN/Product Review fixture lifecycle, the test-only launcher issues synthetic encrypted Auth.js cookies; actual Auth.js decryption/callbacks, server token helper, context BFF, commercial BFF, Go auth/resolver/Casbin and PostgreSQL run without module mocks. There is no production fixture flag or copied commercial DTO/seed owner.

From this owner checkout run:

```sh
node web/listingkit-ui/scripts/commercial-read-fixture.mjs
node web/listingkit-ui/scripts/commercial-read-fixture.mjs --serve --web-dir <350-checkout>/web/listingkit-ui
```

The first command runs real HTTP smoke assertions and verifies cleanup. `--serve` performs those assertions before handing over a private `fixture.json` path and random loopback origin, then runs up to 29 minutes. `--web-dir` selects the consumer checkout's already-installed UI; the Go test binary and domain seed always come from this script's owner checkout. Runtime origins are fixed to the fixture: `COMMERCIAL_API_ORIGIN=goOrigin` and `LISTINGKIT_SERVICE_API_BASE=contextOrigin/api/v1`. The launcher records both source/web Git HEADs.

Read session cookies from `fixture.json.sessions.owner` or `.viewer` into an isolated browser context; do not print cookies/control tokens or put them in URLs. `home-A` is the home identity; `org-B` is initially selected. Both `org-B` and `org-C` are granted and have distinct actual usage, so use B→C→B for switch/race checks. Home membership is not invented. Additional selectable rows cover custom paid plan, missing subscription, expired, disabled, not-started and viewer-only access. The actual context endpoints list grants and switch the HttpOnly organization cookie. Switching session arrays exercises subject changes; do not share this browser with a real signed-in context.

For one controlled scenario at a time, POST `{"scenario":"<name>"}` to the manifest's `scenarioURL`, with `Content-Type: application/json` and `X-Fixture-Control: <manifest.controlToken>`. Names: `normal`, `revoked`, `role-downgraded`, `suspended`, `signed-out`, `provider-unavailable`, `database-unavailable`, `slow`. The first five deny current grants/role/status/token as named; dependency failures return unavailable; `database-unavailable` actually revokes the fixture reader's SELECT permission; `slow` delays only commercial HTTP by two seconds and respects cancellation. `normal` restores those test controls. These endpoints exist only in the integration test binary on its random loopback port. They do not change business rows. Browser logout can also clear its synthetic cookies normally.

Create an empty `stop-fixture` file in `controlDirectory` to stop. The launcher stops only its own Next PID tree, checks its port is released, asks Go to compare all `saas_*` values/xmin and perform registered database/container cleanup, and writes `cleanup.json` with `passed:true` only on successful cleanup. `go.log` contains the zero-write digest; `evidence.json` records the automatic real-server smoke boundary. Keep the manifest private; the output path, origins, status and sanitized evidence may be shared. Forced termination is not successful cleanup: inspect the exact task container and process names before any manual stop. The launcher creates no persistent database volume and touches no real IAM/account/payment data.
