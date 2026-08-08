# 1688 Controlled Replay Validation — 2026-08-08

## Baseline

- Repository: `task-processor`
- Branch: `codex/1688-controlled-replay`
- Replay validation commit: `51a1be28d78f5cf57192b14c533c02314d52ae28`
- Full-gate verification commit: `10a83339` (documentation-only commit after the replay code)
- Scope: deterministic in-process replay through the existing 1688 HTTP handler and ListingKit task-creation boundary.
- Excluded: live 1688 crawling, real store access, database persistence, worker execution, real preview/readiness generation, and SHEIN submission.

## Replay inputs

The replay uses a fixed `Product1688` fixture and an authenticated request context:

| Scenario | Source ID | Source key | Result |
| --- | --- | --- | --- |
| Complete product | `321` | `crawler:1688:321` | creates deterministic test task `task-replay-321` |
| Missing title and assets | `322` | `crawler:1688:322` | rejected before task creation |
| Controlled source failure | `323` | `crawler:1688:323` | rejected before task creation |

The successful replay uses tenant `101`, user `user-1688`, 1688 login account configuration `3001`, target SHEIN store `168811`, and target platform `shein`. The source-side request field is `source_account_id`; `source_store_id` is rejected and has no compatibility path.

## Evidence recorded

The successful replay exercised the real `sourcea1688.Handler` and `a1688.TaskCommandService`. It verified that:

- the authenticated tenant and user reach the generated ListingKit request;
- the 1688 URL is normalized to `https://detail.1688.com/offer/321.html`;
- the source identity and source key are returned by the HTTP response;
- the request contains the normalized `crawler:1688:321` source reference;
- the source title, brand, category, description, price, variant, and four image URLs reach the request text/assets;
- the platform list is normalized to one `shein` entry and the target store ID is retained;
- the normalized 1688 source identity remains product-based while the authenticated login account is used only for tenant-scoped authorization;
- the generated ListingKit request JSON contains no password, cookie, browser `user_data_dir`, profile path, or proxy credentials;
- the deterministic creator is called exactly once.

The missing-facts replay returned `task_creation_failed`, exposed the source identity and warnings, and called the task creator zero times. The source-error replay returned `task_creation_failed`, retained source ID `323` and the `controlled crawler failed` warning, and called the task creator zero times.

The account authorization replay also confirms that disabled and foreign-tenant 1688 login-account configurations are rejected before the creator is called. Disabled and foreign SHEIN target stores continue to be rejected on the same boundary with their existing stable access codes.

## Commands and results

### Focused Product Sourcing validation

Passed:

```powershell
$env:GOWORK='off'; go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/asset/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
$env:GOWORK='off'; go test ./internal/listingkit/... ./tests/... -count=1
```

This includes the three replay scenarios and the existing source, handoff, ListingKit, and boundary tests.

### Full backend test gate

Passed on the extended command timeout:

```powershell
$env:GOWORK='off'; go test ./... -count=1
```

The first 120-second attempt timed out with exit code `124` and was not counted as success. A rerun with a 600-second command limit completed successfully in approximately 138 seconds; all reported packages passed.

### Maintained entrypoint build equivalent

Passed:

```powershell
$env:CGO_ENABLED='0'; $env:GOOS='linux'; go build ./cmd/listing-control-plane ./cmd/product-listing-api ./cmd/shein-listing ./cmd/temu-listing
```

GNU Make was not required for this run; the maintained Makefile-equivalent entrypoint build command completed successfully.

## Acceptance boundary

This replay closes the deterministic data-propagation and failure-reporting evidence gap. It does not close the Product Sourcing MVP as a production capability. The following remain unverified:

- a real or controlled runtime 1688 import creating a persisted task;
- real task IDs and durable source lineage after persistence/reload;
- asynchronous task execution and the resulting canonical product;
- preview/readiness results from the existing SHEIN downstream owner;
- operator acceptance of the end-to-end flow;
- live 1688 crawler behavior and real SHEIN submission.

No source-specific marketplace preview or submission owner was added.

## Local runtime preflight entrypoint follow-up

The existing `scripts/start-listingkit-api-local-replay.ps1` entry point was made
checkout-path independent by resolving the repository root from `$PSScriptRoot`.
The focused PowerShell regression test passed (2 passed, 0 failed), including the
PowerShell parser check and the stale-path assertion. This only proves that the
local process can be started from the current checkout; it does not prove that
the API is running or that credentials, port-forwarding, tenant access, and store
access are available.

The next operator run must remain GET-only until all preflight checks pass:

1. Start the existing local database/Redis/Temporal port-forward stack.
2. Start the local API from the corrected entry point.
3. Check `/health`, bearer-token authentication, tenant visibility, and store visibility.
4. Stop and record a blocker if any dependency is unavailable; do not create a 1688 task automatically.

## Preflight run — blocked

The first read-only preflight from the isolated worktree was stopped before
starting the API or creating a task:

| Check | Result |
| --- | --- |
| Worktree `.env` | missing |
| ListingKit bearer token | not present in the root or worktree token file |
| Local API `127.0.0.1:8085` | not running; health request timed out |
| Kubernetes context | reachable (`default`) |
| `yudao-cloud` namespace | missing, so the documented PostgreSQL/Redis services could not be found |
| `temporal/temporal-frontend` | service exists |

Conclusion: `blocked` on local database/Redis configuration and ListingKit
credentials. No port-forward, API process, 1688 crawler request, task creation,
or marketplace operation was started by this run. Resume only after the
operator supplies a valid worktree runtime configuration and bearer token, or
points the scripts at the current cluster services.

## Preflight run — infrastructure passed, business gate blocked

After correcting the port-forward defaults to the current cluster services, a
second read-only run reached the local API successfully:

| Check | Result |
| --- | --- |
| PostgreSQL forward | `platform-data/shared-postgresql` → `127.0.0.1:15432` |
| Redis forward | `platform-data/redis` → `127.0.0.1:16379` |
| Temporal forward | `temporal/temporal-frontend` → `127.0.0.1:7233` |
| API database connection | `127.0.0.1:15432/ruoyi-vue-pro` |
| `/health` | HTTP 200 |
| `/readyz` | HTTP 200 |
| `/api/v1/listing-kits/settings-health` | HTTP 200 |

The controlled request context used tenant `101` only for GET checks. Its
subscription summary reported `store_management.allowed=false` with reason
`not_configured`, so tenant-store endpoints correctly returned HTTP 402. The
platform subscription read model contained eight configured tenants, but the
tenant-scoped simple-store reads returned no stores for the configured tenants
that were checked. The platform store listing currently contains SHEIN/TEMU
stores and the `platform=1688` read returned zero stores.

Conclusion remains `blocked`: infrastructure is healthy, but there is no
current tenant-owned 1688 login-account configuration plus eligible SHEIN
target-store pair for a real handoff. The historical
`source_account_id=3001` and `shein_store_id=168811` values are test fixtures,
not runtime acceptance data. No POST request or task creation was made.

An enabled tenant-owned 1688 account configuration only establishes that the
handoff may select its browser Profile. It does not establish a valid live 1688
login session. Runtime acceptance still requires an account row, a completed
manual login in that account Profile, a read-only crawl probe, and only then a
later controlled ListingKit task run. No credentials, cookies, profile
contents, or live task IDs are recorded here.
