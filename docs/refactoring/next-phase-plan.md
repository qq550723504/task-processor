# Next Phase Refactoring Plan

> Status: active next-phase plan after the ListingKit boundary checkpoint, service slimming wave, HTTPAPI runtime inventory, generated-baseline cleanup, and product-source MVP planning.
>
> Last reviewed: 2026-07-09.

## 1. Current Position

The project has moved beyond the early “split large files” phase.

Current read:

- ListingKit file-group slimming has reached a checkpoint.
- Submission refactoring has moved from file grouping into boundary migration and guardrail work.
- HTTPAPI runtime assembly inventory exists and shows `internal/app/httpapi` is mostly in the right shape.
- Import boundary tests are part of the active architecture guardrail system.
- Generated package/dependency baselines are local evidence, not committed architecture documents.
- CI has been reported as run for the documentation cleanup wave, with exact job status remaining in GitHub Actions.
- The next useful work is no longer more helper shaving; it is the product-source MVP loop described in `docs/product/product-sourcing-mvp-plan.md`.

This plan supersedes ad-hoc continuation of file splitting when the split does not reduce ownership, import pressure, or runtime/business coupling.

## 2. Main Conclusion

Do **not** keep extracting details from `internal/listingkit` just because a file can be made smaller.

The next phase should focus on:

1. keeping the runtime/boundary checkpoint stable,
2. keeping `httpapi` assembly-only,
3. proving product-source expansion through one source loop,
4. strengthening source/crawler/import boundary tests,
5. keeping generated refactoring evidence out of committed docs unless it is deliberately promoted to a dated validation note.

## 3. Current Phase Status

| Area | Status | Next posture |
| --- | --- | --- |
| Preview | Checkpointed | Do not continue extracting unless ownership clearly moves to `internal/listing/preview` |
| Submission | Late boundary migration | Continue only small policy seams that do not touch Temporal determinism or submit side effects |
| Service slimming | First wave complete | Freeze as compatibility/facade shell; avoid more cosmetic splitting |
| HTTPAPI runtime | Inventory complete | Keep app/runtime assembly thin; watch adapter helper hotspots |
| SHEIN marketplace rules | Active small-step migration | Move new pure rules to marketplace/publishing/workspace homes |
| Product sourcing | Active MVP path | Introduce source identity/envelope, map one source, then bridge to catalog/assets and ListingKit |
| Boundary tests | Active | Stabilize and explain allowlists; add source/crawler guards during the MVP |
| Generated baselines | Local evidence only | Write outputs under `.local/` or a dated validation note, not long-lived refactoring docs |

## 4. Immediate Priorities

### Priority 1: Keep checkpoint validation visible

Before more structural movement, make sure the current checkpoint is either green or explicitly documented.

Recommended commands:

```powershell
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi/... -count=1
go test ./internal/app/httpapi/... -count=1
go test ./tests/... -count=1
```

Then run broader validation when practical:

```powershell
go test ./... -count=1
New-Item -ItemType Directory -Force .local/refactoring | Out-Null
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath .local/refactoring/dependency-baseline-output.txt
```

Acceptance criteria:

- Test failures are classified as refactor regressions, flaky fixtures, stale allowlists, or legacy exceptions.
- Import boundary allowlists explain real temporary exceptions.
- Generated package/dependency outputs are treated as local evidence unless a dated validation note explicitly summarizes them.
- No new broad refactoring starts while basic checkpoint validation is red.

### Priority 2: Start the Product Sourcing MVP

Use `docs/product/product-sourcing-mvp-plan.md` as the active execution guide.

MVP flow:

```text
raw source data
  -> SourceIdentity + SourceEnvelope
  -> source-result normalization
  -> catalog / asset facts
  -> ListingKit batch or task orchestration
  -> existing SHEIN preview / submission path
```

Target package order:

1. `internal/product/sourcing`
2. `internal/catalog` or the approved product/catalog target
3. `internal/asset` or the approved product/asset target
4. `internal/integration/crawler/*`
5. root `internal/listingkit` only as orchestration/DTO bridge after facts are normalized

Acceptance criteria:

- One source path has stable `SourceIdentity`.
- One source path can produce a neutral `SourceEnvelope`.
- Source facts can hand off to product/catalog and asset facts.
- ListingKit receives normalized facts, not raw source payloads.
- Existing SHEIN preview/submission remains the target-platform path.
- Source/crawler boundary tests exist or missing guard coverage is explicitly documented.

### Priority 3: Keep HTTPAPI runtime closed

`docs/refactoring/httpapi-runtime-inventory.md` records the HTTPAPI runtime inventory.

Next action is not another broad split. Instead:

- keep `internal/app/httpapi` assembly-only,
- avoid moving feature policy into app/runtime packages,
- watch `internal/listingkit/httpapi/shein_sync_runtime.go`,
- watch `internal/listingkit/httpapi/ai_clients.go`,
- keep runtime helper files as adapter construction, not domain rule owners.

## 5. Recommended PR Queue

### PR A: Validate checkpoint and keep evidence local

Suggested title:

```text
test: validate listingkit boundary checkpoint
```

Scope:

- Run focused ListingKit, HTTPAPI, and boundary tests.
- Fix true refactor regressions.
- Stabilize flaky fixtures only when required.
- Update import allowlists only with a clear temporary reason.
- Keep generated dependency/package outputs in `.local/` unless their result is summarized in a dated validation note.

Do not:

- move more code,
- rename more files,
- introduce new package boundaries,
- commit generated baseline snapshots.

### PR B: Introduce source identity and envelope

Suggested title:

```text
feat: introduce product sourcing source identity
```

Scope:

- create or extend `internal/product/sourcing`,
- add `SourceIdentity` and `SourceEnvelope` minimal neutral models,
- add validation/fingerprint tests,
- keep the package free of ListingKit, marketplace, HTTP runtime, and crawler runtime dependencies.

Do not:

- change ListingKit API DTOs,
- assemble SHEIN publish payloads,
- pick two product sources at once.

### PR C: Map one source into the envelope

Suggested title:

```text
feat: map first source into product sourcing envelope
```

Scope:

- pick exactly one source path,
- document raw fields and missing fields,
- map raw source data into `SourceEnvelope`,
- keep crawler/runtime adapter code thin,
- represent missing facts as warnings, not hidden defaults.

Do not:

- require browser automation to unit-test normalization,
- let crawler/integration import root ListingKit,
- let crawler/integration import marketplace publishing or workspace packages.

### PR D: Hand off product and asset facts

Suggested title:

```text
feat: hand off product source facts to catalog and assets
```

Scope:

- turn source envelope data into neutral product/catalog facts,
- turn source image/design data into neutral asset facts,
- keep platform-neutral facts outside marketplace packages.

Do not:

- add target-marketplace category or publish rules to product sourcing,
- move broad DTO sets while mapping facts.

### PR E: Add the ListingKit orchestration bridge

Suggested title:

```text
feat: create listing task from product sourcing envelope
```

Scope:

- let ListingKit consume normalized facts,
- create or prepare a batch/task through existing flows,
- record source identity or source reference for lineage,
- reuse existing SHEIN preview/submission behavior.

Do not:

- let ListingKit consume raw source payloads,
- add permanent `if source == ...` branches to root ListingKit,
- introduce another submission owner.

### PR F: Guard source boundaries

Suggested title:

```text
test: guard product sourcing boundaries
```

Scope:

- guard `internal/product/sourcing` from importing root ListingKit, marketplace packages, runtime, or HTTPAPI packages,
- guard crawler/integration packages from importing root ListingKit and marketplace publishing/workspace packages,
- document temporary exceptions with owner and retirement condition.

## 6. Stop Conditions

Pause and document instead of continuing if any proposed slice:

- requires a target package to import `internal/listingkit`,
- touches Temporal determinism or activity retry semantics,
- changes API DTOs or public `Service` contracts before the source envelope is stable,
- moves runtime client construction into a business package,
- combines behavior change with package movement,
- only reduces file size without changing ownership or boundary clarity,
- requires broad allowlist expansion without a migration explanation,
- depends on stale generated package/dependency snapshots,
- starts full new sales-platform workbench expansion during the product-source MVP.

## 7. Boundary Rules For New Code

New code should prefer these homes:

| Kind of code | Preferred home |
| --- | --- |
| Product source identity and normalization | `internal/product/sourcing` |
| Product/catalog facts | `internal/catalog` or approved product/catalog target |
| Product image/design/asset facts | `internal/asset` or approved asset target |
| Crawler execution adapters | `internal/integration/crawler/*` |
| Generic listing submission policy | `internal/listing/submission` |
| Listing preview rules | `internal/listing/preview` |
| SHEIN publishing rules | `internal/marketplace/shein/publishing` or `internal/publishing/shein` depending on compatibility level |
| SHEIN workspace/editor presentation rules | `internal/marketplace/shein/workspace` |
| HTTP runtime assembly | `internal/app/httpapi` or feature-owned `*/httpapi` packages |
| Legacy compatibility / API shell glue | `internal/listingkit` |

## 8. Current Main Risk

The main risk is no longer that the project lacks structure.

The main risk is now:

```text
continuing to split or rename files without proving the structure supports real product-source growth.
```

Mitigation:

- start with one source path,
- prefer target-domain seams over compatibility-shell grooming,
- keep every PR tied to an explicit ownership improvement,
- keep transient evidence under `.local/` unless promoted into a dated validation note,
- keep full new sales-platform workbenches deferred until the source loop proves reusable.

## 9. Definition Of Done For The Next Phase

The next phase is complete when:

- focused ListingKit, HTTPAPI, and boundary tests are green or documented,
- HTTPAPI runtime assembly checkpoint is current,
- one source path has a stable `SourceIdentity`,
- one source path can produce a `SourceEnvelope`,
- source facts can hand off to neutral product/catalog and asset facts,
- ListingKit can create or prepare a batch/task from normalized source facts,
- source/crawler boundary guards exist or missing guard coverage is explicitly documented,
- import boundary allowlists are stable and explained,
- `internal/listingkit` receives no new broad policy ownership,
- generated baseline outputs are not committed as long-lived docs.

## 10. Recommended Immediate Next Step

Start with PR B from this plan:

```text
feat: introduce product sourcing source identity
```

Implementation checklist:

```text
[ ] inspect existing internal/product/sourcing package shape, if present.
[ ] define SourceIdentity and SourceEnvelope with minimal neutral fields.
[ ] add validation/fingerprint tests.
[ ] keep the package free of ListingKit, marketplace, runtime, and HTTPAPI dependencies.
[ ] update docs/product/product-sourcing-mvp-plan.md only if the first chosen source changes the PR order.
```
