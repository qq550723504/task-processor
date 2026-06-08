# Project-wide Refactoring Execution Plan

> Authority: this execution plan implements the direction from [`project-wide-refactoring-plan.md`](./project-wide-refactoring-plan.md) and the boundary rules from [`../architecture/project-boundaries.md`](../architecture/project-boundaries.md).

## 1. Execution Principles

This plan is designed for small, reviewable, behavior-preserving PRs.

Rules:

1. Do not combine behavior changes with package moves.
2. Do not rename broad package trees in the first pass.
3. Do not promote advisory dependency checks to CI until known legacy exceptions are recorded.
4. Every phase should leave the project buildable and testable.
5. Each PR should have one primary purpose.
6. Stop and document when a package move exposes unclear ownership.

## 2. Required Local Baseline Before Code Moves

Before continuing code-level refactoring, run:

```powershell
./scripts/analyze-project-deps.ps1 | Tee-Object -FilePath docs/refactoring/dependency-baseline-output.txt
go test ./internal/listingkit/... -count=1
go test ./internal/app/httpapi/... -count=1
go test ./... -count=1
```

Then update:

- [`dependency-baseline.md`](./dependency-baseline.md)

Minimum baseline fields to fill:

- Root `internal/listingkit` Go file count.
- Top package file counts.
- Largest files.
- Advisory boundary violations.
- Packages importing `internal/listingkit*`.
- Known legacy exceptions.

If `go test ./...` is too slow or flaky, record the failing packages and use focused test commands for subsequent PRs.

## 3. Phase Overview

| Phase | Name | Goal | Risk |
| --- | --- | --- | --- |
| 0 | Baseline and guardrails | Capture current dependency and test state | Low |
| 1 | Preview first cut | Make preview platform logic adapter-ready | Low |
| 2 | Preview package extraction | Move preview aggregation into a bounded package/file group | Medium |
| 3 | Submission consolidation | Gather submit/retry/recovery/state/Temporal adapter logic | Medium |
| 4 | Service object slimming | Reduce root ListingKit service dependency sprawl | Medium |
| 5 | Runtime assembly cleanup | Keep app/httpapi as wiring only | Medium |
| 6 | Marketplace boundary normalization | Prevent platform rules from drifting into ListingKit | Medium-high |
| 7 | Infrastructure interface cleanup | Hide concrete external clients behind narrow interfaces | Medium-high |

## 4. Phase 0: Baseline and Guardrails

### Goal

Make future changes measurable and reversible.

### PR 0.1: Fill dependency baseline

Files:

- `docs/refactoring/dependency-baseline.md`
- `docs/refactoring/dependency-baseline-output.txt`

Steps:

1. Run `./scripts/analyze-project-deps.ps1`.
2. Save raw output to `dependency-baseline-output.txt`.
3. Paste summarized results into `dependency-baseline.md`.
4. Classify advisory violations.

Acceptance criteria:

- Baseline file contains real output, not TODO placeholders.
- Known legacy exceptions are documented.
- No code behavior changes.

### PR 0.2: Add optional baseline test note

Files:

- `docs/refactoring/test-baseline.md` or existing baseline doc.

Steps:

1. Run focused tests.
2. Record pass/fail and known flakes.
3. Do not fix unrelated flaky tests in this PR unless trivial.

Acceptance criteria:

- Refactoring can refer to a known test baseline.

## 5. Phase 1: Preview First Cut

### Goal

Make `preview_builder.go` less platform-hardcoded without changing API behavior.

Current first step already started:

- `shouldBuildPreviewPlatform(...)`
- `isSelectedPreviewPlatform(...)`
- helper tests

### PR 1.1: Extract per-platform preview builder helpers

Files:

- `internal/listingkit/preview_builder.go`
- New optional files:
  - `internal/listingkit/preview_platform_amazon.go`
  - `internal/listingkit/preview_platform_shein.go`
  - `internal/listingkit/preview_platform_temu.go`
  - `internal/listingkit/preview_platform_walmart.go`

Steps:

1. Extract the Amazon branch into `buildAmazonPreviewSection(...)`.
2. Extract the SHEIN branch into `buildSheinPreviewSection(...)`.
3. Extract the TEMU branch into `buildTemuPreviewSection(...)`.
4. Extract the Walmart branch into `buildWalmartPreviewSection(...)`.
5. Keep the same error behavior for unavailable selected platforms.

Acceptance criteria:

- `buildListingKitPreview(...)` becomes shorter.
- No exported API changes.
- Existing preview tests pass.
- New helpers stay package-private.

Suggested tests:

```powershell
go test ./internal/listingkit/... -run Preview -count=1
```

### PR 1.2: Add preview platform section tests

Files:

- `internal/listingkit/preview_platform_selection_test.go`
- Existing preview tests as needed.

Steps:

1. Add targeted tests for missing selected platform payload.
2. Add targeted tests for empty selected platform including multiple payloads where fixtures are cheap.
3. Avoid large fixture rewrites.

Acceptance criteria:

- Selected-platform semantics are locked before adapter extraction.

## 6. Phase 2: Preview Package Extraction

### Goal

Move preview aggregation toward a bounded module while avoiding import cycles.

### PR 2.1: Create internal preview file group without changing package

Preferred first step:

```text
internal/listingkit/preview_base.go
internal/listingkit/preview_platform.go
internal/listingkit/preview_platform_amazon.go
internal/listingkit/preview_platform_shein.go
internal/listingkit/preview_platform_temu.go
internal/listingkit/preview_platform_walmart.go
```

Do not immediately change `package listingkit` to `package preview` if doing so causes dependency churn.

Acceptance criteria:

- Logical grouping improves.
- No import cycles introduced.
- Public behavior unchanged.

### PR 2.2: Introduce adapter-like internal interface

Add an internal interface such as:

```go
type previewPlatformBuilder interface {
    platform() string
    build(task *Task, preview *ListingKitPreview) error
}
```

Keep it package-private at first.

Acceptance criteria:

- `buildListingKitPreview(...)` iterates builders or delegates consistently.
- Platform-specific code is no longer in the central function.
- Existing tests still pass.

### PR 2.3: Evaluate real subpackage extraction

Only after PR 2.1 and PR 2.2:

1. Check import pressure.
2. Determine whether `internal/listingkit/preview` can exist without importing root `listingkit` in a cycle.
3. If not, postpone real package extraction and keep file-group modularization.

Acceptance criteria:

- Decision documented in a short note or PR description.

## 7. Phase 3: Submission Consolidation

### Goal

Collect submit, retry, recovery, execution, state, direct submit, locks, and Temporal adapter concepts.

### PR 3.1: Inventory submission files

Files:

- `docs/refactoring/submission-inventory.md`

Steps:

1. List all `submission`, `submit`, `retry`, `recovery`, `temporal adapter`, and lock-related files.
2. Group by concept.
3. Identify files that are pure facade versus platform-specific rules.

Acceptance criteria:

- Clear migration map exists before moving files.

### PR 3.2: Extract submission facade struct

Inside current package first:

```go
type submissionFacade struct {
    submitter       TaskSubmitter
    recovery        *taskSubmissionRecoveryService
    execution       *taskSubmissionExecutionService
    state           *taskSubmissionStateService
    direct          *taskDirectSubmissionService
    temporalAdapter *taskTemporalSubmissionAdapter
    locks           *submission.SubmitLockManager
}
```

Acceptance criteria:

- Root `service` has fewer direct submission fields.
- Constructor remains behavior-compatible.
- Focused submission tests pass.

### PR 3.3: Move cohesive submission files

Only after facade exists, move files into a subdirectory or file group.

Acceptance criteria:

- No import cycles.
- Root `service` delegates to submission facade/service.

## 8. Phase 4: Service Object Slimming

### Goal

Reduce the root ListingKit `service` from a dependency sink into grouped facades.

### PR 4.1: Introduce grouped facade fields

Target shape:

```go
type service struct {
    task       *taskFacade
    workflow   *workflowFacade
    preview    *previewFacade
    revision   *revisionFacade
    studio     *studioFacade
    submission *submissionFacade
    settings   *settingsFacade
    shein      *sheinFacade
}
```

Do this incrementally. Do not migrate all fields in one PR.

Acceptance criteria:

- One responsibility group moved per PR.
- Existing `Service` interface remains stable.

### PR 4.2+: Move remaining groups one by one

Recommended order:

1. Submission.
2. Preview/export.
3. Revision/history.
4. Studio.
5. Workflow.
6. Settings/admin.
7. SHEIN bridge dependencies.

Acceptance criteria:

- Each PR reduces direct fields on root `service`.
- Constructor grouping becomes clearer.

## 9. Phase 5: Runtime Assembly Cleanup

### Goal

Keep `internal/app/httpapi` as assembly and wiring only.

### PR 5.1: Inventory runtime support files

Files:

- `docs/refactoring/httpapi-runtime-inventory.md`

Steps:

1. List bootstrap and runtime support files.
2. Mark each as assembly-only, infrastructure adapter construction, or suspicious business logic.
3. Identify files that should move or delegate.

Acceptance criteria:

- Clear inventory exists.

### PR 5.2: Move suspicious business logic out of app/httpapi

Acceptance criteria:

- `app/httpapi` continues to build dependencies.
- Business rules move to listing/product/marketplace packages.

## 10. Phase 6: Marketplace Boundary Normalization

### Goal

Stop marketplace rules from drifting into ListingKit.

### PR 6.1: SHEIN rule freeze checklist

Files:

- `docs/refactoring/shein-boundary-checklist.md`

Steps:

1. List root `internal/listingkit/shein_*` files.
2. Classify each as facade bridge, workspace rule, publishing rule, or unclear.
3. Mark files that must stay thin.

Acceptance criteria:

- New SHEIN rules have clear placement guidance.

### PR 6.2: TEMU boundary plan

Files:

- `docs/refactoring/temu-boundary-plan.md`

Steps:

1. Inventory TEMU-specific code.
2. Align to the SHEIN publishing/workspace pattern.
3. Pick one low-risk extraction.

Acceptance criteria:

- TEMU work has a bounded first migration.

## 11. Phase 7: Infrastructure Interface Cleanup

### Goal

Reduce direct dependencies on concrete external clients in business code.

### PR 7.1: External client import inventory

Files:

- `docs/refactoring/infrastructure-import-inventory.md`

Look for direct imports of:

- OpenAI clients.
- S3 clients.
- Redis clients.
- RabbitMQ clients.
- Temporal worker bootstrap.
- Playwright.
- Gin in non-HTTP packages.
- GORM in domain packages.

Acceptance criteria:

- Inventory identifies highest-value interface extractions.

### PR 7.2+: Extract one interface at a time

Acceptance criteria:

- Business package depends on a small local interface.
- Concrete implementation stays in infra/platform/integration/app wiring.

## 12. Stop Conditions

Pause and document before continuing if:

- A move creates an import cycle.
- A test fails in a way unrelated to the moved code.
- A package needs to import root `listingkit` from a lower-level module.
- A refactor requires changing API DTOs.
- A PR grows beyond one bounded responsibility.

## 13. Done Definition for the First Execution Wave

The first execution wave is complete when:

- Dependency baseline is filled with real output.
- Preview platform logic is extracted into helpers or file groups.
- Preview selected-platform behavior is covered by tests.
- At least one root ListingKit complexity hotspot is reduced without behavior changes.
- `docs/refactoring/README.md` points to the active plan, baseline, and guardrails.

## 14. Recommended Immediate Next PR

Next PR after this document:

**PR title:** `refactor: extract preview platform section builders`

Scope:

- Extract Amazon/SHEIN/TEMU/Walmart preview branch bodies into package-private helper functions.
- Keep `buildListingKitPreview(...)` as the orchestration function.
- Do not move packages yet.
- Run focused preview tests.

Suggested command:

```powershell
go test ./internal/listingkit/... -run Preview -count=1
```
