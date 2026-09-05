# Summary

- What changed?
- Why is this needed?

# Product Boundary

Fill this section for architecture-sensitive, security-sensitive, or product-contract changes. For routine bounded changes, write `N/A` where appropriate instead of inventing policy.

## Product Decisions

- Must:
- Should:
- Out of Scope:
- Accepted Risks:

## Threat Model

- Risks this change must protect against:
- Risks explicitly not being solved in this phase:

## Review State

- Architecture review round: `0 / 1 / 2 / frozen`
- Implementation status: `DESIGNING / IMPLEMENTATION_READY / IMPLEMENTING`

- [ ] Reviewer findings were classified before changing architecture.
- [ ] `BLOCKER` findings identify the concrete blocker category from `AGENTS.md`.
- [ ] `IMPLEMENTATION_TEST`, `BACKLOG`, `ACCEPTED_RISK`, and `NOT_APPLICABLE` findings do not block implementation.
- [ ] If the design is `IMPLEMENTATION_READY`, non-blocking findings did not create another versioned design document.

# Legacy Hard-Cut

Fill this section whenever the PR touches an area recorded in `docs/refactoring/legacy-register.md`, root `internal/listingkit`, `internal/compatibility/*`, `internal/tenantbridge`, or an already-retired package/abstraction. Otherwise write `N/A`.

```text
Legacy decision: EXTRACT | RETIRE
Reusable behavior:
Current owner:
Cutover/deletion condition:
```

- [ ] This PR does not add a third “compatibility” decision.
- [ ] New code does not add a consumer of `internal/compatibility/*` or `internal/tenantbridge`.
- [ ] Reusable behavior is extracted to the current owner instead of wrapped through the legacy owner.
- [ ] No new legacy fallback, permanent dual-read, dual-write, bidirectional sync, or second business fact/state owner was introduced.
- [ ] Old tests were preserved only when they still assert a current business/security/authorization/idempotency/platform/deterministic behavior.

# Validation

- [ ] `go test ...`
- [ ] Other validation steps documented below when needed

Validation notes:

- 

# Architecture Checklist

Use this checklist for architecture-sensitive changes, especially changes touching `internal/app`, root `internal/listingkit`, marketplace/listing/product boundaries, Agent/Tool runtime, legacy drain paths, or runtime assembly.

- [ ] The change keeps assembly logic and domain logic separate.
- [ ] `internal/app/*` remains composition / assembly focused and does not absorb new business rules.
- [ ] No new code imports retired paths or recreates retired package roots.
- [ ] No domain `httpapi` package imports `task-processor/internal/app/httpapi`.
- [ ] No new centralized `build*Module` wiring was added where an existing composition contract should be reused.
- [ ] Product facts remain owned by current `internal/product/*` boundaries.
- [ ] Marketplace-specific rules land in the marketplace owner rather than root ListingKit.
- [ ] Internal Task/Workflow remains execution infrastructure; BusinessTask/product UI semantics are not implemented by renaming legacy Task.
- [ ] Agent/Tool code does not directly import GORM repositories, provider SDKs, marketplace clients, retired workflow owners, or legacy compatibility services.
- [ ] Deterministic validation remains outside model authority.
- [ ] No second submission state machine, durable retry owner, IAM/RBAC system, Tool Registry, Product/Listing/Asset fact source, or BusinessTask owner was introduced.
- [ ] If package boundaries changed, the corresponding current authority docs/guards were updated.

# Notes For Reviewers

- Risk areas:
- Follow-up work:
- Any intentional boundary exceptions:
- Finding classification notes (`BLOCKER / IMPLEMENTATION_TEST / BACKLOG / ACCEPTED_RISK / NOT_APPLICABLE`):

Relevant docs when needed:

- `AGENTS.md`
- `docs/refactoring/legacy-hard-cut-policy.md`
- `docs/refactoring/legacy-register.md`
- `docs/architecture/project-target-architecture.md`
- `docs/architecture/project-boundaries.md`
- `docs/architecture/architecture-review-checklist.md`
