# Refactoring Documentation

This directory contains architecture and refactoring plans for the Task Processor / ListingKit codebase.

## Source of truth

Project-wide restructuring should follow:

- [project-wide-refactoring-plan.md](./project-wide-refactoring-plan.md)

Use this document as the default refactoring authority when making broad package, boundary, runtime assembly, marketplace, infrastructure, or ListingKit modularization decisions.

Older local plans are still useful for historical context and detailed task breakdowns, but they should not override the project-wide plan unless a newer ADR or refactoring document explicitly says so.

## Boundary rules and enforcement

Use these files before starting broad package moves:

- [../architecture/project-boundaries.md](../architecture/project-boundaries.md) - allowed ownership, forbidden import directions, placement rules, and review checklist.
- [dependency-baseline.md](./dependency-baseline.md) - worksheet for capturing the current dependency and package-shape baseline before broad moves.
- [../../scripts/analyze-project-deps.ps1](../../scripts/analyze-project-deps.ps1) - advisory dependency analysis script for package file counts, largest files, ListingKit import pressure, and likely boundary violations.

Recommended first command from the repository root:

```powershell
./scripts/analyze-project-deps.ps1 | Tee-Object -FilePath docs/refactoring/dependency-baseline-output.txt
```

To make the script fail when it detects advisory boundary violations:

```powershell
./scripts/analyze-project-deps.ps1 -FailOnViolation
```

The script is advisory at first. Known legacy exceptions should be documented in `dependency-baseline.md` before promoting it to CI enforcement.

## Current active direction

The current active direction is:

1. Keep the project as a modular monolith first; do not split microservices before package boundaries are stable.
2. Keep `app` packages focused on runtime assembly and route / worker registration.
3. Reduce `internal/listingkit` into orchestration, compatibility facade, task flow, preview/export aggregation, revision/history, and API shell responsibilities.
4. Keep marketplace-specific rules in marketplace-specific packages.
5. Keep product facts and reusable visual assets outside ListingKit.
6. Hide infrastructure and external clients behind small interfaces.
7. Prefer small, testable migrations over broad rewrites.

## Current refactoring documents

- [project-wide-refactoring-plan.md](./project-wide-refactoring-plan.md) - project-level target architecture, boundaries, phases, and success metrics.
- [dependency-baseline.md](./dependency-baseline.md) - baseline worksheet for dependency scans and legacy exception tracking.
- [listingkit-refactoring-plan.md](./listingkit-refactoring-plan.md) - detailed ListingKit root-directory reduction plan.
- [studio-migration-plan.md](./studio-migration-plan.md) - Studio migration planning.
- [architecture-improvement-plan.md](./architecture-improvement-plan.md) - earlier architecture quality improvement notes.

## Refactoring principles

1. **Backward compatibility**: preserve API and behavior compatibility unless a migration explicitly changes them.
2. **Small steps**: each refactoring task should be independently reviewable and testable.
3. **Test first**: capture or run relevant tests before and after each migration.
4. **Incremental migration**: avoid one-shot rewrites and package renames.
5. **Boundary ownership**: new business rules should be placed in the package that owns the business concept.
6. **No silent authority drift**: if the project-wide plan is superseded, add a new ADR or refactoring document and link it here.

## Risk control

- Establish a test and dependency baseline before large moves.
- Keep behavior-preserving moves separate from feature changes.
- Record known import-boundary exceptions instead of normalizing accidental dependencies.
- Keep rollback simple by limiting each PR to one bounded capability.
- Require code review for boundary changes, package moves, or new cross-module dependencies.
