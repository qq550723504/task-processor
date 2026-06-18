# Architecture Documentation

## Goal

This index separates stable architecture rules from plans, runbooks, and
historical evaluations. Use the stable documents for code review and new
implementation decisions. Use plans and runbooks for context, not as newer
boundary rules unless they explicitly supersede a stable document.

## Stable Boundary Documents

Use these as the main source of truth for structural work:

- `project-boundaries.md`
  - default package ownership, dependency direction, forbidden imports, and
    placement rules for new code
- `httpapi-assembly-boundaries.md`
  - HTTP API ownership, route/module builder boundaries, and app/httpapi limits
- `app-assembly-boundaries.md`
  - app-layer build/register/start/coordinate vocabulary and package roles
- `temporal-boundaries.md`
  - Temporal versus RabbitMQ responsibilities and workflow/runtime boundaries
- `platform-boundary-strategy.md`
  - historical platform, publishing, ListingKit, and platform registration
    convergence roles
- `historical-platform-migration-inventory.md`
  - migration-cost tiers and next-slice candidates for historical platform
    packages
- `external-client-boundary-inventory.md`
  - direct external client adapter hotspots, local-interface rules, and
    next-slice candidates
- `compatibility-retirement.md`
  - retired compatibility paths, replacement owners, and guard tests
- `listing-preview-boundaries.md`
  - platform-neutral preview ownership, ListingKit facade limits, and guard
    tests for preview extraction
- `architecture-review-checklist.md`
  - repeatable PR review checklist for boundary-sensitive changes

## Development Boundary Documents

These documents live outside `docs/architecture`, but still define long-lived
structure rules that should be reviewed with architecture changes:

- `docs/development/repository-structure.md`
  - top-level directory ownership, local artifact placement, and repository
    layout guard tests

## Current Guard Baseline

Use `docs/architecture/next-steps.md` and its `Current guard coverage` section
as the current guard coverage baseline for active import-boundary tests. Formal
review actions should still start from
`docs/architecture/architecture-review-checklist.md`. This baseline tracks what
reviewers must keep visible while the stable boundary documents remain the
source of truth for long-lived rules.

## Supporting Context

These documents are useful background, but should not override stable boundary
documents unless they say so explicitly:

- `project-target-architecture.md`
  - target architecture context; use stable boundary documents for current
    review policy
- `task-status-lifecycle.md`
  - status lifecycle context; use stable boundary documents for package
    ownership and dependency rules
- `temu-architecture-patterns.md`
  - TEMU architecture pattern context; use stable boundary documents for
    cross-platform dependency rules
- `temu-pipeline-stages.md`
  - TEMU pipeline stage context; use stable boundary documents for runtime and
    assembly boundaries
- `listingkit-refactor-status.md`
  - ListingKit refactor status context; use stable boundary documents for
    long-lived ListingKit boundaries
- `amazon-crawler-runtime-flow.md`
  - Amazon crawler runtime flow context; use stable boundary documents for
    review policy

## Plans, runbooks, and evaluations

Documents with names such as `*-plan.md`, `*-runbook.md`, `*-evaluation.md`,
`*-checklist.md`, `*-status.md`, `*-playbook.md`, `*-validation.md`,
`*-split.md`, or `*-management.md` are normally time-bounded. They may explain
why a decision was made, but stable boundary rules should be copied or linked
into one of the stable documents above before being treated as review policy.

## Working Rule

When a structural question comes up, start with this index. If two documents
appear to disagree, prefer the stable boundary document and update the older
plan or status note with a link instead of creating a third interpretation.
