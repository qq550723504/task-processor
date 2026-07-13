# Current Refactoring Status

> Status: active current-state document.
>
> Last reviewed: 2026-07-13.
>
> Calibrated against: `master` at `5c72f406c18b40d3860fb8f0c7518c6606f38b85`.
>
> Scope: current product maturity, validation gates, refactoring closeout, Product Sourcing closeout, and the active Now / Next / Later direction for Task Processor / ListingKit.

## 1. Current position

The project is past both the early broad-splitting phase and the initial Product Sourcing foundation phase.

The current posture is:

```text
Stabilize and validate the current SHEIN production path first;
close the implemented Product Sourcing MVP with controlled evidence second;
select exactly one next product source only after that;
defer full new sales-platform workbenches until the SHEIN template is stable.
```

This is a modular-monolith stabilization and growth-readiness phase. It is not a greenfield architecture phase and it is not the right time for another broad directory rewrite.

Use this file together with:

- `docs/refactoring/next-phase-plan.md`
- `docs/refactoring/listingkit-boundary-checkpoint.md`
- `docs/product/product-sourcing-handoff.md`
- `docs/product/product-sourcing-mvp-plan.md`
- `docs/refactoring/decisions/2026-06-26-next-growth-sequence.md`
- `docs/development/repository-structure.md`

### 1.1 Evidence vocabulary

Use these terms consistently in status documents and review notes:

- **Implemented**: the code path exists on the calibrated `master` baseline.
- **Repository-validated**: the exact baseline has recorded automated test/build results.
- **Production-validated**: a real environment or real API run is recorded in a dated validation note.
- **Deferred**: code or runtime assets may exist, but the capability is not an active product-expansion commitment.

Do not treat “implemented” as equivalent to “repository-validated” or “production-validated”.

## 2. Current system reality

### 2.1 Product and runtime shape

ListingKit is the product entrypoint. The repository is no longer best described as a generic task processor.

The maintained runtime entrypoints are:

- `cmd/product-listing-api`
- `cmd/listing-control-plane`
- `cmd/shein-listing`
- `cmd/temu-listing`

An official runtime entrypoint means that the command is maintained and structurally supported. It does not mean every target-platform product experience has the same maturity.

### 2.2 Platform maturity

| Capability | Current status | Current interpretation |
| --- | --- | --- |
| SHEIN target listing | Production main path; active stabilization | The current product and release focus. Pricing, promotion, readiness, caching, browser startup, save-draft, publish, and recovery behavior require current-baseline evidence. |
| SDS POD source/design flow | Active capability; active stabilization | Treated as a POD/design capability rather than a generic product-source integration. |
| 1688 product source | Implemented through normalization and a narrow ListingKit task bridge; controlled validation pending | The first business-priority product-source path to close with repository and real-flow evidence. |
| Amazon product source | Implemented as a source-envelope boundary-validation path | Useful for source modeling and tests; it does not mean a full Amazon ListingKit workbench is active. |
| TEMU target listing | Runtime and integration assets retained; full workbench deferred | Maintain existing runtime correctness without starting a broad new workbench. |
| Amazon target listing | Existing historical/target code retained; full workbench deferred | Do not infer current product parity with SHEIN. |
| 大建云仓 / next warehouse source | Planned candidate | Start only after the current Product Sourcing MVP is closed and its contract is clear. |

### 2.3 Product Sourcing implementation status

The Product Sourcing MVP foundation is implemented on the calibrated baseline:

1. `internal/product/sourcing` owns `SourceIdentity`, fingerprinting, validation, and `SourceEnvelope`.
2. Amazon and 1688 source results map into the neutral envelope.
3. Source envelopes hand off to `internal/catalog.ProductFacts` and `internal/asset.Facts`.
4. `internal/product/sourcehandoff` adapts neutral facts into the existing ListingKit `GenerateRequest`.
5. The 1688 handoff has a narrow command and HTTP adapter path to the existing task-creation boundary.
6. Product-source, crawler, catalog, asset, and ListingKit bridge dependency directions have guard tests.

The remaining Product Sourcing work is not “introduce the model”. It is:

- record focused and full validation against the current baseline;
- exercise one controlled 1688 import-to-task-to-preview path;
- verify lineage, warnings, and missing-fact behavior are visible enough for operators;
- close the MVP explicitly before choosing the next source.

## 3. Now

Current work should focus on validation, production stabilization, and closeout—not broad feature expansion.

### 3.1 Validate the current baseline

Keep exact results visible for the calibrated `master` commit or for the newer commit being considered for release:

```powershell
go test ./... -count=1

go test -race ./internal/app/runtime/listingcontrol `
  -run TestControlPlaneService -count=1

go test -race ./internal/listingadmin `
  -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" `
  -count=1

make build-all
```

Frontend validation:

```powershell
Set-Location web/listingkit-ui
npm ci
npm run lint
npm run typecheck
npm test
npm run build
```

Required posture:

1. Treat `.github/workflows/ci.yml` as the executable core gate.
2. Treat GitHub Actions as the source of truth for exact job status and logs.
3. Record manual smoke or production evidence in a dated validation note when it affects a release decision.
4. Do not call the current baseline green unless the exact workflow or command result is visible.

### 3.2 Stabilize the SHEIN production path

The recent change stream concentrates on production-sensitive behavior rather than cosmetic refactoring.

Keep focused evidence for:

1. synchronized SHEIN supply-price usage;
2. promotion drop-rate and breakeven calculations;
3. multi-SKU price and cost completeness;
4. republish resolution-cache preservation;
5. SDS baseline and canonical metadata behavior;
6. action-aware submit-readiness and POD-readiness policy;
7. save-draft / publish idempotency and recovery;
8. SHEIN listing browser startup and rollout smoke behavior.

New pure SHEIN policies should continue moving behind marketplace/publishing/workspace seams. Root `internal/listingkit` should consume stable seams rather than acquire new marketplace ownership.

### 3.3 Close the Product Sourcing MVP

Required next work:

1. Run the focused Product Sourcing, catalog, asset, source-handoff, ListingKit, and boundary tests.
2. Exercise one controlled 1688 path:
   - source request or URL;
   - source normalization;
   - `SourceEnvelope`;
   - catalog and asset facts;
   - ListingKit task creation;
   - existing preview/readiness path.
3. Verify the task records source lineage or a durable source reference.
4. Verify missing facts remain warnings or explicit errors rather than hidden defaults.
5. Record the result in a dated validation note.
6. Decide explicitly whether the MVP is closed before starting 大建云仓 or another warehouse source.

### 3.4 Preserve runtime and package boundaries

Required posture:

1. Keep listing runtime and app runtime free of retired broad management-service semantics.
2. Keep `internal/app/*` focused on runtime assembly.
3. Keep `internal/listingkit` focused on orchestration, compatibility, DTO adaptation, persistence ordering, and API-shell glue.
4. Prefer small target-domain policy seams with focused tests.
5. Update import allowlists only with an owner, reason, and retirement condition.
6. Do not use stale generated package maps or dependency snapshots as architecture authority.
7. Do not continue splitting files merely because a helper can move.

### 3.5 Control Plane posture

The Go Listing Control Plane has recorded production validation for leader election, standby readiness, leader takeover, dispatch persistence, rollback, and roll-forward behavior.

Current work should preserve that result and keep current-baseline evidence visible for:

- listing control-plane race tests;
- listingadmin dispatch / rollback / recovery race tests;
- leader status and takeover behavior;
- dispatch event distribution and `failed > 0` reporting;
- production rollout and rollback procedures.

Do not introduce another scheduler or watchdog owner.

## 4. Next

After the current validation and closeout gates are green or explicitly documented:

### 4.1 Select one next product source

The preferred next growth direction is one warehouse or catalog source, with 大建云仓 as the current named candidate.

Before implementation:

1. define the raw source contract;
2. identify stable source identity fields;
3. identify missing and optional facts;
4. confirm authentication, pagination, rate-limit, and snapshot behavior;
5. map the contract into the existing `SourceEnvelope`;
6. keep crawler/access concerns outside product normalization;
7. keep target-marketplace publishing rules outside source packages.

Start exactly one source integration. Do not combine it with a new target-platform workbench.

### 4.2 Improve operational evidence

After the controlled 1688 path is validated:

- make source lineage and warnings inspectable;
- keep dispatch and submission failure reasons operator-visible;
- record real successful and failed task examples;
- close gaps in configuration health checks and recovery guidance;
- convert recurring operational findings into tests or stable runbooks.

### 4.3 Keep HTTP runtime closed

Continue to keep:

- `internal/app/httpapi` assembly-only;
- feature policy in feature-owned packages;
- external clients behind small interfaces;
- source-specific adapters outside root ListingKit;
- runtime helper files limited to adapter construction and composition.

## 5. Later

Full new sales-platform expansion should wait until the SHEIN template and current source loop are stable enough to copy without copying legacy coupling.

Allowed preparatory work:

1. platform capability inventory;
2. API and readiness contract design;
3. mapping-cost assessment;
4. read-only package guards;
5. payload-preview exploration that does not introduce a second submission state machine.

Deferred:

1. full TEMU / Amazon / Walmart workbench expansion;
2. new platform auto-publish runtime;
3. another dispatch scheduler or watchdog owner;
4. marketplace-specific rules in root `internal/listingkit`;
5. a new submission state machine outside `internal/listing/submission` and marketplace-owned publishing packages;
6. microservice extraction before package and runtime boundaries are stable.

## 6. Do not do now

Do not start work that:

- renames broad package trees for directory consistency only;
- moves files without reducing ownership or dependency pressure;
- combines behavior changes with package movement;
- expands import-boundary allowlists without a migration explanation;
- adds business rules to `internal/app/*`;
- adds SHEIN, TEMU, Amazon, or Walmart policy to root `internal/listingkit`;
- starts another product source before the current source loop is validated or explicitly closed;
- launches a full new sales-platform workbench during current stabilization;
- treats official command existence as proof of equal product maturity;
- treats generated dependency/package snapshots as current without regenerating them;
- treats unrecorded test execution as a green release gate.

## 7. Current execution checklist

Before approving the next structural or release-sensitive PR:

```text
[ ] The exact base commit is named.
[ ] Current automated test/build results are linked or the missing evidence is explicit.
[ ] Production-sensitive SHEIN behavior has focused regression coverage.
[ ] Any real smoke or production result is recorded in a dated validation note.
[ ] The target package does not import internal/listingkit unless it is an approved compatibility bridge.
[ ] The moved logic is a stable rule or policy, not runtime wiring.
[ ] ListingKit keeps only compatibility, DTO adaptation, orchestration, persistence ordering, or API-shell responsibilities.
[ ] Source packages receive neutral source contracts and do not assemble marketplace publish payloads.
[ ] The PR includes a focused test or an import-boundary guard.
[ ] Behavior changes are separated from file moves.
[ ] Temporal determinism or activity retry semantics are untouched unless explicitly reviewed.
[ ] The change does not add a second owner for dispatch, recovery, or submission state.
[ ] Import allowlist changes include an owner, reason, and retirement condition.
[ ] Generated baseline artifacts are local evidence or a deliberately dated validation note.
```

## 8. Source of truth summary

Current order of authority:

1. `current-refactoring-status.md` for current product maturity and Now / Next / Later.
2. `next-phase-plan.md` for the immediate execution queue.
3. `listingkit-boundary-checkpoint.md` for ListingKit stop lines.
4. `docs/product/product-sourcing-handoff.md` for product-source ownership boundaries.
5. `docs/product/product-sourcing-mvp-plan.md` for Product Sourcing implementation and closeout status.
6. `project-wide-refactoring-plan.md` for long-term architecture direction.
7. `project-wide-execution-plan.md` and dated progress snapshots as historical references.
8. Generated package/dependency snapshots only as dated evidence when freshly regenerated.
