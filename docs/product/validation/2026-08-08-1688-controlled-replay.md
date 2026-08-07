# 1688 Controlled Replay Validation — 2026-08-08

## Baseline

- Repository: `task-processor`
- Branch: `codex/1688-controlled-replay`
- Replay validation commit: `51a1be28d78f5cf57192b14c533c02314d52ae28`
- Scope: deterministic in-process replay through the existing 1688 HTTP handler and ListingKit task-creation boundary.
- Excluded: live 1688 crawling, real store access, database persistence, worker execution, real preview/readiness generation, and SHEIN submission.

## Replay inputs

The replay uses a fixed `Product1688` fixture and an authenticated request context:

| Scenario | Source ID | Source key | Result |
| --- | --- | --- | --- |
| Complete product | `321` | `crawler:1688:321` | creates deterministic test task `task-replay-321` |
| Missing title and assets | `322` | `crawler:1688:322` | rejected before task creation |
| Controlled source failure | `323` | `crawler:1688:323` | rejected before task creation |

The successful replay uses tenant `101`, user `user-1688`, source store `3001`, target SHEIN store `168811`, and target platform `shein`.

## Evidence recorded

The successful replay exercised the real `sourcea1688.Handler` and `a1688.TaskCommandService`. It verified that:

- the authenticated tenant and user reach the generated ListingKit request;
- the 1688 URL is normalized to `https://detail.1688.com/offer/321.html`;
- the source identity and source key are returned by the HTTP response;
- the request contains the normalized `crawler:1688:321` source reference;
- the source title, brand, category, description, price, variant, and four image URLs reach the request text/assets;
- the platform list is normalized to one `shein` entry and the target store ID is retained;
- the deterministic creator is called exactly once.

The missing-facts replay returned `task_creation_failed`, exposed the source identity and warnings, and called the task creator zero times. The source-error replay returned `task_creation_failed`, retained source ID `323` and the `controlled crawler failed` warning, and called the task creator zero times.

## Commands and results

### Focused Product Sourcing validation

Passed:

```powershell
$env:GOWORK='off'; go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/asset/... ./internal/product/sourcehandoff/... ./internal/productenrich/httpapi/sourcea1688/... -count=1
$env:GOWORK='off'; go test ./internal/listingkit/... ./tests/... -count=1
```

This includes the three replay scenarios and the existing source, handoff, ListingKit, and boundary tests.

### Full backend test gate

Not completed:

```powershell
$env:GOWORK='off'; go test ./... -count=1
```

The command timed out after approximately 124 seconds with exit code `124`. This is recorded as unresolved environment/test-suite evidence, not as a passing result.

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
