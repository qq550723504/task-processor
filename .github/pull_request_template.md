# Summary

- What changed?
- Why is this needed?

# Task and Acceptance

Follow [Issue-driven development V1](https://github.com/qq550723504/task-processor/blob/main/docs/engineering/issue-driven-development.md) and [AGENTS.md](https://github.com/qq550723504/task-processor/blob/main/AGENTS.md). Keep the complete current delivery summary in this PR; update Issue progress only at milestones.

- Execution Issue: Refs #<number>
- Parent Issue / slice (if applicable):
- Scope owner / implementing session / independent Reviewer:
- Acceptance covered by this PR; remaining slices or dependencies:

| Issue acceptance criterion | Evidence / result | Verified SHA | Remaining limitation or blocker |
| --- | --- | --- | --- |
| Replace with each applicable criterion | Test / CI / review link, or NOT_RUN with reason | | |

Partial delivery uses Refs / Relates to and must not auto-close the parent Issue. Close an execution Issue only after all its acceptance is met and closure is authorized. Keep in-scope findings and CI fixes in this PR.

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
- [ ] `IMPLEMENTATION_TEST` is resolved through implementation and validation without reopening design; unmet current-slice Must requirements still block merge.
- [ ] `BACKLOG`, `ACCEPTED_RISK`, and `NOT_APPLICABLE` do not block implementation within approved boundaries; accepted-risk / non-applicable classifications cite the approved basis.
- [ ] Findings identify the affected Must and blocking level; two normal architecture review rounds do not waive later real Must defects.
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

# Final HEAD and Review Evidence

Maintain rolling evidence here, not in repository commits. Historical evidence is not approval of the current HEAD.

- Final HEAD (full SHA):
- Applicable CI runs / tested head or merge SHA / results:
- Independent Reviewer / reviewed SHA / review link / findings and disposition:
- NOT_RUN / limitations (separate local tests, fixtures, cross-platform compilation and real-environment acceptance):
- Remaining blockers: missing condition / owner / unblock condition / level (start, slice, merge, rollout), or none:

- [ ] Independent review examined the full slice diff and applicable call paths, including sibling success/failure paths and platform branches; resolved threads, green CI or a completed review alone are not full acceptance.
- [ ] No duplicate review was triggered for an already-running review of the same SHA; final reviewed SHA is recorded and old Security Review is not presented as current approval.

# Merge and Production Authority

Default authority covers scoped implementation, isolated tests, commit, push and PR maintenance only. Labels, priority and ordinary comments do not grant merge, deployment, real-platform mutation, business data / profile / accounting operations or protection-rule changes.

- Merge approval record (PR number + final HEAD + action + designated executor), or NOT_AUTHORIZED:
- Production wiring / deployment / real-environment action authorization (separate references), or NOT_AUTHORIZED:
- Main/dependency changes checked and applicable combination validation before merge:
- After authorized merge only: actual merge commit / main CI evidence (not the old baseline result):

- [ ] Required repository documentation is delivered before requesting final approval.
- [ ] The designated executor must match the expected HEAD and obey branch protection; changed HEAD requires renewed verification and corresponding authorization. No implied conditional or administrator merge permission.
- [ ] Merge, production wiring and actual environment operations have separate authority; merged does not mean production accepted, and “do not retain data” does not authorize immediate deletion.

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
