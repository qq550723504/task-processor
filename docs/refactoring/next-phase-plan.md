# Next Phase Refactoring Plan

> Status: active next-phase plan after the ListingKit boundary checkpoint, service slimming wave, HTTPAPI runtime inventory, and generated-baseline cleanup.
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
- Further work should prioritize ownership reduction and boundary enforcement, not helper shaving.

This plan supersedes ad-hoc continuation of file splitting when the split does not reduce ownership, import pressure, or runtime/business coupling.

## 2. Main Conclusion

Do **not** keep extracting details from `internal/listingkit` just because a file can be made smaller.

The next phase should focus on:

1. validating and freezing the current checkpoint,
2. keeping `httpapi` assembly-only,
3. moving only small, stable ownership seams into target domains,
4. strengthening boundary tests and allowlist discipline,
5. keeping generated refactoring evidence out of committed docs unless it is deliberately promoted to a dated validation note.

## 3. Current Phase Status

| Area | Status | Next posture |
| --- | --- | --- |
| Preview | Checkpointed | Do not continue extracting unless ownership clearly moves to `internal/listing/preview` |
| Submission | Late boundary migration | Continue only small policy seams that do not touch Temporal determinism or submit side effects |
| Service slimming | First wave complete | Freeze as compatibility/facade shell; avoid more cosmetic splitting |
| HTTPAPI runtime | Inventory complete | Keep app/runtime assembly thin; watch adapter helper hotspots |
| SHEIN marketplace rules | Active small-step migration | Move new pure rules to marketplace/publishing/workspace homes |
| Product sourcing | Target exists | Move only source normalization seams when crawler adapters remain thin |
| Boundary tests | Active | Stabilize and explain allowlists before more migration |
| Generated baselines | Local evidence only | Write outputs under `.local/` or a dated validation note, not long-lived refactoring docs |

## 4. Immediate Priorities

### Priority 1: Checkpoint validation

Before more structural movement, validate the current state.

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

### Priority 2: HTTPAPI runtime checkpoint closeout

`docs/refactoring/httpapi-runtime-inventory.md` records the current HTTPAPI runtime inventory.

Next action is not another broad split. Instead:

- keep `internal/app/httpapi` assembly-only,
- avoid moving feature policy into app/runtime packages,
- watch `internal/listingkit/httpapi/shein_sync_runtime.go`,
- watch `internal/listingkit/httpapi/ai_clients.go`,
- keep runtime helper files as adapter construction, not domain rule owners.

Acceptance criteria:

- `internal/app/httpapi` remains module/bootstrap/runtime assembly only.
- Any new business rule is placed in ListingKit, marketplace, product, or integration domain packages instead of app runtime.
- New runtime helpers do not import or own platform business policies.

### Priority 3: Small target-domain ownership seams

Prefer work that reduces accidental root ownership.

Recommended target order:

1. `internal/listing/submission`
2. `internal/marketplace/shein/publishing`
3. `internal/marketplace/shein/workspace`
4. `internal/product/sourcing`
5. `internal/integration/crawler/*`

Do not move broad DTO sets or runtime clients as part of these slices.

Acceptance criteria:

- The target package does not import `internal/listingkit`.
- The moved logic is a stable rule/policy, not runtime wiring.
- ListingKit keeps only compatibility, DTO adaptation, persistence callback, or orchestration glue.
- A boundary test or existing guard protects the new dependency direction.

## 5. Recommended PR Queue

### PR A: Validate checkpoint and stabilize guards

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

### PR B: Close HTTPAPI runtime checkpoint

Suggested title:

```text
docs: close httpapi runtime assembly checkpoint
```

Scope:

- Update `httpapi-runtime-inventory.md` if the latest split changed file names or ownership.
- Explicitly mark `internal/app/httpapi` as assembly-only at the current checkpoint.
- Keep `shein_sync_runtime.go` and `ai_clients.go` on the watchlist if they continue to own adapter shaping.

Do not:

- reopen app-layer assembly if the inventory shows no real business logic there.

### PR C: Move one read-only submission policy seam

Suggested title:

```text
refactor: move one submission policy seam
```

Scope:

- Pick a single read-only policy currently still owned by root ListingKit.
- Move it to `internal/listing/submission` only if it does not require root models.
- Keep persistence, side effects, Temporal activity flow, and SHEIN API calls out of the target package.

Good candidates:

- decision predicates,
- normalization rules,
- status classification rules,
- request/record matching policies with neutral inputs.

Bad candidates:

- submit remote execution,
- persistence callbacks,
- Temporal workflow/activity orchestration,
- SHEIN package mutation with concrete runtime dependencies.

### PR D: Move one SHEIN publishing rule seam

Suggested title:

```text
refactor: move one shein publishing policy seam
```

Scope:

- Move one pure SHEIN publishing rule to `internal/marketplace/shein/publishing` or keep it in `internal/publishing/shein` if it remains a compatibility/model-layer rule.
- Add or reuse guard tests proving marketplace publishing does not import ListingKit.

Good candidates:

- response interpretation rules,
- fallback/default-confirmed rules,
- remote record selection predicates,
- supplier/lookup normalization with neutral inputs.

Bad candidates:

- runtime API client construction,
- database/repository wiring,
- ListingKit DTO shaping,
- workflow activity host code.

### PR E: Product sourcing inventory or first source-normalization seam

Suggested title:

```text
docs: inventory product sourcing handoff
```

or, if a safe seam is already obvious:

```text
refactor: introduce product sourcing normalization seam
```

Scope:

- Keep crawler packages focused on extraction/runtime adapters.
- Move source-result normalization and source identity toward `internal/product/sourcing`.
- Avoid marketplace publishing packages absorbing crawler ownership.
- Use `docs/product/product-sourcing-handoff.md` as the current handoff guide.

## 6. Stop Conditions

Pause and document instead of continuing if any proposed slice:

- requires a target package to import `internal/listingkit`,
- touches Temporal determinism or activity retry semantics,
- changes API DTOs or public `Service` contracts,
- moves runtime client construction into a business package,
- combines behavior change with package movement,
- only reduces file size without changing ownership or boundary clarity,
- requires broad allowlist expansion without a migration explanation,
- depends on stale generated package/dependency snapshots.

## 7. Boundary Rules For New Code

New code should prefer these homes:

| Kind of code | Preferred home |
| --- | --- |
| Generic listing submission policy | `internal/listing/submission` |
| Listing preview rules | `internal/listing/preview` |
| SHEIN publishing rules | `internal/marketplace/shein/publishing` or `internal/publishing/shein` depending on compatibility level |
| SHEIN workspace/editor presentation rules | `internal/marketplace/shein/workspace` |
| Product source normalization | `internal/product/sourcing` |
| Crawler execution adapters | `internal/integration/crawler/*` |
| HTTP runtime assembly | `internal/app/httpapi` or feature-owned `*/httpapi` packages |
| Legacy compatibility / API shell glue | `internal/listingkit` |

## 8. Current Main Risk

The main risk is no longer that the project lacks structure.

The main risk is now:

```text
continuing to split or rename files without reducing real ownership or enforcing better dependency direction.
```

Mitigation:

- prefer tests and guardrails before new extraction,
- prefer target-domain seams over compatibility-shell grooming,
- keep every PR tied to an explicit ownership improvement,
- keep transient evidence under `.local/` unless promoted into a dated validation note.

## 9. Definition Of Done For The Next Phase

The next phase is complete when:

- focused ListingKit, HTTPAPI, and boundary tests are green or documented,
- HTTPAPI runtime assembly checkpoint is current,
- at least one small target-domain ownership seam is migrated with guard coverage,
- import boundary allowlists are stable and explained,
- product-source handoff rules are documented and followed by new source work,
- `internal/listingkit` receives no new broad policy ownership,
- generated baseline outputs are not committed as long-lived docs.

## 10. Recommended Immediate Next Step

Start with checkpoint validation.

Do this before another migration PR:

```powershell
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi/... -count=1
go test ./internal/app/httpapi/... -count=1
go test ./tests/... -count=1
```

Then choose either:

1. update the HTTPAPI runtime checkpoint if the latest split changed the inventory,
2. move one small read-only policy seam to `internal/listing/submission`, or
3. start the product sourcing handoff work described in `docs/product/product-sourcing-handoff.md`.
