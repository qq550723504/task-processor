# ListingKit Refactoring Plan

> Status: active only as a ListingKit-specific supplement. For architecture authority and implementation order, follow [`project-wide-refactoring-plan.md`](./project-wide-refactoring-plan.md) and [`project-wide-execution-plan.md`](./project-wide-execution-plan.md) first.

## 1. Purpose

This document narrows the project-wide refactoring program down to the `internal/listingkit` area.

It should help us:

- reduce root `internal/listingkit` complexity,
- keep `listingkit` focused on orchestration and compatibility,
- move platform-specific behavior out of the root package,
- sequence small, testable, behavior-preserving PRs.

It should not be treated as a competing source of truth.

## 2. Current Position

Observed in the local workspace on 2026-06-09:

| Metric | Current snapshot |
| --- | ---: |
| Root `internal/listingkit` Go files excluding tests | 304 |
| Root `internal/listingkit` Go files including tests | 512 |
| `internal/listingkit/core` | already exists |
| `internal/listingkit/service` | already exists |
| `internal/listingkit/submission` | already exists |

Implications:

- Older goals based on root-file-count `532` are no longer accurate.
- Early extraction work has already started, so future work should continue from the current package shape rather than recreate it.
- The next meaningful work is boundary tightening, preview modularization, submission consolidation, and service slimming, not broad directory creation.

## 3. ListingKit Target Role

Per the project-wide boundary rules, `internal/listingkit` should converge toward:

- task lifecycle and orchestration,
- workflow entrypoints,
- request normalization,
- persistence coordination,
- preview and export aggregation,
- revision and history facade behavior,
- API-facing shell models,
- cross-platform listing task concepts.

Avoid adding new long-lived business rules here when they belong elsewhere, especially:

- SHEIN category, attribute, pricing, or publishing rules,
- SHEIN workspace, repair, editor, and revision UX rules,
- new marketplace-specific behavior that can live in marketplace-owned packages,
- concrete infrastructure client behavior that should sit behind interfaces.

## 4. Non-goals

The following are out of scope for the first-pass ListingKit refactor:

- broad package-tree renames,
- microservice extraction,
- combining feature delivery with file moves,
- moving files solely to satisfy arbitrary file-count targets,
- promoting advisory dependency checks to CI before legacy exceptions are documented.

## 5. Required Baseline Before More Moves

Before starting the next substantial code move, use the same baseline flow defined by the project-wide execution plan.

Commands:

```powershell
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath docs/refactoring/dependency-baseline-output.txt
go test ./internal/listingkit/... -count=1
go test ./internal/app/httpapi/... -count=1
go test ./... -count=1
```

Then update:

- [`dependency-baseline.md`](./dependency-baseline.md)

Minimum baseline fields to refresh:

- root `internal/listingkit` Go file count,
- largest ListingKit files,
- packages importing `internal/listingkit*`,
- advisory boundary violations,
- known legacy exceptions,
- unstable or slow test packages that affect refactoring cadence.

If `go test ./...` is too slow or flaky, record that explicitly and use focused test commands per PR.

## 6. Execution Principles

All ListingKit refactoring work should follow these rules:

1. Keep each PR behavior-preserving unless the PR is explicitly a feature change.
2. Keep one primary purpose per PR.
3. Prefer extracting internal file groups before forcing real subpackage moves.
4. Stop when ownership becomes unclear and document the ambiguity.
5. Keep rollback simple by limiting cross-package movement per PR.
6. Add tests before moves when semantics are not already locked.

## 7. Phase Alignment

This document follows the same order as the project-wide execution plan.

### Phase 0: Baseline and Guardrails

Goal:

- make ListingKit pressure measurable before further moves.

Preferred outputs:

- [`dependency-baseline.md`](./dependency-baseline.md)
- `docs/refactoring/dependency-baseline-output.txt`
- optional focused test baseline notes for ListingKit-heavy packages

Acceptance criteria:

- baseline is filled with real data,
- known exceptions are documented,
- no behavior changes are mixed into the baseline update.

Stop conditions:

- if the dependency scan reveals unclear ownership,
- if test instability makes later PR validation unreliable.

### Phase 1: Preview First Cut

Goal:

- reduce hardcoded platform branching inside preview assembly without changing API behavior.

Recommended work slices:

1. keep `preview_builder.go` thin and delegate to smaller helpers,
2. split per-platform preview assembly into dedicated file-group helpers,
3. add targeted tests for selected-platform semantics and missing payload cases.

Candidate files:

- `internal/listingkit/preview_builder.go`
- `internal/listingkit/preview_header.go`
- `internal/listingkit/preview_platform_sections.go`
- `internal/listingkit/preview_builder_shein.go`
- preview-related tests

Acceptance criteria:

- central preview entrypoint becomes shorter and easier to review,
- platform-specific preview logic is no longer mixed together in one large function,
- preview behavior remains unchanged,
- focused preview tests pass.

Stop conditions:

- if helper extraction starts requiring broad model churn,
- if a real subpackage move would create import cycles before interfaces are ready.

### Phase 2: Preview Package Extraction

Goal:

- group preview aggregation into a bounded module or file cluster without forcing premature package churn.

Recommended work slices:

1. finish internal preview file grouping while staying in `package listingkit` if necessary,
2. introduce package-private adapter-like interfaces where they reduce central branching,
3. evaluate whether a real `preview` subpackage is viable without cycles.

Acceptance criteria:

- logical ownership of preview code is clearer,
- import pressure is lower or at least better understood,
- no cycles are introduced,
- public behavior is unchanged.

Decision rule:

- if package extraction increases coupling or requires widespread model duplication, keep the preview grouping inside `package listingkit` and record that decision.

### Phase 3: Submission Consolidation

Goal:

- gather submit, retry, recovery, lock, and Temporal-adjacent coordination into a tighter submission-oriented surface.

Recommended work slices:

1. inventory submit, retry, recovery, lock, and direct-submit flows,
2. reduce root service field sprawl behind a submission-focused facade or coordinator,
3. keep platform-specific submit rules outside the root package when possible,
4. expand tests around state transitions and lock behavior before moving sensitive logic.

Candidate areas:

- `internal/listingkit/submission/`
- `internal/listingkit/task_requeue_service.go`
- task submission and retry services in root `listingkit`
- submission-related API handlers and tests

Acceptance criteria:

- root service has fewer direct submission dependencies,
- submit and retry flows remain behavior-compatible,
- lock semantics and state transitions are covered by focused tests,
- no platform-specific logic drifts back into generic ListingKit code.

Stop conditions:

- if marketplace-specific submit rules cannot be separated cleanly yet,
- if Temporal-facing behavior and domain behavior are still too entangled to move safely.

### Phase 4: Service Object Slimming

Goal:

- reduce root ListingKit service constructor and dependency sprawl.

Recommended work slices:

1. identify clusters such as preview, submission, revision/history, and studio coordination,
2. introduce private facade structs for coherent clusters,
3. make service construction easier to read and test without changing public API.

Acceptance criteria:

- root service object has fewer direct fields,
- constructor wiring is clearer,
- tests still pass without broad fixture rewrites,
- ownership of each dependency cluster is easier to explain.

Stop conditions:

- if slimming starts to hide unclear boundaries instead of clarifying them,
- if constructors are being rearranged without reducing responsibility.

### Phase 5: Runtime Assembly Cleanup

Goal:

- keep `internal/app/httpapi` and other runtime assembly layers focused on wiring only.

Recommended work slices:

1. remove business-rule leakage from app assembly code,
2. keep route and worker registration thin,
3. move business branching back into ListingKit or marketplace-owned packages.

Acceptance criteria:

- app-layer code is mostly construction, registration, and wiring,
- no new business logic is added to runtime assembly packages.

### Phase 6: Marketplace Boundary Normalization

Goal:

- keep marketplace-specific behavior in marketplace-owned packages and out of generic ListingKit surfaces.

Recommended work slices:

1. identify SHEIN-specific rules still living under root ListingKit,
2. move marketplace-owned logic to the appropriate package when a safe target already exists,
3. leave compatibility facades behind when needed to avoid broad breakage.

Acceptance criteria:

- new marketplace logic no longer lands in root ListingKit by default,
- existing moves reduce cross-domain ambiguity rather than merely shifting files.

### Phase 7: Infrastructure Interface Cleanup

Goal:

- hide concrete external clients behind narrow interfaces where ListingKit currently depends on implementation details.

Recommended work slices:

1. identify direct infrastructure coupling in service construction and task flows,
2. shrink interfaces to what ListingKit actually uses,
3. keep interface introduction incremental and tied to real boundaries.

Acceptance criteria:

- ListingKit depends on narrower abstractions,
- external-client behavior remains unchanged,
- tests or fakes become easier to write where coupling was reduced.

## 8. Candidate Backlog by Area

This is a planning backlog, not a mandatory move list.

### Preview

- central preview builder branching,
- per-platform preview section assembly,
- selected-platform validation behavior,
- preview header and overview composition.

### Submission

- direct submit flow,
- retry and recovery orchestration,
- submission locks,
- task requeue coordination,
- Temporal-facing submit adapters.

### Revision and History

- revision history queries and identifiers,
- restore and validation coordination,
- preview-facing revision presentation seams.

### Studio and Workspace Coordination

- studio batch and session orchestration that still lives in root ListingKit,
- bridge code that may belong in marketplace workspace packages,
- compatibility shims that can stay in ListingKit temporarily.

## 9. PR Template for ListingKit Refactoring

Each refactoring PR should answer:

1. What single boundary or responsibility is being improved?
2. What behavior is intentionally unchanged?
3. Which focused tests were run?
4. Did the change reduce root ListingKit pressure, dependency sprawl, or ownership ambiguity?
5. Did it introduce any temporary compatibility seam or known exception?

## 10. Success Metrics

Measure progress using trend lines, not arbitrary promises.

Primary signals:

- root `internal/listingkit` file pressure trends downward,
- largest root files become smaller or more isolated,
- more responsibilities have an obvious owning package,
- fewer marketplace-specific rules remain in generic ListingKit code,
- fewer direct dependencies hang off the root service object.

Secondary signals:

- test setup becomes easier,
- targeted test suites get faster and more reliable,
- reviewers can explain package ownership with less ambiguity.

## 11. Maintenance Rule

Update this document when:

- the project-wide execution order changes,
- a major ListingKit boundary decision is made,
- a phase is completed and the next bottleneck becomes clearer,
- current metrics materially change.

Do not leave stale hardcoded file counts, branch names, or package-creation steps here after the codebase has moved on.
