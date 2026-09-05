# Architecture Documentation

## Goal

This index separates stable architecture rules from plans, runbooks, and
historical evaluations. Use the stable documents for code review and new
implementation decisions. Use plans and runbooks for context, not as newer
boundary rules unless they explicitly supersede a stable document.

## Approved Authorities by Responsibility

- [Final UI / IA](../product/final-ui-ia-authority.md): final navigation, naming
  and BusinessTask projection (#298), not current release capability.
- [Product Domain](../superpowers/specs/2026-09-01-internal-target-architecture-phase3-product-design.md),
  [Identity / Organization](../superpowers/specs/2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md)
  and current Store/Resource/CommerceTool contracts: facts, access and side effects.
  Preserve valid domain rules even when a document also contains older UI or
  implementation observations; explicit supersession controls the affected section.
- [Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md),
  [Legacy Register](../refactoring/legacy-register.md) and
  [Module Mapping](../refactoring/module-target-mapping.md): only EXTRACT / RETIRE;
  existing legacy is not a new dependency permission.
- [Current status](../refactoring/current-refactoring-status.md): baseline-bound
  implementation and evidence. [#137](https://github.com/qq550723504/task-processor/issues/137)
  and execution Issues own scheduling and scope under the
  [dispatch rules](../engineering/issue-driven-development.md).
- [Sourcing guide](../product/product-sourcing-handoff.md) and
  [closeout](../product/product-sourcing-mvp-plan.md) distinguish current legacy
  wiring, prepared imports and target acceptance. [Clean-slate / #307](../product/issue30-clean-slate-cutover.md)
  supersedes historical publication migration, while protecting account/profile,
  IAM, Store and financial/effect evidence; it grants no real deletion authority.
- [next-phase-plan.md](../refactoring/next-phase-plan.md): HISTORICAL implementation
  record, not an execution queue. The [Publication Identity draft](../refactoring/2026-09-05-publication-identity-cutover-issue-draft.md)
  is SUPERSEDED under #307, not a migration prerequisite.

ACTIVE contracts, CURRENT STATE observations and HISTORICAL/SUPERSEDED evidence
serve different purposes. An old Active label or a newer date cannot override
an approved contract. Follow the explicit responsibility/supersession above.

## Recommended Reading Order

After identifying the applicable approved authorities above, review structural rules in this order:

1. Start with `project-boundaries.md` for default package ownership,
   dependency direction, and repository-wide placement rules.
2. Then open the most relevant specialized boundary document such as
   `httpapi-assembly-boundaries.md`, `app-assembly-boundaries.md`, or
   `platform-boundary-strategy.md`.
3. Use `architecture-review-checklist.md` to turn those rules into concrete PR
   review questions.
4. Use `next-steps.md` only as the current guard coverage ledger, not as a
   competing source of architecture policy.

If a specialized document appears broader than `project-boundaries.md`, treat
`project-boundaries.md` as the default review entrypoint and tighten the
specialized note instead of creating a second top-level policy.

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
  - retained ownership/inventory evidence for historical platform packages;
    candidate slices require current mapping and an execution Issue
- `external-client-boundary-inventory.md`
  - local-interface rules and baseline-bound adapter hotspots; suggested
    slices do not override current owners or Issue scheduling
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

Use this development document when the question is mainly about repository
layout, entrypoint placement, or runtime artifact location. If the question is
mainly about package ownership or dependency direction, start from the default
project boundary entrypoint first and only then drop to the development
document.

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
- `auth-and-tenancy.md`
  - verified effective Organization, home identity distinction, route
    authorization and remaining legacy paths; supporting context, not a new IAM
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
- `task-event-v2-migration.md`
  - RabbitMQ complete-task event V2 schema, compatibility window, and removal
    gate; Listing Control ID-only dispatch is out of scope
- `openmeter-shadow-metering-poc-report.md`
  - time-bounded, evidence-backed local PoC decision; it does not authorize a
    production integration, billing, payment, deployment, or data migration
- `pay-041-usage-ledger.md`
  - time-bounded PAY-041 reconciliation evidence and PAY-042 handoff; it does
    not authorize production data repair, entrypoint cutover, or billing
    integration
- `pay-042-listingkit-generation-usage-cutover.md`
  - time-bounded first-slice PAY-042 generation settlement boundary; it does
    not authorize payment-provider changes or enable the rollout flag

## Plans, runbooks, and evaluations

Documents with names such as `*-plan.md`, `*-runbook.md`, `*-evaluation.md`,
`*-checklist.md`, `*-status.md`, `*-playbook.md`, `*-validation.md`,
`*-split.md`, or `*-management.md` are normally time-bounded. Names are discovery hints, not retirement decisions. They may explain
why a decision was made. Still-approved domain contracts remain effective; link
applicable boundary rules into stable entries for review policy. Classify mixed
sections by explicit supersession and evidence, never mechanically by filename.
Every architecture document must be either indexed above or match a
time-bounded context pattern.

## Working Rule

When a structural question comes up, start with this index. If two documents
appear to disagree, first apply the approved responsibility/supersession above;
within that boundary, prefer the stable boundary document and update the older
contextual note with a link instead of creating a third interpretation.
Every stable or development boundary document must have a document test before
it is treated as a long-lived review entrypoint.
