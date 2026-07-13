# Product Sourcing MVP Status and Closeout Plan

> Status: foundation implemented; validation closeout active.
>
> Last reviewed: 2026-07-13.
>
> Calibrated against: `master` at `49a202c0a964e54f9864b3a57be5bed4bfbf2cf1`.
>
> Scope: implementation status, remaining validation, and the decision gate before starting another product source.
>
> Current authority: use this plan together with `product-sourcing-handoff.md`, `current-refactoring-status.md`, and `next-phase-plan.md`.

## 1. Goal

Prove that external product sources can enter ListingKit through a stable, reusable, platform-neutral path without pushing raw source logic or marketplace publishing rules into root `internal/listingkit`.

The target loop is:

```text
raw source data
  -> SourceIdentity + SourceEnvelope
  -> source-result normalization
  -> catalog / asset facts
  -> ListingKit task orchestration
  -> existing SHEIN preview / submission path
```

SDS remains a POD/design capability and is not the generic product-source validation target for this MVP.

## 2. Current status

The foundation slices that were previously described as future PRs are implemented on the calibrated baseline.

| Capability | Implementation status | Validation status |
| --- | --- | --- |
| `SourceIdentity` normalization, validation, and fingerprinting | Implemented | Current-baseline focused evidence recorded. |
| Neutral `SourceEnvelope` | Implemented | Current-baseline focused evidence recorded. |
| Amazon source-result mapping | Implemented | Fixture-friendly focused validation recorded. |
| 1688 source-result mapping | Implemented | Focused validation recorded; controlled business flow pending. |
| Catalog fact handoff | Implemented | Focused validation recorded. |
| Asset fact handoff | Implemented | Focused validation recorded. |
| ListingKit `GenerateRequest` bridge | Implemented | Focused validation recorded. |
| 1688 command and HTTP adapter to task creation | Implemented | Focused validation recorded; controlled source-to-task-to-preview flow pending. |
| Source/crawler/catalog/asset/bridge boundary guards | Implemented | Green on the recorded checkpoint baseline. |
| Operator-visible lineage and warning behavior | Partially represented in contracts | Real or controlled-flow verification pending. |
| MVP closeout decision | Not complete | Requires a dated validation note and explicit outcome. |

“Implemented” here means that the path exists in the repository. It does not claim that the exact calibrated commit has independently recorded green tests or production validation.

## 3. Implemented topology

### 3.1 Neutral source contracts

```text
internal/product/sourcing.SourceIdentity
internal/product/sourcing.SourceEnvelope
internal/product/sourcing.ProductCandidate
internal/product/sourcing.AssetCandidate
internal/product/sourcing.SupplierOrCostFacts
internal/product/sourcing.SourceWarning
internal/product/sourcing.SourceTrace
```

The contracts keep target-marketplace categories, publishing payloads, browser clients, HTTP runtimes, and ListingKit DTOs outside the source model.

### 3.2 Source mappings

```text
Amazon product result
  -> internal/product/sourcing.AmazonSourceEnvelope

1688 crawler product result
  -> internal/product/sourcing.Alibaba1688SourceEnvelope
```

Amazon is the fixture-friendly boundary-validation path. It does not imply that a full Amazon target-platform workbench is active.

1688 is the current business-priority source to use for MVP closeout.

### 3.3 Neutral fact handoff

```text
internal/product/sourcing.SourceEnvelope
  -> internal/catalog.ProductFacts
  -> internal/asset.Facts
```

Source identity and warnings remain attached to the neutral handoff. Marketplace packages do not own source identity or source normalization.

### 3.4 ListingKit orchestration bridge

```text
internal/catalog.ProductFacts + internal/asset.Facts
  -> internal/listingkit.SourceFactsGenerateRequestInput
  -> internal/listingkit.GenerateRequest

internal/product/sourcehandoff.ListingKitRequestInput
  -> existing CreateGenerateTask boundary
```

The bridge is DTO adaptation and orchestration only. It does not submit marketplace payloads or create another submission state owner.

### 3.5 1688 task-creation adapter

```text
internal/product/sourcehandoff/a1688.CreateTaskCommand
  -> internal/product/sourcehandoff/a1688.ListingKitTaskInput
  -> internal/product/sourcehandoff.ListingKitRequestInput
  -> existing ListingKit CreateGenerateTask boundary

internal/productenrich/httpapi/sourcea1688
  -> feature-owned HTTP adapter
  -> app HTTP module composition
```

Root ListingKit receives normalized facts through its existing request shape, not a raw 1688 crawler payload.

## 4. Implementation checklist

The following items are complete by repository inspection on the calibrated baseline:

```text
[x] SourceIdentity and SourceEnvelope exist with platform-neutral fields.
[x] Identity validation distinguishes a missing source ID from a weak-but-fingerprintable identity.
[x] Stable fingerprint behavior is implemented.
[x] Amazon source results map into SourceEnvelope.
[x] 1688 source results map into SourceEnvelope.
[x] Missing source facts can be represented as warnings instead of hidden defaults.
[x] SourceEnvelope maps into internal/catalog and internal/asset facts.
[x] Neutral facts map into an existing ListingKit GenerateRequest.
[x] A narrow 1688 command/HTTP path delegates to existing task creation.
[x] Product sourcing is guarded from root ListingKit, marketplace, runtime, HTTPAPI, infra, and platform ownership.
[x] Crawler/integration packages are guarded from ListingKit and marketplace publishing/workspace ownership.
[x] Catalog and asset packages have dependency-direction guards.
[x] The ListingKit source-facts bridge has a narrow import boundary.
```

The following items remain open because they require execution evidence rather than code inspection:

```text
[x] Focused Product Sourcing tests are recorded for the exact checkpoint commit in `product-sourcing-validation-2026-07-13.md`.
[ ] Full backend tests are recorded for the exact closeout commit.
[ ] One controlled 1688 import reaches task creation and the existing preview/readiness path.
[ ] Source lineage or a durable source reference is verified on the created task or traceable result.
[ ] Missing title, assets, price/cost, or source identity produces explainable warnings/errors.
[ ] A failed controlled flow produces enough context for an operator or engineer to act.
[ ] Existing SHEIN preview/submission behavior is reused without a new source-specific submission owner.
[ ] A dated Product Sourcing closeout note records the exact inputs, commit, commands, task IDs, and outcome.
[ ] The MVP is explicitly declared closed or has a short blocker list.
```

## 5. Validation plan

### 5.1 Focused repository validation

Run from the repository root:

```powershell
go test ./internal/product/sourcing/... -count=1
go test ./internal/catalog/... -count=1
go test ./internal/asset/... -count=1
go test ./internal/product/sourcehandoff/... -count=1
go test ./internal/productenrich/httpapi/sourcea1688/... -count=1
go test ./internal/listingkit/... -count=1
go test ./tests/... -count=1
```

Then run the full backend gate:

```powershell
go test ./... -count=1
make build-all
```

Acceptance criteria:

- the exact commit SHA is recorded;
- failures are classified rather than silently ignored;
- import-boundary guards remain green;
- no generated dependency/package snapshot is committed as permanent architecture documentation.

### 5.2 Controlled 1688 flow

Use one deterministic 1688 fixture, snapshot, or controlled source result and record:

1. input URL or source ID;
2. normalized `SourceIdentity` and `SourceKey`;
3. source warnings;
4. product title, variants, assets, supplier, and cost facts;
5. generated ListingKit request fields;
6. created task ID and tenant/user context;
7. source lineage or durable source reference;
8. preview/readiness result;
9. any blocker, failure phase, and next action;
10. whether the existing SHEIN preview/submission path remains the downstream owner.

The closeout does not require uncontrolled browser automation merely to test normalization. Raw-source access and real-flow smoke can be recorded separately from deterministic fixture tests.

### 5.3 Missing-fact cases

At minimum, validate:

- missing source product;
- missing source ID with a fingerprintable URL/version;
- missing title;
- missing image assets;
- missing or invalid cost/price;
- duplicate asset URLs;
- empty or partially populated variants;
- source adapter error propagation.

Missing facts must remain warnings or explicit errors. Do not silently manufacture product truth.

## 6. Boundary constraints

The MVP and future sources must retain these constraints:

- no crawler/integration package imports root `internal/listingkit`;
- no crawler/integration package imports marketplace publishing or workspace packages;
- no `internal/product/sourcing` import of root ListingKit, marketplace, runtime, HTTPAPI, infra, or platform packages;
- no SHEIN/TEMU/Amazon publish payload assembly in source packages;
- no source-specific permanent policy branch in root ListingKit;
- no new submission state machine;
- no behavior-changing package move hidden inside a source-modeling PR;
- no two new source integrations in one MVP slice.

## 7. MVP closeout decision

The MVP can be declared closed when:

```text
[ ] Focused and full tests are recorded for the exact baseline.
[ ] One controlled 1688 path creates a ListingKit task from normalized facts.
[ ] Source lineage and warnings are verified.
[ ] The existing preview/readiness/submission ownership remains unchanged.
[ ] Boundary guards are green.
[ ] A dated validation note records the result.
[ ] No unresolved blocker requires raw source payloads or marketplace policy in Product Sourcing.
```

A closeout note should state one of:

- **Closed**: the contract is reusable for the next source.
- **Conditionally closed**: the contract is reusable, with named operational follow-ups that do not change ownership.
- **Blocked**: list the minimal contract or runtime blockers before another source begins.

## 8. Next-source gate

Do not start another source until the closeout decision above exists.

After closeout, select exactly one next source. The current named candidate is 大建云仓 or another overseas warehouse catalog with an available business contract.

Before implementation, document:

- stable source identity;
- product and variant fields;
- images/design/assets;
- supplier, cost, currency, and availability fields;
- authentication and tenant/store scope;
- pagination and incremental sync behavior;
- rate limits and retry semantics;
- raw snapshot or evidence references;
- missing-field and error behavior;
- mapping into the existing `SourceEnvelope`.

The next source should reuse the current catalog, asset, source-handoff, and ListingKit boundaries. It should not create a parallel product model or a new marketplace submission path.

## 9. Stop conditions

Pause and document before continuing if:

- the controlled 1688 path requires ListingKit to consume raw crawler payloads;
- Product Sourcing needs target-marketplace publish fields;
- source lineage cannot be retained without broad task schema changes;
- basic normalization requires live browser automation to be testable;
- boundary allowlists grow without an owner and retirement condition;
- the next source requires a second source contract instead of reusing `SourceEnvelope`;
- a new target-platform workbench is being mixed into source closeout;
- implementation is being mistaken for repository or production validation.

## 10. Immediate next action

Run the focused validation suite, then execute and document one controlled 1688 source-to-task-to-preview flow.

Do not start the next warehouse source until that result is recorded and the MVP closeout state is explicit.
