# Refactoring Documentation

This directory contains architecture and refactoring plans for the Task Processor / ListingKit codebase.

## Document authority

Use the documents in this order when they disagree.

### Current execution authority

- [current-refactoring-status.md](./current-refactoring-status.md) - current Now / Next / Later status, active focus, deferred work, and immediate validation gates.
- [next-phase-plan.md](./next-phase-plan.md) - active next-phase execution plan for checkpoint validation, HTTPAPI runtime closeout, small target-domain seams, and boundary guard stabilization.
- [listingkit-boundary-checkpoint.md](./listingkit-boundary-checkpoint.md) - current approved ListingKit stop-lines, remaining root ownership, and submission / marketplace boundary checkpoints.

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

## Boundary rules and enforcement

Use these files before starting broad package moves:

- [../architecture/project-boundaries.md](../architecture/project-boundaries.md) - allowed ownership, forbidden import directions, placement rules, and review checklist.
- [dependency-baseline.md](./dependency-baseline.md) - worksheet for capturing the current dependency and package-shape baseline before broad moves.
- [../../scripts/analyze-project-deps.ps1](../../scripts/analyze-project-deps.ps1) - advisory dependency analysis script for package file counts, largest files, ListingKit import pressure, and likely boundary violations.

Recommended first command from the repository root:

```powershell
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath docs/refactoring/dependency-baseline-output.txt
```

To make the script fail when it detects advisory boundary violations:

```powershell
./scripts/analyze-project-deps.ps1 -FailOnViolation
```

The script is advisory at first. Known legacy exceptions should be documented in `dependency-baseline.md` before promoting it to CI enforcement.

## Current active direction

The current active direction is:

1. Keep the project as a modular monolith first; do not split microservices before package boundaries are stable.
2. Finish refactoring closeout and runtime boundary stabilization before broad new feature expansion.
3. Reduce `internal/listingkit` into orchestration, compatibility facade, task flow, preview/export aggregation, revision/history, and API shell responsibilities.
4. Keep marketplace-specific rules in marketplace-specific packages.
5. Keep product facts, product-source identity, and reusable visual assets outside ListingKit.
6. Hide infrastructure and external clients behind small interfaces.
7. Prefer small, testable migrations over broad rewrites.
8. For submission refactoring, prefer `internal/listing/submission` for generic mechanics, `internal/marketplace/*/publishing` for marketplace rules, and keep `internal/listingkit` as a shrinking orchestration and compatibility surface.
9. Expand product sources before building another full sales-platform workbench.
10. Defer large TEMU / Amazon / Walmart workbench expansion until the SHEIN template, CI/race/build gates, and runtime smoke tests are stable.

## Current refactoring documents

- [current-refactoring-status.md](./current-refactoring-status.md) - active Now / Next / Later status and strategic focus.
- [next-phase-plan.md](./next-phase-plan.md) - active next-phase goals: checkpoint validation, HTTPAPI runtime closeout, small target-domain seams, and boundary guard stabilization.
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

## Risk control

- Establish a test and dependency baseline before large moves.
- Keep behavior-preserving moves separate from feature changes.
- Record known import-boundary exceptions instead of normalizing accidental dependencies.
- Keep rollback simple by limiting each PR to one bounded capability.
- Require code review for boundary changes, package moves, or new cross-module dependencies.
- Do not continue helper shaving, file splitting, or package renaming unless it reduces real ownership or improves dependency direction.
