# Product Sourcing MVP Plan

> Status: active execution plan.
>
> Last reviewed: 2026-07-09.
>
> Scope: first product-source expansion loop after refactoring/documentation closeout.
>
> Current authority: use this plan together with `product-sourcing-handoff.md`, `current-refactoring-status.md`, and `next-phase-plan.md`.

## 1. Goal

Build the smallest product-source expansion loop that proves the current boundaries can support new source growth without pushing source logic back into root `internal/listingkit`.

The target loop is:

```text
raw source data
  -> SourceIdentity + SourceEnvelope
  -> source-result normalization
  -> catalog / asset facts
  -> ListingKit batch or task orchestration
  -> existing SHEIN preview / submission path
```

The goal is not to build a full product-source platform in one PR. The goal is to create one clean path that future 1688, warehouse catalog, or other product-source integrations can copy.

SDS is intentionally treated as a POD/design capability, not a normal product-source validation target.

## 2. MVP constraints

This MVP must stay inside these constraints:

- no full TEMU / Amazon / Walmart workbench expansion;
- no new sales-platform auto-publish runtime;
- no new submission state machine;
- no broad ListingKit root refactor;
- no crawler package importing root `internal/listingkit`;
- no crawler package importing marketplace publishing or workspace packages;
- no `internal/product/sourcing` package importing root `internal/listingkit`;
- no SHEIN publish payload assembly inside product-source packages;
- no behavior-changing package move hidden inside a modeling PR.

## 3. Preferred first source

Prefer the source with the lowest integration risk and highest business signal.

Current recommendation:

```text
Option A: normalize an existing product-source path first, such as 1688, if code already exists and test fixtures are available.
Option B: use 大建云仓 / overseas warehouse source catalog as the first new source when the business data contract is already clear.
```

Current implementation choice:

```text
PR 2 starts with the existing Amazon product fetch/crawler result path for boundary validation.
The next business-source validation path is 1688.
```

Reason:

- the Amazon path already has source request planning, product fetch tests, and a stable `internal/model.Product` crawler result shape;
- it is useful for boundary validation before adding a business-priority source;
- it lets `SourceEnvelope` mapping be tested without browser automation;
- it does not imply full Amazon listing workbench expansion;
- 1688 already has crawler models and product-sourcing normalization code, making it the lowest-risk business source after boundary guards are in place.

Choose Option A when the first objective is boundary validation or using an existing product-source path.
Choose Option B when the first objective is new warehouse business expansion.

Do not start two source integrations in the same PR.

## 4. Proposed PR sequence

### PR 1: introduce source identity and envelope

Suggested title:

```text
feat: introduce product sourcing source identity
```

Scope:

- create or extend `internal/product/sourcing`;
- introduce neutral source identity and source envelope types;
- keep the package free of ListingKit, marketplace, HTTP runtime, and crawler runtime dependencies;
- add focused unit tests for identity normalization, fingerprinting, and validation rules.

Minimum model:

```text
SourceIdentity
  SourceType
  SourcePlatform
  SourceID
  SourceURL
  SourceVersion
  SourceFingerprint

SourceEnvelope
  Identity
  RawReference
  ProductCandidate
  AssetCandidates
  SupplierOrCostFacts
  Warnings
  Trace
```

Acceptance criteria:

```text
[ ] internal/product/sourcing does not import internal/listingkit.
[ ] internal/product/sourcing does not import marketplace publishing/workspace packages.
[ ] identity validation distinguishes missing source id from weak-but-fingerprintable identity.
[ ] tests cover stable fingerprint behavior.
[ ] no ListingKit API DTOs are changed.
```

### PR 2: inventory one source and map it to the envelope

Suggested title:

```text
docs: inventory first product source mapping
```

or, if implementation is safe:

```text
feat: map first source into product sourcing envelope
```

Scope:

- pick exactly one source path;
- document available raw fields and missing fields;
- map raw source data into `SourceEnvelope`;
- keep crawler/runtime adapter code thin;
- avoid marketplace publish payload decisions.

Current selected source:

```text
Amazon source product result -> internal/product/sourcing.SourceEnvelope
1688 source product result -> internal/product/sourcing.SourceEnvelope
```

Acceptance criteria:

```text
[ ] one source path produces a SourceEnvelope or a documented mapping table.
[ ] missing facts are represented as warnings, not hidden defaults.
[ ] source mapping can be tested without running browser automation.
[ ] crawler/integration package does not import internal/listingkit.
```

### PR 3: catalog and asset handoff

Suggested title:

```text
feat: hand off product source facts to catalog and assets
```

Scope:

- introduce the narrow handoff from source envelope to catalog/product facts;
- introduce the narrow handoff from source envelope to asset/image facts;
- keep platform-neutral facts outside marketplace packages;
- avoid changing ListingKit task orchestration until the facts are stable.

Current implementation path:

```text
internal/product/sourcing.SourceEnvelope
  -> internal/catalog.ProductFacts
  -> internal/asset.Facts
```

Acceptance criteria:

```text
[ ] source facts can produce a neutral product candidate.
[ ] source image/design facts can produce neutral asset candidates.
[ ] marketplace packages do not own source identity or source normalization.
[ ] tests cover at least one product candidate and one asset candidate mapping.
```

### PR 4: ListingKit orchestration bridge

Suggested title:

```text
feat: create listing task from product sourcing envelope
```

Scope:

- add a narrow ListingKit orchestration bridge that consumes normalized facts;
- create or prepare a batch/task through existing ListingKit flows;
- keep ListingKit as orchestration and DTO adaptation only;
- avoid adding source-specific branches to root ListingKit except temporary adapter shells with follow-up notes.

Current implementation path:

```text
internal/catalog.ProductFacts + internal/asset.Facts
  -> internal/listingkit.SourceFactsGenerateRequestInput
  -> internal/listingkit.GenerateRequest

internal/product/sourcehandoff.ListingKitRequestInput
  -> internal/listingkit.GenerateRequest
  -> existing CreateGenerateTask boundary when a caller provides a creator
```

The bridge is intentionally a pure DTO-adaptation function. It does not create tasks, submit packages, assemble marketplace payloads, or introduce another submission state owner.

Acceptance criteria:

```text
[ ] ListingKit receives normalized facts, not raw source payloads.
[ ] source-specific behavior remains outside root ListingKit.
[ ] existing SHEIN preview/submission path is reused.
[ ] task lineage records source identity or a source reference.
[ ] failure and missing-fact states are explainable to operators.
```

### PR 5: source boundary guard tests

Suggested title:

```text
test: guard product sourcing boundaries
```

Scope:

- add import-boundary tests for product-source packages;
- guard crawler/integration packages from importing ListingKit root and marketplace publishing/workspace packages;
- guard `internal/product/sourcing` from importing runtime, HTTPAPI, ListingKit root, or marketplace packages;
- document any temporary exception with owner and retirement condition.

Current guard coverage:

```text
internal/product/sourcing/boundary_guard_test.go
internal/integration/crawler/boundary_guard_test.go
internal/catalog/boundary_guard_test.go
internal/asset/boundary_guard_test.go
internal/listingkit/product_source_bridge_boundary_test.go
```

Guard intent:

- crawler/integration remains raw source collection only;
- product sourcing does not import ListingKit, marketplace, runtime, HTTPAPI, infra, or platform packages;
- catalog and asset fact packages do not import source, ListingKit, marketplace, crawler, runtime, infra, or platform packages;
- the ListingKit product-source bridge can import only standard library plus neutral `internal/catalog` and `internal/asset` facts.

Acceptance criteria:

```text
[ ] crawler/integration -> internal/listingkit root import is blocked or allowlisted with a retirement reason.
[ ] crawler/integration -> marketplace publishing/workspace import is blocked or allowlisted with a retirement reason.
[ ] internal/product/sourcing -> internal/listingkit import is blocked.
[ ] internal/product/sourcing -> runtime/httpapi import is blocked.
[ ] guard names are referenced from docs when they become stable review policy.
```

## 5. Definition of done for the MVP

The MVP is done when:

```text
[ ] one source path has a stable SourceIdentity.
[ ] one source path can produce a SourceEnvelope.
[ ] source facts can hand off to neutral product/catalog facts.
[ ] source image/design facts can hand off to asset facts.
[ ] ListingKit can create or prepare a task/batch from normalized facts.
[ ] existing SHEIN preview/submission behavior remains the target-platform path.
[ ] source/crawler/product-sourcing boundary tests exist or missing guard coverage is explicitly documented.
[ ] no new broad policy is added to root internal/listingkit.
```

## 6. Validation checklist

Run focused tests first:

```powershell
go test ./internal/product/sourcing/... -count=1
go test ./internal/product/sourcehandoff/... -count=1
go test ./internal/catalog/... -count=1
go test ./internal/asset/... -count=1
go test ./tests/... -count=1
```

Then run broader validation when the bridge touches ListingKit:

```powershell
go test ./internal/listingkit/... -count=1
go test ./... -count=1
```

If dependency evidence is needed, keep it local:

```powershell
New-Item -ItemType Directory -Force .local/refactoring | Out-Null
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath .local/refactoring/dependency-baseline-output.txt
```

Do not commit generated package/dependency baselines as long-lived docs.

## 7. Stop conditions

Pause and document before continuing if:

- the first source requires large schema changes before identity is clear;
- the source adapter needs browser/runtime behavior to test basic normalization;
- ListingKit must receive raw source payloads to proceed;
- product sourcing needs to know SHEIN/TEMU/Amazon publish payload fields;
- two source integrations are being built in the same PR;
- a package move changes behavior;
- import-boundary allowlists grow without a retirement condition.

## 8. Immediate next action

PR 1 through PR 5 now establish identity/envelope, the first Amazon source-envelope mapping, neutral catalog/asset facts, a narrow ListingKit request bridge, and source/crawler/facts boundary guards. 1688 now has a source-envelope mapping, a flow-level request test, and a controlled source-envelope handoff into the existing ListingKit generate-task create boundary.

Next, run focused validation and then choose whether to:

1. expose the controlled 1688 handoff through a narrow API/application adapter, or
2. start the next new warehouse/source catalog path such as 大建云仓.

Implementation checklist:

```text
[ ] inspect existing internal/product/sourcing package shape, if present.
[ ] define SourceIdentity and SourceEnvelope with minimal neutral fields.
[ ] add validation/fingerprint tests.
[ ] add or plan import-boundary guard coverage.
[ ] map the existing Amazon product result path into SourceEnvelope.
[ ] map the existing 1688 product result path into SourceEnvelope.
[ ] map SourceEnvelope into internal/catalog and internal/asset neutral facts.
[ ] map internal/catalog and internal/asset neutral facts into a ListingKit GenerateRequest bridge.
[ ] add controlled SourceEnvelope -> GenerateRequest -> CreateGenerateTask handoff.
[ ] guard product-source, catalog, asset, crawler, and ListingKit bridge dependency direction.
[ ] update this plan only if the first chosen source changes the PR order.
```
