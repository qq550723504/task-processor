# Architecture Review Checklist

## Goal

Use this checklist when reviewing framework, refactoring, HTTP API assembly,
Temporal, or platform-boundary changes. It turns the current structure rules
into repeatable review questions so boundaries do not depend on memory.

## Required Checks

Before merging a structural or feature PR, verify:

1. No new reverse dependency points from domain, product, or marketplace code
   back into `internal/app/httpapi`.
2. No new direct imports of retired compatibility paths such as
   `internal/app/processor` or `internal/app/state`.
3. New route registration lives in the owning module `internal/*/httpapi`
   package, with `internal/app/httpapi` limited to shared runtime aggregation.
4. Business helpers are not added to app-layer assembly packages.
5. New Temporal usage follows `docs/architecture/temporal-boundaries.md` and
   keeps SDK types out of domain-facing contracts.
6. New platform-specific rules do not grow the root `internal/listingkit`
   facade when a marketplace or product module owns the behavior.
7. Any boundary exception is documented with a narrow scope and a follow-up
   cleanup path.
8. Relevant import-boundary and architecture tests were run.

## Review References

Use these documents as the stable source of truth:

- `docs/architecture/project-boundaries.md`
- `docs/architecture/httpapi-assembly-boundaries.md`
- `docs/architecture/app-assembly-boundaries.md`
- `docs/architecture/temporal-boundaries.md`
- `docs/development/repository-structure.md`

## Working Rule

If a change makes the dependency direction less obvious, require either a
smaller adapter, a local interface, or an explicit documented exception before
merging.
