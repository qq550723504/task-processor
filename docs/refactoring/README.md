# Refactoring Documentation

This directory contains architecture and refactoring plans for the Task Processor / ListingKit codebase.

## Document authority

Use the documents in this order when they disagree.

### Current execution authority

- [current-refactoring-status.md](./current-refactoring-status.md) - current product maturity, Now / Next / Later status, active focus, deferred work, and validation gates.
- [next-phase-plan.md](./next-phase-plan.md) - active execution plan for current-baseline validation, SHEIN stabilization, Product Sourcing MVP closeout, HTTPAPI runtime closure, and boundary guard stabilization.
- [listingkit-boundary-checkpoint.md](./listingkit-boundary-checkpoint.md) - current approved ListingKit stop-lines, remaining root ownership, and submission / marketplace boundary checkpoints.
- [../product/product-sourcing-mvp-plan.md](../product/product-sourcing-mvp-plan.md) - Product Sourcing implementation and closeout plan.
- [../product/product-sourcing-handoff.md](../product/product-sourcing-handoff.md) - Product Sourcing ownership boundaries and handoff expectations.

### Long-term architecture authority

- [project-wide-refactoring-plan.md](./project-wide-refactoring-plan.md) - project-level target architecture, boundaries, phases, and success metrics.
- [../architecture/project-target-architecture.md](../architecture/project-target-architecture.md) - approved target architecture shape.
- [project-migration-roadmap.md](./project-migration-roadmap.md) - staged migration sequence from the current layout to the approved target architecture.
- [module-target-mapping.md](./module-target-mapping.md) - working map from current package areas to target domains.

### Strategic decision records

- [decisions/2026-06-26-next-growth-sequence.md](./decisions/2026-06-26-next-growth-sequence.md) - decision to prioritize refactoring closeout, then product-source expansion, then full sales-platform expansion.

### Historical execution references

- [project-wide-execution-plan.md](./project-wide-execution-plan.md) - earlier broad execution plan. Keep for detailed historical PR slices, but do not treat it as the active sprint queue when it conflicts with `current-refactoring-status.md`, `next-phase-plan.md`, or `listingkit-boundary-checkpoint.md`.
- [listingkit-refactoring-progress-2026-06-24.md](./listingkit-refactoring-progress-2026-06-24.md) - dated progress snapshot and backend Control Plane closeout evidence. Treat it as evidence, not the current execution queue.
- Older local plans remain useful for background and task breakdowns, but should not override the current execution authority unless a newer ADR or refactoring document explicitly says so.

For submission and Temporal-related ListingKit work, treat `listingkit-boundary-checkpoint.md` as the checkpoint authority when older local inventories or phase notes disagree with newer package direction.

## Status vocabulary

Use these terms consistently across refactoring and product documents:

- **Implemented**: the code path exists on the referenced baseline.
- **Repository-validated**: the exact baseline has recorded automated test/build results.
- **Production-validated**: a real environment or real API run is recorded in a dated validation note.
- **Deferred**: code or runtime assets may exist, but the capability is not an active product-expansion commitment.

Do not use official command existence, retained historical code, or an implemented adapter as proof of full product maturity.

## Boundary rules and enforcement

Use these files before starting broad package moves:

- [../architecture/project-boundaries.md](../architecture/project-boundaries.md) - allowed ownership, forbidden import directions, placement rules, and review checklist.
- [dependency-baseline.md](./dependency-baseline.md) - worksheet for capturing the current dependency and package-shape baseline before broad moves.
- [../../scripts/analyze-project-deps.ps1](../../scripts/analyze-project-deps.ps1) - advisory dependency analysis script for package file counts, largest files, ListingKit import pressure, and likely boundary violations.

Recommended first command from the repository root when fresh dependency evidence is needed:

```powershell
New-Item -ItemType Directory -Force .local/refactoring | Out-Null
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath .local/refactoring/dependency-baseline-output.txt
```

To make the script fail when it detects advisory boundary violations:

```powershell
./scripts/analyze-project-deps.ps1 -FailOnViolation
```

The script is advisory unless a current PR explicitly promotes a specific check to CI. Generated dependency/package outputs should remain under `.local/` unless a dated validation note summarizes the relevant result.

## Current active direction

The current active direction is:

1. Keep the project as a modular monolith first; do not split microservices before package boundaries are stable.
2. Stabilize the SHEIN production path and keep exact validation evidence visible before release decisions.
3. Close the implemented Product Sourcing MVP with focused tests and one controlled 1688 source-to-task path before starting another source.
4. Select exactly one next product source after the current loop is closed; 大建云仓 / warehouse catalog remains a candidate, not an active multi-source expansion.
5. Reduce root `internal/listingkit` into orchestration, compatibility facade, task flow, preview/export aggregation, revision/history, persistence ordering, and API shell responsibilities.
6. Keep marketplace-specific rules in marketplace-specific packages.
7. Keep product facts, product-source identity, source normalization, and reusable visual assets outside root ListingKit.
8. Hide infrastructure and external clients behind small interfaces.
9. Prefer small, testable migrations over broad rewrites.
10. For submission refactoring, prefer `internal/listing/submission` for generic mechanics, `internal/marketplace/*/publishing` or approved `internal/publishing/*` seams for marketplace rules, and keep `internal/listingkit` as a shrinking orchestration and compatibility surface.
11. Defer full TEMU / Amazon / Walmart workbench expansion until the SHEIN template, source loop, CI/race/build gates, and runtime smoke tests are stable.

## Current refactoring documents

- [current-refactoring-status.md](./current-refactoring-status.md) - active product maturity, Now / Next / Later status, and strategic focus.
- [next-phase-plan.md](./next-phase-plan.md) - active next-phase goals: baseline validation, SHEIN stabilization, Product Sourcing MVP closeout, HTTPAPI runtime closure, and boundary guard stabilization.
- [listingkit-boundary-checkpoint.md](./listingkit-boundary-checkpoint.md) - current ListingKit boundary stop-lines and approved seam directions.
- [project-wide-refactoring-plan.md](./project-wide-refactoring-plan.md) - project-level target architecture, boundaries, phases, and success metrics.
- [project-wide-execution-plan.md](./project-wide-execution-plan.md) - earlier broad execution plan; historical/reference when it conflicts with current checkpoint docs.
- [project-migration-roadmap.md](./project-migration-roadmap.md) - staged migration sequence from the current layout to the approved target architecture.
- [module-target-mapping.md](./module-target-mapping.md) - working map from current package areas to target domains.
- [product-sourcing-inventory.md](./product-sourcing-inventory.md) - Amazon / 1688 source-flow inventory and first migration slices for `product/sourcing` plus crawler adapters.
- [listing-preview-migration-map.md](./listing-preview-migration-map.md) - first detailed migration inventory for moving preview ownership out of legacy ListingKit.
- [dependency-baseline.md](./dependency-baseline.md) - baseline worksheet for dependency scans and legacy exception tracking.
- [preview-subpackage-feasibility.md](./preview-subpackage-feasibility.md) - current decision note explaining why preview stays inside `package listingkit` before a real subpackage extraction.
- [submission-inventory.md](./submission-inventory.md) - historical submission consolidation inventory; useful for file grouping history, but current submission target direction is governed by the boundary checkpoint plus current status docs.
- [service-slimming-checkpoint.md](./service-slimming-checkpoint.md) - Phase 4 first checkpoint for root service object file-group slimming.
- [checkpoint-validation-2026-06-17.md](./checkpoint-validation-2026-06-17.md) - focused next-phase validation run covering ListingKit, HTTPAPI, boundary tests, dependency scan, and blocker classification.
- [listingkit-refactoring-plan.md](./listingkit-refactoring-plan.md) - detailed ListingKit root-directory reduction plan.
- [studio-migration-plan.md](./studio-migration-plan.md) - Studio migration planning.
- [architecture-improvement-plan.md](./architecture-improvement-plan.md) - earlier architecture quality improvement notes.

## Refactoring principles

1. **Backward compatibility**: preserve API and behavior compatibility unless a migration explicitly changes them.
2. **Small steps**: each refactoring task should be independently reviewable and testable.
3. **Test first**: capture or run relevant tests before and after each migration.
4. **Incremental migration**: avoid one-shot rewrites and package renames.
5. **Boundary ownership**: new business rules should be placed in the package that owns the business concept.
6. **No silent authority drift**: if the active execution direction changes, update `current-refactoring-status.md` and add or update a decision record.
7. **Evidence discipline**: do not claim a baseline is green unless the exact workflow, command output, or validation note is visible.

## Risk control

- Establish a test and dependency baseline before large moves.
- Keep behavior-preserving moves separate from feature changes.
- Record known import-boundary exceptions instead of normalizing accidental dependencies.
- Keep rollback simple by limiting each PR to one bounded capability.
- Require code review for boundary changes, package moves, production-sensitive SHEIN behavior, pricing logic, submission behavior, or new cross-module dependencies.
- Do not continue helper shaving, file splitting, or package renaming unless it reduces real ownership or improves dependency direction.
- Do not use Product Sourcing implementation progress as justification to start several new source integrations at once.
