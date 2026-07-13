# Next Phase Execution Plan

> Status: active execution plan after the ListingKit boundary checkpoint and Product Sourcing foundation implementation.
>
> Last reviewed: 2026-07-13.
>
> Calibrated against: `master` at `5c72f406c18b40d3860fb8f0c7518c6606f38b85`.
>
> Scope: current-baseline validation, SHEIN production stabilization, Product Sourcing MVP closeout, and selection of one next product source.

## 1. Current position

The project has moved beyond the early “split large files” phase. It has also moved beyond the initial Product Sourcing modeling phase.

Current read:

- ListingKit file-group slimming has reached a checkpoint.
- Preview and submission work is now boundary migration and guardrail work, not file-size reduction.
- `internal/app/httpapi` is primarily runtime assembly and should remain closed to feature policy.
- Import-boundary tests are active architecture enforcement.
- `SourceIdentity`, `SourceEnvelope`, Amazon and 1688 mappings, catalog/asset handoff, a ListingKit request bridge, and source-boundary guards are implemented.
- The recent SHEIN change stream contains production-sensitive pricing, promotion, cache, readiness, and publishing-boundary changes.
- Exact current-baseline CI and smoke evidence is more important than starting another structural migration.
- Generated package/dependency baselines remain local evidence, not committed architecture authority.

This plan supersedes the earlier queue that began with introducing Product Sourcing identity and envelope types. Those foundation slices now exist.

## 2. Main conclusion

The next phase should focus on:

1. making the current baseline release-decision-ready;
2. validating the recent SHEIN production-sensitive changes;
3. closing the implemented Product Sourcing MVP through one controlled 1688 flow;
4. preserving runtime and package boundaries;
5. selecting exactly one next warehouse/catalog source only after closeout.

Do **not** continue extracting details from root `internal/listingkit` merely because a file can be made smaller.

## 3. Current phase status

| Area | Status | Next posture |
| --- | --- | --- |
| Current `master` validation | Evidence incomplete | Record exact CI, race, build, frontend, and smoke results for the commit used in release decisions. |
| SHEIN production path | Active stabilization | Validate synchronized pricing, promotion calculations, resolution caches, readiness, save-draft, publish, and recovery behavior. |
| Preview | Checkpointed | Continue only when ownership clearly moves to `internal/listing/preview`. |
| Submission | Late boundary migration | Continue only small policy seams that do not change Temporal determinism or introduce another submission owner. |
| Service slimming | First wave complete | Freeze as compatibility/facade shell; avoid cosmetic splitting. |
| HTTPAPI runtime | Inventory complete | Keep app/runtime assembly thin and feature policy in feature-owned packages. |
| Product Sourcing foundation | Implemented | Validate and close the MVP rather than recreating its model or package structure. |
| 1688 source handoff | Implemented; controlled-flow evidence pending | Exercise source-to-task-to-preview behavior and verify lineage/warnings. |
| Amazon source mapping | Implemented boundary-validation path | Maintain tests; do not interpret it as an active full Amazon workbench. |
| Next warehouse source | Not started | Define one source contract only after Product Sourcing closeout. |
| Boundary tests | Active | Keep guards stable and explain every temporary allowlist exception. |
| Generated baselines | Local evidence only | Write outputs under `.local/` or summarize them in a dated validation note. |

## 4. Immediate priorities

### Priority 1: Validate the exact baseline

Run focused checks first:

```powershell
go test ./internal/product/sourcing/... -count=1
go test ./internal/catalog/... -count=1
go test ./internal/asset/... -count=1
go test ./internal/product/sourcehandoff/... -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi/... -count=1
go test ./internal/app/httpapi/... -count=1
go test ./tests/... -count=1
```

Run the full backend and runtime gates:

```powershell
go test ./... -count=1

go test -race ./internal/app/runtime/listingcontrol `
  -run TestControlPlaneService -count=1

go test -race ./internal/listingadmin `
  -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" `
  -count=1

make build-all
```

Run frontend gates:

```powershell
Set-Location web/listingkit-ui
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```

Acceptance criteria:

- The exact commit is named in the result.
- Every failure is classified as a regression, stale expectation, flaky fixture, legacy exception, or environment issue.
- No release note says “green” without a visible command result or workflow run.
- Generated dependency/package output stays local unless deliberately summarized in a dated note.

### Priority 2: Validate the SHEIN stabilization wave

Maintain a focused regression and smoke matrix for:

1. synchronized supply price used by activity enrollment;
2. promotion drop-rate and breakeven calculations;
3. multi-SKU retail-price and cost completeness;
4. fallback behavior when synchronized supply price is absent;
5. final resolution-cache preservation during republish;
6. SDS baseline and canonical metadata behavior;
7. action-aware submit-readiness and POD-readiness policy;
8. save-draft and publish idempotency/recovery;
9. SHEIN listing browser startup;
10. rollout and rollback behavior when the change is release-sensitive.

Acceptance criteria:

- Focused unit or integration coverage exists for each changed business rule.
- At least one real or controlled smoke run records task ID, store, action, result, and failure context.
- Pricing and promotion behavior is reviewed as business semantics, not only as package refactoring.
- Missing smoke coverage is explicitly called out before release.

### Priority 3: Close the Product Sourcing MVP

The implemented path is:

```text
Amazon / 1688 source result
  -> internal/product/sourcing.SourceIdentity + SourceEnvelope
  -> internal/catalog.ProductFacts + internal/asset.Facts
  -> internal/product/sourcehandoff.ListingKitRequestInput
  -> internal/listingkit.GenerateRequest
  -> existing task creation and SHEIN preview/submission path
```

Required closeout:

1. validate identity normalization, fingerprinting, and weak-identity behavior;
2. validate Amazon and 1688 envelope mapping without browser automation;
3. validate catalog and asset fact handoff;
4. validate the ListingKit request bridge and 1688 command/HTTP adapter;
5. exercise one controlled 1688 source-to-task-to-preview flow;
6. verify source lineage or a durable source reference is retained;
7. verify missing facts remain explicit warnings/errors;
8. record the result in a dated validation note;
9. declare the MVP closed or list the exact remaining blockers.

Do not start 大建云仓 or another source merely because the model exists. Close the current loop first.

### Priority 4: Keep runtime and boundaries closed

- Keep `internal/app/*` as runtime assembly.
- Keep root `internal/listingkit` as orchestration, compatibility, DTO adaptation, persistence ordering, and API-shell glue.
- Keep source normalization in `internal/product/sourcing`.
- Keep crawler access/execution in `internal/integration/crawler/*` or an approved adapter boundary.
- Keep generic submission mechanics in `internal/listing/submission`.
- Keep marketplace rules in marketplace/publishing/workspace packages.
- Hide concrete infrastructure and external clients behind small interfaces.
- Add owner, reason, and retirement condition to every import allowlist expansion.

## 5. Recommended PR queue

### PR A: Record current-baseline validation

Suggested title:

```text
test: record current listingkit baseline validation
```

Scope:

- run focused, full, race, build, and frontend gates;
- classify failures;
- fix only true regressions or stale test expectations needed to establish the baseline;
- add a dated validation note with exact evidence.

Do not:

- move packages;
- introduce new product features;
- combine unrelated refactoring.

### PR B: Close SHEIN stabilization evidence

Suggested title:

```text
test: validate shein pricing readiness and republish behavior
```

Scope:

- consolidate focused tests for recent pricing, promotion, cache, and readiness changes;
- add or update a real-flow validation note;
- call out any release blocker explicitly.

### PR C: Validate the controlled 1688 Product Sourcing path

Suggested title:

```text
test: close product sourcing mvp with 1688 flow
```

Scope:

- run source, facts, bridge, and boundary tests;
- exercise one controlled import-to-task-to-preview path;
- verify lineage, warnings, and operator-visible failures;
- update `docs/product/product-sourcing-mvp-plan.md` with the final closeout result.

### PR D: Inventory one next warehouse source

Suggested title:

```text
docs: define next warehouse source contract
```

Scope:

- select exactly one source, currently expected to be 大建云仓 if its contract is available;
- document identity, product, variant, asset, cost, pagination, authentication, and error fields;
- map fields to the existing `SourceEnvelope` without changing runtime behavior.

### PR E: Implement the selected source adapter

Start only after PR D is approved and the current MVP is closed.

Scope:

- implement source access through an approved adapter boundary;
- normalize into the existing source contract;
- reuse catalog, asset, and ListingKit handoff paths;
- add boundary and fixture-based tests.

Do not combine this PR with a TEMU, Amazon, or Walmart workbench expansion.

## 6. Boundary rules for new code

| Kind of code | Preferred home |
| --- | --- |
| Product-source identity and normalization | `internal/product/sourcing` |
| Product/catalog facts | `internal/catalog` or approved product/catalog target |
| Product image/design/asset facts | `internal/asset` or approved asset target |
| Source access and crawler execution adapters | `internal/integration/crawler/*` or approved integration adapter |
| Source-to-ListingKit adaptation | `internal/product/sourcehandoff` |
| Generic listing submission policy | `internal/listing/submission` |
| Listing preview rules | `internal/listing/preview` |
| SHEIN publishing rules | `internal/marketplace/shein/publishing` or `internal/publishing/shein` according to the current compatibility seam |
| SHEIN workspace/editor presentation rules | `internal/marketplace/shein/workspace` |
| HTTP runtime assembly | `internal/app/httpapi` or feature-owned `*/httpapi` packages |
| Legacy compatibility / API-shell glue | `internal/listingkit` |

## 7. Stop conditions

Pause and document instead of continuing if a proposed slice:

- depends on an unverified current baseline;
- requires a target package to import root `internal/listingkit` outside an approved compatibility bridge;
- touches Temporal determinism or activity retry semantics without explicit review;
- moves runtime client construction into a business package;
- combines behavior changes with package movement;
- only reduces file size without changing ownership or dependency direction;
- requires broad allowlist expansion without an owner and retirement condition;
- depends on stale generated package/dependency snapshots;
- starts another product source before the current source loop is closed;
- starts a full new sales-platform workbench during current stabilization;
- treats an official command or deployment manifest as proof of product maturity.

## 8. Main risks

The main risks are now:

```text
production-sensitive SHEIN changes without exact current-baseline evidence;
implemented Product Sourcing code being mistaken for a fully validated business flow;
status documents drifting behind code;
continued helper/package churn that does not improve ownership.
```

Mitigation:

- name the exact baseline;
- keep tests, smoke results, and release evidence visible;
- close one source loop before selecting the next;
- update status documents when implementation state changes;
- tie every structural PR to an explicit ownership improvement.

## 9. Definition of done for this phase

This phase is complete when:

- focused and full backend tests are green or explicitly classified for the exact baseline;
- listing-control and listingadmin race tests are green or explicitly classified;
- all maintained runtime commands build through `make build-all`;
- frontend lint, typecheck, tests, and build are green or explicitly classified;
- recent SHEIN pricing, promotion, cache, readiness, and republish behavior has focused evidence;
- one controlled 1688 path reaches the existing task/preview flow with lineage and warnings verified;
- the Product Sourcing MVP is explicitly marked closed or has a short blocker list;
- source/crawler/catalog/asset/bridge boundary guards remain green;
- import-boundary allowlists are stable and explained;
- root `internal/listingkit` receives no new broad policy ownership;
- generated baseline outputs are not committed as long-lived documentation;
- exactly one next product source is selected through a documented contract.

## 10. Recommended immediate next step

Start with PR A:

```text
test: record current listingkit baseline validation
```

Until that evidence exists, treat the calibrated baseline as implemented but not independently confirmed release-ready.
